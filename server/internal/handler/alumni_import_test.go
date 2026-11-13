package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func buildMultipartXLSX(t *testing.T, rows [][]string) (*bytes.Buffer, string) {
	t.Helper()

	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		t.Fatalf("create stream writer: %v", err)
	}

	headers := []string{"姓名", "年级", "班级", "届数", "辅导员", "导师", "专业", "培养方式", "行业", "工作单位", "职务", "通讯地址", "性别", "手机号", "邮箱"}
	headerVals := make([]any, len(headers))
	for i, h := range headers {
		headerVals[i] = h
	}
	if err := sw.SetRow("A1", headerVals); err != nil {
		t.Fatalf("write header: %v", err)
	}

	for i, row := range rows {
		vals := make([]any, len(row))
		for j, v := range row {
			vals[j] = v
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := sw.SetRow(cell, vals); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}

	if err := sw.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}

	return &buf, "alumni_import.xlsx"
}

type fakeImportStore struct {
	stubAlumniStore
}

type stubDataDomainStore struct{}

func (stubDataDomainStore) ListActiveDataDomains(context.Context) ([]*model.DataDomain, error) {
	return []*model.DataDomain{
		{ID: 1, Code: common.DataDomainUndergraduate, Name: "本科生", Status: common.DataDomainStatusActive},
		{ID: 2, Code: common.DataDomainMPA, Name: "MPA专业学位研究生", Status: common.DataDomainStatusActive},
	}, nil
}

func (stubDataDomainStore) ListDataDomains(ctx context.Context) ([]*model.DataDomain, error) {
	return stubDataDomainStore{}.ListActiveDataDomains(ctx)
}

func (stubDataDomainStore) ListAdminDataDomainIDs(context.Context, uint64) ([]uint64, error) {
	return nil, nil
}

func (stubDataDomainStore) ListAdminPermissionCodes(context.Context, uint64) ([]string, error) {
	return nil, nil
}

func newImportTestService(store *fakeImportStore) *service.AlumniService {
	return service.NewAlumniService(store, nil, stubDataDomainStore{})
}

func TestImportHandlerSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &fakeImportStore{}
	svc := newImportTestService(store)
	h := NewAlumniHandler(svc)

	engine := gin.New()
	engine.POST("/admin/alumni/import", func(c *gin.Context) {
		c.Set(middleware.AccessContextKey, &common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin})
		h.Import(c)
	})

	buf, filename := buildMultipartXLSX(t, [][]string{{"张三", "2020级"}})

	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"data_domain_id\"\r\n\r\n")
	body.WriteString("1\r\n")
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"")
	body.WriteString(filename)
	body.WriteString("\"\r\n")
	body.WriteString("Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet\r\n\r\n")
	body.Write(buf.Bytes())
	body.WriteString("\r\n--boundary--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/admin/alumni/import", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestImportHandlerUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &fakeImportStore{}
	svc := newImportTestService(store)
	h := NewAlumniHandler(svc)

	engine := gin.New()
	engine.POST("/admin/alumni/import", h.Import)

	buf, filename := buildMultipartXLSX(t, [][]string{{"张三", "2020级"}})

	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"")
	body.WriteString(filename)
	body.WriteString("\"\r\n")
	body.WriteString("Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet\r\n\r\n")
	body.Write(buf.Bytes())
	body.WriteString("\r\n--boundary--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/admin/alumni/import", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestImportHandlerBadFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &fakeImportStore{}
	svc := newImportTestService(store)
	h := NewAlumniHandler(svc)

	engine := gin.New()
	engine.POST("/admin/alumni/import", func(c *gin.Context) {
		c.Set(middleware.AccessContextKey, &common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin})
		h.Import(c)
	})

	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"test.txt\"\r\n")
	body.WriteString("Content-Type: text/plain\r\n\r\n")
	body.WriteString("this is not an xlsx file")
	body.WriteString("\r\n--boundary--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/admin/alumni/import", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestImportHandlerReportsMissingDataDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := newImportTestService(&fakeImportStore{})
	h := NewAlumniHandler(svc)

	engine := gin.New()
	engine.POST("/admin/alumni/import", func(c *gin.Context) {
		c.Set(middleware.AccessContextKey, &common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin})
		h.Import(c)
	})

	buf, filename := buildMultipartXLSX(t, [][]string{{"张三", "2020级"}})
	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"")
	body.WriteString(filename)
	body.WriteString("\"\r\n")
	body.WriteString("Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet\r\n\r\n")
	body.Write(buf.Bytes())
	body.WriteString("\r\n--boundary--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/admin/alumni/import", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "未指定培养类别") {
		t.Fatalf("expected actionable message, got %s", rec.Body.String())
	}
}
