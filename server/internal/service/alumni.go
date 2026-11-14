package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/cache"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"go.uber.org/zap"
)

// AlumniFileCleaner 校友删除、文件移除时级联清理对象存储的接口。
type AlumniFileCleaner interface {
	CascadeSoftDelete(ctx context.Context, alumniID uint64) error
	RemoveObjects(ctx context.Context, objectKeys []string)
}

// OperationLogWriter 记录不含敏感原值的业务审计日志。
type OperationLogWriter interface {
	Write(ctx context.Context, log *model.OperationLog) error
}

type ExportResult struct {
	Data        []byte
	ContentType string
	Filename    string
}

type AlumniService struct {
	alumni      repository.AlumniStore
	users       repository.UserStore
	files       AlumniFileCleaner
	dataDomains repository.AccessControlStore
	opLogger    OperationLogWriter
	auditWriter AuditEventWriter
	countCache  *cache.CountCache
	exportCache *cache.ExportCache
}

func NewAlumniService(alumni repository.AlumniStore, files AlumniFileCleaner, dataDomains ...repository.AccessControlStore) *AlumniService {
	service := &AlumniService{alumni: alumni, files: files}
	if len(dataDomains) > 0 {
		service.dataDomains = dataDomains[0]
	}
	return service
}

// WithCountCache 注入主动缓存计数器，用于优化无过滤条件时的 COUNT 查询。
func (s *AlumniService) WithCountCache(c *cache.CountCache) *AlumniService {
	s.countCache = c
	return s
}

// WithExportCache 注入导出结果缓存，避免每次导出都全表扫描。
func (s *AlumniService) WithExportCache(c *cache.ExportCache) *AlumniService {
	s.exportCache = c
	return s
}

// WithOperationLogger 注入导出和导入操作审计记录器。
func (s *AlumniService) WithOperationLogger(logger OperationLogWriter) *AlumniService {
	s.opLogger = logger
	if writer, ok := logger.(AuditEventWriter); ok {
		s.auditWriter = writer
	}
	return s
}

// WithAuditWriter 注入档案新增、修改、删除的审计事件写入器。
func (s *AlumniService) WithAuditWriter(writer AuditEventWriter) *AlumniService {
	s.auditWriter = writer
	return s
}

// WithAuditUserStore 注入操作历史需要读取的操作用户存储。
func (s *AlumniService) WithAuditUserStore(users repository.UserStore) *AlumniService {
	s.users = users
	return s
}

func scopedDataDomainIDs(access common.AccessContext) ([]uint64, bool) {
	if access.Role == common.RoleAdmin {
		return access.DomainIDs, true
	}
	return nil, false
}

func personalContainsSensitiveFields(profile do.AlumniPersonalInput) bool {
	return profile.WorkUnit != nil || profile.Position != nil || profile.MailingAddress != nil ||
		profile.Mobile != nil || profile.Email != nil
}

func validExplicitTrainingLevel(level string) bool {
	switch level {
	case common.TrainingLevelUndergraduate, common.TrainingLevelMaster, common.TrainingLevelDoctor:
		return true
	default:
		return false
	}
}

func validStoredTrainingLevel(level string) bool {
	return validExplicitTrainingLevel(level) || level == common.TrainingLevelUnverified
}

// assignEnrollmentDomain 在新增学籍时确定目标数据域。
// 单域普通管理员自动落到唯一授权域；多域管理员与超级管理员必须显式提交可访问的数据域。
func assignEnrollmentDomain(item *do.AlumniEnrollmentInput, operator common.AccessContext) error {
	if operator.IsSuperAdmin() {
		if item.DataDomainID == 0 {
			return common.ErrInvalidDataDomain
		}
		return nil
	}
	if operator.Role != common.RoleAdmin || len(operator.DomainIDs) == 0 {
		return common.ErrPermissionDenied
	}
	if len(operator.DomainIDs) == 1 {
		// 单域管理员的表单只有一个可写数据域，提交值一律归一到唯一授权域。
		item.DataDomainID = operator.DomainIDs[0]
		return nil
	}
	if item.DataDomainID == 0 || !operator.CanAccessDomain(item.DataDomainID) {
		return common.ErrPermissionDenied
	}
	return nil
}

// scopeListQuery 把调用者可访问的数据域写入查询条件。
func (s *AlumniService) scopeListQuery(query do.AlumniListQuery, viewer common.AccessContext) (do.AlumniListQuery, error) {
	if query.DataDomainID != nil {
		if !viewer.IsSuperAdmin() && !viewer.CanAccessDomain(*query.DataDomainID) {
			return query, common.ErrPermissionDenied
		}
		query.DataDomainIDs = []uint64{*query.DataDomainID}
	} else if domainIDs, restricted := scopedDataDomainIDs(viewer); restricted {
		if len(domainIDs) == 0 {
			return query, common.ErrPermissionDenied
		}
		query.DataDomainIDs = append([]uint64(nil), domainIDs...)
	}
	return query.Normalize(), nil
}

// List 根据查询条件分页获取校友列表，一人一行。
func (s *AlumniService) List(ctx context.Context, req dto.AlumniListRequest, viewer common.AccessContext) (common.Pager[dto.AlumniListItem], error) {
	query := req.ToQuery().Normalize()
	empty := common.NewPager[dto.AlumniListItem](nil, query.Page, 0)
	if s.alumni == nil {
		return empty, common.ErrDatabaseUnavailable
	}
	if !viewer.HasPermission(common.PermissionAlumniSensitiveRead) &&
		(query.WorkUnit != "" || query.Position != "" || query.Mobile != "") {
		return empty, common.ErrPermissionDenied
	}
	query, err := s.scopeListQuery(query, viewer)
	if err != nil {
		return empty, err
	}
	query.CanReadSensitive = viewer.HasPermission(common.PermissionAlumniSensitiveRead)

	domainNames, err := s.domainNameMap(ctx)
	if err != nil {
		return empty, err
	}

	// 无过滤条件且无数据域范围时优先用缓存计数，跳过 DB COUNT(*)
	if query.IsUnfiltered() && len(query.DataDomainIDs) == 0 && s.countCache != nil {
		total, hit, _ := s.countCache.Get(ctx)
		if !hit {
			if n, err := s.alumni.CountActive(ctx); err == nil {
				total = n
				_ = s.countCache.Set(ctx, n)
			}
		}
		records, err := s.alumni.FindOnly(ctx, query)
		if err != nil {
			return empty, err
		}
		mapped := mapAlumniListItems(records, domainNames)
		s.maskListItems(mapped, viewer)
		return common.NewPager(mapped, query.Page, total), nil
	}

	records, total, err := s.alumni.List(ctx, query)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Error(err))
		return empty, common.ErrDatabaseUnavailable
	}
	if err != nil {
		logger.Error("failed to list alumni", zap.Error(err))
		return empty, err
	}

	mapped := mapAlumniListItems(records, domainNames)
	s.maskListItems(mapped, viewer)
	return common.NewPager(mapped, query.Page, total), nil
}

// maskListItems 默认屏蔽列表中的敏感字段，仅当授权上下文包含敏感字段查看权限时才放行。
func (s *AlumniService) maskListItems(items []dto.AlumniListItem, viewer common.AccessContext) {
	if viewer.HasPermission(common.PermissionAlumniSensitiveRead) {
		return
	}
	for i := range items {
		items[i].Mobile = nil
		items[i].Email = nil
		items[i].WorkUnit = nil
		items[i].Position = nil
	}
}

// GetByID 根据 ID 获取校友详情，并根据请求授权上下文裁剪字段和学籍。
func (s *AlumniService) GetByID(ctx context.Context, id uint64, viewer common.AccessContext) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	domainIDs, restricted := scopedDataDomainIDs(viewer)
	if restricted && len(domainIDs) == 0 {
		return nil, common.ErrAlumniNotFound
	}
	record, err := s.alumni.GetDetail(ctx, id, domainIDs)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("alumni_id", id), zap.Error(err))
		return nil, common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrAlumniNotFound) {
		logger.Warn("alumni not found", zap.Uint64("alumni_id", id))
		return nil, common.ErrAlumniNotFound
	}
	if err != nil {
		logger.Error("failed to get alumni", zap.Uint64("alumni_id", id), zap.Error(err))
		return nil, err
	}

	domainNames, err := s.domainNameMap(ctx)
	if err != nil {
		return nil, err
	}
	detail := mapAlumniDetail(record, domainNames, viewer)
	s.maskSensitiveFields(detail, id, viewer)
	return detail, nil
}

// maskSensitiveFields 默认屏蔽详情中的敏感字段，仅当授权上下文确认查看者为管理员或本人时才放行。
func (s *AlumniService) maskSensitiveFields(detail *dto.AlumniDetail, alumniID uint64, viewer common.AccessContext) {
	if detail == nil {
		return
	}
	if viewer.HasPermission(common.PermissionAlumniSensitiveRead) {
		return
	}
	if viewer.AlumniID != nil && *viewer.AlumniID == alumniID {
		return
	}
	detail.Mobile = nil
	detail.Email = nil
	detail.WorkUnit = nil
	detail.Position = nil
	detail.MailingAddress = nil
}

// Create 由管理员新增校友档案，同时创建个人信息和至少一段初始学籍。
func (s *AlumniService) Create(ctx context.Context, operator common.AccessContext, req dto.AdminAlumniCreateRequest) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}

	input := req.ToInput().Normalize()
	if input.Personal.Name == "" || len(input.Enrollments) == 0 {
		return nil, common.ErrInvalidRequest
	}
	if input.Status != common.AlumniStatusActive {
		return nil, common.ErrInvalidRequest
	}
	if !operator.HasPermission(common.PermissionAlumniSensitiveRead) && personalContainsSensitiveFields(input.Personal) {
		return nil, common.ErrPermissionDenied
	}
	for i := range input.Enrollments {
		item := &input.Enrollments[i]
		if item.ID != 0 || len(item.ActivateFileIDs) > 0 || len(item.RemoveFileIDs) > 0 {
			return nil, common.ErrInvalidRequest
		}
		if item.Grade == "" {
			return nil, common.ErrInvalidRequest
		}
		if !validExplicitTrainingLevel(item.TrainingLevel) {
			return nil, common.ErrInvalidTrainingLevel
		}
		if err := assignEnrollmentDomain(item, operator); err != nil {
			return nil, err
		}
	}
	if err := s.validateEnrollmentCategories(ctx, input.Enrollments); err != nil {
		return nil, err
	}

	record, err := s.alumni.Create(ctx, input, operator.UserID)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("operator_id", operator.UserID), zap.Error(err))
		return nil, common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrInvalidRequest) || errors.Is(err, common.ErrInvalidDataDomain) {
		return nil, err
	}
	if err != nil {
		logger.Error("failed to create alumni", zap.Uint64("operator_id", operator.UserID), zap.Error(err))
		return nil, err
	}

	domainNames, _ := s.domainNameMap(ctx)
	s.writeCreateAudits(ctx, operator, record, domainNames)

	if s.countCache != nil {
		_ = s.countCache.IncrBy(ctx, 1)
	}
	if s.exportCache != nil {
		_ = s.exportCache.Invalidate(ctx)
	}
	detail := mapAlumniDetail(record, domainNames, operator)
	return detail, nil
}

func (s *AlumniService) writeCreateAudits(ctx context.Context, operator common.AccessContext, record *do.AlumniRecord, domainNames map[uint64]string) {
	if record == nil || s.auditWriter == nil || s.users == nil {
		return
	}
	domainIDs := enrollmentDomainIDs(record.Enrollments)
	writeProfileAudit(ctx, s.auditWriter, s.users, operator.UserID, AuditActionCreate, "admin",
		"管理员新增校友档案", defaultAuditScope, domainIDs, nil, record.Profile)
	for _, enrollment := range record.Enrollments {
		writeEnrollmentAudit(ctx, s.auditWriter, s.users, operator.UserID, AuditActionCreate, "admin",
			"管理员新增学籍", domainNames[enrollment.DataDomainID], record.Profile.ID, record.Profile.Name,
			enrollmentLabel(enrollment, domainNames), enrollment.DataDomainID, domainNames, nil, enrollment)
	}
}

// Update 保存个人信息、提交的学籍段和文件变更。任一失败整体回滚。
func (s *AlumniService) Update(ctx context.Context, operator common.AccessContext, id uint64, req dto.AdminAlumniUpdateRequest) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}

	input := req.ToInput().Normalize()
	if input.Personal.Name == "" {
		return nil, common.ErrInvalidRequest
	}
	if !operator.HasPermission(common.PermissionAlumniSensitiveRead) && personalContainsSensitiveFields(input.Personal) {
		return nil, common.ErrPermissionDenied
	}
	for _, enrollment := range input.Enrollments {
		if (len(enrollment.ActivateFileIDs) > 0 || len(enrollment.RemoveFileIDs) > 0) &&
			!operator.HasPermission(common.PermissionAlumniFilesManage) {
			return nil, common.ErrPermissionDenied
		}
	}

	domainIDs, restricted := scopedDataDomainIDs(operator)
	if restricted && len(domainIDs) == 0 {
		return nil, common.ErrAlumniNotFound
	}

	before, err := s.alumni.GetDetail(ctx, id, domainIDs)
	if errors.Is(err, common.ErrAlumniNotFound) {
		return nil, common.ErrAlumniNotFound
	}
	if err != nil {
		return nil, err
	}
	accessible := make(map[uint64]*model.AlumniEnrollment, len(before.Enrollments))
	for _, enrollment := range before.Enrollments {
		accessible[enrollment.ID] = enrollment
	}

	seen := make(map[uint64]struct{}, len(input.Enrollments))
	for i := range input.Enrollments {
		item := &input.Enrollments[i]
		if item.ID != 0 {
			if _, ok := seen[item.ID]; ok {
				return nil, common.ErrInvalidRequest
			}
			seen[item.ID] = struct{}{}
		}

		if item.Deleted {
			// 删除已保存学籍必须显式标记，并携带已加载的版本用于并发冲突检查。
			if item.ID == 0 || item.Version == 0 {
				return nil, common.ErrInvalidRequest
			}
			if accessible[item.ID] == nil {
				return nil, common.ErrEnrollmentNotFound
			}
			if len(item.ActivateFileIDs) > 0 || len(item.RemoveFileIDs) > 0 {
				// 删除学籍时不再处理文件变更，避免误删附件或激活草稿。
				return nil, common.ErrInvalidRequest
			}
			continue
		}

		if item.Grade == "" {
			return nil, common.ErrInvalidRequest
		}
		if item.ID == 0 {
			if len(item.ActivateFileIDs) > 0 || len(item.RemoveFileIDs) > 0 {
				// 新增学籍尚未落库，不能提前绑定文件草稿。
				return nil, common.ErrInvalidRequest
			}
			if !validExplicitTrainingLevel(item.TrainingLevel) {
				return nil, common.ErrInvalidTrainingLevel
			}
			if err := assignEnrollmentDomain(item, operator); err != nil {
				return nil, err
			}
			continue
		}
		existing := accessible[item.ID]
		if existing == nil {
			return nil, common.ErrEnrollmentNotFound
		}
		if !validStoredTrainingLevel(item.TrainingLevel) {
			return nil, common.ErrInvalidTrainingLevel
		}
		if item.Version == 0 {
			return nil, common.ErrVersionConflict
		}
		if item.DataDomainID == 0 {
			item.DataDomainID = existing.DataDomainID
		}
		if item.DataDomainID != existing.DataDomainID {
			// 培养类别更正：普通管理员必须同时拥有原域和目标域权限。
			// 原域已由 accessible 保证；隐藏学籍不会进入 accessible，
			// 因此不能借“先看到隐藏学籍再转入本人域”绕过数据域边界。
			if !operator.IsSuperAdmin() && !operator.CanAccessDomain(item.DataDomainID) {
				return nil, common.ErrPermissionDenied
			}
		}
	}
	if err := s.validateEnrollmentCategories(ctx, input.Enrollments); err != nil {
		return nil, err
	}

	result, err := s.alumni.Save(ctx, id, input, operator.UserID, domainIDs)
	switch {
	case errors.Is(err, common.ErrDatabaseUnavailable):
		logger.Error("database is unavailable", zap.Uint64("operator_id", operator.UserID), zap.Uint64("alumni_id", id), zap.Error(err))
		return nil, common.ErrDatabaseUnavailable
	case errors.Is(err, common.ErrAlumniNotFound):
		return nil, common.ErrAlumniNotFound
	case errors.Is(err, common.ErrVersionConflict):
		return nil, common.ErrVersionConflict
	case errors.Is(err, common.ErrEnrollmentNotFound):
		return nil, common.ErrEnrollmentNotFound
	case errors.Is(err, common.ErrEnrollmentCategoryMismatch):
		return nil, common.ErrEnrollmentCategoryMismatch
	case errors.Is(err, common.ErrLastEnrollment):
		return nil, common.ErrLastEnrollment
	case errors.Is(err, common.ErrInvalidDataDomain):
		return nil, err
	case errors.Is(err, common.ErrFileNotFound):
		return nil, common.ErrFileNotFound
	case errors.Is(err, common.ErrInvalidRequest), errors.Is(err, common.ErrInvalidTrainingLevel):
		return nil, err
	case err != nil:
		logger.Error("failed to save alumni", zap.Uint64("operator_id", operator.UserID), zap.Uint64("alumni_id", id), zap.Error(err))
		return nil, err
	}

	// 数据库事务提交后再清理被移除文件的 MinIO 对象；回滚时不会误删。
	if len(result.RemovedObjectKeys) > 0 && s.files != nil {
		s.files.RemoveObjects(ctx, result.RemovedObjectKeys)
	}

	domainNames, _ := s.domainNameMap(ctx)
	s.writeUpdateAudits(ctx, operator, id, before, result, accessible, input.Enrollments, domainNames)

	if s.exportCache != nil {
		_ = s.exportCache.Invalidate(ctx)
	}
	if !result.Visible {
		// 保存已成功，但操作者不再有权查看该校友：返回空详情让前端回到列表。
		return nil, nil
	}
	detail := mapAlumniDetail(result.Record, domainNames, operator)
	s.maskSensitiveFields(detail, id, operator)
	return detail, nil
}

// validateEnrollmentCategories 校验培养层次与培养类别的组合。
// 无法解析类别编码时（未注入数据域配置）跳过组合校验，避免历史档案在
// 配置缺失时无法保存；生产环境始终注入数据域仓储，未知数据域由仓储拒绝。
func (s *AlumniService) validateEnrollmentCategories(ctx context.Context, enrollments []do.AlumniEnrollmentInput) error {
	if s.dataDomains == nil {
		return nil
	}
	codes, err := s.dataDomainCodeMap(ctx)
	if err != nil {
		return err
	}
	for _, item := range enrollments {
		if item.Deleted {
			continue
		}
		code := codes[item.DataDomainID]
		if code == "" {
			continue
		}
		if !common.TrainingLevelAllowsDataDomainCode(item.TrainingLevel, code) {
			return common.ErrEnrollmentCategoryMismatch
		}
	}
	return nil
}

func (s *AlumniService) writeUpdateAudits(
	ctx context.Context,
	operator common.AccessContext,
	alumniID uint64,
	before *do.AlumniRecord,
	result *do.AlumniSaveResult,
	accessible map[uint64]*model.AlumniEnrollment,
	inputs []do.AlumniEnrollmentInput,
	domainNames map[uint64]string,
) {
	if s.auditWriter == nil || s.users == nil || before == nil || result == nil || result.Record == nil {
		return
	}
	after := result.Record
	writeProfileAudit(ctx, s.auditWriter, s.users, operator.UserID, AuditActionUpdate, "admin",
		"管理员修改个人信息", defaultAuditScope, enrollmentDomainIDs(after.Enrollments),
		before.Profile, after.Profile)

	for _, enrollment := range after.Enrollments {
		previous := accessible[enrollment.ID]
		if previous == nil {
			writeEnrollmentAudit(ctx, s.auditWriter, s.users, operator.UserID, AuditActionCreate, "admin",
				"管理员新增学籍", domainNames[enrollment.DataDomainID], alumniID, after.Profile.Name,
				enrollmentLabel(enrollment, domainNames), enrollment.DataDomainID, domainNames, nil, enrollment)
			continue
		}
		writeEnrollmentAudit(ctx, s.auditWriter, s.users, operator.UserID, AuditActionUpdate, "admin",
			"管理员修改学籍", domainNames[enrollment.DataDomainID], alumniID, after.Profile.Name,
			enrollmentLabel(enrollment, domainNames), enrollment.DataDomainID, domainNames, previous, enrollment, enrollmentFileAuditChanges(enrollment.ID, result.FileChanges)...)
	}

	// 只有显式标记删除的学籍才写删除日志；未提交的已有学籍保持不变。
	for _, input := range inputs {
		if !input.Deleted {
			continue
		}
		previous := accessible[input.ID]
		if previous == nil {
			continue
		}
		writeEnrollmentAudit(ctx, s.auditWriter, s.users, operator.UserID, AuditActionDelete, "admin",
			"管理员删除学籍", domainNames[previous.DataDomainID], alumniID, after.Profile.Name,
			enrollmentLabel(previous, domainNames), previous.DataDomainID, domainNames, previous, nil)
	}
}

// Delete 由管理员软删除只有一段学籍的校友档案。
// 多段校友的整人删除语义尚未定义，统一拒绝，避免误删隐藏学籍或共享身份。
func (s *AlumniService) Delete(ctx context.Context, operator common.AccessContext, id uint64) error {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return common.ErrDatabaseUnavailable
	}
	if !operator.IsAdministrator() {
		return common.ErrPermissionDenied
	}

	domainIDs, restricted := scopedDataDomainIDs(operator)
	if restricted && len(domainIDs) == 0 {
		return common.ErrAlumniNotFound
	}
	var before *do.AlumniRecord
	if s.auditWriter != nil && s.users != nil {
		before, _ = s.alumni.GetDetail(ctx, id, domainIDs)
	}
	if err := s.alumni.Delete(ctx, id, operator.UserID, domainIDs); err != nil {
		switch {
		case errors.Is(err, common.ErrDatabaseUnavailable):
			logger.Error("database is unavailable", zap.Uint64("operator_id", operator.UserID), zap.Uint64("alumni_id", id), zap.Error(err))
			return common.ErrDatabaseUnavailable
		case errors.Is(err, common.ErrAlumniNotFound):
			logger.Warn("alumni not found", zap.Uint64("alumni_id", id), zap.Uint64("operator_id", operator.UserID))
			return common.ErrAlumniNotFound
		case errors.Is(err, common.ErrAlumniMultiEnrollment):
			return common.ErrAlumniMultiEnrollment
		default:
			logger.Error("failed to delete alumni", zap.Uint64("operator_id", operator.UserID), zap.Uint64("alumni_id", id), zap.Error(err))
			return err
		}
	}

	if before != nil {
		domainNames, _ := s.domainNameMap(ctx)
		_ = domainNames
		writeProfileAudit(ctx, s.auditWriter, s.users, operator.UserID, AuditActionDelete, "admin",
			"管理员删除校友档案", defaultAuditScope, enrollmentDomainIDs(before.Enrollments), before.Profile, nil)
	}

	if s.countCache != nil {
		_ = s.countCache.IncrBy(ctx, -1)
	}
	if s.exportCache != nil {
		_ = s.exportCache.Invalidate(ctx)
	}

	if s.files != nil {
		if err := s.files.CascadeSoftDelete(ctx, id); err != nil {
			logger.Warn("failed to cascade delete alumni files", zap.Uint64("alumni_id", id), zap.Error(err))
		}
	}
	return nil
}

// GetMe 获取当前登录校友绑定的本人资料。
func (s *AlumniService) GetMe(ctx context.Context, access common.AccessContext) (*dto.AlumniDetail, error) {
	alumniID, err := accessAlumniID(access)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, alumniID, access)
}

// UpdateMe 更新当前登录校友本人允许维护的字段，并返回更新后的资料。
func (s *AlumniService) UpdateMe(ctx context.Context, access common.AccessContext, req dto.AlumniProfileUpdateRequest) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	alumniID, err := accessAlumniID(access)
	if err != nil {
		return nil, err
	}

	profile := req.ToProfile().Normalize()
	var before *model.AlumniProfile
	var domainIDs []uint64
	if s.auditWriter != nil && s.users != nil {
		before, _ = s.alumni.GetPersonalProfile(ctx, alumniID)
		if record, err := s.alumni.GetDetail(ctx, alumniID, nil); err == nil {
			domainIDs = enrollmentDomainIDs(record.Enrollments)
		}
	}
	if !profile.IsEmpty() {
		if err := s.alumni.UpdateEditableFields(ctx, alumniID, access.UserID, profile); err != nil {
			if errors.Is(err, common.ErrDatabaseUnavailable) {
				logger.Error("database is unavailable", zap.Uint64("alumni_id", alumniID), zap.Uint64("user_id", access.UserID), zap.Error(err))
				return nil, common.ErrDatabaseUnavailable
			}
			if errors.Is(err, common.ErrAlumniNotFound) {
				logger.Warn("alumni not found", zap.Uint64("alumni_id", alumniID), zap.Uint64("user_id", access.UserID))
				return nil, common.ErrAlumniNotFound
			}
			logger.Error("failed to update alumni profile", zap.Uint64("alumni_id", alumniID), zap.Uint64("user_id", access.UserID), zap.Error(err))
			return nil, err
		}
	}
	if before != nil {
		after, _ := s.alumni.GetPersonalProfile(ctx, alumniID)
		writeProfileAudit(ctx, s.auditWriter, s.users, access.UserID, AuditActionUpdate, "alumni_self",
			"校友本人修改个人资料", defaultAuditScope, domainIDs, before, after)
	}

	if s.exportCache != nil {
		_ = s.exportCache.Invalidate(ctx)
	}
	return s.GetByID(ctx, alumniID, access)
}

// accessAlumniID 从已经加载的授权上下文中获取校友账号绑定的档案 ID。
func accessAlumniID(access common.AccessContext) (uint64, error) {
	if access.Role != common.RoleAlumni {
		return 0, common.ErrPermissionDenied
	}
	if access.AlumniID == nil || *access.AlumniID == 0 {
		return 0, common.ErrAlumniProfileUnbound
	}
	return *access.AlumniID, nil
}

// domainNameMap 返回数据域 ID 到名称的映射；包含已停用数据域，保证历史档案可展示。
func (s *AlumniService) domainNameMap(ctx context.Context) (map[uint64]string, error) {
	names := make(map[uint64]string)
	if s.dataDomains == nil {
		return names, nil
	}
	domains, err := s.dataDomains.ListDataDomains(ctx)
	if err != nil {
		return nil, err
	}
	for _, domain := range domains {
		if domain != nil {
			names[domain.ID] = domain.Name
		}
	}
	return names, nil
}

func enrollmentDomainIDs(enrollments []*model.AlumniEnrollment) []uint64 {
	seen := make(map[uint64]struct{}, len(enrollments))
	ids := make([]uint64, 0, len(enrollments))
	for _, enrollment := range enrollments {
		if enrollment == nil {
			continue
		}
		if _, ok := seen[enrollment.DataDomainID]; ok {
			continue
		}
		seen[enrollment.DataDomainID] = struct{}{}
		ids = append(ids, enrollment.DataDomainID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func enrollmentLabel(enrollment *model.AlumniEnrollment, domainNames map[uint64]string) string {
	if enrollment == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if enrollment.Grade != "" {
		parts = append(parts, enrollment.Grade+"级")
	}
	if label := TrainingLevelLabel(enrollment.TrainingLevel); label != "未填写" {
		parts = append(parts, label)
	}
	if name := domainNames[enrollment.DataDomainID]; name != "" {
		parts = append(parts, name)
	}
	return strings.Join(parts, " · ")
}

// mapAlumniListItems 将个人档案与可访问学籍转换为列表项。
func mapAlumniListItems(records []do.AlumniRecord, domainNames map[uint64]string) []dto.AlumniListItem {
	result := make([]dto.AlumniListItem, 0, len(records))
	for _, record := range records {
		if record.Profile == nil {
			continue
		}
		item := dto.AlumniListItem{
			ID:          record.Profile.ID,
			Name:        record.Profile.Name,
			Gender:      record.Profile.Gender,
			Industry:    record.Profile.Industry,
			WorkUnit:    record.Profile.WorkUnit,
			Position:    record.Profile.Position,
			Mobile:      record.Profile.Mobile,
			Email:       record.Profile.Email,
			Version:     record.Profile.Version,
			UpdatedAt:   record.Profile.UpdatedAt,
			Enrollments: make([]dto.AlumniEnrollmentSummary, 0, len(record.Enrollments)),
		}
		for _, enrollment := range record.Enrollments {
			item.Enrollments = append(item.Enrollments, dto.AlumniEnrollmentSummary{
				ID:             enrollment.ID,
				DataDomainID:   enrollment.DataDomainID,
				DataDomainName: domainNames[enrollment.DataDomainID],
				TrainingLevel:  enrollment.TrainingLevel,
				Grade:          enrollment.Grade,
				ClassName:      enrollment.ClassName,
				Cohort:         enrollment.Cohort,
				Counselor:      enrollment.Counselor,
				Mentor:         enrollment.Mentor,
				Major:          enrollment.Major,
				TrainingMode:   enrollment.TrainingMode,
			})
		}
		result = append(result, item)
	}
	return result
}

// mapAlumniDetail 将个人档案与可访问学籍转换为详情响应。
func mapAlumniDetail(record *do.AlumniRecord, domainNames map[uint64]string, viewer common.AccessContext) *dto.AlumniDetail {
	if record == nil || record.Profile == nil {
		return nil
	}
	profile := record.Profile
	detail := &dto.AlumniDetail{
		ID:             profile.ID,
		Name:           profile.Name,
		Gender:         profile.Gender,
		Mobile:         profile.Mobile,
		Email:          profile.Email,
		Industry:       profile.Industry,
		WorkUnit:       profile.WorkUnit,
		Position:       profile.Position,
		MailingAddress: profile.MailingAddress,
		Status:         profile.Status,
		Version:        profile.Version,
		CreatedAt:      profile.CreatedAt,
		UpdatedAt:      profile.UpdatedAt,
		Enrollments:    make([]dto.AlumniEnrollmentDetail, 0, len(record.Enrollments)),
	}
	for _, enrollment := range record.Enrollments {
		item := dto.AlumniEnrollmentDetail{
			ID:             enrollment.ID,
			DataDomainID:   enrollment.DataDomainID,
			DataDomainName: domainNames[enrollment.DataDomainID],
			TrainingLevel:  enrollment.TrainingLevel,
			Grade:          enrollment.Grade,
			ClassName:      enrollment.ClassName,
			Cohort:         enrollment.Cohort,
			Counselor:      enrollment.Counselor,
			Mentor:         enrollment.Mentor,
			Major:          enrollment.Major,
			TrainingMode:   enrollment.TrainingMode,
			Version:        enrollment.Version,
			CreatedAt:      enrollment.CreatedAt,
			UpdatedAt:      enrollment.UpdatedAt,
		}
		// 学籍内备注只给管理员，且仅在当前查看者可访问该学籍时由仓储返回。
		if viewer.IsAdministrator() {
			item.Remark = enrollment.Remark
		}
		detail.Enrollments = append(detail.Enrollments, item)
	}
	return detail
}
