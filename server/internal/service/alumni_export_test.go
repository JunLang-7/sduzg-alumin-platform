package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/xuri/excelize/v2"
)

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
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Export(context.Background(), dto.AlumniExportRequest{Format: "xlsx"})
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
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Export(context.Background(), dto.AlumniExportRequest{Format: "csv"})
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
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Export(context.Background(), dto.AlumniExportRequest{})
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
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Export(context.Background(), dto.AlumniExportRequest{})
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
	svc := NewAlumniService(store, nil, nil)

	_, err := svc.Export(context.Background(), dto.AlumniExportRequest{
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
	svc := NewAlumniService(store, nil, nil)

	// Test CSV format
	result, err := svc.Export(context.Background(), dto.AlumniExportRequest{Format: "csv"})
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
	result, err = svc.Export(context.Background(), dto.AlumniExportRequest{Format: "xlsx"})
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

