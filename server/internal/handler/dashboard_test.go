package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

type fakeDashboardStore struct {
	distributionItems []do.DashboardDistributionItem
}

func (s *fakeDashboardStore) Overview(_ context.Context) (do.DashboardOverviewStats, error) {
	return do.DashboardOverviewStats{}, nil
}

func (s *fakeDashboardStore) Distribution(_ context.Context, _ do.DashboardDistributionQuery) ([]do.DashboardDistributionItem, error) {
	return s.distributionItems, nil
}

func TestDashboardDistributionRejectsMissingDimension(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	handler := NewDashboardHandler(service.NewDashboardService(&fakeDashboardStore{}))
	engine.GET("/dashboard/distribution", handler.Distribution)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/distribution", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40000`) {
		t.Fatalf("expected bad request response, got %s", rec.Body.String())
	}
}

func TestDashboardDistributionRejectsInvalidDimension(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	handler := NewDashboardHandler(service.NewDashboardService(&fakeDashboardStore{}))
	engine.GET("/dashboard/distribution", handler.Distribution)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/distribution?dimension=name", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"message":"invalid dimension"`) {
		t.Fatalf("expected invalid dimension response, got %s", rec.Body.String())
	}
}

func TestDashboardDistributionReturnsItems(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	handler := NewDashboardHandler(service.NewDashboardService(&fakeDashboardStore{
		distributionItems: []do.DashboardDistributionItem{
			{Name: "2020级", Value: 12},
		},
	}))
	engine.GET("/dashboard/distribution", handler.Distribution)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/distribution?dimension=grade", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"name":"2020级"`) || !strings.Contains(rec.Body.String(), `"value":12`) {
		t.Fatalf("expected distribution response, got %s", rec.Body.String())
	}
}
