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

func TestExportXlsxFormat(t *testing.T) {
	workUnit := "山东大学"
	position := "主任"
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{
			{
				ID:        1,
				Name:      "张三",
				Grade:     "2020级",
				ClassName: new("2020级MPA周末班"),
				Cohort:    new("2020"),
				Major:     new("公共管理"),
				WorkUnit:  &workUnit,
				Position:  &position,
				Status:    "active",
			},
		},
	}
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
	if len(rows) < 2 {
		t.Fatalf("expected header + at least 1 data row, got %d rows", len(rows))
	}
	if rows[0][0] != "姓名" {
		t.Fatalf("expected first header column 姓名, got %q", rows[0][0])
	}
	if rows[1][0] != "张三" {
		t.Fatalf("expected first data row first column 张三, got %q", rows[1][0])
	}
}

func TestExportCsvFormat(t *testing.T) {
	workUnit := "山东大学"
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{
			{ID: 1, Name: "张三", Grade: "2020级", WorkUnit: &workUnit, Status: "active"},
		},
	}
	svc := NewAlumniService(store, nil)

	result, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{Format: "csv"})
	if err != nil {
		t.Fatalf("expected csv export success, got %v", err)
	}
	if result.ContentType != "text/csv; charset=utf-8" {
		t.Fatalf("expected csv content type, got %q", result.ContentType)
	}

	// UTF-8 BOM
	if len(result.Data) < 3 || result.Data[0] != 0xEF || result.Data[1] != 0xBB || result.Data[2] != 0xBF {
		t.Fatal("expected UTF-8 BOM prefix")
	}

	r := csv.NewReader(bytes.NewReader(result.Data[3:])) // skip BOM
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to read csv: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected header + at least 1 data row, got %d rows", len(records))
	}
	if records[0][0] != "姓名" {
		t.Fatalf("expected 姓名 header, got %q", records[0][0])
	}
	if records[1][0] != "张三" {
		t.Fatalf("expected 张三 in first data row, got %q", records[1][0])
	}
}

func TestExportDefaultFormatIsXlsx(t *testing.T) {
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{
			{ID: 1, Name: "李四", Grade: "2021级", Status: "active"},
		},
	}
	svc := NewAlumniService(store, nil)

	result, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{})
	if err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	if result.ContentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("expected default xlsx content type, got %q", result.ContentType)
	}
}

func TestExportEmptyData(t *testing.T) {
	store := &fakeAlumniStore{
		items: nil,
	}
	svc := NewAlumniService(store, nil)

	result, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{})
	if err != nil {
		t.Fatalf("expected export success for empty data, got %v", err)
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
	if len(rows) < 1 {
		t.Fatal("expected at least header row")
	}
	if len(rows) > 1 {
		t.Fatalf("expected only header row for empty data, got %d rows", len(rows))
	}
}

func TestExportFilterPropagation(t *testing.T) {
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{
			{ID: 1, Name: "王五", Grade: "2023级", Status: "active"},
		},
	}
	svc := NewAlumniService(store, nil)

	_, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{
		Grade:    "2023级",
		Industry: "政府",
		Format:   "csv",
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
}

func TestExportScopesDomainAndMasksSensitiveFields(t *testing.T) {
	mobile := "13800000000"
	email := "zhangsan@example.com"
	workUnit := "山东大学"
	position := "主任"
	address := "济南市"
	store := &fakeAlumniStore{items: []*model.AlumniProfile{{
		ID:             1,
		Name:           "张三",
		Grade:          "2020级",
		Mobile:         &mobile,
		Email:          &email,
		WorkUnit:       &workUnit,
		Position:       &position,
		MailingAddress: &address,
	}}}
	svc := NewAlumniService(store, nil)

	result, err := svc.Export(context.Background(), dto.AlumniExportRequest{Format: "csv"}, common.AccessContext{
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{2, 1, 2},
	})
	if err != nil {
		t.Fatalf("expected scoped export success, got %v", err)
	}
	if len(store.query.DataDomainIDs) != 2 || store.query.DataDomainIDs[0] != 1 || store.query.DataDomainIDs[1] != 2 {
		t.Fatalf("expected export to query normalized domains [1 2], got %v", store.query.DataDomainIDs)
	}
	if store.query.CanReadSensitive {
		t.Fatal("expected export query to mark sensitive fields unavailable")
	}

	records, err := csv.NewReader(bytes.NewReader(result.Data[3:])).ReadAll()
	if err != nil {
		t.Fatalf("read exported csv: %v", err)
	}
	row := records[1]
	if row[9] != "" || row[10] != "" || row[11] != "" || row[13] != "" || row[14] != "" {
		t.Fatalf("expected sensitive export cells to be blank, got %v", row)
	}
}

func TestExportWritesAuditWithoutSensitiveValues(t *testing.T) {
	mobile := "13800000000"
	logger := &fakeExportOperationLogger{}
	svc := NewAlumniService(&fakeAlumniStore{items: []*model.AlumniProfile{{ID: 1, Name: "张三", Grade: "2020级", Mobile: &mobile}}}, nil).
		WithOperationLogger(logger)

	_, err := svc.Export(context.Background(), dto.AlumniExportRequest{Format: "csv"}, common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{2}})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(logger.logs) != 1 || logger.logs[0].Action != "export_alumni" || logger.logs[0].Detail == nil {
		t.Fatalf("unexpected audit logs: %+v", logger.logs)
	}
	if strings.Contains(*logger.logs[0].Detail, mobile) || strings.Contains(*logger.logs[0].Detail, "张三") {
		t.Fatalf("export audit detail contains sensitive or profile value: %s", *logger.logs[0].Detail)
	}
}

func TestExportRejectsSensitiveFilterWithoutPermission(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)
	_, err := svc.Export(context.Background(), dto.AlumniExportRequest{Mobile: "13800000000"}, common.AccessContext{
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{1},
	})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected sensitive export filter to be rejected, got %v", err)
	}

	_, err = svc.Export(context.Background(), dto.AlumniExportRequest{WorkUnit: "山东大学"}, common.AccessContext{
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{1},
	})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected sensitive work-unit export filter to be rejected, got %v", err)
	}
}

func TestExportRejectsAdminWithoutDataDomain(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)
	_, err := svc.Export(context.Background(), dto.AlumniExportRequest{}, common.AccessContext{Role: common.RoleAdmin})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected empty data-domain scope to be rejected, got %v", err)
	}
}

func TestExportSanitizesFormulaInjection(t *testing.T) {
	formulaWorkUnit := "=HYPERLINK(\"http://evil.com\")"
	plusValue := "+SUM(A1:A10)"
	minusValue := "-SUM(A1:A10)"
	atValue := "@SUM(A1:A10)"
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{
			{
				ID:       1,
				Name:     "=cmd|'/C calc'!A0",
				Grade:    "2020级",
				WorkUnit: &formulaWorkUnit,
				Position: &plusValue,
				Mentor:   &minusValue,
				Major:    &atValue,
				Status:   "active",
			},
		},
	}
	svc := NewAlumniService(store, nil)

	// Test CSV format
	result, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{Format: "csv"})
	if err != nil {
		t.Fatalf("expected csv export success, got %v", err)
	}
	r := csv.NewReader(bytes.NewReader(result.Data[3:]))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to read csv: %v", err)
	}
	nameVal := records[1][0]
	workUnitVal := records[1][9]
	positionVal := records[1][10]
	mentorVal := records[1][5]
	majorVal := records[1][6]

	if nameVal != "'=cmd|'/C calc'!A0" {
		t.Fatalf("expected name to be escaped, got %q", nameVal)
	}
	if workUnitVal != "'=HYPERLINK(\"http://evil.com\")" {
		t.Fatalf("expected work unit to be escaped, got %q", workUnitVal)
	}
	if positionVal != "'+SUM(A1:A10)" {
		t.Fatalf("expected position + prefix to be escaped, got %q", positionVal)
	}
	if mentorVal != "'-SUM(A1:A10)" {
		t.Fatalf("expected mentor - prefix to be escaped, got %q", mentorVal)
	}
	if majorVal != "'@SUM(A1:A10)" {
		t.Fatalf("expected major @ prefix to be escaped, got %q", majorVal)
	}

	// Test XLSX format
	result, err = exportAsSuperAdmin(svc, dto.AlumniExportRequest{Format: "xlsx"})
	if err != nil {
		t.Fatalf("expected xlsx export success, got %v", err)
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
	if len(rows) < 2 {
		t.Fatal("expected header + data row")
	}
	if rows[1][0] != "'=cmd|'/C calc'!A0" {
		t.Fatalf("expected xlsx name to be escaped, got %q", rows[1][0])
	}
	if rows[1][9] != "'=HYPERLINK(\"http://evil.com\")" {
		t.Fatalf("expected xlsx work unit to be escaped, got %q", rows[1][9])
	}
}

func testDataDomainStore() *fakeImportDataDomainStore {
	return &fakeImportDataDomainStore{domains: []*model.DataDomain{
		{ID: 1, Code: common.DataDomainUndergraduate, Name: "本科生", Status: common.DataDomainStatusActive},
		{ID: 2, Code: common.DataDomainMPA, Name: "MPA专业学位研究生", Status: common.DataDomainStatusActive},
	}}
}

func TestExportIncludesDataDomainCode(t *testing.T) {
	store := &fakeAlumniStore{items: []*model.AlumniProfile{
		{ID: 1, DataDomainID: 2, Name: "张三", Grade: "2020级"},
		{ID: 2, DataDomainID: 1, Name: "李四", Grade: "2019级"},
	}}
	svc := NewAlumniService(store, nil, testDataDomainStore())

	t.Run("xlsx", func(t *testing.T) {
		result, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{Format: "xlsx"})
		if err != nil {
			t.Fatalf("expected xlsx export success, got %v", err)
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
		if len(rows) != 3 || len(rows[0]) != len(alumniImportColumnHeaders) {
			t.Fatalf("unexpected export shape: %d rows, header %d columns", len(rows), len(rows[0]))
		}
		if rows[0][len(rows[0])-1] != "所属领域" {
			t.Fatalf("expected 所属领域 header, got %q", rows[0][len(rows[0])-1])
		}
		if rows[1][len(rows[1])-1] != "mpa" || rows[2][len(rows[2])-1] != "undergraduate" {
			t.Fatalf("unexpected domain codes: %q, %q", rows[1][len(rows[1])-1], rows[2][len(rows[2])-1])
		}
	})

	t.Run("csv", func(t *testing.T) {
		result, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{Format: "csv"})
		if err != nil {
			t.Fatalf("expected csv export success, got %v", err)
		}
		records, err := csv.NewReader(bytes.NewReader(result.Data[3:])).ReadAll()
		if err != nil {
			t.Fatalf("failed to read csv: %v", err)
		}
		if len(records) != 3 || len(records[0]) != len(alumniImportColumnHeaders) {
			t.Fatalf("unexpected csv shape: %d rows, header %d columns", len(records), len(records[0]))
		}
		if records[0][len(records[0])-1] != "所属领域" || records[1][len(records[1])-1] != "mpa" {
			t.Fatalf("unexpected csv domain column: %+v", records[0])
		}
	})
}

func TestExportFiltersDataDomainAndExportsCode(t *testing.T) {
	domainID := uint64(2)
	store := &fakeAlumniStore{items: []*model.AlumniProfile{
		{ID: 1, DataDomainID: 2, Name: "张三", Grade: "2020级"},
	}}
	svc := NewAlumniService(store, nil, testDataDomainStore())

	result, err := exportAsSuperAdmin(svc, dto.AlumniExportRequest{Format: "csv", DataDomainID: &domainID})
	if err != nil {
		t.Fatalf("expected export success, got %v", err)
	}
	if len(store.query.DataDomainIDs) != 1 || store.query.DataDomainIDs[0] != domainID {
		t.Fatalf("unexpected domain filter: %+v", store.query.DataDomainIDs)
	}
	records, err := csv.NewReader(bytes.NewReader(result.Data[3:])).ReadAll()
	if err != nil {
		t.Fatalf("failed to read csv: %v", err)
	}
	if records[1][len(records[1])-1] != "mpa" {
		t.Fatalf("expected filtered row code mpa, got %q", records[1][len(records[1])-1])
	}
}

func TestExportedFileRoundTripsIntoImport(t *testing.T) {
	domains := testDataDomainStore()
	exportStore := &fakeAlumniStore{items: []*model.AlumniProfile{
		{ID: 1, DataDomainID: 2, Name: "MPA校友", Grade: "2020级"},
		{ID: 2, DataDomainID: 1, Name: "本科校友", Grade: "2019级"},
	}}
	exported, err := exportAsSuperAdmin(NewAlumniService(exportStore, nil, domains), dto.AlumniExportRequest{Format: "xlsx"})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	importStore := &fakeAlumniStore{}
	result, err := NewAlumniService(importStore, nil, domains).Import(
		context.Background(),
		common.AccessContext{UserID: 9, Role: common.RoleSuperAdmin},
		nil,
		bytes.NewReader(exported.Data),
	)
	if err != nil {
		t.Fatalf("importing the exported file should not fail, got %v", err)
	}
	if result.Total != 2 || result.Success != 2 || len(result.Errors) != 0 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if len(importStore.batchProfiles) != 2 {
		t.Fatalf("expected 2 created profiles, got %d", len(importStore.batchProfiles))
	}
	for i, want := range []uint64{2, 1} {
		got := importStore.batchProfiles[i].DataDomainID
		if got == nil || *got != want {
			t.Fatalf("row %d data domain = %v, want %d", i+1, got, want)
		}
	}
}

func TestExportedFileDedupesInsteadOfFormatError(t *testing.T) {
	domains := testDataDomainStore()
	exportStore := &fakeAlumniStore{items: []*model.AlumniProfile{
		{ID: 1, DataDomainID: 2, Name: "MPA校友", Grade: "2020级"},
	}}
	exported, err := exportAsSuperAdmin(NewAlumniService(exportStore, nil, domains), dto.AlumniExportRequest{Format: "xlsx"})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	key := do.AlumniDedupKey{Name: "MPA校友", Grade: "2020级"}.Key()
	importStore := &fakeAlumniStore{existing: map[string]bool{key: true}}
	result, err := NewAlumniService(importStore, nil, domains).Import(
		context.Background(),
		common.AccessContext{UserID: 9, Role: common.RoleSuperAdmin},
		nil,
		bytes.NewReader(exported.Data),
	)
	if err != nil {
		t.Fatalf("duplicate rows should be reported per row, got error %v", err)
	}
	if result.Success != 0 || len(result.Errors) != 1 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if !strings.Contains(result.Errors[0].Message, "已存在") {
		t.Fatalf("expected duplicate message, got %q", result.Errors[0].Message)
	}
}

func TestMaskedExportRoundTripCanCreateDuplicate(t *testing.T) {
	mobile := "13800000000"
	domains := testDataDomainStore()
	exportStore := &fakeAlumniStore{items: []*model.AlumniProfile{
		{ID: 1, DataDomainID: 2, Name: "MPA校友", Grade: "2020级", Mobile: &mobile},
	}}
	access := common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{2}}
	exported, err := NewAlumniService(exportStore, nil, domains).Export(
		context.Background(),
		dto.AlumniExportRequest{Format: "xlsx"},
		access,
	)
	if err != nil {
		t.Fatalf("masked export failed: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(exported.Data))
	if err != nil {
		t.Fatalf("open masked export: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("read masked export: %v", err)
	}
	if rows[1][13] != "" {
		t.Fatalf("expected masked mobile to be empty, got %q", rows[1][13])
	}

	// 模拟库内已有真实记录（手机号非空）。脱敏文件回灌时去重键缺少手机号，
	// 因此不会命中已有记录，会重复建档；这是文档中明确禁止脱敏文件回灌的原因。
	realKey := do.AlumniDedupKey{Name: "MPA校友", Grade: "2020级", Mobile: mobile}.Key()
	importStore := &fakeAlumniStore{existing: map[string]bool{realKey: true}}
	result, err := NewAlumniService(importStore, nil, domains).Import(
		context.Background(),
		access,
		nil,
		bytes.NewReader(exported.Data),
	)
	if err != nil {
		t.Fatalf("masked round-trip import failed: %v", err)
	}
	if result.Success != 1 || len(result.Errors) != 0 {
		t.Fatalf("expected the masked row to create a duplicate, got %+v", result)
	}
}

func TestCSVExportRoundTripsIntoImport(t *testing.T) {
	domains := testDataDomainStore()
	exportStore := &fakeAlumniStore{items: []*model.AlumniProfile{
		{ID: 1, DataDomainID: 2, Name: "=SUM(A1:A2)", Grade: "2020级", ClassName: new("Formula-Class")},
	}}
	exported, err := exportAsSuperAdmin(NewAlumniService(exportStore, nil, domains), dto.AlumniExportRequest{Format: "csv"})
	if err != nil {
		t.Fatalf("csv export failed: %v", err)
	}

	importStore := &fakeAlumniStore{}
	result, err := NewAlumniService(importStore, nil, domains).Import(
		context.Background(),
		common.AccessContext{UserID: 9, Role: common.RoleSuperAdmin},
		nil,
		bytes.NewReader(exported.Data),
	)
	if err != nil {
		t.Fatalf("CSV export should be importable, got %v", err)
	}
	if result.Success != 1 || len(importStore.batchProfiles) != 1 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if name := importStore.batchProfiles[0].Name; name != "=SUM(A1:A2)" {
		t.Fatalf("formula guard prefix was not restored: %q", name)
	}
	if domain := importStore.batchProfiles[0].DataDomainID; domain == nil || *domain != 2 {
		t.Fatalf("data domain = %v, want 2", domain)
	}
}

func TestFormulaGuardRoundTripIsSymmetric(t *testing.T) {
	domains := testDataDomainStore()
	exportStore := &fakeAlumniStore{items: []*model.AlumniProfile{
		{ID: 1, DataDomainID: 1, Name: "'=SUM(A1:A2)", Grade: "2020级"},
	}}
	exported, err := exportAsSuperAdmin(NewAlumniService(exportStore, nil, domains), dto.AlumniExportRequest{Format: "csv"})
	if err != nil {
		t.Fatalf("csv export failed: %v", err)
	}
	records, err := csv.NewReader(bytes.NewReader(exported.Data[3:])).ReadAll()
	if err != nil {
		t.Fatalf("read exported csv: %v", err)
	}
	if records[1][0] != "''=SUM(A1:A2)" {
		t.Fatalf("export should add one guard layer, got %q", records[1][0])
	}

	importStore := &fakeAlumniStore{}
	if _, err := NewAlumniService(importStore, nil, domains).Import(
		context.Background(),
		common.AccessContext{UserID: 9, Role: common.RoleSuperAdmin},
		nil,
		bytes.NewReader(exported.Data),
	); err != nil {
		t.Fatalf("import exported csv: %v", err)
	}
	if name := importStore.batchProfiles[0].Name; name != "'=SUM(A1:A2)" {
		t.Fatalf("formula guard should be symmetric, got %q", name)
	}
}
