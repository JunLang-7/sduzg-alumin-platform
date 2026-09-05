package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/cache"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

// AlumniFileCleaner 校友删除时级联清理文件的接口。
type AlumniFileCleaner interface {
	CascadeSoftDelete(ctx context.Context, alumniID uint64) error
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

var alumniColumnHeaders = []string{"姓名", "年级", "班级", "届数", "辅导员", "导师", "专业", "培养方式", "行业", "工作单位", "职务", "通讯地址", "性别", "手机号", "邮箱"}

// alumniImportColumnHeaders 在通用校友字段外增加所属领域，供多域管理员逐行指定目标领域。
var alumniImportColumnHeaders = append(append([]string{}, alumniColumnHeaders...), "所属领域")

func exportRow(item *model.AlumniProfile) []string {
	return []string{
		sanitizeExportValue(item.Name),
		sanitizeExportValue(item.Grade),
		sanitizeExportValue(stringOrEmpty(item.ClassName)),
		sanitizeExportValue(stringOrEmpty(item.Cohort)),
		sanitizeExportValue(stringOrEmpty(item.Counselor)),
		sanitizeExportValue(stringOrEmpty(item.Mentor)),
		sanitizeExportValue(stringOrEmpty(item.Major)),
		sanitizeExportValue(stringOrEmpty(item.TrainingMode)),
		sanitizeExportValue(stringOrEmpty(item.Industry)),
		sanitizeExportValue(stringOrEmpty(item.WorkUnit)),
		sanitizeExportValue(stringOrEmpty(item.Position)),
		sanitizeExportValue(stringOrEmpty(item.MailingAddress)),
		sanitizeExportValue(stringOrEmpty(item.Gender)),
		sanitizeExportValue(stringOrEmpty(item.Mobile)),
		sanitizeExportValue(stringOrEmpty(item.Email)),
	}
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// sanitizeExportValue 防止电子表格公式注入。
// 当值以 =、+、-、@ 开头时，前加单引号使其被解释为纯文本。
func sanitizeExportValue(v string) string {
	if v == "" {
		return v
	}
	if v[0] == '=' || v[0] == '+' || v[0] == '-' || v[0] == '@' {
		return "'" + v
	}
	return v
}

type AlumniService struct {
	alumni      repository.AlumniStore
	files       AlumniFileCleaner
	dataDomains repository.AccessControlStore
	opLogger    OperationLogWriter
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

// WithOperationLogger 注入导出操作审计记录器。
func (s *AlumniService) WithOperationLogger(logger OperationLogWriter) *AlumniService {
	s.opLogger = logger
	return s
}

// List 根据查询条件分页获取校友列表。
func (s *AlumniService) List(ctx context.Context, req dto.AlumniListRequest, viewer common.AccessContext) (common.Pager[dto.AlumniListItem], error) {
	query := req.ToQuery().Normalize()
	if s.alumni == nil {
		return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), common.ErrDatabaseUnavailable
	}
	if !viewer.HasPermission(common.PermissionAlumniSensitiveRead) && (query.Position != "" || query.Mobile != "") {
		return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), common.ErrPermissionDenied
	}
	if query.DataDomainID != nil {
		if !viewer.IsSuperAdmin() && !viewer.CanAccessDomain(*query.DataDomainID) {
			return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), common.ErrPermissionDenied
		}
		query.DataDomainIDs = []uint64{*query.DataDomainID}
	} else if domainIDs, restricted := scopedDataDomainIDs(viewer); restricted {
		if len(domainIDs) == 0 {
			return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), common.ErrPermissionDenied
		}
		query.DataDomainIDs = domainIDs
	}
	query.CanReadSensitive = viewer.HasPermission(common.PermissionAlumniSensitiveRead)
	query = query.Normalize()

	// 无过滤条件时优先用缓存计数，跳过 DB COUNT(*)
	if query.IsUnfiltered() && len(query.DataDomainIDs) == 0 && s.countCache != nil {
		total, hit, _ := s.countCache.Get(ctx)
		if !hit {
			if n, err := s.alumni.CountActive(ctx); err == nil {
				total = n
				_ = s.countCache.Set(ctx, n)
			}
		}
		items, err := s.alumni.FindOnly(ctx, query)
		if err != nil {
			return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), err
		}
		mapped := mapAlumniListItems(items)
		s.maskListItems(mapped, viewer)
		return common.NewPager(mapped, query.Page, total), nil
	}

	items, total, err := s.alumni.List(ctx, query)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Error(err))
		return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), common.ErrDatabaseUnavailable
	}
	if err != nil {
		logger.Error("failed to list alumni", zap.Error(err))
		return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), err
	}

	mapped := mapAlumniListItems(items)
	s.maskListItems(mapped, viewer)
	return common.NewPager(mapped, query.Page, total), nil
}

// maskListItems 默认屏蔽列表中的敏感字段，仅当授权上下文包含敏感字段查看权限时才放行。
func (s *AlumniService) maskListItems(items []dto.AlumniListItem, viewer common.AccessContext) {
	mask := func() {
		for i := range items {
			items[i].Mobile = nil
			items[i].Email = nil
			items[i].Position = nil
		}
	}

	if viewer.HasPermission(common.PermissionAlumniSensitiveRead) {
		return
	}

	mask()
}

func scopedDataDomainIDs(access common.AccessContext) ([]uint64, bool) {
	if access.Role == common.RoleAdmin {
		return access.DomainIDs, true
	}
	return nil, false
}

func profileContainsSensitiveFields(profile do.AlumniCreateProfile) bool {
	return profile.Position != nil || profile.MailingAddress != nil || profile.Mobile != nil || profile.Email != nil
}

func updateContainsSensitiveFields(profile do.AlumniUpdateProfile) bool {
	return profile.Position != nil || profile.MailingAddress != nil || profile.Mobile != nil || profile.Email != nil
}

func assignCreateDataDomain(profile *do.AlumniCreateProfile, operator common.AccessContext) error {
	if operator.IsSuperAdmin() {
		return nil
	}
	if operator.Role != common.RoleAdmin || len(operator.DomainIDs) == 0 {
		return common.ErrPermissionDenied
	}
	if len(operator.DomainIDs) == 1 {
		profile.DataDomainID = &operator.DomainIDs[0]
		return nil
	}
	if profile.DataDomainID == nil || !operator.CanAccessDomain(*profile.DataDomainID) {
		return common.ErrPermissionDenied
	}
	return nil
}

// GetByID 根据 ID 获取校友详情，并根据请求授权上下文屏蔽字段。
func (s *AlumniService) GetByID(ctx context.Context, id uint64, viewer common.AccessContext) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	domainIDs, restricted := scopedDataDomainIDs(viewer)
	if restricted && len(domainIDs) == 0 {
		return nil, common.ErrAlumniNotFound
	}
	item, err := s.alumni.GetByID(ctx, id, domainIDs)
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

	detail := mapAlumniDetail(item)
	s.maskSensitiveFields(detail, id, viewer)
	return detail, nil
}

// maskSensitiveFields 当查看者为普通校友且查看的不是本人资料时，屏蔽敏感字段。
// maskSensitiveFields 默认屏蔽详情中的敏感字段，仅当授权上下文确认查看者为管理员或本人时才放行。
func (s *AlumniService) maskSensitiveFields(detail *dto.AlumniDetail, alumniID uint64, viewer common.AccessContext) {
	if detail == nil {
		return
	}

	mask := func() {
		detail.Mobile = nil
		detail.Email = nil
		detail.Position = nil
		detail.MailingAddress = nil
	}

	if viewer.HasPermission(common.PermissionAlumniSensitiveRead) {
		return
	}

	// 校友查看本人资料时不屏蔽
	if viewer.AlumniID != nil && *viewer.AlumniID == alumniID {
		return
	}

	mask()
}

// Create 由管理员新增校友档案。
func (s *AlumniService) Create(ctx context.Context, operator common.AccessContext, req dto.AdminAlumniCreateRequest) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}

	profile := req.ToProfile().Normalize()
	if profile.Name == "" || profile.Grade == "" {
		return nil, common.ErrInvalidRequest
	}
	if profile.Status != common.AlumniStatusActive {
		return nil, common.ErrInvalidRequest
	}
	if !operator.HasPermission(common.PermissionAlumniSensitiveRead) && profileContainsSensitiveFields(profile) {
		return nil, common.ErrPermissionDenied
	}
	if err := assignCreateDataDomain(&profile, operator); err != nil {
		return nil, err
	}

	created, err := s.alumni.Create(ctx, &profile, operator.UserID)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("operator_id", operator.UserID), zap.Error(err))
		return nil, common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrInvalidRequest) {
		return nil, common.ErrInvalidRequest
	}
	if err != nil {
		logger.Error("failed to create alumni", zap.Uint64("operator_id", operator.UserID), zap.Error(err))
		return nil, err
	}

	if s.countCache != nil {
		_ = s.countCache.IncrBy(ctx, 1)
	}
	if s.exportCache != nil {
		_ = s.exportCache.Invalidate(ctx)
	}
	return mapAlumniDetail(created), nil
}

// Update 由管理员编辑校友档案。
func (s *AlumniService) Update(ctx context.Context, operator common.AccessContext, id uint64, req dto.AdminAlumniUpdateRequest) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}

	profile := req.ToProfile().Normalize()
	if profile.Name == "" || profile.Grade == "" {
		return nil, common.ErrInvalidRequest
	}
	if !operator.HasPermission(common.PermissionAlumniSensitiveRead) && updateContainsSensitiveFields(profile) {
		return nil, common.ErrPermissionDenied
	}
	domainIDs, restricted := scopedDataDomainIDs(operator)
	if restricted && len(domainIDs) == 0 {
		return nil, common.ErrAlumniNotFound
	}

	if err := s.alumni.Update(ctx, id, operator.UserID, profile, domainIDs); err != nil {
		if errors.Is(err, common.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Uint64("operator_id", operator.UserID), zap.Uint64("alumni_id", id), zap.Error(err))
			return nil, common.ErrDatabaseUnavailable
		}
		if errors.Is(err, common.ErrAlumniNotFound) {
			logger.Warn("alumni not found", zap.Uint64("alumni_id", id), zap.Uint64("operator_id", operator.UserID))
			return nil, common.ErrAlumniNotFound
		}
		logger.Error("failed to update alumni", zap.Uint64("operator_id", operator.UserID), zap.Uint64("alumni_id", id), zap.Error(err))
		return nil, err
	}

	updated, err := s.GetByID(ctx, id, operator)
	if err != nil {
		return nil, err
	}

	return updated, nil
}

// Delete 由管理员软删除校友档案。
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
	if err := s.alumni.Delete(ctx, id, operator.UserID, domainIDs); err != nil {
		if errors.Is(err, common.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Uint64("operator_id", operator.UserID), zap.Uint64("alumni_id", id), zap.Error(err))
			return common.ErrDatabaseUnavailable
		}
		if errors.Is(err, common.ErrAlumniNotFound) {
			logger.Warn("alumni not found", zap.Uint64("alumni_id", id), zap.Uint64("operator_id", operator.UserID))
			return common.ErrAlumniNotFound
		}
		logger.Error("failed to delete alumni", zap.Uint64("operator_id", operator.UserID), zap.Uint64("alumni_id", id), zap.Error(err))
		return err
	}

	if s.countCache != nil {
		_ = s.countCache.IncrBy(ctx, -1)
	}
	if s.exportCache != nil {
		_ = s.exportCache.Invalidate(ctx)
	}

	// 级联清理关联的档案文件（best-effort）
	if s.files != nil {
		if err := s.files.CascadeSoftDelete(ctx, id); err != nil {
			logger.Warn("failed to cascade delete alumni files",
				zap.Uint64("alumni_id", id),
				zap.Error(err),
			)
		}
	}

	return nil
}

// Export 导出校友数据为 xlsx 或 csv 格式。
func (s *AlumniService) Export(ctx context.Context, req dto.AlumniExportRequest, operator common.AccessContext) (*ExportResult, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}

	query := req.ToQuery().Normalize()
	if !operator.HasPermission(common.PermissionAlumniSensitiveRead) && (query.Position != "" || query.Mobile != "") {
		return nil, common.ErrPermissionDenied
	}
	if query.DataDomainID != nil {
		if !operator.IsSuperAdmin() && !operator.CanAccessDomain(*query.DataDomainID) {
			return nil, common.ErrPermissionDenied
		}
		query.DataDomainIDs = []uint64{*query.DataDomainID}
	} else if domainIDs, restricted := scopedDataDomainIDs(operator); restricted {
		if len(domainIDs) == 0 {
			return nil, common.ErrPermissionDenied
		}
		query.DataDomainIDs = domainIDs
	}
	query.CanReadSensitive = operator.HasPermission(common.PermissionAlumniSensitiveRead)
	query = query.Normalize()
	format := req.FormatOrDefault()

	// 优先读缓存，避免全表扫描
	if s.exportCache != nil {
		if cached, err := s.exportCache.Get(ctx, query); err == nil {
			var items []*model.AlumniProfile
			if json.Unmarshal(cached, &items) == nil {
				return s.buildAndAuditExport(ctx, operator, query, items, format)
			}
		}
	}

	items, err := s.alumni.ListAll(ctx, query)
	if err != nil {
		if errors.Is(err, common.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Error(err))
			return nil, common.ErrDatabaseUnavailable
		}
		logger.Error("failed to list alumni for export", zap.Error(err))
		return nil, err
	}

	// 缓存查询结果（best-effort）
	if s.exportCache != nil {
		if data, err := json.Marshal(items); err == nil {
			_ = s.exportCache.Set(ctx, query, data)
		}
	}

	return s.buildAndAuditExport(ctx, operator, query, items, format)
}

type exportAuditDetail struct {
	Format                  string   `json:"format"`
	DataDomainIDs           []uint64 `json:"data_domain_ids,omitempty"`
	RecordCount             int      `json:"record_count"`
	SensitiveFieldsIncluded bool     `json:"sensitive_fields_included"`
}

func (s *AlumniService) buildAndAuditExport(ctx context.Context, operator common.AccessContext, query do.AlumniListQuery, items []*model.AlumniProfile, format string) (*ExportResult, error) {
	result, err := s.buildExport(maskExportItems(items, operator), format)
	if err != nil {
		return nil, err
	}
	if s.opLogger != nil {
		detail, err := json.Marshal(exportAuditDetail{
			Format:                  format,
			DataDomainIDs:           query.DataDomainIDs,
			RecordCount:             len(items),
			SensitiveFieldsIncluded: operator.HasPermission(common.PermissionAlumniSensitiveRead),
		})
		if err == nil {
			detailText := string(detail)
			_ = s.opLogger.Write(ctx, &model.OperationLog{
				OperatorID:   operator.UserID,
				OperatorRole: operator.Role,
				Action:       "export_alumni",
				TargetType:   "alumni_export",
				Detail:       &detailText,
			})
		}
	}
	return result, nil
}

func (s *AlumniService) buildExport(items []*model.AlumniProfile, format string) (*ExportResult, error) {
	switch format {
	case "csv":
		return buildCSV(items)
	default:
		return buildXLSX(items)
	}
}

// maskExportItems 为无敏感字段权限的导出请求清空受保护字段，且不修改缓存或仓储返回对象。
func maskExportItems(items []*model.AlumniProfile, operator common.AccessContext) []*model.AlumniProfile {
	if operator.HasPermission(common.PermissionAlumniSensitiveRead) {
		return items
	}
	masked := make([]*model.AlumniProfile, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		copyItem := *item
		copyItem.Mobile = nil
		copyItem.Email = nil
		copyItem.Position = nil
		copyItem.MailingAddress = nil
		masked = append(masked, &copyItem)
	}
	return masked
}

// ExportTemplate 生成导入模板 Excel 文件，包含表头行和一条示例空行。
func (s *AlumniService) ExportTemplate(ctx context.Context) (*ExportResult, error) {
	result, err := buildTemplateXLSX()
	if err != nil {
		logger.Error("failed to build template xlsx", zap.Error(err))
		return nil, fmt.Errorf("build template: %w", err)
	}
	return result, nil
}

func buildTemplateXLSX() (*ExportResult, error) {
	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		logger.Error("build template: create stream writer", zap.Error(err))
		return nil, fmt.Errorf("create stream writer: %w", err)
	}

	// 写表头行
	headerRow := make([]any, len(alumniImportColumnHeaders))
	for i, h := range alumniImportColumnHeaders {
		headerRow[i] = h
	}
	if err := sw.SetRow("A1", headerRow); err != nil {
		logger.Error("build template: write header", zap.Error(err))
		return nil, fmt.Errorf("write header: %w", err)
	}

	// 写一条空行，提示用户按此结构填写
	emptyRow := make([]any, len(alumniImportColumnHeaders))
	for i := range emptyRow {
		emptyRow[i] = ""
	}
	if err := sw.SetRow("A2", emptyRow); err != nil {
		logger.Error("build template: write empty row", zap.Error(err))
		return nil, fmt.Errorf("write empty row: %w", err)
	}

	if err := sw.Flush(); err != nil {
		logger.Error("build template: flush stream writer", zap.Error(err))
		return nil, fmt.Errorf("flush stream: %w", err)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		logger.Error("build template: write xlsx to buffer", zap.Error(err))
		return nil, fmt.Errorf("write xlsx: %w", err)
	}

	return &ExportResult{
		Data:        buf.Bytes(),
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Filename:    "alumni_import_template.xlsx",
	}, nil
}

func buildXLSX(items []*model.AlumniProfile) (*ExportResult, error) {
	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, fmt.Errorf("create stream writer: %w", err)
	}

	headerRow := make([]any, len(alumniColumnHeaders))
	for i, h := range alumniColumnHeaders {
		headerRow[i] = h
	}
	if err := sw.SetRow("A1", headerRow); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}

	for i, item := range items {
		row := exportRow(item)
		vals := make([]any, len(row))
		for j, v := range row {
			vals[j] = v
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := sw.SetRow(cell, vals); err != nil {
			return nil, fmt.Errorf("write row %d: %w", i+2, err)
		}
	}

	if err := sw.Flush(); err != nil {
		return nil, fmt.Errorf("flush stream: %w", err)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}

	return &ExportResult{
		Data:        buf.Bytes(),
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Filename:    "alumni_export.xlsx",
	}, nil
}

func buildCSV(items []*model.AlumniProfile) (*ExportResult, error) {
	var buf bytes.Buffer

	// UTF-8 BOM
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(&buf)
	if err := w.Write(alumniColumnHeaders); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}
	for _, item := range items {
		if err := w.Write(exportRow(item)); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}

	return &ExportResult{
		Data:        buf.Bytes(),
		ContentType: "text/csv; charset=utf-8",
		Filename:    "alumni_export.csv",
	}, nil
}

// Import 从上传的 xlsx 文件批量导入校友档案。逐行校验，姓名和年级为必填。
func (s *AlumniService) Import(ctx context.Context, operator common.AccessContext, dataDomainID *uint64, file io.Reader) (*dto.AlumniImportResult, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	targetProfile := do.AlumniCreateProfile{DataDomainID: dataDomainID}
	if operator.Role == common.RoleAdmin {
		if len(operator.DomainIDs) == 0 {
			return nil, common.ErrPermissionDenied
		}
		if len(operator.DomainIDs) == 1 {
			targetProfile.DataDomainID = &operator.DomainIDs[0]
		} else if dataDomainID != nil && !operator.CanAccessDomain(*dataDomainID) {
			return nil, common.ErrPermissionDenied
		}
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read uploaded file: %w", err)
	}

	if len(data) < 4 || data[0] != 0x50 || data[1] != 0x4B || data[2] != 0x03 || data[3] != 0x04 {
		return nil, common.ErrInvalidRequest
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, common.ErrInvalidRequest
	}
	defer f.Close()

	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		return nil, fmt.Errorf("read sheet rows: %w", err)
	}

	const maxRows = 5001 // 表头 + 最多 5000 行数据
	if len(rows) > maxRows {
		return nil, fmt.Errorf("单次最多导入 5000 条数据，当前文件 %d 行", len(rows)-1)
	}

	if len(rows) < 2 {
		return nil, common.ErrInvalidRequest
	}

	header := rows[0]
	hasDomainColumn := matchesImportHeaders(header, alumniImportColumnHeaders)
	if !hasDomainColumn && !matchesImportHeaders(header, alumniColumnHeaders) {
		return nil, fmt.Errorf("表头不正确，应使用 %d 列通用模板或 %d 列含所属领域模板", len(alumniColumnHeaders), len(alumniImportColumnHeaders))
	}

	domainByCode := map[string]*model.DataDomain{}
	if targetProfile.DataDomainID == nil && hasDomainColumn {
		if s.dataDomains == nil {
			return nil, common.ErrDatabaseUnavailable
		}
		activeDomains, err := s.dataDomains.ListActiveDataDomains(ctx)
		if err != nil {
			return nil, err
		}
		for _, domain := range activeDomains {
			if domain != nil {
				domainByCode[domain.Code] = domain
			}
		}
	}
	if targetProfile.DataDomainID == nil && !hasDomainColumn {
		return nil, common.ErrInvalidRequest
	}

	type rowProfile struct {
		rowNum  int
		profile do.AlumniCreateProfile
	}
	var validRows []rowProfile
	rowErrors := make([]dto.AlumniRowError, 0)

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		profile := parseRowToProfile(row)
		profile = profile.Normalize()
		if targetProfile.DataDomainID != nil {
			profile.DataDomainID = targetProfile.DataDomainID
		} else {
			code := strings.TrimSpace(cellValue(row, len(alumniImportColumnHeaders)-1))
			domain, exists := domainByCode[code]
			if code == "" {
				rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: profile.Name, Message: "所属领域为空"})
				continue
			}
			if !exists {
				rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: profile.Name, Message: "所属领域无效"})
				continue
			}
			if !operator.IsSuperAdmin() && !operator.CanAccessDomain(domain.ID) {
				rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: profile.Name, Message: "无权导入该所属领域"})
				continue
			}
			profile.DataDomainID = &domain.ID
		}

		if profile.Name == "" {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: profile.Name, Message: "姓名为空"})
			continue
		}
		if profile.Grade == "" {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: profile.Name, Message: "年级为空"})
			continue
		}
		if !operator.HasPermission(common.PermissionAlumniSensitiveRead) && profileContainsSensitiveFields(profile) {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: profile.Name, Message: "无权导入敏感字段"})
			continue
		}

		validRows = append(validRows, rowProfile{rowNum: rowNum, profile: profile})
	}

	if len(validRows) > 0 {
		dedupKeys := make([]do.AlumniDedupKey, 0, len(validRows))
		for _, rp := range validRows {
			dedupKeys = append(dedupKeys, alumniImportDedupKey(rp.profile))
		}

		existing, err := s.alumni.FindExistingByDedupKey(ctx, dedupKeys)
		if err != nil {
			logger.Error("failed to check duplicates", zap.Uint64("operator_id", operator.UserID), zap.Error(err))
			return nil, err
		}

		var dedupedProfiles []do.AlumniCreateProfile
		for _, rp := range validRows {
			key := alumniImportDedupKey(rp.profile).Key()
			if existing[key] {
				rowErrors = append(rowErrors, dto.AlumniRowError{Row: rp.rowNum, Name: rp.profile.Name, Message: "已存在相同姓名、年级、班级、届数和手机号的记录"})
			} else {
				dedupedProfiles = append(dedupedProfiles, rp.profile)
				existing[key] = true
			}
		}
		validProfiles := dedupedProfiles

		result := &dto.AlumniImportResult{
			Total:  len(rows) - 1,
			Errors: rowErrors,
		}

		if len(validProfiles) > 0 {
			if err := s.alumni.BatchCreate(ctx, validProfiles, operator.UserID); err != nil {
				logger.Error("failed to batch create alumni", zap.Uint64("operator_id", operator.UserID), zap.Error(err))
				return nil, err
			}
			result.Success = len(validProfiles)
			if s.countCache != nil {
				_ = s.countCache.IncrBy(ctx, int64(len(validProfiles)))
			}
			if s.exportCache != nil {
				_ = s.exportCache.Invalidate(ctx)
			}
		}

		return result, nil
	}

	return &dto.AlumniImportResult{
		Total:  len(rows) - 1,
		Errors: rowErrors,
	}, nil
}

func matchesImportHeaders(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i, value := range actual {
		if strings.TrimSpace(value) != expected[i] {
			return false
		}
	}
	return true
}

func cellValue(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func alumniImportDedupKey(profile do.AlumniCreateProfile) do.AlumniDedupKey {
	return do.AlumniDedupKey{
		Name:      profile.Name,
		Grade:     profile.Grade,
		ClassName: stringOrEmpty(profile.ClassName),
		Cohort:    stringOrEmpty(profile.Cohort),
		Mobile:    strings.TrimSpace(stringOrEmpty(profile.Mobile)),
	}
}

func parseRowToProfile(row []string) do.AlumniCreateProfile {
	p := do.AlumniCreateProfile{Status: common.AlumniStatusActive}

	get := func(idx int) string {
		if idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}
	optionalStr := func(idx int) *string {
		v := get(idx)
		if v == "" {
			return nil
		}
		return &v
	}

	p.Name = get(0)
	p.Grade = get(1)
	p.ClassName = optionalStr(2)
	p.Cohort = optionalStr(3)
	p.Counselor = optionalStr(4)
	p.Mentor = optionalStr(5)
	p.Major = optionalStr(6)
	p.TrainingMode = optionalStr(7)
	p.Industry = optionalStr(8)
	p.WorkUnit = optionalStr(9)
	p.Position = optionalStr(10)
	p.MailingAddress = optionalStr(11)
	p.Gender = optionalStr(12)
	p.Mobile = optionalStr(13)
	p.Email = optionalStr(14)

	return p
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

// mapAlumniListItems 将 AlumniProfile 列表转换为 AlumniListItem 列表
func mapAlumniListItems(items []*model.AlumniProfile) []dto.AlumniListItem {
	result := make([]dto.AlumniListItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, dto.AlumniListItem{
			ID:           item.ID,
			DataDomainID: item.DataDomainID,
			Name:         item.Name,
			Grade:        item.Grade,
			ClassName:    item.ClassName,
			Cohort:       item.Cohort,
			Counselor:    item.Counselor,
			Mentor:       item.Mentor,
			Major:        item.Major,
			TrainingMode: item.TrainingMode,
			Industry:     item.Industry,
			WorkUnit:     item.WorkUnit,
			Position:     item.Position,
			Gender:       item.Gender,
			Mobile:       item.Mobile,
			Email:        item.Email,
			UpdatedAt:    item.UpdatedAt,
		})
	}
	return result
}

// mapAlumniDetail 将 AlumniProfile 转换为详情响应
func mapAlumniDetail(item *model.AlumniProfile) *dto.AlumniDetail {
	if item == nil {
		return nil
	}

	return &dto.AlumniDetail{
		ID:             item.ID,
		DataDomainID:   item.DataDomainID,
		Name:           item.Name,
		Grade:          item.Grade,
		ClassName:      item.ClassName,
		Cohort:         item.Cohort,
		Counselor:      item.Counselor,
		Mentor:         item.Mentor,
		Major:          item.Major,
		TrainingMode:   item.TrainingMode,
		Industry:       item.Industry,
		WorkUnit:       item.WorkUnit,
		Position:       item.Position,
		MailingAddress: item.MailingAddress,
		Gender:         item.Gender,
		Mobile:         item.Mobile,
		Email:          item.Email,
		Status:         item.Status,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}
