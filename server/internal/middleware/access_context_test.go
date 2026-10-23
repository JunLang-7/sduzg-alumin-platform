package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"
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
	domainStarted     chan struct{}
	permissionStarted chan struct{}
	release           <-chan struct{}
}

func (s *fakeAccessContextStore) ListActiveDataDomains(context.Context) ([]*model.DataDomain, error) {
	s.activeDomainCalls++
	return s.activeDomains, s.activeDomainsErr
}

func (s *fakeAccessContextStore) ListAdminDataDomainIDs(context.Context, uint64) ([]uint64, error) {
	s.domainCalls++
	if s.domainStarted != nil {
		close(s.domainStarted)
	}
	if s.release != nil {
		<-s.release
	}
	return s.domainIDs, s.domainErr
}

func (s *fakeAccessContextStore) ListAdminPermissionCodes(context.Context, uint64) ([]string, error) {
	s.permissionCalls++
	if s.permissionStarted != nil {
		close(s.permissionStarted)
	}
	if s.release != nil {
		<-s.release
	}
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

func TestAccessContextLoaderReflectsUpdatedAssignmentsOnNextRequest(t *testing.T) {
	users := &fakeAccessContextUserStore{user: &model.User{ID: 7, Role: common.RoleAdmin, Status: common.UserStatusActive}}
	accessControl := &fakeAccessContextStore{
		activeDomains:   []*model.DataDomain{{ID: 2}, {ID: 5}},
		domainIDs:       []uint64{2},
		permissionCodes: []string{common.PermissionAlumniSensitiveRead},
	}
	loader := NewAccessContextLoader(users, accessControl)

	first, err := loader.Load(context.Background(), 7)
	if err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	if !first.CanAccessDomain(2) || !first.HasPermission(common.PermissionAlumniSensitiveRead) {
		t.Fatalf("unexpected first access context: %+v", first)
	}

	accessControl.domainIDs = []uint64{5}
	accessControl.permissionCodes = []string{common.PermissionAlumniFilesManage}
	next, err := loader.Load(context.Background(), 7)
	if err != nil {
		t.Fatalf("next Load() error = %v", err)
	}
	if next.CanAccessDomain(2) || !next.CanAccessDomain(5) || next.HasPermission(common.PermissionAlumniSensitiveRead) || !next.HasPermission(common.PermissionAlumniFilesManage) {
		t.Fatalf("next request did not reflect updated assignments: %+v", next)
	}
}

func TestAccessContextLoaderLoadsAssignmentsConcurrently(t *testing.T) {
	release := make(chan struct{})
	accessControl := &fakeAccessContextStore{
		activeDomains:     []*model.DataDomain{{ID: 2}},
		domainIDs:         []uint64{2},
		permissionCodes:   []string{common.PermissionAlumniSensitiveRead},
		domainStarted:     make(chan struct{}),
		permissionStarted: make(chan struct{}),
		release:           release,
	}
	loader := NewAccessContextLoader(
		&fakeAccessContextUserStore{user: &model.User{ID: 7, Role: common.RoleAdmin, Status: common.UserStatusActive}},
		accessControl,
	)

	done := make(chan error, 1)
	go func() {
		_, err := loader.Load(context.Background(), 7)
		done <- err
	}()

	for _, started := range []<-chan struct{}{accessControl.domainStarted, accessControl.permissionStarted} {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("independent authorization queries were not started concurrently")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Load() error = %v", err)
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
			name: "nil user",
			want: common.ErrUserNotFound,
		},
		{
			name: "wrong identity",
			user: &model.User{ID: 8, Role: common.RoleSuperAdmin, Status: common.UserStatusActive},
			want: common.ErrUserNotFound,
		},
		{
			name: "soft deleted",
			user: &model.User{ID: 7, Role: common.RoleSuperAdmin, Status: common.UserStatusActive, DeletedAt: gorm.DeletedAt{Valid: true}},
			want: common.ErrUserNotFound,
		},
		{
			name: "unknown role",
			user: &model.User{ID: 7, Role: "owner", Status: common.UserStatusActive},
			want: common.ErrPermissionDenied,
		},
		{
			name: "empty role",
			user: &model.User{ID: 7, Status: common.UserStatusActive},
			want: common.ErrPermissionDenied,
		},
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

func TestAccessContextLoaderLogsUnknownDatabaseRole(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	previous := logger.L()
	logger.SetLogger(zap.New(core))
	t.Cleanup(func() { logger.SetLogger(previous) })
	users := &fakeAccessContextUserStore{user: &model.User{
		ID: 7, Role: "Admin", Status: common.UserStatusActive, Account: "private-account", PasswordHash: "must-not-be-logged",
	}}
	_, err := NewAccessContextLoader(users, nil).Load(context.Background(), 7)
	if !errors.Is(err, common.ErrPermissionDenied) {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := logs.FilterMessage("unknown database role").All()
	if len(entries) != 1 {
		t.Fatalf("expected one structured warning, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if len(fields) != 2 || fields["user_id"] != uint64(7) || fields["role"] != "Admin" {
		t.Fatalf("warning must contain only user_id and the persisted role: %+v", fields)
	}
}

func TestAccessContextLoaderRejectsZeroIdentityWithoutQuery(t *testing.T) {
	users := &fakeAccessContextUserStore{}
	_, err := NewAccessContextLoader(users, nil).Load(context.Background(), 0)
	if !errors.Is(err, common.ErrUserNotFound) || users.calls != 0 {
		t.Fatalf("zero identity must fail before query: err=%v calls=%d", err, users.calls)
	}
}

func TestUnknownDatabaseRoleCannotReachAuthenticatedHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) { c.Set(CurrentUserIDKey, uint64(7)) })
	engine.Use(LoadAccessContext(NewAccessContextLoader(
		&fakeAccessContextUserStore{user: &model.User{ID: 7, Role: "owner", Status: common.UserStatusActive}}, nil,
	)))
	engine.GET("/history", func(c *gin.Context) { t.Error("unknown role reached protected handler") })
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/history?role=super_admin", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unknown role status=%d, want 403: %s", rec.Code, rec.Body.String())
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
