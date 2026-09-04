package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/gin-gonic/gin"
)

func TestRequireRolesAllowsAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(AccessContextKey, &common.AccessContext{UserID: 1, Role: common.RoleAdmin})
		c.Next()
	})
	engine.Use(RequireRoles(common.RoleAdmin, common.RoleSuperAdmin))
	engine.GET("/admin/alumni", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/alumni", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestRequireRolesRejectsAlumni(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(AccessContextKey, &common.AccessContext{UserID: 1, Role: common.RoleAlumni})
		c.Next()
	})
	engine.Use(RequireRoles(common.RoleAdmin, common.RoleSuperAdmin))
	engine.GET("/admin/alumni", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/alumni", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestRequireRolesRejectsMissingAccessContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(RequireRoles(common.RoleAdmin, common.RoleSuperAdmin))
	engine.GET("/admin/alumni", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/alumni", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}
