package service

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
)

// buildSparseWorkbook 生成一个只在 Excel 允许的末尾行号写入数据的合法工作簿。
// 读取端若不提前止损，会物化上百万行空行。
func buildSparseWorkbook(t *testing.T) *bytes.Reader {
	t.Helper()

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	for i, header := range alumniColumnHeaders {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			t.Fatalf("coordinates for header %d: %v", i, err)
		}
		if err := f.SetCellValue("Sheet1", cell, header); err != nil {
			t.Fatalf("set header %d: %v", i, err)
		}
	}
	if err := f.SetCellValue("Sheet1", "A1048576", "稀疏行校友"); err != nil {
		t.Fatalf("set last row: %v", err)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	return bytes.NewReader(buf.Bytes())
}

func buildWorkbookWithDataRows(t *testing.T, dataRows int) *bytes.Reader {
	t.Helper()

	rows := make([][]string, 0, dataRows)
	for i := range dataRows {
		rows = append(rows, []string{fmt.Sprintf("校友%05d", i), "2020级"})
	}
	reader, err := buildXLSXReader(alumniColumnHeaders, rows)
	if err != nil {
		t.Fatalf("build xlsx with %d rows: %v", dataRows, err)
	}
	return reader
}

// 上限内的文件必须整表读完，边界值不能被误判。
func TestReadImportWorkbookRowsAcceptsRowLimit(t *testing.T) {
	rows, err := readImportWorkbookRows(readAll(t, buildWorkbookWithDataRows(t, maxImportDataRows)))
	if err != nil {
		t.Fatalf("expected the row limit to be accepted, got %v", err)
	}
	if want := maxImportDataRows + 1; len(rows) != want {
		t.Fatalf("parsed %d rows, want %d", len(rows), want)
	}
}

// 超出上限一行即拒绝，并且要能在循环中判出来（不依赖后续是否还有行）。
func TestReadImportWorkbookRowsRejectsRowLimitExceeded(t *testing.T) {
	_, err := readImportWorkbookRows(readAll(t, buildWorkbookWithDataRows(t, maxImportDataRows+1)))
	if !errors.Is(err, common.ErrImportRowLimit) {
		t.Fatalf("expected row limit error, got %v", err)
	}
}

// 数据写在末尾行号的稀疏工作簿必须在读取阶段就被拦住，而不是先物化上百万行。
func TestReadImportWorkbookRowsStopsAtRowLimitOnSparseWorkbook(t *testing.T) {
	_, err := readImportWorkbookRows(readAll(t, buildSparseWorkbook(t)))
	if !errors.Is(err, common.ErrImportRowLimit) {
		t.Fatalf("expected row limit error for sparse workbook, got %v", err)
	}
}

// 导入接口对超限文件返回可识别的业务错误，而不是通用内部错误。
func TestImportRejectsSparseWorkbookOverRowLimit(t *testing.T) {
	_, err := importAsSuperAdmin(NewAlumniService(&fakeAlumniStore{}, nil), buildSparseWorkbook(t))
	if !errors.Is(err, common.ErrImportRowLimit) {
		t.Fatalf("expected row limit error from Import, got %v", err)
	}
}

func readAll(t *testing.T, reader *bytes.Reader) []byte {
	t.Helper()

	data := make([]byte, reader.Len())
	if _, err := reader.Read(data); err != nil {
		t.Fatalf("read workbook: %v", err)
	}
	return data
}
