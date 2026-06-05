package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

type fakeExportStore struct {
	items []*model.AlumniProfile
	err   error
}

func (s *fakeExportStore) List(_ context.Context, _ do.AlumniListQuery) ([]*model.AlumniProfile, int64, error) {
	return nil, 0, nil
}

func (s *fakeExportStore) ListAll(_ context.Context, _ do.AlumniListQuery) ([]*model.AlumniProfile, error) {
	return s.items, s.err
}

func (s *fakeExportStore) GetByID(_ context.Context, _ uint64) (*model.AlumniProfile, error) {
	return nil, common.ErrAlumniNotFound
}

func (s *fakeExportStore) Create(_ context.Context, _ *do.AlumniCreateProfile, _ uint64) (*model.AlumniProfile, error) {
	return nil, nil
}

func (s *fakeExportStore) Update(_ context.Context, _ uint64, _ uint64, _ do.AlumniUpdateProfile) error {
	return nil
}

func (s *fakeExportStore) Delete(_ context.Context, _ uint64, _ uint64) error {
	return nil
}

func (s *fakeExportStore) BatchCreate(_ context.Context, _ []do.AlumniCreateProfile, _ uint64) error {
	return nil
}

func (s *fakeExportStore) FindExistingByDedupKey(_ context.Context, _ []do.AlumniDedupKey) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (s *fakeExportStore) UpdateEditableFields(_ context.Context, _ uint64, _ uint64, _ do.AlumniEditableProfile) error {
	return nil
}

func TestExportHandlerXlsxSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &fakeExportStore{
		items: []*model.AlumniProfile{
			{ID: 1, Name: "张三", Grade: "2020级", Status: common.AlumniStatusActive},
		},
	}
	h := NewAlumniHandler(service.NewAlumniService(store, nil))

	engine := gin.New()
	engine.GET("/admin/alumni/export", h.Export)

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

	store := &fakeExportStore{
		items: []*model.AlumniProfile{
			{ID: 1, Name: "李四", Grade: "2021级", Status: common.AlumniStatusActive},
		},
	}
	h := NewAlumniHandler(service.NewAlumniService(store, nil))

	engine := gin.New()
	engine.GET("/admin/alumni/export", h.Export)

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
	engine.GET("/admin/alumni/export", h.Export)

	req := httptest.NewRequest(http.MethodGet, "/admin/alumni/export", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}
