package service

import (
	"bytes"
	"context"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/xuri/excelize/v2"
)

func buildXLSXReader(headers []string, rows [][]string) (*bytes.Reader, error) {
	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, err
	}

	headerVals := make([]interface{}, len(headers))
	for i, h := range headers {
		headerVals[i] = h
	}
	if err := sw.SetRow("A1", headerVals); err != nil {
		return nil, err
	}

	for i, row := range rows {
		vals := make([]interface{}, len(row))
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

func TestImportAllValidRows(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "李老师", "王教授", "公共管理", "非全日制", "政府", "山东大学", "主任", "济南市", "男", "13800000000"},
		{"李四", "2021级", "2021级MPA", "2021", "赵老师", "钱教授", "工商管理", "全日制", "金融", "银行", "经理", "青岛市", "女", "13900000000"},
		{"王五", "2022级", "", "", "", "", "", "", "", "", "", "", "", ""},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Import(context.Background(), 1, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected total 3, got %d", result.Total)
	}
	if result.Success != 3 {
		t.Fatalf("expected success 3, got %d", result.Success)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected 0 errors, got %+v", result.Errors)
	}
}

func TestImportKeepsRowsWithDifferentMobile(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13800000000"},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13900000000"},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Import(context.Background(), 1, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Success != 2 {
		t.Fatalf("expected success 2 for different mobile values, got %d with errors %+v", result.Success, result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no duplicate errors, got %+v", result.Errors)
	}
}

func TestImportDeduplicatesRowsWithSameMobile(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13800000000"},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13800000000"},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Import(context.Background(), 1, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("expected success 1 for duplicate mobile values, got %d with errors %+v", result.Success, result.Errors)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 duplicate error, got %+v", result.Errors)
	}
}

func TestImportDeduplicatesRowsWithPaddedMobile(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", "13800000000"},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", " 13800000000 "},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Import(context.Background(), 1, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("expected success 1 for padded duplicate mobile values, got %d with errors %+v", result.Success, result.Errors)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 duplicate error, got %+v", result.Errors)
	}
}

func TestImportDeduplicatesRowsWithEmptyMobile(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", ""},
		{"张三", "2020级", "2020级MPA", "2020", "", "", "", "", "", "", "", "", "", " "},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Import(context.Background(), 1, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("expected success 1 for empty duplicate mobile values, got %d with errors %+v", result.Success, result.Errors)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 duplicate error, got %+v", result.Errors)
	}
}

func TestImportPartialErrors(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级"},
		{"", "2021级"},
		{"王五", ""},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Import(context.Background(), 1, reader)
	if err != nil {
		t.Fatalf("expected import success (partial), got %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected total 3, got %d", result.Total)
	}
	if result.Success != 1 {
		t.Fatalf("expected success 1, got %d", result.Success)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(result.Errors))
	}
}

func TestImportEmptyFile(t *testing.T) {
	headers := alumniColumnHeaders
	reader, err := buildXLSXReader(headers, nil)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, nil)

	_, err = svc.Import(context.Background(), 1, reader)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestImportHeaderMismatch(t *testing.T) {
	badHeaders := []string{"名称", "年级"}
	reader, err := buildXLSXReader(badHeaders, [][]string{{"张三", "2020级"}})
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, nil)

	_, err = svc.Import(context.Background(), 1, reader)
	if err == nil {
		t.Fatal("expected error for header mismatch")
	}
}

func TestImportDatabaseUnavailable(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{{"张三", "2020级"}}
	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	svc := NewAlumniService(nil, nil, nil)

	_, err = svc.Import(context.Background(), 1, reader)
	if err != common.ErrDatabaseUnavailable {
		t.Fatalf("expected database unavailable, got %v", err)
	}
}

func TestParseRowToProfile(t *testing.T) {
	row := []string{"张三", "2020级", "2020级MPA", "2020", "李老师", "王教授", "公共管理", "非全日制", "政府", "山东大学", "主任", "济南市", "男", "13800000000"}
	profile := parseRowToProfile(row)
	profile = profile.Normalize()

	if profile.Name != "张三" {
		t.Fatalf("expected Name 张三, got %q", profile.Name)
	}
	if profile.Grade != "2020级" {
		t.Fatalf("expected Grade 2020级, got %q", profile.Grade)
	}
	if profile.ClassName == nil || *profile.ClassName != "2020级MPA" {
		t.Fatalf("expected ClassName 2020级MPA, got %v", profile.ClassName)
	}
	if profile.Mobile == nil || *profile.Mobile != "13800000000" {
		t.Fatalf("expected Mobile 13800000000, got %v", profile.Mobile)
	}
	if profile.Status != common.AlumniStatusActive {
		t.Fatalf("expected Status active, got %q", profile.Status)
	}
}

func TestParseRowToProfileEmptyOptionalFields(t *testing.T) {
	row := []string{"王五", "2022级"}
	profile := parseRowToProfile(row)
	profile = profile.Normalize()

	if profile.Name != "王五" {
		t.Fatalf("expected Name 王五, got %q", profile.Name)
	}
	if profile.ClassName != nil {
		t.Fatalf("expected nil ClassName, got %v", *profile.ClassName)
	}
	if profile.Mobile != nil {
		t.Fatalf("expected nil Mobile, got %v", *profile.Mobile)
	}
}

func TestParseRowToProfileShortRow(t *testing.T) {
	row := []string{"测试"}
	profile := parseRowToProfile(row)
	profile = profile.Normalize()

	if profile.Name != "测试" {
		t.Fatalf("expected Name 测试, got %q", profile.Name)
	}
	if profile.Grade != "" {
		t.Fatalf("expected empty Grade, got %q", profile.Grade)
	}
}

func TestImportResultErrorsIncludeRowNumbers(t *testing.T) {
	headers := alumniColumnHeaders
	rows := [][]string{
		{"张三", "2020级"},
		{"", "2021级"},
		{"李四", ""},
		{"王五", ""},
	}

	reader, err := buildXLSXReader(headers, rows)
	if err != nil {
		t.Fatalf("failed to build xlsx: %v", err)
	}

	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, nil)

	result, err := svc.Import(context.Background(), 1, reader)
	if err != nil {
		t.Fatalf("expected import success, got %v", err)
	}

	if len(result.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(result.Errors))
	}
	if result.Errors[0].Row != 3 {
		t.Fatalf("expected first error at row 3, got %d", result.Errors[0].Row)
	}
	if result.Errors[1].Row != 4 {
		t.Fatalf("expected second error at row 4, got %d", result.Errors[1].Row)
	}
	if result.Errors[2].Row != 5 {
		t.Fatalf("expected third error at row 5, got %d", result.Errors[2].Row)
	}
}
