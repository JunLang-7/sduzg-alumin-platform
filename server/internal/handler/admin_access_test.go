package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

func TestAdminAccessHandlersRejectMissingOrNonSuperAdminContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAdminHandler(service.NewAdminService(nil))

	engine := gin.New()
	engine.POST("/admins", handler.Create)
	engine.PUT("/admins/:id/access", func(c *gin.Context) {
		c.Set(middleware.AccessContextKey, &common.AccessContext{UserID: 2, Role: common.RoleAdmin})
		handler.ReplaceAccess(c)
	})

	createReq := httptest.NewRequest(http.MethodPost, "/admins", bytes.NewBufferString(`{"account":"manager01","password":"InitPass123","domain_ids":[1]}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	engine.ServeHTTP(createRecorder, createReq)
	if createRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("create without access context status = %d, want %d", createRecorder.Code, http.StatusUnauthorized)
	}

	replaceReq := httptest.NewRequest(http.MethodPut, "/admins/3/access", bytes.NewBufferString(`{"domain_ids":[1]}`))
	replaceReq.Header.Set("Content-Type", "application/json")
	replaceRecorder := httptest.NewRecorder()
	engine.ServeHTTP(replaceRecorder, replaceReq)
	if replaceRecorder.Code != http.StatusForbidden {
		t.Fatalf("replace by non-super-admin status = %d, want %d", replaceRecorder.Code, http.StatusForbidden)
	}
}
