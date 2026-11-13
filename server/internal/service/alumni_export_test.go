package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/xuri/excelize/v2"
)

type fakeExportOperationLogger struct {
	logs []*model.OperationLog
}

func (l *fakeExportOperationLogger) Write(_ context.Context, log *model.OperationLog) error {
	l.logs = append(l.logs, log)
	return nil
}

func exportAsSuperAdmin(svc *AlumniService, req dto.AlumniExportRequest) (*ExportResult, error) {
	return svc.Export(context.Background(), req, common.AccessContext{Role: common.RoleSuperAdmin})
}

func testExportRecord(name string, domainID uint64, grade string) ([]*model.AlumniProfile, map[uint64][]*model.AlumniEnrollment) {
	profile := &model.AlumniProfile{ID: 1, Name: name, Status: common.AlumniStatusActive, Version: 1}
	enrollment := &model.AlumniEnrollment{
		ID: 11, AlumniID: profile.ID, DataDomainID: domainID, TrainingLevel: common.TrainingLevelMaster,
		Grade: grade, ClassName: new("MPA周末班"), Cohort: new("2020"), Major: new("公共管理"),
		TrainingMode: new("非全日制"), Status: common.AlumniStatusActive, Version: 1,
	}
	return []*model.AlumniProfile{profile}, map[uint64][]*model.AlumniEnrollment{profile.ID: {enrollment}}
}

func exportCSVRecords(t *testing.T, result *ExportResult) [][]string {
	t.Helper()
	records, err := csv.NewReader(bytes.NewReader(result.Data[3:])).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	return records
}

func TestExportXlsxFormat(t *testing.T) {
	workUnit := "山东大学"
	position := "主任"
	profiles, enrollments := testExportRecord("张三", 3, "2020级")
	profiles[0].WorkUnit = &workUnit
	profiles[0].Position = &position
	store := &fakeAlumniStore{items: profiles, itemEnrollments: enrollments}
	svc := NewAlumniService(store, nil)

	result, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{Format: "xlsx"})
	if err != nil {
		t.Fatalf("expected xlsx export success, got %v", err)
	}
	if result.ContentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("expected xlsx content type, got %q", result.ContentType)
	}

	f, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatalf("failed to open xlsx: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("failed to get rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected header + 1 data row, got %d rows", len(rows))
	}
	if !equalStrings(rows[0], alumniExportColumnHeaders) {
		t.Fatalf("unexpected headers: %v", rows[0])
	}
	if rows[1][0] != "张三" || rows[1][1] != "硕士" || rows[1][2] != "2020级" {
		t.Fatalf("unexpected data row: %v", rows[1])
	}
}

func TestExportCsvFormat(t *testing.T) {
	profiles, enrollments := testExportRecord("张三", 3, "2020级")
	store := &fakeAlumniStore{items: profiles, itemEnrollments: enrollments}
	svc := NewAlumniService(store, nil)

	result, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{Format: "csv"})
	if err != nil {
		t.Fatalf("expected csv export success, got %v", err)
	}
	if result.ContentType != "text/csv; charset=utf-8" {
		t.Fatalf("expected csv content type, got %q", result.ContentType)
	}
	if len(result.Data) < 3 || result.Data[0] != 0xEF || result.Data[1] != 0xBB || result.Data[2] != 0xBF {
		t.Fatal("expected UTF-8 BOM prefix")
	}
	records := exportCSVRecords(t, result)
	if len(records) != 2 || records[1][0] != "张三" || records[1][1] != "硕士" {
		t.Fatalf("unexpected csv records: %v", records)
	}
}

func TestExportDefaultFormatIsXlsxAndEmptyData(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)
	result, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{})
	if err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	if !strings.HasSuffix(result.Filename, ".xlsx") {
		t.Fatalf("expected default xlsx filename, got %q", result.Filename)
	}
	f, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatalf("open empty xlsx: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("get empty rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected header only, got %d rows", len(rows))
	}
}

func TestExportFilterPropagation(t *testing.T) {
	profiles, enrollments := testExportRecord("张三", 3, "2023级")
	store := &fakeAlumniStore{items: profiles, itemEnrollments: enrollments}
	svc := NewAlumniService(store, nil)

	_, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{
		Grade: "2023级", Industry: "政府", TrainingLevel: common.TrainingLevelMaster,
	})
	if err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	if store.query.Grade != "2023级" {
		t.Fatalf("expected grade filter 2023级, got %q", store.query.Grade)
	}
	if store.query.Industry != "政府" {
		t.Fatalf("expected industry filter 政府, got %q", store.query.Industry)
	}
	if store.query.TrainingLevel != common.TrainingLevelMaster {
		t.Fatalf("expected training level filter, got %q", store.query.TrainingLevel)
	}
}

func TestExportScopesDomainAndMasksSensitiveFields(t *testing.T) {
	mobile := "13800000000"
	email := "zhangsan@example.com"
	workUnit := "山东大学"
	position := "主任"
	profiles, enrollments := testExportRecord("张三", 2, "2020级")
	profiles[0].Mobile = &mobile
	profiles[0].Email = &email
	profiles[0].WorkUnit = &workUnit
	profiles[0].Position = &position
	store := &fakeAlumniStore{items: profiles, itemEnrollments: enrollments}
	svc := NewAlumniService(store, nil)

	result, err := svc.Export(context.Background(), dto.AlumniExportRequest{Format: "csv"}, common.AccessContext{
		Role: common.RoleAdmin, DomainIDs: []uint64{2, 1},
	})
	if err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	if len(store.query.DataDomainIDs) != 2 || store.query.DataDomainIDs[0] != 1 || store.query.DataDomainIDs[1] != 2 {
		t.Fatalf("expected export to query normalized domains [1 2], got %v", store.query.DataDomainIDs)
	}
	if store.query.CanReadSensitive {
		t.Fatal("expected sensitive fields to remain unavailable")
	}
	records := exportCSVRecords(t, result)
	header := records[0]
	row := records[1]
	for _, column := range []string{"工作单位", "职务", "通讯地址", "手机号", "邮箱"} {
		index := indexOf(header, column)
		if index < 0 {
			t.Fatalf("missing column %q in %v", column, header)
		}
		if row[index] != "" {
			t.Fatalf("expected masked %s, got %q", column, row[index])
		}
	}
}

func TestExportWritesAuditWithoutSensitiveValues(t *testing.T) {
	mobile := "13800000000"
	profiles, enrollments := testExportRecord("张三", 3, "2020级")
	profiles[0].Mobile = &mobile
	logger := &fakeExportOperationLogger{}
	svc := NewAlumniService(&fakeAlumniStore{items: profiles, itemEnrollments: enrollments}, nil).
		WithOperationLogger(logger)

	if _, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{}); err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	if len(logger.logs) != 1 || logger.logs[0].Detail == nil {
		t.Fatalf("expected one export audit log, got %+v", logger.logs)
	}
	if strings.Contains(*logger.logs[0].Detail, mobile) {
		t.Fatalf("export audit must not contain sensitive values: %s", *logger.logs[0].Detail)
	}
	if !strings.Contains(*logger.logs[0].Detail, `"record_count":1`) {
		t.Fatalf("expected record count in audit detail: %s", *logger.logs[0].Detail)
	}
}

func TestExportRejectsSensitiveFilterWithoutPermission(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)
	viewer := common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{1}}
	for _, req := range []dto.AlumniExportRequest{{WorkUnit: "山东大学"}, {Position: "主任"}, {Mobile: "13800000000"}} {
		if _, err := svc.Export(context.Background(), req, viewer); err != common.ErrPermissionDenied {
			t.Fatalf("expected sensitive filter rejection for %+v, got %v", req, err)
		}
	}
	if _, err := svc.Export(context.Background(), dto.AlumniExportRequest{}, common.AccessContext{Role: common.RoleAdmin}); err != common.ErrPermissionDenied {
		t.Fatalf("expected empty domain scope rejection, got %v", err)
	}
}

func TestExportSanitizesFormulaInjection(t *testing.T) {
	profiles, enrollments := testExportRecord("=cmd|'/C calc'!A0", 3, "2020级")
	profiles[0].WorkUnit = new(`=HYPERLINK("http://evil.com")`)
	store := &fakeAlumniStore{items: profiles, itemEnrollments: enrollments}

	result, err := NewAlumniService(store, nil).Export(context.Background(), dto.AlumniExportRequest{Format: "xlsx"}, common.AccessContext{Role: common.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("get rows: %v", err)
	}
	if rows[1][0] != "'=cmd|'/C calc'!A0" {
		t.Fatalf("expected xlsx name to be escaped, got %q", rows[1][0])
	}
	if rows[1][10] != `'=HYPERLINK("http://evil.com")` {
		t.Fatalf("expected xlsx work unit to be escaped, got %q", rows[1][10])
	}
}

func TestExportIncludesTrainingLevelAndDataDomainCode(t *testing.T) {
	profiles := []*model.AlumniProfile{
		{ID: 1, Name: "张三", Status: common.AlumniStatusActive},
		{ID: 2, Name: "李四", Status: common.AlumniStatusActive},
	}
	enrollments := map[uint64][]*model.AlumniEnrollment{
		1: {
			{ID: 11, AlumniID: 1, DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级", Status: common.AlumniStatusActive},
			{ID: 12, AlumniID: 1, DataDomainID: 1, TrainingLevel: common.TrainingLevelUndergraduate, Grade: "2015级", Status: common.AlumniStatusActive},
		},
		2: {{ID: 21, AlumniID: 2, DataDomainID: 1, TrainingLevel: common.TrainingLevelUnverified, Grade: "2010级", Status: common.AlumniStatusActive}},
	}
	store := &fakeAlumniStore{items: profiles, itemEnrollments: enrollments}
	dataDomains := &fakeImportDataDomainStore{domains: []*model.DataDomain{
		{ID: 1, Code: "undergraduate", Name: "本科生", Status: common.DataDomainStatusActive},
		{ID: 3, Code: "mpa", Name: "MPA专业学位研究生", Status: common.DataDomainStatusActive},
	}}

	result, err := NewAlumniService(store, nil, dataDomains).Export(context.Background(), dto.AlumniExportRequest{Format: "csv"}, common.AccessContext{Role: common.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	records := exportCSVRecords(t, result)
	if len(records) != 4 {
		t.Fatalf("expected 3 enrollment rows + header, got %d", len(records))
	}
	levelIndex := indexOf(records[0], "培养层次")
	domainIndex := indexOf(records[0], "所属领域")
	if levelIndex < 0 || domainIndex < 0 {
		t.Fatalf("missing training level or domain column: %v", records[0])
	}
	got := []string{records[1][levelIndex] + "/" + records[1][domainIndex], records[2][levelIndex] + "/" + records[2][domainIndex], records[3][levelIndex] + "/" + records[3][domainIndex]}
	want := []string{"硕士/mpa", "本科/undergraduate", "待核验/undergraduate"}
	if !equalStrings(got, want) {
		t.Fatalf("unexpected level/domain rows: got %v want %v", got, want)
	}
}

func TestExportFiltersDataDomainAndExportsCode(t *testing.T) {
	domainID := uint64(3)
	profiles, enrollments := testExportRecord("张三", domainID, "2020级")
	store := &fakeAlumniStore{items: profiles, itemEnrollments: enrollments}
	svc := NewAlumniService(store, nil)

	_, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{DataDomainID: &domainID})
	if err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	if len(store.query.DataDomainIDs) != 1 || store.query.DataDomainIDs[0] != domainID {
		t.Fatalf("unexpected domain filter: %+v", store.query.DataDomainIDs)
	}
}

func exportRoundTrip(t *testing.T, sensitiveReadable bool) (*ExportResult, [][]string, *fakeAlumniStore) {
	t.Helper()
	mobile := "13800000000"
	email := "zhangsan@example.com"
	className := "MPA周末班"
	cohort := "2020"
	profiles := []*model.AlumniProfile{{
		ID: 1, Name: "张三", Mobile: &mobile, Email: &email, Industry: new("政府"),
		WorkUnit: new("山东大学"), Position: new("主任"), MailingAddress: new("济南市"),
		Gender: new("男"), Status: common.AlumniStatusActive,
	}}
	enrollments := map[uint64][]*model.AlumniEnrollment{1: {{
		ID: 11, AlumniID: 1, DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster,
		Grade: "2020级", ClassName: &className, Cohort: &cohort, Major: new("公共管理"),
		TrainingMode: new("非全日制"), Status: common.AlumniStatusActive,
	}}}
	exportStore := &fakeAlumniStore{items: profiles, itemEnrollments: enrollments}
	svc := NewAlumniService(exportStore, nil)
	viewer := common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{3}}
	if sensitiveReadable {
		viewer.Permissions = map[string]bool{common.PermissionAlumniSensitiveRead: true}
	}
	result, err := svc.Export(context.Background(), dto.AlumniExportRequest{Format: "csv"}, viewer)
	if err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	return result, exportCSVRecords(t, result), exportStore
}

func TestExportedFileRoundTripsIntoImport(t *testing.T) {
	result, records, _ := exportRoundTrip(t, true)
	importStore := &fakeAlumniStore{}
	importResult, err := NewAlumniService(importStore, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{3}, Permissions: map[string]bool{common.PermissionAlumniSensitiveRead: true}},
		nil,
		bytes.NewReader(result.Data),
	)
	if err != nil {
		t.Fatalf("expected round-trip import success, got %v", err)
	}
	if importResult.Success != 1 || len(importResult.Errors) != 0 {
		t.Fatalf("unexpected round-trip import result: %+v (rows=%v)", importResult, records)
	}
	if len(importStore.batchInputs) != 1 || len(importStore.batchInputs[0].Enrollments) != 1 {
		t.Fatalf("expected one imported person with one enrollment, got %+v", importStore.batchInputs)
	}
	enrollment := importStore.batchInputs[0].Enrollments[0]
	if enrollment.TrainingLevel != common.TrainingLevelMaster || enrollment.Grade != "2020级" || enrollment.DataDomainID != 3 {
		t.Fatalf("unexpected round-trip enrollment: %+v", enrollment)
	}
}

func TestExportedFileDedupesInsteadOfFormatError(t *testing.T) {
	result, records, exportStore := exportRoundTrip(t, true)
	_ = records
	key := do.AlumniDedupKey{
		Name: "张三", Mobile: "13800000000", DataDomainID: 3, Grade: "2020级",
		ClassName: "MPA周末班", Cohort: "2020",
	}.Key()
	importStore := &fakeAlumniStore{existing: map[string]bool{key: true}}
	importResult, err := NewAlumniService(importStore, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{3}, Permissions: map[string]bool{common.PermissionAlumniSensitiveRead: true}},
		nil,
		bytes.NewReader(result.Data),
	)
	if err != nil {
		t.Fatalf("expected dedupe import success, got %v", err)
	}
	if importResult.Success != 0 || len(importResult.Errors) != 1 {
		t.Fatalf("expected duplicate row error, got %+v (export rows=%v, domain store=%+v)", importResult, records, exportStore)
	}
}

func TestMaskedExportRoundTripDoesNotLeakOrCrash(t *testing.T) {
	result, records, _ := exportRoundTrip(t, false)
	mobileIndex := indexOf(records[0], "手机号")
	if records[1][mobileIndex] != "" {
		t.Fatalf("expected masked mobile to be empty, got %q", records[1][mobileIndex])
	}
	importStore := &fakeAlumniStore{}
	importResult, err := NewAlumniService(importStore, nil, testDataDomainStore()).Import(
		context.Background(),
		common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{3}},
		nil,
		bytes.NewReader(result.Data),
	)
	if err != nil {
		t.Fatalf("expected masked round-trip import success, got %v", err)
	}
	if importResult.Success != 1 || len(importResult.Errors) != 0 {
		t.Fatalf("unexpected masked import result: %+v", importResult)
	}
	if importStore.batchInputs[0].Personal.Mobile != nil {
		t.Fatalf("masked mobile must stay empty, got %+v", importStore.batchInputs[0].Personal.Mobile)
	}
}

func TestFormulaGuardRoundTripIsSymmetric(t *testing.T) {
	name := "=cmd|'/C calc'!A0"
	workUnit := `=HYPERLINK("http://evil.com")`
	profiles := []*model.AlumniProfile{{
		ID: 1, Name: name, WorkUnit: &workUnit, Status: common.AlumniStatusActive,
	}}
	enrollments := map[uint64][]*model.AlumniEnrollment{1: {{
		ID: 11, AlumniID: 1, DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster,
		Grade: "2020级", Status: common.AlumniStatusActive,
	}}}
	exportStore := &fakeAlumniStore{items: profiles, itemEnrollments: enrollments}
	result, err := NewAlumniService(exportStore, nil, testDataDomainStore()).Export(context.Background(), dto.AlumniExportRequest{Format: "csv"}, common.AccessContext{Role: common.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	importStore := &fakeAlumniStore{}
	if _, err := NewAlumniService(importStore, nil, testDataDomainStore()).Import(context.Background(), common.AccessContext{Role: common.RoleSuperAdmin}, nil, bytes.NewReader(result.Data)); err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if len(importStore.batchInputs) != 1 {
		t.Fatalf("expected one imported person, got %+v", importStore.batchInputs)
	}
	if importStore.batchInputs[0].Personal.Name != name || importStore.batchInputs[0].Personal.WorkUnit == nil || *importStore.batchInputs[0].Personal.WorkUnit != workUnit {
		t.Fatalf("formula guard did not round-trip: %+v", importStore.batchInputs[0].Personal)
	}
}

func indexOf(values []string, target string) int {
	for i, value := range values {
		if strings.TrimSpace(value) == target {
			return i
		}
	}
	return -1
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func testDataDomainStore() *fakeImportDataDomainStore {
	return &fakeImportDataDomainStore{domains: []*model.DataDomain{
		{ID: 1, Code: common.DataDomainUndergraduate, Name: "本科生", Status: common.DataDomainStatusActive},
		{ID: 2, Code: common.DataDomainMPA, Name: "MPA专业学位研究生", Status: common.DataDomainStatusActive},
		{ID: 3, Code: common.DataDomainAcademicGraduate, Name: "学术学位研究生", Status: common.DataDomainStatusActive},
	}}
}

// alumniColumnHeaders 保留历史 15 列模板顺序，供导入兼容测试使用。
var alumniColumnHeaders = append([]string{}, legacyAlumniColumnHeaders...)
