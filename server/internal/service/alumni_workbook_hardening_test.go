package service

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
)

// oversizedZipEntry 生成一个 zip 流，其中唯一条目声明解压后为 2^63 字节，实际只写入
// 1 字节。对应上游公告 GHSA-fw94-4wwp-w8pw：Zip64 声明异常的解压尺寸会让
// excelize.OpenReader 在分配缓冲时 panic（makeslice: cap out of range）。
func oversizedZipEntry(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name:               "huge.bin",
		Method:             zip.Store,
		UncompressedSize64: 1 << 63,
		CompressedSize64:   1,
	})
	if err != nil {
		t.Fatalf("create raw entry: %v", err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// TestExcelizeOpenReaderRejectsOversizedZipEntry 是钉住依赖版本的回归测试：公告
// GHSA-fw94-4wwp-w8pw 描述的 panic 只在上游 master 修复，尚未发布正式版本，所以
// go.mod 临时固定到修复提交。若后续把 excelize 降级回不含修复的版本，本测试会以
// panic 失败，而不是让导入接口在线上退化。
func TestExcelizeOpenReaderRejectsOversizedZipEntry(t *testing.T) {
	data := oversizedZipEntry(t)

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("excelize.OpenReader panicked on a crafted workbook: %v", recovered)
		}
	}()

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err == nil {
		_ = f.Close()
		t.Fatal("expected an error for an entry declaring an oversized uncompressed size")
	}
}

// TestImportRejectsOversizedZipEntryWorkbook 确认导入路径对同一构造输入返回业务
// 错误，既不 panic 也不会把畸形文件当成有效工作簿继续解析。
func TestImportRejectsOversizedZipEntryWorkbook(t *testing.T) {
	reader := bytes.NewReader(oversizedZipEntry(t))

	_, err := importAsSuperAdmin(NewAlumniService(&fakeAlumniStore{}, nil), reader)
	if err != common.ErrInvalidRequest {
		t.Fatalf("expected invalid request for crafted workbook, got %v", err)
	}
}
