package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
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

	headers := []string{"姓名", "年级", "班级", "届数", "辅导员", "导师", "专业", "培养方式", "行业", "工作单位", "职务", "通讯地址", "性别", "手机号"}
	headerVals := make([]interface{}, len(headers))
	for i, h := range headers {
		headerVals[i] = h
	}
	if err := sw.SetRow("A1", headerVals); err != nil {
		t.Fatalf("write header: %v", err)
	}

	for i, row := range rows {
		vals := make([]interface{}, len(row))
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

type fakeImportStore struct{}

func (s *fakeImportStore) List(_ context.Context, _ do.AlumniListQuery) ([]*model.AlumniProfile, int64, error) {
	return nil, 0, nil
}

func (s *fakeImportStore) ListAll(_ context.Context, _ do.AlumniListQuery) ([]*model.AlumniProfile, error) {
	return nil, nil
}

func (s *fakeImportStore) GetByID(_ context.Context, _ uint64) (*model.AlumniProfile, error) {
	return nil, common.ErrAlumniNotFound
}

func (s *fakeImportStore) Create(_ context.Context, _ *do.AlumniCreateProfile, _ uint64) (*model.AlumniProfile, error) {
	return nil, nil
}

func (s *fakeImportStore) BatchCreate(_ context.Context, _ []do.AlumniCreateProfile, _ uint64) error {
	return nil
}

func (s *fakeImportStore) FindExistingByDedupKey(_ context.Context, _ []do.AlumniDedupKey) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (s *fakeImportStore) Update(_ context.Context, _ uint64, _ uint64, _ do.AlumniUpdateProfile) error {
	return nil
}

func (s *fakeImportStore) Delete(_ context.Context, _ uint64, _ uint64) error { return nil }

func (s *fakeImportStore) UpdateEditableFields(_ context.Context, _ uint64, _ uint64, _ do.AlumniEditableProfile) error {
	return nil
}

func (s *fakeImportStore) CountActive(_ context.Context) (int64, error) { return 0, nil }

func (s *fakeImportStore) FindOnly(_ context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, error) {
	return nil, nil
}

type fakeImportUserStore struct{}

func (s *fakeImportUserStore) FindByAccount(_ context.Context, _ string) (*model.User, error) {
	return nil, common.ErrUserNotFound
}

func (s *fakeImportUserStore) FindByID(_ context.Context, _ uint64) (*model.User, error) {
	return &model.User{ID: 1, Role: common.RoleAdmin, Status: common.UserStatusActive}, nil
}

func (s *fakeImportUserStore) ListAdmins(_ context.Context, _ do.AdminListQuery) ([]*model.User, int64, error) {
	return nil, 0, nil
}

func (s *fakeImportUserStore) CreateAdmin(_ context.Context, _ do.AdminCreateProfile, _ string) (*model.User, error) {
	return nil, nil
}

func (s *fakeImportUserStore) DeleteAdmin(_ context.Context, _ uint64) error { return nil }

func (s *fakeImportUserStore) UpdateLastLoginAt(_ context.Context, _ uint64, _ time.Time) error {
	return nil
}

func (s *fakeImportUserStore) UpdatePasswordHash(_ context.Context, _ uint64, _ string) error {
	return nil
}

func TestImportHandlerSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &fakeImportStore{}
	users := &fakeImportUserStore{}
	svc := service.NewAlumniService(store, users, nil)
	h := NewAlumniHandler(svc)

	engine := gin.New()
	engine.POST("/admin/alumni/import", func(c *gin.Context) {
		c.Set(middleware.CurrentUserIDKey, uint64(1))
		h.Import(c)
	})

	buf, filename := buildMultipartXLSX(t, [][]string{{"张三", "2020级"}})

	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"");body.WriteString(filename);body.WriteString("\"\r\n")
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
	svc := service.NewAlumniService(store, nil, nil)
	h := NewAlumniHandler(svc)

	engine := gin.New()
	engine.POST("/admin/alumni/import", h.Import)

	buf, filename := buildMultipartXLSX(t, [][]string{{"张三", "2020级"}})

	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"");body.WriteString(filename);body.WriteString("\"\r\n")
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
	svc := service.NewAlumniService(store, nil, nil)
	h := NewAlumniHandler(svc)

	engine := gin.New()
	engine.POST("/admin/alumni/import", func(c *gin.Context) {
		c.Set(middleware.CurrentUserIDKey, uint64(1))
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
