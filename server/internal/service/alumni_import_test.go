package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/xuri/excelize/v2"
)

func buildXLSXReader(headers []string, rows [][]string) (*bytes.Reader, error) {
	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, err
	}

	headerVals := make([]any, len(headers))
	for i, h := range headers {
		headerVals[i] = h
	}
	if err := sw.SetRow("A1", headerVals); err != nil {
		return nil, err
	}

	for i, row := range rows {
		vals := make([]any, len(row))
		for j, v := range row {
			vals[j] = v
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := sw.SetRow(cell, vals); err != nil {
			return nil, err
		}
	}

	if err := sw.Flush(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}

	return bytes.NewReader(buf.Bytes()), nil
}

func importAsSuperAdmin(svc *AlumniService, reader *bytes.Reader) (*dto.AlumniImportResult, error) {
	domainID := uint64(1)
	return svc.Import(context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, &domainID, reader)
}

type fakeImportDataDomainStore struct {
	domains []*model.DataDomain
}

func (s *fakeImportDataDomainStore) ListActiveDataDomains(context.Context) ([]*model.DataDomain, error) {
	return s.domains, nil
}

func (s *fakeImportDataDomainStore) ListDataDomains(context.Context) ([]*model.DataDomain, error) {
	return s.domains, nil
}

func (s *fakeImportDataDomainStore) ListAdminDataDomainIDs(context.Context, uint64) ([]uint64, error) {
	return nil, nil
}

func (s *fakeImportDataDomainStore) ListAdminPermissionCodes(context.Context, uint64) ([]string, error) {
	return nil, nil
}

// currentRow 构造当前 17 列模板的一行。
func currentRow(name, level, grade, domain string) []string {
	return []string{name, level, grade, "", "", "", "", "", "", "", "", "", "", "", "", "", domain}
}

// legacyRow 构造 15 列历史模板的一行（无培养层次、无所属领域）。
func legacyRow(name, grade string) []string {
	return []string{name, grade, "", "", "", "", "", "", "", "", "", "", "", "", ""}
}

// legacyDomainRow 构造 16 列历史模板的一行（含所属领域）。
func legacyDomainRow(name, grade, domain string) []string {
	return append(legacyRow(name, grade), domain)
}

func TestImportValidatesDataDomainPerRowForMultiDomainAdmin(t *testing.T) {
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{
		currentRow("本科生校友", "本科", "2020级", common.DataDomainUndergraduate),
		currentRow("MPA校友", "硕士", "2020级", common.DataDomainMPA),
		currentRow("未知领域", "硕士", "2020级", "unknown"),
		{"空领域", "硕士", "2020级"},
	})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, &fakeImportDataDomainStore{domains: []*model.DataDomain{
		{ID: 1, Code: common.DataDomainUndergraduate, Status: common.DataDomainStatusActive},
		{ID: 2, Code: common.DataDomainMPA, Status: common.DataDomainStatusActive},
	}})

	result, err := svc.Import(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{1, 3}}, nil, reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 1 || len(result.Errors) != 3 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if len(store.batchInputs) != 1 || store.batchInputs[0].Enrollments[0].DataDomainID != 1 {
		t.Fatalf("unexpected imported data domains: %+v", store.batchInputs)
	}
}

func TestImportAllValidRowsWithLegacyTemplate(t *testing.T) {
	reader, err := buildXLSXReader(legacyAlumniColumnHeaders, [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "李老师", "王教授", "公共管理", "非全日制", "政府", "山东大学", "主任", "济南市", "男", "13800000000"},
		{"李四", "2021级", "2021级MPA", "2021", "赵老师", "钱教授", "工商管理", "全日制", "金融", "银行", "经理", "青岛市", "女", "13900000000"},
		{"王五", "2022级"},
	})
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	result, err := importAsSuperAdmin(NewAlumniService(store, nil, testDataDomainStore()), reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Total != 3 || result.Success != 3 || len(result.Errors) != 0 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if len(store.batchInputs) != 3 {
		t.Fatalf("expected 3 people, got %+v", store.batchInputs)
	}
	if level := store.batchInputs[0].Enrollments[0].TrainingLevel; level != common.TrainingLevelUndergraduate {
		t.Fatalf("legacy undergraduate domain should infer 本科, got %q", level)
	}
}

func TestLegacyTemplateRequiresLevelForAcademicGraduateRows(t *testing.T) {
	reader, err := buildXLSXReader(legacyDomainColumnHeaders, [][]string{
		legacyDomainRow("学硕校友", "2020级", common.DataDomainAcademicGraduate),
	})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
		nil,
		reader,
	)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, "培养层次") {
		t.Fatalf("expected explicit level requirement, got %+v", result)
	}
	if len(store.batchInputs) != 0 {
		t.Fatalf("row without level must not be imported: %+v", store.batchInputs)
	}
}

func TestCurrentTemplateRequiresExplicitTrainingLevel(t *testing.T) {
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{
		currentRow("无层次校友", "", "2020级", "mpa"),
	})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	result, err := NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin},
		nil,
		reader,
	)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, "培养层次无效或缺失") {
		t.Fatalf("expected level error, got %+v", result)
	}
}

func TestImportAuditTruncatesBatchRecords(t *testing.T) {
	writer := &fakeExportOperationLogger{}
	svc := NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).WithOperationLogger(writer)
	domainID := uint64(1)
	total := maxAuditBatchRecords + 1
	inputs := make([]do.AlumniCreateInput, 0, total)
	createdIDs := make([]uint64, 0, total)
	for i := 0; i < total; i++ {
		inputs = append(inputs, do.AlumniCreateInput{
			Personal: do.AlumniPersonalInput{Name: fmt.Sprintf("校友%04d", i+1)},
			Enrollments: []do.AlumniEnrollmentInput{{
				DataDomainID: domainID, TrainingLevel: common.TrainingLevelUndergraduate, Grade: "2020级",
			}},
		})
		createdIDs = append(createdIDs, uint64(i+1))
	}

	svc.writeImportAudit(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin},
		inputs, createdIDs, &dto.AlumniImportResult{Total: total, Success: total})

	if len(writer.logs) != 1 || writer.logs[0].Detail == nil {
		t.Fatalf("expected one import audit log, got %+v", writer.logs)
	}
	var detail importAuditDetail
	if err := json.Unmarshal([]byte(*writer.logs[0].Detail), &detail); err != nil {
		t.Fatalf("parse import audit detail: %v", err)
	}
	if len(detail.BatchCreatedAlumni) != maxAuditBatchRecords {
		t.Fatalf("saved batch records = %d, want %d", len(detail.BatchCreatedAlumni), maxAuditBatchRecords)
	}
	if detail.BatchCreatedAlumniTotal != total || !detail.BatchCreatedAlumniTruncated {
		t.Fatalf("unexpected truncation metadata: total=%d truncated=%v", detail.BatchCreatedAlumniTotal, detail.BatchCreatedAlumniTruncated)
	}
	if len(*writer.logs[0].Detail) > 256*1024 {
		t.Fatalf("import audit detail is too large: %d bytes", len(*writer.logs[0].Detail))
	}
}

func TestImportAuditKeepsBatchRecordsUnderLimit(t *testing.T) {
	writer := &fakeExportOperationLogger{}
	svc := NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).WithOperationLogger(writer)
	inputs := make([]do.AlumniCreateInput, 0, maxAuditBatchRecords)
	createdIDs := make([]uint64, 0, maxAuditBatchRecords)
	for i := 0; i < maxAuditBatchRecords; i++ {
		inputs = append(inputs, do.AlumniCreateInput{
			Personal: do.AlumniPersonalInput{Name: fmt.Sprintf("校友%04d", i+1)},
			Enrollments: []do.AlumniEnrollmentInput{{
				DataDomainID: 1, TrainingLevel: common.TrainingLevelUndergraduate, Grade: "2020级",
			}},
		})
		createdIDs = append(createdIDs, uint64(i+1))
	}
	svc.writeImportAudit(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin},
		inputs, createdIDs, &dto.AlumniImportResult{Total: maxAuditBatchRecords, Success: maxAuditBatchRecords})

	if len(writer.logs) != 1 || writer.logs[0].Detail == nil {
		t.Fatalf("expected one import audit log, got %+v", writer.logs)
	}
	var detail importAuditDetail
	if err := json.Unmarshal([]byte(*writer.logs[0].Detail), &detail); err != nil {
		t.Fatalf("parse import audit detail: %v", err)
	}
	if len(detail.BatchCreatedAlumni) != maxAuditBatchRecords || detail.BatchCreatedAlumniTruncated {
		t.Fatalf("under-limit import should keep all records: %+v", detail)
	}
}

func TestImportWritesAuditWithoutProfileValues(t *testing.T) {
	reader, err := buildXLSXReader(legacyAlumniColumnHeaders, [][]string{{"张三", "2020级", "", "", "", "", "", "", "", "山东大学"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	writer := &fakeExportOperationLogger{}
	domainID := uint64(1)
	svc := NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).WithOperationLogger(writer)

	result, err := svc.Import(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, &domainID, reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 1 || len(writer.logs) != 1 {
		t.Fatalf("expected one successful import audit, result=%+v logs=%+v", result, writer.logs)
	}
	log := writer.logs[0]
	if log.Action != AuditActionImport || log.TargetType != auditTargetBatch || log.OperatorID != 7 || log.OperatorRole != common.RoleSuperAdmin || log.Detail == nil {
		t.Fatalf("unexpected import audit log: %+v", log)
	}
	if !strings.Contains(*log.Detail, "张三") {
		t.Fatalf("import audit detail does not contain the created alumni name: %s", *log.Detail)
	}
	if strings.Contains(*log.Detail, "山东大学") {
		t.Fatalf("import audit detail contains sensitive profile value: %s", *log.Detail)
	}
	var detail importAuditDetail
	if err := json.Unmarshal([]byte(*log.Detail), &detail); err != nil {
		t.Fatalf("parse import audit detail: %v", err)
	}
	if detail.Total != 1 || detail.Success != 1 || detail.Failed != 0 || len(detail.DataDomainIDs) != 1 || detail.DataDomainIDs[0] != domainID || !detail.SensitiveFieldsIncluded {
		t.Fatalf("unexpected import audit detail: %+v", detail)
	}
	if len(detail.BatchCreatedAlumni) != 1 || detail.BatchCreatedAlumni[0].Name != "张三" {
		t.Fatalf("expected created alumni in import detail: %+v", detail.BatchCreatedAlumni)
	}
	if _, ok := detail.BatchCreatedAlumni[0].FieldValues["work_unit"]; ok {
		t.Fatalf("sensitive work unit was persisted in import detail: %+v", detail.BatchCreatedAlumni[0].FieldValues)
	}
}

func TestImportForcesAssignedDataDomainAndReportsUnauthorizedSensitiveWrite(t *testing.T) {
	reader, err := buildXLSXReader(legacyAlumniColumnHeaders, [][]string{{"张三", "2020级"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, testDataDomainStore())
	assignedDomainID := uint64(2)
	if _, err := svc.Import(context.Background(), common.AccessContext{
		UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{assignedDomainID},
	}, nil, reader); err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if len(store.batchInputs) != 1 || store.batchInputs[0].Enrollments[0].DataDomainID != assignedDomainID {
		t.Fatalf("expected imported profile to use assigned domain %d, got %+v", assignedDomainID, store.batchInputs)
	}

	reader, err = buildXLSXReader(legacyAlumniColumnHeaders, [][]string{{"李四", "2020级", "", "", "", "", "", "", "", "", "主任"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	result, err := svc.Import(context.Background(), common.AccessContext{
		UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{assignedDomainID},
	}, nil, reader)
	if err != nil {
		t.Fatalf("expected partial import result, got %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 || result.Errors[0].Row != 2 || result.Errors[0].Message != "无权导入敏感字段" {
		t.Fatalf("expected sensitive row error, got %+v", result)
	}
}

func TestImportDedupesExactEnrollmentButKeepsDifferentPeople(t *testing.T) {
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13800000000"},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13800000000"},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13900000000"},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", " 13800000000 "},
	}
	reader, err := buildXLSXReader(legacyAlumniColumnHeaders, rows)
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	result, err := importAsSuperAdmin(NewAlumniService(store, nil, testDataDomainStore()), reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Success != 2 || len(result.Errors) != 2 {
		t.Fatalf("expected duplicate rows rejected, got %+v", result)
	}
	if len(store.batchInputs) != 2 {
		t.Fatalf("expected two distinct people, got %+v", store.batchInputs)
	}
}

func TestImportMergesMultipleEnrollmentsOfOnePerson(t *testing.T) {
	rows := [][]string{
		{"张同学", "本科", "2015", "", "", "", "", "行政管理", "", "", "", "", "", "", "13800000001", "", "undergraduate"},
		{"张同学", "硕士", "2019", "", "", "", "", "行政管理", "", "", "", "", "", "", "13800000001", "", "academic_graduate"},
	}
	reader, err := buildXLSXReader(alumniImportColumnHeaders, rows)
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	result, err := importAsSuperAdmin(NewAlumniService(store, nil, testDataDomainStore()), reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Success != 2 || len(result.Errors) != 0 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if len(store.batchInputs) != 1 || len(store.batchInputs[0].Enrollments) != 2 {
		t.Fatalf("expected one person with two enrollments, got %+v", store.batchInputs)
	}
	if store.batchInputs[0].Enrollments[0].TrainingLevel != common.TrainingLevelUndergraduate ||
		store.batchInputs[0].Enrollments[1].TrainingLevel != common.TrainingLevelMaster {
		t.Fatalf("unexpected merged enrollment levels: %+v", store.batchInputs[0].Enrollments)
	}
}

func TestImportPartialErrors(t *testing.T) {
	reader, err := buildXLSXReader(legacyAlumniColumnHeaders, [][]string{
		{"张三", "2020级"},
		{"", "2021级"},
		{"王五", ""},
	})
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}
	result, err := importAsSuperAdmin(NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()), reader)
	if err != nil {
		t.Fatalf("expected import success (partial), got %v", err)
	}
	if result.Total != 3 || result.Success != 1 || len(result.Errors) != 2 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if result.Errors[0].Row != 3 || result.Errors[1].Row != 4 {
		t.Fatalf("expected row numbers 3 and 4, got %+v", result.Errors)
	}
}

func TestImportEmptyMalformedAndHeaderMismatch(t *testing.T) {
	empty, err := buildXLSXReader(legacyAlumniColumnHeaders, nil)
	if err != nil {
		t.Fatalf("build empty xlsx: %v", err)
	}
	if _, err := importAsSuperAdmin(NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()), empty); !errors.Is(err, common.ErrInvalidRequest) {
		t.Fatalf("expected invalid request for empty file, got %v", err)
	}

	malformed := bytes.NewReader([]byte{'P', 'K', 0x03, 0x04, 0xff, 0x00, 0x00})
	if _, err := importAsSuperAdmin(NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()), malformed); err != common.ErrInvalidRequest {
		t.Fatalf("expected invalid request for malformed workbook, got %v", err)
	}

	bad, err := buildXLSXReader([]string{"名称", "年级"}, [][]string{{"张三", "2020级"}})
	if err != nil {
		t.Fatalf("build bad header xlsx: %v", err)
	}
	if _, err := importAsSuperAdmin(NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()), bad); !errors.Is(err, common.ErrImportInvalidHeader) {
		t.Fatalf("expected header mismatch, got %v", err)
	}
}

func TestParseImportRowSupportsCurrentAndLegacyLayouts(t *testing.T) {
	current, err := parseImportRow(currentRow("张三", "硕士", "2020级", "mpa"), importLayoutCurrent)
	if err != nil {
		t.Fatalf("parse current row: %v", err)
	}
	if current.personal.Name != "张三" || current.enrollment.Grade != "2020级" ||
		current.enrollment.TrainingLevel != common.TrainingLevelMaster {
		t.Fatalf("unexpected current parse: %+v", current)
	}

	legacy, err := parseImportRow(legacyRow("王五", "2022级"), importLayoutLegacy)
	if err != nil {
		t.Fatalf("parse legacy row: %v", err)
	}
	if legacy.personal.Name != "王五" || legacy.enrollment.Grade != "2022级" ||
		legacy.enrollment.ClassName != nil || legacy.personal.Mobile != nil {
		t.Fatalf("unexpected legacy parse: %+v", legacy)
	}

	short, err := parseImportRow([]string{"测试"}, importLayoutLegacy)
	if err != nil {
		t.Fatalf("parse short row: %v", err)
	}
	if short.personal.Name != "测试" || short.enrollment.Grade != "" {
		t.Fatalf("unexpected short row parse: %+v", short)
	}
}

func TestImportAcceptsDataDomainCodeNameAndID(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{name: "code", value: "mpa"},
		{name: "code is case insensitive", value: "MPA"},
		{name: "display name", value: "MPA专业学位研究生"},
		{name: "numeric id", value: "2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{currentRow("张三", "硕士", "2020级", tc.value)})
			if err != nil {
				t.Fatalf("build xlsx: %v", err)
			}
			store := &fakeAlumniStore{}
			result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
				context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, nil, reader)
			if err != nil {
				t.Fatalf("Import() error = %v", err)
			}
			if result.Success != 1 || len(store.batchInputs) != 1 {
				t.Fatalf("unexpected import result: %+v", result)
			}
			if domain := store.batchInputs[0].Enrollments[0].DataDomainID; domain != 2 {
				t.Fatalf("data domain = %d, want 2", domain)
			}
		})
	}
}

func TestImportInvalidDataDomainListsOptions(t *testing.T) {
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{currentRow("张三", "硕士", "2020级", "unknown")})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	result, err := NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, nil, reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	message := result.Errors[0].Message
	if !strings.Contains(message, "可选值") || !strings.Contains(message, "undergraduate（本科生）") || !strings.Contains(message, "mpa（MPA专业学位研究生）") {
		t.Fatalf("expected actionable domain error, got %q", message)
	}
}

func TestImportRequiresDomainColumnWithoutTargetDomain(t *testing.T) {
	reader, err := buildXLSXReader(legacyAlumniColumnHeaders, [][]string{{"张三", "2020级"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	_, err = NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, nil, reader)
	if !errors.Is(err, common.ErrImportDomainRequired) {
		t.Fatalf("Import() error = %v, want ErrImportDomainRequired", err)
	}
}

func TestImportTemplateExplainsDomainsAndLevelsAndExampleIsImportable(t *testing.T) {
	template, err := buildTemplateXLSX()
	if err != nil {
		t.Fatalf("build template: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(template.Data))
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("read data sheet: %v", err)
	}
	if len(rows) != 1 || len(rows[0]) != len(alumniImportColumnHeaders) {
		t.Fatalf("unexpected template data sheet: %d rows", len(rows))
	}
	if rows[0][len(rows[0])-1] != "所属领域" {
		t.Fatalf("expected 所属领域 header, got %q", rows[0][len(rows[0])-1])
	}

	notes, err := f.GetRows("填写说明")
	if err != nil {
		t.Fatalf("read note sheet: %v", err)
	}
	var noteText strings.Builder
	for _, row := range notes {
		noteText.WriteString(strings.Join(row, " "))
		noteText.WriteString("\n")
	}
	for _, want := range []string{"undergraduate", "academic_graduate", "mpa", "MPA专业学位研究生", "示例校友", "2024", "数字 ID", "培养层次"} {
		if !strings.Contains(noteText.String(), want) {
			t.Fatalf("template guide missing %q: %s", want, noteText.String())
		}
	}

	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{currentRow("示例校友", "硕士", "2024", "mpa")})
	if err != nil {
		t.Fatalf("build example xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, nil, reader)
	if err != nil {
		t.Fatalf("example row import failed: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("example row should import successfully: %+v", result)
	}
}

func TestImportTemplateBlankRowIsRejected(t *testing.T) {
	template, err := buildTemplateXLSX()
	if err != nil {
		t.Fatalf("build template: %v", err)
	}
	_, err = NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, nil, bytes.NewReader(template.Data))
	if !errors.Is(err, common.ErrImportNoDataRows) {
		t.Fatalf("Import() error = %v, want ErrImportNoDataRows", err)
	}
}

func TestSingleDomainAdminDomainRules(t *testing.T) {
	crossDomain := currentRow("跨域校友", "硕士", "2020级", "mpa")
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{crossDomain})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 2, Role: common.RoleAdmin, DomainIDs: []uint64{1}}, nil, reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, "不一致") {
		t.Fatalf("cross-domain row should be rejected, got %+v", result)
	}
	if len(store.batchInputs) != 0 {
		t.Fatalf("cross-domain row must not be created: %+v", store.batchInputs)
	}

	matching, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{currentRow("MPA校友", "硕士", "2020级", "mpa")})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store = &fakeAlumniStore{}
	result, err = NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 2, Role: common.RoleAdmin, DomainIDs: []uint64{2}}, nil, matching)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 1 || len(store.batchInputs) != 1 || store.batchInputs[0].Enrollments[0].DataDomainID != 2 {
		t.Fatalf("matching row should import into domain 2, got %+v / %+v", result, store.batchInputs)
	}

	blank, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{currentRow("空白领域校友", "本科", "2020级", "")})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store = &fakeAlumniStore{}
	result, err = NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 2, Role: common.RoleAdmin, DomainIDs: []uint64{1}}, nil, blank)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 1 || len(store.batchInputs) != 1 || store.batchInputs[0].Enrollments[0].DataDomainID != 1 {
		t.Fatalf("blank domain should use the only authorized domain, got %+v / %+v", result, store.batchInputs)
	}

	unauthorizedTarget, err := buildXLSXReader(legacyAlumniColumnHeaders, [][]string{{"张三", "2020级"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	otherDomainID := uint64(2)
	_, err = NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 2, Role: common.RoleAdmin, DomainIDs: []uint64{1}}, &otherDomainID, unauthorizedTarget)
	if !errors.Is(err, common.ErrPermissionDenied) {
		t.Fatalf("Import() error = %v, want ErrPermissionDenied", err)
	}
}

func TestImportRejectsDisabledDataDomain(t *testing.T) {
	domains := &fakeImportDataDomainStore{domains: []*model.DataDomain{
		{ID: 1, Code: common.DataDomainUndergraduate, Name: "本科生", Status: common.DataDomainStatusActive},
		{ID: 3, Code: "legacy_program", Name: "历史项目", Status: common.DataDomainStatusDisabled},
	}}
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{currentRow("历史校友", "硕士", "2010级", "legacy_program")})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	result, err := NewAlumniService(&fakeAlumniStore{}, nil, domains).Import(
		context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, nil, reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, "已停用") {
		t.Fatalf("expected disabled domain message, got %+v", result)
	}
}

func TestImportRestoresFormulaGuardPrefixes(t *testing.T) {
	cases := map[string]string{
		"'=SUM(A1:A2)":    "=SUM(A1:A2)",
		"'+8613800000000": "+8613800000000",
		"'-1":             "-1",
		"'@user":          "@user",
		"'normal":         "'normal",
	}
	var b strings.Builder
	b.WriteString(strings.Join(alumniImportColumnHeaders, ","))
	b.WriteString("\n")
	for name := range cases {
		b.WriteString(strings.Join(currentRow(name, "本科", "2020级", "undergraduate"), ","))
		b.WriteString("\n")
	}

	store := &fakeAlumniStore{}
	result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, nil, strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != len(cases) || len(store.batchInputs) != len(cases) {
		t.Fatalf("unexpected import result: %+v", result)
	}
	got := make(map[string]bool, len(store.batchInputs))
	for _, input := range store.batchInputs {
		got[input.Personal.Name] = true
	}
	for input, want := range cases {
		if !got[want] {
			t.Fatalf("input %q was not restored to %q; got %v", input, want, got)
		}
	}
}

func TestImportCSVRowLimit(t *testing.T) {
	line := strings.Join(currentRow("张三", "本科", "2020级", "undergraduate"), ",") + "\n"
	var b strings.Builder
	b.WriteString(strings.Join(alumniImportColumnHeaders, ","))
	b.WriteString("\n")
	for i := 0; i < maxImportDataRows+1; i++ {
		b.WriteString(line)
	}

	_, err := NewAlumniService(&fakeAlumniStore{}, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, nil, strings.NewReader(b.String()))
	if !errors.Is(err, common.ErrImportRowLimit) {
		t.Fatalf("Import() error = %v, want ErrImportRowLimit", err)
	}
}

func TestImportDatabaseUnavailable(t *testing.T) {
	reader, err := buildXLSXReader(legacyAlumniColumnHeaders, [][]string{{"张三", "2020级"}})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	_, err = importAsSuperAdmin(NewAlumniService(nil, nil), reader)
	if err != common.ErrDatabaseUnavailable {
		t.Fatalf("expected database unavailable, got %v", err)
	}
}

func TestImportExplicitTargetDomainIgnoresDomainColumn(t *testing.T) {
	reader, err := buildXLSXReader(alumniImportColumnHeaders, [][]string{
		currentRow("目标域校友", "硕士", "2020级", "mpa"),
	})
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	store := &fakeAlumniStore{}
	targetDomainID := uint64(1)
	result, err := NewAlumniService(store, nil, testDataDomainStore()).Import(
		context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, &targetDomainID, reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 1 || len(store.batchInputs) != 1 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if got := store.batchInputs[0].Enrollments[0].DataDomainID; got != targetDomainID {
		t.Fatalf("explicit target domain should win over file column: got %d, want %d", got, targetDomainID)
	}
}
