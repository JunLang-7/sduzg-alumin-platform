package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

type handlerAuditStore struct {
	query do.AuditQuery
}

func (s *handlerAuditStore) List(_ context.Context, query do.AuditQuery) ([]repository.AuditEntry, int64, error) {
	s.query = query
	return []repository.AuditEntry{}, 0, nil
}

func (s *handlerAuditStore) GetByID(context.Context, uint64) (*repository.AuditEntry, error) {
	return nil, nil
}

func TestAuditHandlerListBindsFiltersAndReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &handlerAuditStore{}
	router := gin.New()
	router.GET("/audit", NewAuditHandler(service.NewAuditService(store)).List)

	req := httptest.NewRequest(http.MethodGet, "/audit?action=update&management_scope=MPA%E4%B8%93%E4%B8%9A%E5%AD%A6%E4%BD%8D%E7%A0%94%E7%A9%B6%E7%94%9F", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("response code = %d, want 0", body.Code)
	}
	if store.query.Action != "update" || store.query.ManagementScope != "MPA专业学位研究生" {
		t.Fatalf("handler did not bind filters: %+v", store.query)
	}
}

func TestAuditHandlerDetailRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/audit/:id", NewAuditHandler(service.NewAuditService(&handlerAuditStore{})).Detail)

	req := httptest.NewRequest(http.MethodGet, "/audit/not-a-number", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}
