package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/gin-gonic/gin"
)

type fakeAccessContextUserStore struct {
	user  *model.User
	err   error
	calls int
}

func (s *fakeAccessContextUserStore) FindByID(context.Context, uint64) (*model.User, error) {
	s.calls++
	return s.user, s.err
}

type fakeAccessContextStore struct {
	activeDomains     []*model.DataDomain
	activeDomainsErr  error
	domainIDs         []uint64
	permissionCodes   []string
	domainErr         error
	permissionErr     error
	activeDomainCalls int
	domainCalls       int
	permissionCalls   int
}

func (s *fakeAccessContextStore) ListActiveDataDomains(context.Context) ([]*model.DataDomain, error) {
	s.activeDomainCalls++
	return s.activeDomains, s.activeDomainsErr
}

func (s *fakeAccessContextStore) ListAdminDataDomainIDs(context.Context, uint64) ([]uint64, error) {
	s.domainCalls++
	return s.domainIDs, s.domainErr
}

func (s *fakeAccessContextStore) ListAdminPermissionCodes(context.Context, uint64) ([]string, error) {
	s.permissionCalls++
	return s.permissionCodes, s.permissionErr
}

func TestAccessContextLoaderLoadsAdminAssignments(t *testing.T) {
	users := &fakeAccessContextUserStore{user: &model.User{ID: 7, Role: common.RoleAdmin, Status: common.UserStatusActive}}
	accessControl := &fakeAccessContextStore{
		activeDomains:   []*model.DataDomain{{ID: 2}, {ID: 5}},
		domainIDs:       []uint64{2, 5},
		permissionCodes: []string{common.PermissionAlumniSensitiveRead},
	}
	loader := NewAccessContextLoader(users, accessControl)

	access, err := loader.Load(context.Background(), 7)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !access.CanAccessDomain(2) || !access.CanAccessDomain(5) || access.CanAccessDomain(3) {
		t.Fatalf("unexpected domain access result: %+v", access)
	}
	if !access.HasPermission(common.PermissionAlumniSensitiveRead) || access.HasPermission(common.PermissionAlumniFilesManage) {
		t.Fatalf("unexpected permission result: %+v", access)
	}
	if users.calls != 1 || accessControl.activeDomainCalls != 1 || accessControl.domainCalls != 1 || accessControl.permissionCalls != 1 {
		t.Fatalf("unexpected load calls: users=%d active_domains=%d scopes=%d permissions=%d", users.calls, accessControl.activeDomainCalls, accessControl.domainCalls, accessControl.permissionCalls)
	}
}

func TestAccessContextLoaderSuperAdminIsImplicitlyAuthorized(t *testing.T) {
	users := &fakeAccessContextUserStore{user: &model.User{ID: 1, Role: common.RoleSuperAdmin, Status: common.UserStatusActive}}
	accessControl := &fakeAccessContextStore{}
	loader := NewAccessContextLoader(users, accessControl)

	access, err := loader.Load(context.Background(), 1)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !access.CanAccessDomain(9) || !access.HasPermission(common.PermissionAlumniFilesManage) {
		t.Fatalf("unexpected super admin access: %+v", access)
	}
	if accessControl.activeDomainCalls != 0 || accessControl.domainCalls != 0 || accessControl.permissionCalls != 0 {
		t.Fatalf("super admin should not load assignments: %+v", accessControl)
	}
}

func TestAccessContextLoaderRejectsInactiveOrMissingUser(t *testing.T) {
	tests := []struct {
		name string
		user *model.User
		err  error
		want error
	}{
		{
			name: "disabled",
			user: &model.User{ID: 7, Role: common.RoleAdmin, Status: common.UserStatusDisabled},
			want: common.ErrAccountDisabled,
		},
		{
			name: "deleted",
			err:  common.ErrUserNotFound,
			want: common.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeAccessContextUserStore{user: tt.user, err: tt.err}
			accessControl := &fakeAccessContextStore{}
			loader := NewAccessContextLoader(users, accessControl)

			_, err := loader.Load(context.Background(), 7)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Load() error = %v, want %v", err, tt.want)
			}
			if accessControl.activeDomainCalls != 0 || accessControl.domainCalls != 0 || accessControl.permissionCalls != 0 {
				t.Fatalf("inactive or missing user must not load assignments: %+v", accessControl)
			}
		})
	}
}

func TestLoadAccessContextFailsClosedAndLoadsOncePerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	users := &fakeAccessContextUserStore{user: &model.User{ID: 7, Role: common.RoleAdmin, Status: common.UserStatusActive}}
	accessControl := &fakeAccessContextStore{activeDomains: []*model.DataDomain{{ID: 2}}, domainIDs: []uint64{2}}
	loader := NewAccessContextLoader(users, accessControl)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(CurrentUserIDKey, uint64(7))
		c.Next()
	})
	engine.Use(LoadAccessContext(loader))
	engine.Use(RequireRoles(common.RoleAdmin))
	engine.GET("/admin/alumni", func(c *gin.Context) {
		first, firstOK := CurrentAccessContext(c)
		second, secondOK := CurrentAccessContext(c)
		if !firstOK || !secondOK || first != second {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/alumni", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if users.calls != 1 || accessControl.activeDomainCalls != 1 || accessControl.domainCalls != 1 || accessControl.permissionCalls != 1 {
		t.Fatalf("expected one context load, got users=%d active_domains=%d scopes=%d permissions=%d", users.calls, accessControl.activeDomainCalls, accessControl.domainCalls, accessControl.permissionCalls)
	}

	failing := gin.New()
	failing.Use(func(c *gin.Context) {
		c.Set(CurrentUserIDKey, uint64(7))
		c.Next()
	})
	failing.Use(LoadAccessContext(NewAccessContextLoader(
		&fakeAccessContextUserStore{user: &model.User{ID: 7, Role: common.RoleAdmin, Status: common.UserStatusActive}},
		&fakeAccessContextStore{activeDomainsErr: errors.New("database read failed")},
	)))
	failing.GET("/admin/alumni", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	rec = httptest.NewRecorder()
	failing.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/alumni", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected failure to be denied with status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestAccessContextLoaderExcludesDisabledDataDomain(t *testing.T) {
	users := &fakeAccessContextUserStore{user: &model.User{ID: 7, Role: common.RoleAdmin, Status: common.UserStatusActive}}
	accessControl := &fakeAccessContextStore{
		activeDomains: []*model.DataDomain{{ID: 2}},
		domainIDs:     []uint64{2, 5},
	}
	loader := NewAccessContextLoader(users, accessControl)

	access, err := loader.Load(context.Background(), 7)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !access.CanAccessDomain(2) || access.CanAccessDomain(5) {
		t.Fatalf("disabled domain must be excluded: %+v", access)
	}
}
