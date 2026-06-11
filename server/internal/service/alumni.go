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

type ExportResult struct {
	Data        []byte
	ContentType string
	Filename    string
}

var alumniColumnHeaders = []string{"姓名", "年级", "班级", "届数", "辅导员", "导师", "专业", "培养方式", "行业", "工作单位", "职务", "通讯地址", "性别", "手机号", "邮箱"}

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
	users       repository.UserStore
	files       AlumniFileCleaner
	countCache  *cache.CountCache
	exportCache *cache.ExportCache
}

func NewAlumniService(alumni repository.AlumniStore, users repository.UserStore, files AlumniFileCleaner) *AlumniService {
	return &AlumniService{alumni: alumni, users: users, files: files}
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

// List 根据查询条件分页获取校友列表。
func (s *AlumniService) List(ctx context.Context, req dto.AlumniListRequest, viewerID uint64) (common.Pager[dto.AlumniListItem], error) {
	query := req.ToQuery().Normalize()
	if s.alumni == nil {
		return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), common.ErrDatabaseUnavailable
	}

	// 无过滤条件时优先用缓存计数，跳过 DB COUNT(*)
	if query.IsUnfiltered() && s.countCache != nil {
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
		s.maskListItems(ctx, mapped, viewerID)
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
	s.maskListItems(ctx, mapped, viewerID)
	return common.NewPager(mapped, query.Page, total), nil
}

// maskListItems 默认屏蔽列表中的敏感字段，仅当确认查看者为管理员时才放行。
func (s *AlumniService) maskListItems(ctx context.Context, items []dto.AlumniListItem, viewerID uint64) {
	mask := func() {
		for i := range items {
			items[i].Mobile = nil
			items[i].Email = nil
			items[i].Position = nil
		}
	}

	if s.users == nil {
		logger.Error("user repository is not initialized, masking list sensitive fields by default")
		mask()
		return
	}

	viewer, err := s.users.FindByID(ctx, viewerID)
	if err != nil {
		logger.Error("failed to find viewer for list, masking by default", zap.Uint64("viewer_id", viewerID), zap.Error(err))
		mask()
		return
	}

	// 仅管理员和超级管理员可查看完整信息
	if viewer.Role == common.RoleAdmin || viewer.Role == common.RoleSuperAdmin {
		return
	}

	mask()
}

// GetByID 根据 ID 获取校友详情。viewerID 为查看者用户 ID，用于基于角色的字段屏蔽。
func (s *AlumniService) GetByID(ctx context.Context, id uint64, viewerID uint64) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	item, err := s.alumni.GetByID(ctx, id)
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
	s.maskSensitiveFields(ctx, detail, id, viewerID)
	return detail, nil
}

// maskSensitiveFields 当查看者为普通校友且查看的不是本人资料时，屏蔽敏感字段。
// maskSensitiveFields 默认屏蔽详情中的敏感字段，仅当确认查看者有权限时才放行。
func (s *AlumniService) maskSensitiveFields(ctx context.Context, detail *dto.AlumniDetail, alumniID uint64, viewerID uint64) {
	if detail == nil {
		return
	}

	mask := func() {
		detail.Mobile = nil
		detail.Email = nil
		detail.Position = nil
		detail.MailingAddress = nil
	}

	if s.users == nil {
		logger.Error("user repository is not initialized, masking detail sensitive fields by default")
		mask()
		return
	}

	viewer, err := s.users.FindByID(ctx, viewerID)
	if err != nil {
		logger.Error("failed to find viewer for detail, masking by default", zap.Uint64("viewer_id", viewerID), zap.Error(err))
		mask()
		return
	}

	// 管理员和超级管理员可查看完整信息
	if viewer.Role == common.RoleAdmin || viewer.Role == common.RoleSuperAdmin {
		return
	}

	// 校友查看本人资料时不屏蔽
	if viewer.AlumniID != nil && *viewer.AlumniID == alumniID {
		return
	}

	mask()
}

// Create 由管理员新增校友档案。
func (s *AlumniService) Create(ctx context.Context, operatorID uint64, req dto.AdminAlumniCreateRequest) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	profile := req.ToProfile().Normalize()
	if profile.Name == "" || profile.Grade == "" {
		return nil, common.ErrInvalidRequest
	}
	if profile.Status != common.AlumniStatusActive {
		return nil, common.ErrInvalidRequest
	}

	created, err := s.alumni.Create(ctx, &profile, operatorID)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("operator_id", operatorID), zap.Error(err))
		return nil, common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrInvalidRequest) {
		return nil, common.ErrInvalidRequest
	}
	if err != nil {
		logger.Error("failed to create alumni", zap.Uint64("operator_id", operatorID), zap.Error(err))
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
func (s *AlumniService) Update(ctx context.Context, operatorID uint64, id uint64, req dto.AdminAlumniUpdateRequest) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	profile := req.ToProfile().Normalize()
	if profile.Name == "" || profile.Grade == "" {
		return nil, common.ErrInvalidRequest
	}

	if err := s.alumni.Update(ctx, id, operatorID, profile); err != nil {
		if errors.Is(err, common.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Uint64("operator_id", operatorID), zap.Uint64("alumni_id", id), zap.Error(err))
			return nil, common.ErrDatabaseUnavailable
		}
		if errors.Is(err, common.ErrAlumniNotFound) {
			logger.Warn("alumni not found", zap.Uint64("alumni_id", id), zap.Uint64("operator_id", operatorID))
			return nil, common.ErrAlumniNotFound
		}
		logger.Error("failed to update alumni", zap.Uint64("operator_id", operatorID), zap.Uint64("alumni_id", id), zap.Error(err))
		return nil, err
	}

	updated, err := s.GetByID(ctx, id, operatorID)
	if err != nil {
		return nil, err
	}

	return updated, nil
}

// Delete 由管理员软删除校友档案。
func (s *AlumniService) Delete(ctx context.Context, operatorID uint64, id uint64) error {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return common.ErrDatabaseUnavailable
	}

	if err := s.alumni.Delete(ctx, id, operatorID); err != nil {
		if errors.Is(err, common.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Uint64("operator_id", operatorID), zap.Uint64("alumni_id", id), zap.Error(err))
			return common.ErrDatabaseUnavailable
		}
		if errors.Is(err, common.ErrAlumniNotFound) {
			logger.Warn("alumni not found", zap.Uint64("alumni_id", id), zap.Uint64("operator_id", operatorID))
			return common.ErrAlumniNotFound
		}
		logger.Error("failed to delete alumni", zap.Uint64("operator_id", operatorID), zap.Uint64("alumni_id", id), zap.Error(err))
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
func (s *AlumniService) Export(ctx context.Context, req dto.AlumniExportRequest) (*ExportResult, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	query := req.ToQuery().Normalize()
	format := req.FormatOrDefault()

	// 优先读缓存，避免全表扫描
	if s.exportCache != nil {
		if cached, err := s.exportCache.Get(ctx, query); err == nil {
			var items []*model.AlumniProfile
			if json.Unmarshal(cached, &items) == nil {
				switch format {
				case "csv":
					return buildCSV(items)
				default:
					return buildXLSX(items)
				}
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

	switch format {
	case "csv":
		return buildCSV(items)
	default:
		return buildXLSX(items)
	}
}

// ExportTemplate 生成导入模板 Excel 文件，包含表头行和一条示例空行。
func (s *AlumniService) ExportTemplate(ctx context.Context) (*ExportResult, error) {
	return buildTemplateXLSX()
}

func buildTemplateXLSX() (*ExportResult, error) {
	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, fmt.Errorf("create stream writer: %w", err)
	}

	// 写表头行
	headerRow := make([]interface{}, len(alumniColumnHeaders))
	for i, h := range alumniColumnHeaders {
		headerRow[i] = h
	}
	if err := sw.SetRow("A1", headerRow); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}

	// 写一条空行，提示用户按此结构填写
	emptyRow := make([]interface{}, len(alumniColumnHeaders))
	for i := range emptyRow {
		emptyRow[i] = ""
	}
	if err := sw.SetRow("A2", emptyRow); err != nil {
		return nil, fmt.Errorf("write empty row: %w", err)
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

	headerRow := make([]interface{}, len(alumniColumnHeaders))
	for i, h := range alumniColumnHeaders {
		headerRow[i] = h
	}
	if err := sw.SetRow("A1", headerRow); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}

	for i, item := range items {
		row := exportRow(item)
		vals := make([]interface{}, len(row))
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
func (s *AlumniService) Import(ctx context.Context, operatorID uint64, file io.Reader) (*dto.AlumniImportResult, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
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
	if len(header) != len(alumniColumnHeaders) {
		return nil, fmt.Errorf("表头列数不正确，期望 %d 列，实际 %d 列", len(alumniColumnHeaders), len(header))
	}
	for i, h := range header {
		if strings.TrimSpace(h) != alumniColumnHeaders[i] {
			return nil, fmt.Errorf("表头第 %d 列应为「%s」，实际为「%s」", i+1, alumniColumnHeaders[i], h)
		}
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

		if profile.Name == "" {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: profile.Name, Message: "姓名为空"})
			continue
		}
		if profile.Grade == "" {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: profile.Name, Message: "年级为空"})
			continue
		}

		validRows = append(validRows, rowProfile{rowNum: rowNum, profile: profile})
	}

	if len(validRows) > 0 {
		dedupKeys := make([]do.AlumniDedupKey, 0, len(validRows))
		for _, rp := range validRows {
			cn := ""
			if rp.profile.ClassName != nil {
				cn = *rp.profile.ClassName
			}
			ch := ""
			if rp.profile.Cohort != nil {
				ch = *rp.profile.Cohort
			}
			dedupKeys = append(dedupKeys, do.AlumniDedupKey{
				Name:      rp.profile.Name,
				Grade:     rp.profile.Grade,
				ClassName: cn,
				Cohort:    ch,
			})
		}

		existing, err := s.alumni.FindExistingByDedupKey(ctx, dedupKeys)
		if err != nil {
			logger.Error("failed to check duplicates", zap.Uint64("operator_id", operatorID), zap.Error(err))
			return nil, err
		}

		var dedupedProfiles []do.AlumniCreateProfile
		for _, rp := range validRows {
			cn := ""
			if rp.profile.ClassName != nil {
				cn = *rp.profile.ClassName
			}
			ch := ""
			if rp.profile.Cohort != nil {
				ch = *rp.profile.Cohort
			}
			if existing[do.AlumniDedupKey{Name: rp.profile.Name, Grade: rp.profile.Grade, ClassName: cn, Cohort: ch}.Key()] {
				rowErrors = append(rowErrors, dto.AlumniRowError{Row: rp.rowNum, Name: rp.profile.Name, Message: "已存在相同姓名、年级、班级和届数的记录"})
			} else {
				dedupedProfiles = append(dedupedProfiles, rp.profile)
				existing[do.AlumniDedupKey{Name: rp.profile.Name, Grade: rp.profile.Grade, ClassName: cn, Cohort: ch}.Key()] = true
			}
		}
		validProfiles := dedupedProfiles

		result := &dto.AlumniImportResult{
			Total:  len(rows) - 1,
			Errors: rowErrors,
		}

		if len(validProfiles) > 0 {
			if err := s.alumni.BatchCreate(ctx, validProfiles, operatorID); err != nil {
				logger.Error("failed to batch create alumni", zap.Uint64("operator_id", operatorID), zap.Error(err))
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
func (s *AlumniService) GetMe(ctx context.Context, userID uint64) (*dto.AlumniDetail, error) {
	alumniID, err := s.currentAlumniID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.GetByID(ctx, alumniID, userID)
}

// UpdateMe 更新当前登录校友本人允许维护的字段，并返回更新后的资料。
func (s *AlumniService) UpdateMe(ctx context.Context, userID uint64, req dto.AlumniProfileUpdateRequest) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	alumniID, err := s.currentAlumniID(ctx, userID)
	if err != nil {
		return nil, err
	}

	profile := req.ToProfile().Normalize()
	if !profile.IsEmpty() {
		if err := s.alumni.UpdateEditableFields(ctx, alumniID, userID, profile); err != nil {
			if errors.Is(err, common.ErrDatabaseUnavailable) {
				logger.Error("database is unavailable", zap.Uint64("alumni_id", alumniID), zap.Uint64("user_id", userID), zap.Error(err))
				return nil, common.ErrDatabaseUnavailable
			}
			if errors.Is(err, common.ErrAlumniNotFound) {
				logger.Warn("alumni not found", zap.Uint64("alumni_id", alumniID), zap.Uint64("user_id", userID))
				return nil, common.ErrAlumniNotFound
			}
			logger.Error("failed to update alumni profile", zap.Uint64("alumni_id", alumniID), zap.Uint64("user_id", userID), zap.Error(err))
			return nil, err
		}
	}

	return s.GetByID(ctx, alumniID, userID)
}

// currentAlumniID 获取当前用户绑定的校友 ID。如果用户不存在、不是校友、或未绑定校友资料，返回相应错误。
func (s *AlumniService) currentAlumniID(ctx context.Context, userID uint64) (uint64, error) {
	if s.users == nil {
		logger.Error("user repository is not initialized")
		return 0, common.ErrDatabaseUnavailable
	}

	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("user_id", userID), zap.Error(err))
		return 0, common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrUserNotFound) {
		logger.Warn("current user not found", zap.Uint64("user_id", userID))
		return 0, common.ErrUserNotFound
	}
	if err != nil {
		logger.Error("failed to find current user", zap.Uint64("user_id", userID), zap.Error(err))
		return 0, err
	}
	if user.Status != common.UserStatusActive {
		logger.Warn("current user account is disabled", zap.Uint64("user_id", userID), zap.String("status", user.Status))
		return 0, common.ErrAccountDisabled
	}
	if user.Role != common.RoleAlumni {
		logger.Warn("current user is not alumni", zap.Uint64("user_id", userID), zap.String("role", user.Role))
		return 0, common.ErrPermissionDenied
	}
	if user.AlumniID == nil || *user.AlumniID == 0 {
		logger.Warn("current user has no bound alumni profile", zap.Uint64("user_id", userID))
		return 0, common.ErrAlumniProfileUnbound
	}

	return *user.AlumniID, nil
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
