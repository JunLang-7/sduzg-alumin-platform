package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

// maxImportDataRows 是单次导入允许的数据行上限，不含表头。
const maxImportDataRows = 20000

// maxAuditBatchRecords 限制单条导入审计保存的明细条数，给 detail 体积设上限；
// 超出部分只保留真实总数，并在详情中标明已截断。
const maxAuditBatchRecords = 200

type importRowLayout int

const (
	importLayoutCurrent importRowLayout = iota
	importLayoutLegacyWithDomain
	importLayoutLegacy
)

type parsedImportRow struct {
	rowNum     int
	personal   do.AlumniPersonalInput
	enrollment do.AlumniEnrollmentInput
}

type importPersonGroup struct {
	key         string
	personal    do.AlumniPersonalInput
	enrollments []do.AlumniEnrollmentInput
	firstRow    int
}

// Import 从上传的 xlsx/csv 文件批量导入校友档案。
// 一行代表一段学籍；同一姓名与手机号/邮箱的多行合并为同一校友的多段学籍，
// 但不会凭姓名或手机号挂接到数据库中已有的校友身份。
func (s *AlumniService) Import(ctx context.Context, operator common.AccessContext, dataDomainID *uint64, file io.Reader) (*dto.AlumniImportResult, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}

	targetProfile := do.AlumniEnrollmentInput{DataDomainID: 0}
	autoTargetDomain := false
	if dataDomainID != nil {
		targetProfile.DataDomainID = *dataDomainID
	}
	if operator.Role == common.RoleAdmin {
		if len(operator.DomainIDs) == 0 {
			return nil, common.ErrPermissionDenied
		}
		if len(operator.DomainIDs) == 1 {
			if targetProfile.DataDomainID != 0 && !operator.CanAccessDomain(targetProfile.DataDomainID) {
				return nil, common.ErrPermissionDenied
			}
			if targetProfile.DataDomainID == 0 {
				autoTargetDomain = true
				targetProfile.DataDomainID = operator.DomainIDs[0]
			}
		} else if targetProfile.DataDomainID != 0 && !operator.CanAccessDomain(targetProfile.DataDomainID) {
			return nil, common.ErrPermissionDenied
		}
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read uploaded file: %w", err)
	}
	rows, err := readImportRows(data)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, common.ErrInvalidRequest
	}

	layout, hasDomainColumn, ok := detectImportLayout(rows[0])
	if !ok {
		return nil, common.ErrImportInvalidHeader
	}

	if targetProfile.DataDomainID == 0 && !hasDomainColumn {
		return nil, common.ErrImportDomainRequired
	}

	domainByCode := map[string]*model.DataDomain{}
	domainByName := map[string]*model.DataDomain{}
	domainByID := map[uint64]*model.DataDomain{}
	domainOptions := ""
	var domainLookup func(raw string) *model.DataDomain
	needDomainLookup := hasDomainColumn || targetProfile.DataDomainID == 0 || layout != importLayoutCurrent
	if needDomainLookup {
		if s.dataDomains == nil {
			return nil, common.ErrDatabaseUnavailable
		}
		domains, err := s.dataDomains.ListDataDomains(ctx)
		if err != nil {
			return nil, err
		}
		activeDomains := make([]*model.DataDomain, 0, len(domains))
		for _, domain := range domains {
			if domain == nil {
				continue
			}
			domainByCode[strings.ToLower(domain.Code)] = domain
			domainByName[strings.ToLower(domain.Name)] = domain
			domainByID[domain.ID] = domain
			if domain.Status == common.DataDomainStatusActive {
				activeDomains = append(activeDomains, domain)
			}
		}
		domainOptions = importDomainOptions(activeDomains)
		domainLookup = func(raw string) *model.DataDomain {
			key := strings.ToLower(strings.TrimSpace(raw))
			if domain := domainByCode[key]; domain != nil {
				return domain
			}
			if domain := domainByName[key]; domain != nil {
				return domain
			}
			if id, err := strconv.ParseUint(key, 10, 64); err == nil {
				return domainByID[id]
			}
			return nil
		}
	}
	var parsedRows []parsedImportRow
	rowErrors := make([]dto.AlumniRowError, 0)
	dataRowCount := 0

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1
		if isBlankImportRow(row) {
			continue
		}
		dataRowCount++

		parsed, parseErr := parseImportRow(row, layout)
		parsed.rowNum = rowNum
		if parseErr != nil {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: parseErr.Error()})
			continue
		}

		// 学籍数据域：单域管理员留空时使用唯一授权域；显式填写其他域按行报错。
		domainID := targetProfile.DataDomainID
		if hasDomainColumn && targetProfile.DataDomainID == 0 {
			rawDomain := strings.TrimSpace(cellValue(row, domainColumnIndex(layout)))
			if rawDomain == "" {
				rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: "所属领域为空"})
				continue
			}
			domain := domainLookup(rawDomain)
			if domain == nil {
				rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: "所属领域无效，可选值：" + domainOptions})
				continue
			}
			if domain.Status != common.DataDomainStatusActive {
				rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: "所属领域已停用，不能导入"})
				continue
			}
			if !operator.IsSuperAdmin() && !operator.CanAccessDomain(domain.ID) {
				rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: "无权导入该所属领域"})
				continue
			}
			domainID = domain.ID
		} else if hasDomainColumn && autoTargetDomain {
			// 单域管理员自动落到唯一授权域时，显式填写其他仍应报错，避免跨域档案被静默改写。
			rawDomain := strings.TrimSpace(cellValue(row, domainColumnIndex(layout)))
			if rawDomain != "" {
				domain := domainLookup(rawDomain)
				if domain == nil {
					rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: "所属领域无效，可选值：" + domainOptions})
					continue
				}
				if domain.Status != common.DataDomainStatusActive {
					rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: "所属领域已停用，不能导入"})
					continue
				}
				if domain.ID != targetProfile.DataDomainID {
					rowErrors = append(rowErrors, dto.AlumniRowError{
						Row:     rowNum,
						Name:    parsed.personal.Name,
						Message: fmt.Sprintf("该行所属领域为 %s，与本次导入目标培养类别不一致：请改用多域账号导入，或选择正确的导入目标培养类别", domain.Code),
					})
					continue
				}
			}
		}
		// 显式指定 data_domain_id 时，文件中“所属领域”列被忽略。
		if domainID == 0 {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: "所属领域为空"})
			continue
		}
		parsed.enrollment.DataDomainID = domainID
		if layout != importLayoutCurrent && parsed.enrollment.TrainingLevel == "" {
			code := ""
			if domain := domainByID[domainID]; domain != nil {
				code = domain.Code
			}
			parsed.enrollment.TrainingLevel = resolveLegacyTrainingLevel(code)
			if parsed.enrollment.TrainingLevel == "" {
				rowErrors = append(rowErrors, dto.AlumniRowError{
					Row:     rowNum,
					Name:    parsed.personal.Name,
					Message: "历史模板无法确定培养层次：请使用新版模板填写 本科 / 硕士 / 博士",
				})
				continue
			}
		}

		if parsed.personal.Name == "" {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: "", Message: "姓名为空"})
			continue
		}
		if parsed.enrollment.Grade == "" {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: "年级为空"})
			continue
		}
		if !validExplicitTrainingLevel(parsed.enrollment.TrainingLevel) {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: "培养层次无效或缺失：请填写 本科 / 硕士 / 博士"})
			continue
		}
		if !operator.HasPermission(common.PermissionAlumniSensitiveRead) && personalContainsSensitiveFields(parsed.personal) {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: rowNum, Name: parsed.personal.Name, Message: "无权导入敏感字段"})
			continue
		}
		parsedRows = append(parsedRows, parsed)
	}

	if dataRowCount == 0 {
		return nil, common.ErrImportNoDataRows
	}

	// 数据库去重：只按完整学籍组合判重，不凭姓名/手机号自动挂接已有校友。
	dedupKeys := make([]do.AlumniDedupKey, 0, len(parsedRows))
	for _, parsed := range parsedRows {
		dedupKeys = append(dedupKeys, alumniImportDedupKey(parsed))
	}
	existing, err := s.alumni.FindExistingByDedupKey(ctx, dedupKeys)
	if err != nil {
		logger.Error("failed to check duplicates", zap.Uint64("operator_id", operator.UserID), zap.Error(err))
		return nil, err
	}

	groups := make(map[string]*importPersonGroup)
	groupOrder := make([]string, 0)
	selected := make([]parsedImportRow, 0, len(parsedRows))
	for _, parsed := range parsedRows {
		key := alumniImportDedupKey(parsed).Key()
		if existing[key] {
			rowErrors = append(rowErrors, dto.AlumniRowError{Row: parsed.rowNum, Name: parsed.personal.Name, Message: "已存在相同姓名、手机号、培养类别、年级、班级和届数的学籍记录"})
			continue
		}
		existing[key] = true
		selected = append(selected, parsed)

		groupKey := importPersonGroupKey(parsed)
		group := groups[groupKey]
		if group == nil {
			group = &importPersonGroup{key: groupKey, personal: parsed.personal, firstRow: parsed.rowNum}
			groups[groupKey] = group
			groupOrder = append(groupOrder, groupKey)
		}
		group.enrollments = append(group.enrollments, parsed.enrollment)
	}

	inputs := make([]do.AlumniCreateInput, 0, len(groupOrder))
	for _, key := range groupOrder {
		group := groups[key]
		if group == nil || len(group.enrollments) == 0 {
			continue
		}
		inputs = append(inputs, do.AlumniCreateInput{
			Personal:    group.personal,
			Enrollments: group.enrollments,
		})
	}

	result := &dto.AlumniImportResult{Total: dataRowCount, Errors: rowErrors}
	if len(inputs) == 0 {
		s.writeImportAudit(ctx, operator, nil, nil, result)
		return result, nil
	}

	createdIDs, err := s.alumni.BatchCreate(ctx, inputs, operator.UserID)
	if err != nil {
		logger.Error("failed to batch create alumni", zap.Uint64("operator_id", operator.UserID), zap.Error(err))
		return nil, err
	}
	for _, input := range inputs {
		result.Success += len(input.Enrollments)
	}
	if s.countCache != nil {
		_ = s.countCache.IncrBy(ctx, int64(len(inputs)))
	}
	if s.exportCache != nil {
		_ = s.exportCache.Invalidate(ctx)
	}
	s.writeImportAudit(ctx, operator, inputs, createdIDs, result)
	return result, nil
}

// detectImportLayout 识别当前模板、16 列历史模板和 15 列历史模板。
func detectImportLayout(header []string) (importRowLayout, bool, bool) {
	switch {
	case matchesImportHeaders(header, alumniImportColumnHeaders):
		return importLayoutCurrent, true, true
	case matchesImportHeaders(header, legacyDomainColumnHeaders):
		return importLayoutLegacyWithDomain, true, true
	case matchesImportHeaders(header, legacyAlumniColumnHeaders):
		return importLayoutLegacy, false, true
	default:
		return importLayoutCurrent, false, false
	}
}

func domainColumnIndex(layout importRowLayout) int {
	switch layout {
	case importLayoutLegacy:
		return -1
	case importLayoutLegacyWithDomain:
		return len(legacyDomainColumnHeaders) - 1
	default:
		return len(alumniImportColumnHeaders) - 1
	}
}

// parseImportRow 解析一行；当前模板按新列顺序，历史模板按旧列顺序。
func parseImportRow(row []string, layout importRowLayout) (parsedImportRow, error) {
	get := func(idx int) string {
		if idx < 0 {
			return ""
		}
		return stripExportFormulaGuard(strings.TrimSpace(cellValue(row, idx)))
	}
	optional := func(idx int) *string {
		value := get(idx)
		if value == "" {
			return nil
		}
		return &value
	}

	result := parsedImportRow{}
	if layout == importLayoutCurrent {
		result.personal.Name = get(0)
		result.personal.Industry = optional(9)
		result.personal.WorkUnit = optional(10)
		result.personal.Position = optional(11)
		result.personal.MailingAddress = optional(12)
		result.personal.Gender = optional(13)
		result.personal.Mobile = optional(14)
		result.personal.Email = optional(15)

		result.enrollment.TrainingLevel = parseTrainingLevel(get(1))
		result.enrollment.Grade = get(2)
		result.enrollment.ClassName = optional(3)
		result.enrollment.Cohort = optional(4)
		result.enrollment.Counselor = optional(5)
		result.enrollment.Mentor = optional(6)
		result.enrollment.Major = optional(7)
		result.enrollment.TrainingMode = optional(8)
		return result, nil
	}

	result.personal.Name = get(0)
	result.enrollment.Grade = get(1)
	result.enrollment.ClassName = optional(2)
	result.enrollment.Cohort = optional(3)
	result.enrollment.Counselor = optional(4)
	result.enrollment.Mentor = optional(5)
	result.enrollment.Major = optional(6)
	result.enrollment.TrainingMode = optional(7)
	result.personal.Industry = optional(8)
	result.personal.WorkUnit = optional(9)
	result.personal.Position = optional(10)
	result.personal.MailingAddress = optional(11)
	result.personal.Gender = optional(12)
	result.personal.Mobile = optional(13)
	result.personal.Email = optional(14)
	// 历史模板没有培养层次列，由数据域推断；学术学位研究生无法区分硕/博，
	// 导入时要求改用新模板明确填写，不能猜测。
	// 具体数据域在行级解析后才知道，见 resolveLegacyTrainingLevel。
	return result, nil
}

func parseTrainingLevel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "本科", "undergraduate", "bachelor":
		return common.TrainingLevelUndergraduate
	case "硕士", "master":
		return common.TrainingLevelMaster
	case "博士", "doctor", "phd":
		return common.TrainingLevelDoctor
	case "待核验", "unverified":
		return common.TrainingLevelUnverified
	default:
		return ""
	}
}

func resolveLegacyTrainingLevel(domainCode string) string {
	switch domainCode {
	case "undergraduate":
		return common.TrainingLevelUndergraduate
	case "mpa":
		return common.TrainingLevelMaster
	default:
		return ""
	}
}

// readImportRows 兼容 xlsx 与 csv：ZIP 魔数走工作簿解析，其余按 UTF-8 CSV 解析。
func readImportRows(data []byte) ([][]string, error) {
	if len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04 {
		return readImportWorkbookRows(data)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	rows := make([][]string, 0, 64)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, common.ErrInvalidRequest
		}
		rows = append(rows, record)
		if len(rows) > maxImportDataRows+1 {
			return nil, common.ErrImportRowLimit
		}
	}
	if len(rows) == 0 {
		return nil, common.ErrInvalidRequest
	}
	return rows, nil
}

// stripExportFormulaGuard 还原导出时为防止电子表格公式注入而加的单引号前缀。
func stripExportFormulaGuard(value string) string {
	if len(value) < 2 || value[0] != '\'' {
		return value
	}
	if strings.ContainsRune("=+-@", rune(value[1])) || value[1] == '\'' {
		return value[1:]
	}
	return value
}

// readImportWorkbookRows 读取工作簿，并在累计行数超过上限时立即停止。
func readImportWorkbookRows(data []byte) (rows [][]string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = common.ErrInvalidRequest
		}
	}()

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, common.ErrInvalidRequest
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, common.ErrInvalidRequest
	}
	iterator, err := f.Rows(sheetName)
	if err != nil {
		return nil, common.ErrInvalidRequest
	}
	defer iterator.Close()

	for iterator.Next() {
		row, columnErr := iterator.Columns()
		if columnErr != nil {
			return nil, common.ErrInvalidRequest
		}
		rows = append(rows, row)
		if len(rows) > maxImportDataRows+1 {
			return nil, common.ErrImportRowLimit
		}
	}
	if err := iterator.Error(); err != nil {
		return nil, common.ErrInvalidRequest
	}
	return rows, nil
}

// isBlankImportRow 判断整行是否为空，用于跳过模板空行与用户文件末尾的空白行。
func isBlankImportRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func importDomainOptions(domains []*model.DataDomain) string {
	parts := make([]string, 0, len(domains))
	for _, domain := range domains {
		if domain == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s（%s）", domain.Code, domain.Name))
	}
	return strings.Join(parts, "、")
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

func alumniImportDedupKey(parsed parsedImportRow) do.AlumniDedupKey {
	return do.AlumniDedupKey{
		Name:         parsed.personal.Name,
		Mobile:       strings.TrimSpace(stringOrEmpty(parsed.personal.Mobile)),
		DataDomainID: parsed.enrollment.DataDomainID,
		Grade:        parsed.enrollment.Grade,
		ClassName:    stringOrEmpty(parsed.enrollment.ClassName),
		Cohort:       stringOrEmpty(parsed.enrollment.Cohort),
	}
}

// importPersonGroupKey 在同一文件内合并同一校友的多段学籍。
// 只有姓名与手机号（或邮箱）同时存在时才合并，避免仅凭姓名误并不同人员。
func importPersonGroupKey(parsed parsedImportRow) string {
	name := strings.TrimSpace(parsed.personal.Name)
	if mobile := strings.TrimSpace(stringOrEmpty(parsed.personal.Mobile)); mobile != "" {
		return name + "|mobile|" + mobile
	}
	if email := strings.TrimSpace(strings.ToLower(stringOrEmpty(parsed.personal.Email))); email != "" {
		return name + "|email|" + email
	}
	return name + "|row|" + strconv.Itoa(parsed.rowNum)
}

type importAuditDetail struct {
	SchemaVersion               int                `json:"schema_version"`
	TargetName                  string             `json:"target_name,omitempty"`
	TargetMeta                  string             `json:"target_meta,omitempty"`
	ManagementScope             string             `json:"management_scope,omitempty"`
	Source                      string             `json:"source,omitempty"`
	Reason                      string             `json:"reason,omitempty"`
	Status                      string             `json:"status,omitempty"`
	DataDomainIDs               []uint64           `json:"data_domain_ids,omitempty"`
	Total                       int                `json:"total"`
	Success                     int                `json:"success"`
	Failed                      int                `json:"failed"`
	SensitiveFieldsIncluded     bool               `json:"sensitive_fields_included"`
	BatchImportFields           []auditBatchField  `json:"batch_import_fields,omitempty"`
	BatchCreatedAlumni          []auditBatchRecord `json:"batch_created_alumni,omitempty"`
	BatchCreatedAlumniTotal     int                `json:"batch_created_alumni_total,omitempty"`
	BatchCreatedAlumniTruncated bool               `json:"batch_created_alumni_truncated,omitempty"`
	BatchHiddenSensitiveCount   int                `json:"batch_hidden_sensitive_count,omitempty"`
}

// writeImportAudit 记录导入结果和可跳转的公开字段，不写入任何校友敏感字段原值。
func (s *AlumniService) writeImportAudit(ctx context.Context, operator common.AccessContext, inputs []do.AlumniCreateInput, createdIDs []uint64, result *dto.AlumniImportResult) {
	if s.opLogger == nil || result == nil {
		return
	}
	domainSet := make(map[uint64]struct{})
	sensitiveFieldsIncluded := false
	for _, input := range inputs {
		if personalContainsSensitiveFields(input.Personal) {
			sensitiveFieldsIncluded = true
		}
		for _, enrollment := range input.Enrollments {
			domainSet[enrollment.DataDomainID] = struct{}{}
		}
	}
	domainIDs := make([]uint64, 0, len(domainSet))
	for domainID := range domainSet {
		domainIDs = append(domainIDs, domainID)
	}
	sort.Slice(domainIDs, func(i, j int) bool { return domainIDs[i] < domainIDs[j] })

	batchRecords := make([]auditBatchRecord, 0, len(inputs))
	for i, input := range inputs {
		if i >= maxAuditBatchRecords {
			break
		}
		var id uint64
		if i < len(createdIDs) {
			id = createdIDs[i]
		}
		if id == 0 {
			continue
		}
		values := map[string]string{"name": input.Personal.Name}
		meta := ""
		if len(input.Enrollments) > 0 {
			enrollment := input.Enrollments[0]
			values["grade"] = enrollment.Grade
			values["training_level"] = TrainingLevelLabel(enrollment.TrainingLevel)
			values["class_name"] = stringOrEmpty(enrollment.ClassName)
			values["major"] = stringOrEmpty(enrollment.Major)
			values["training_mode"] = stringOrEmpty(enrollment.TrainingMode)
			meta = enrollment.Grade
			if name := stringOrEmpty(enrollment.Major); name != "" {
				meta += " · " + name
			}
		}
		batchRecords = append(batchRecords, auditBatchRecord{
			ID:          id,
			Name:        input.Personal.Name,
			TargetMeta:  meta,
			FieldValues: values,
		})
	}

	detail, err := json.Marshal(importAuditDetail{
		SchemaVersion:               1,
		DataDomainIDs:               domainIDs,
		Total:                       result.Total,
		Success:                     result.Success,
		Failed:                      result.Total - result.Success,
		SensitiveFieldsIncluded:     sensitiveFieldsIncluded,
		TargetName:                  "校友档案批量导入",
		TargetMeta:                  fmt.Sprintf("%d 条校友档案", result.Success),
		ManagementScope:             s.importManagementScope(ctx, domainIDs),
		Source:                      "admin_import",
		Reason:                      "导入校友档案",
		Status:                      "applied",
		BatchImportFields:           auditBatchFields(),
		BatchCreatedAlumni:          batchRecords,
		BatchCreatedAlumniTotal:     len(createdIDs),
		BatchCreatedAlumniTruncated: len(createdIDs) > len(batchRecords),
		BatchHiddenSensitiveCount:   len(auditBatchSensitiveFields),
	})
	if err != nil {
		return
	}
	detailText := string(detail)
	_ = s.opLogger.Write(ctx, &model.OperationLog{
		OperatorID:   operator.UserID,
		OperatorRole: operator.Role,
		Action:       AuditActionImport,
		TargetType:   auditTargetBatch,
		Detail:       &detailText,
	})
}

func (s *AlumniService) importManagementScope(ctx context.Context, domainIDs []uint64) string {
	if len(domainIDs) == 0 || s.dataDomains == nil {
		return defaultAuditScope
	}
	domains, err := s.dataDomains.ListActiveDataDomains(ctx)
	if err != nil {
		return defaultAuditScope
	}
	wanted := make(map[uint64]struct{}, len(domainIDs))
	for _, id := range domainIDs {
		wanted[id] = struct{}{}
	}
	names := make([]string, 0, len(domainIDs))
	for _, domain := range domains {
		if domain == nil {
			continue
		}
		if _, ok := wanted[domain.ID]; ok {
			names = append(names, domain.Name)
		}
	}
	if len(names) == 0 {
		return defaultAuditScope
	}
	return strings.Join(names, "、")
}
