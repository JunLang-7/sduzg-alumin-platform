package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

type fakeExportStore struct {
	stubAlumniStore
	rows []do.AlumniExportRow
	err  error
}

func (s *fakeExportStore) ListAll(context.Context, do.AlumniListQuery) ([]do.AlumniExportRow, error) {
	return s.rows, s.err
}

func (s *fakeExportStore) CountActive(context.Context) (int64, error) {
	return int64(len(s.rows)), s.err
}

func exportTestRows(name, grade string) []do.AlumniExportRow {
	profile := &model.AlumniProfile{ID: 1, Name: name, Status: common.AlumniStatusActive, Version: 1}
	return []do.AlumniExportRow{{
		Profile: profile,
		Enrollment: &model.AlumniEnrollment{
			ID: 11, AlumniID: 1, DataDomainID: 1, TrainingLevel: common.TrainingLevelUndergraduate,
			Grade: grade, Status: common.AlumniStatusActive, Version: 1,
		},
	}}
}

func TestExportHandlerXlsxSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &fakeExportStore{rows: exportTestRows("张三", "2020级")}
	h := NewAlumniHandler(service.NewAlumniService(store, nil))

	engine := gin.New()
	engine.GET("/admin/alumni/export", withExportAccess(h.Export))

	req := httptest.NewRequest(http.MethodGet, "/admin/alumni/export", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("expected xlsx content type, got %q", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if cd == "" {
		t.Fatal("expected Content-Disposition header")
	}
	if len(rec.Body.Bytes()) == 0 {
		t.Fatal("expected non-empty body")
	}
}

func TestExportHandlerCsvSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &fakeExportStore{rows: exportTestRows("李四", "2021级")}
	h := NewAlumniHandler(service.NewAlumniService(store, nil))

	engine := gin.New()
	engine.GET("/admin/alumni/export", withExportAccess(h.Export))

	req := httptest.NewRequest(http.MethodGet, "/admin/alumni/export?format=csv", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/csv; charset=utf-8" {
		t.Fatalf("expected csv content type, got %q", ct)
	}
}

func TestExportHandlerDatabaseUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &fakeExportStore{
		err: common.ErrDatabaseUnavailable,
	}
	h := NewAlumniHandler(service.NewAlumniService(store, nil))

	engine := gin.New()
	engine.GET("/admin/alumni/export", withExportAccess(h.Export))

	req := httptest.NewRequest(http.MethodGet, "/admin/alumni/export", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func withExportAccess(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.AccessContextKey, &common.AccessContext{Role: common.RoleSuperAdmin})
		handler(c)
	}
}
