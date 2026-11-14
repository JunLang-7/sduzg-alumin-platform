package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

// alumniExportColumnHeaders 是当前导出/导入模板列：一行一段学籍，个人信息重复填充。
var alumniExportColumnHeaders = []string{
	"姓名", "培养层次", "年级", "班级", "届数", "辅导员", "导师", "专业",
	"培养方式", "行业", "工作单位", "职务", "通讯地址", "性别", "手机号", "邮箱", "所属领域",
}

// legacyAlumniColumnHeaders 是不含所属领域的 15 列历史模板顺序，仍可导入。
var legacyAlumniColumnHeaders = []string{
	"姓名", "年级", "班级", "届数", "辅导员", "导师", "专业", "培养方式",
	"行业", "工作单位", "职务", "通讯地址", "性别", "手机号", "邮箱",
}

// legacyDomainColumnHeaders 是 16 列历史模板（15 列 + 所属领域）。
var legacyDomainColumnHeaders = append(append([]string{}, legacyAlumniColumnHeaders...), "所属领域")

// alumniImportColumnHeaders 保持与当前导出列一致，导出文件可直接回灌。
var alumniImportColumnHeaders = append([]string{}, alumniExportColumnHeaders...)

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
	// 值本身以单引号开头时再补一层，配合导入侧只剥一层，保证回灌无损。
	if v[0] == '\'' {
		return "'" + v
	}
	return v
}

func exportRow(row do.AlumniExportRow, domainCode string) []string {
	profile := row.Profile
	enrollment := row.Enrollment
	if profile == nil || enrollment == nil {
		return nil
	}
	return []string{
		sanitizeExportValue(profile.Name),
		sanitizeExportValue(TrainingLevelLabel(enrollment.TrainingLevel)),
		sanitizeExportValue(enrollment.Grade),
		sanitizeExportValue(stringOrEmpty(enrollment.ClassName)),
		sanitizeExportValue(stringOrEmpty(enrollment.Cohort)),
		sanitizeExportValue(stringOrEmpty(enrollment.Counselor)),
		sanitizeExportValue(stringOrEmpty(enrollment.Mentor)),
		sanitizeExportValue(stringOrEmpty(enrollment.Major)),
		sanitizeExportValue(stringOrEmpty(enrollment.TrainingMode)),
		sanitizeExportValue(stringOrEmpty(profile.Industry)),
		sanitizeExportValue(stringOrEmpty(profile.WorkUnit)),
		sanitizeExportValue(stringOrEmpty(profile.Position)),
		sanitizeExportValue(stringOrEmpty(profile.MailingAddress)),
		sanitizeExportValue(stringOrEmpty(profile.Gender)),
		sanitizeExportValue(stringOrEmpty(profile.Mobile)),
		sanitizeExportValue(stringOrEmpty(profile.Email)),
		sanitizeExportValue(domainCode),
	}
}

// Export 导出当前筛选结果。平铺时一行一段有权限且符合筛选的学籍。
func (s *AlumniService) Export(ctx context.Context, req dto.AlumniExportRequest, operator common.AccessContext) (*ExportResult, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}

	query := req.ToQuery().Normalize()
	if !operator.HasPermission(common.PermissionAlumniSensitiveRead) &&
		(query.WorkUnit != "" || query.Position != "" || query.Mobile != "") {
		return nil, common.ErrPermissionDenied
	}
	query, err := s.scopeListQuery(query, operator)
	if err != nil {
		return nil, err
	}
	query.CanReadSensitive = operator.HasPermission(common.PermissionAlumniSensitiveRead)
	format := req.FormatOrDefault()

	// 优先读缓存，避免全表扫描
	if s.exportCache != nil {
		if cached, err := s.exportCache.Get(ctx, query); err == nil {
			var rows []do.AlumniExportRow
			if json.Unmarshal(cached, &rows) == nil {
				return s.buildAndAuditExport(ctx, operator, query, rows, format)
			}
		}
	}

	rows, err := s.alumni.ListAll(ctx, query)
	if err != nil {
		if errors.Is(err, common.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Error(err))
			return nil, common.ErrDatabaseUnavailable
		}
		logger.Error("failed to list alumni for export", zap.Error(err))
		return nil, err
	}

	if s.exportCache != nil {
		if data, err := json.Marshal(rows); err == nil {
			_ = s.exportCache.Set(ctx, query, data)
		}
	}

	return s.buildAndAuditExport(ctx, operator, query, rows, format)
}

type exportAuditDetail struct {
	Format                  string   `json:"format"`
	DataDomainIDs           []uint64 `json:"data_domain_ids,omitempty"`
	RecordCount             int      `json:"record_count"`
	SensitiveFieldsIncluded bool     `json:"sensitive_fields_included"`
}

func (s *AlumniService) buildAndAuditExport(ctx context.Context, operator common.AccessContext, query do.AlumniListQuery, rows []do.AlumniExportRow, format string) (*ExportResult, error) {
	domainCodes, err := s.dataDomainCodeMap(ctx)
	if err != nil {
		logger.Error("failed to load data domains for export", zap.Error(err))
		return nil, err
	}
	missingCodeRows := 0
	nonReimportableRows := 0
	for _, row := range rows {
		if row.Profile == nil || row.Enrollment == nil {
			continue
		}
		if domainCodes[row.Enrollment.DataDomainID] == "" {
			missingCodeRows++
		}
		if strings.TrimSpace(row.Profile.Name) == "" || strings.TrimSpace(row.Enrollment.Grade) == "" {
			nonReimportableRows++
		}
	}
	if missingCodeRows > 0 {
		logger.Warn("export rows missing data domain code", zap.Int("rows", missingCodeRows))
	}
	if nonReimportableRows > 0 {
		logger.Warn("export contains rows that cannot be re-imported", zap.Int("rows", nonReimportableRows))
	}
	result, err := s.buildExport(maskExportRows(rows, operator), format, domainCodes)
	if err != nil {
		return nil, err
	}
	if s.opLogger != nil {
		detail, err := json.Marshal(exportAuditDetail{
			Format:                  format,
			DataDomainIDs:           query.DataDomainIDs,
			RecordCount:             len(rows),
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

func (s *AlumniService) buildExport(rows []do.AlumniExportRow, format string, domainCodes map[uint64]string) (*ExportResult, error) {
	switch format {
	case "csv":
		return buildCSV(rows, domainCodes)
	default:
		return buildXLSX(rows, domainCodes)
	}
}

// dataDomainCodeMap 返回数据域 ID 到编码的映射；包含已停用数据域，保证历史档案也能导出编码。
func (s *AlumniService) dataDomainCodeMap(ctx context.Context) (map[uint64]string, error) {
	codes := make(map[uint64]string)
	if s.dataDomains == nil {
		return codes, nil
	}
	domains, err := s.dataDomains.ListDataDomains(ctx)
	if err != nil {
		return nil, err
	}
	for _, domain := range domains {
		if domain != nil {
			codes[domain.ID] = domain.Code
		}
	}
	return codes, nil
}

// maskExportRows 为无敏感字段权限的导出请求清空受保护字段，且不修改缓存或仓储返回对象。
func maskExportRows(rows []do.AlumniExportRow, operator common.AccessContext) []do.AlumniExportRow {
	if operator.HasPermission(common.PermissionAlumniSensitiveRead) {
		return rows
	}
	masked := make([]do.AlumniExportRow, 0, len(rows))
	for _, row := range rows {
		if row.Profile == nil {
			continue
		}
		copyProfile := *row.Profile
		copyProfile.Mobile = nil
		copyProfile.Email = nil
		copyProfile.WorkUnit = nil
		copyProfile.Position = nil
		copyProfile.MailingAddress = nil
		copied := row
		copied.Profile = &copyProfile
		masked = append(masked, copied)
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

	headerRow := make([]any, len(alumniImportColumnHeaders))
	for i, h := range alumniImportColumnHeaders {
		headerRow[i] = h
	}
	if err := f.SetSheetRow("Sheet1", "A1", &headerRow); err != nil {
		logger.Error("build template: write header", zap.Error(err))
		return nil, fmt.Errorf("write header: %w", err)
	}

	// 数据表只写表头 + 一条空白行；示例与填写规则放到「填写说明」，避免示例数据被误导入。
	emptyRow := make([]any, len(alumniImportColumnHeaders))
	for i := range emptyRow {
		emptyRow[i] = ""
	}
	if err := f.SetSheetRow("Sheet1", "A2", &emptyRow); err != nil {
		logger.Error("build template: write empty row", zap.Error(err))
		return nil, fmt.Errorf("write empty row: %w", err)
	}

	const noteSheet = "填写说明"
	if _, err := f.NewSheet(noteSheet); err != nil {
		logger.Error("build template: create note sheet", zap.Error(err))
		return nil, fmt.Errorf("create note sheet: %w", err)
	}
	notes := [][]any{
		{"所属领域与培养层次填写规则"},
		{"所属领域编码", "显示名称"},
		{"undergraduate", "本科生"},
		{"academic_graduate", "学术学位研究生"},
		{"mpa", "MPA专业学位研究生"},
		{"培养层次可选值：本科 / 硕士 / 博士。"},
		{"所属领域可填写编码（推荐）、显示名称或数字 ID；单域管理员留空时按唯一培养类别处理；显式选择导入目标培养类别时本列被忽略。"},
		{"每个有效行独立创建个人档案及一段初始学籍，不凭姓名、手机号或邮箱自动归并。为同一校友添加学籍，请核验身份后编辑既有校友，不要用多行导入来关联身份。"},
		{"示例：姓名「示例校友」、培养层次「硕士」、年级「2024」、所属领域「mpa」。"},
	}
	for i, row := range notes {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(noteSheet, cell, &row); err != nil {
			logger.Error("build template: write note row", zap.Error(err))
			return nil, fmt.Errorf("write note row: %w", err)
		}
	}
	f.SetActiveSheet(0)

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

func buildXLSX(rows []do.AlumniExportRow, domainCodes map[uint64]string) (*ExportResult, error) {
	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, fmt.Errorf("create stream writer: %w", err)
	}

	headerRow := make([]any, len(alumniExportColumnHeaders))
	for i, h := range alumniExportColumnHeaders {
		headerRow[i] = h
	}
	if err := sw.SetRow("A1", headerRow); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}

	rowIndex := 2
	for _, item := range rows {
		row := exportRow(item, domainCodes[rowDomainID(item)])
		if row == nil {
			continue
		}
		vals := make([]any, len(row))
		for j, v := range row {
			vals[j] = v
		}
		cell, _ := excelize.CoordinatesToCellName(1, rowIndex)
		if err := sw.SetRow(cell, vals); err != nil {
			return nil, fmt.Errorf("write row %d: %w", rowIndex, err)
		}
		rowIndex++
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

func rowDomainID(row do.AlumniExportRow) uint64 {
	if row.Enrollment == nil {
		return 0
	}
	return row.Enrollment.DataDomainID
}

func buildCSV(rows []do.AlumniExportRow, domainCodes map[uint64]string) (*ExportResult, error) {
	var buf bytes.Buffer

	// UTF-8 BOM
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(&buf)
	if err := w.Write(alumniExportColumnHeaders); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}
	for _, item := range rows {
		row := exportRow(item, domainCodes[rowDomainID(item)])
		if row == nil {
			continue
		}
		if err := w.Write(row); err != nil {
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
