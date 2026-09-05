package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
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
