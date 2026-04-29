package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/gin-gonic/gin"
)

type fakeRoleUserStore struct {
	user *model.User
	err  error
}

func (s fakeRoleUserStore) FindByID(context.Context, uint64) (*model.User, error) {
	return s.user, s.err
}

func TestRequireRolesAllowsAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(CurrentUserIDKey, uint64(1))
		c.Next()
	})
	engine.Use(RequireRoles(fakeRoleUserStore{
		user: &model.User{ID: 1, Role: common.RoleAdmin, Status: common.UserStatusActive},
	}, common.RoleAdmin, common.RoleSuperAdmin))
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
		c.Set(CurrentUserIDKey, uint64(1))
		c.Next()
	})
	engine.Use(RequireRoles(fakeRoleUserStore{
		user: &model.User{ID: 1, Role: common.RoleAlumni, Status: common.UserStatusActive},
	}, common.RoleAdmin, common.RoleSuperAdmin))
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

func TestRequireRolesMapsDatabaseUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(CurrentUserIDKey, uint64(1))
		c.Next()
	})
	engine.Use(RequireRoles(fakeRoleUserStore{
		err: common.ErrDatabaseUnavailable,
	}, common.RoleAdmin, common.RoleSuperAdmin))
	engine.GET("/admin/alumni", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/alumni", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}
