package service

import (
	"context"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

type fakeAdminAccessStore struct {
	domains     []*model.DataDomain
	domainsByID map[uint64][]uint64
	permissions map[uint64][]string
	err         error
}

func (s *fakeAdminAccessStore) ListActiveDataDomains(context.Context) ([]*model.DataDomain, error) {
	return s.domains, s.err
}

func (s *fakeAdminAccessStore) ListAdminDataDomainIDs(_ context.Context, userID uint64) ([]uint64, error) {
	return append([]uint64(nil), s.domainsByID[userID]...), s.err
}

func (s *fakeAdminAccessStore) ListAdminPermissionCodes(_ context.Context, userID uint64) ([]string, error) {
	return append([]string(nil), s.permissions[userID]...), s.err
}

func activeAdminDomains() []*model.DataDomain {
	return []*model.DataDomain{
		{ID: 1, Code: common.DataDomainUndergraduate, Name: "本科生", Status: common.DataDomainStatusActive},
		{ID: 2, Code: common.DataDomainMPA, Name: "MPA专业学位研究生", Status: common.DataDomainStatusActive},
	}
}

func TestAdminServiceListMapsPagerAndItems(t *testing.T) {
	realName := "系统管理员"
	mobile := "13800000000"
	lastLoginAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	store := &fakeUserStore{
		users: []*model.User{
			{
				ID:          1,
				Account:     "admin",
				Role:        common.RoleSuperAdmin,
				RealName:    &realName,
				Mobile:      &mobile,
				Status:      common.UserStatusActive,
				LastLoginAt: &lastLoginAt,
				CreatedAt:   createdAt,
			},
		},
		total: 1,
	}
	access := &fakeAdminAccessStore{
		domains:     activeAdminDomains(),
		domainsByID: map[uint64][]uint64{2: {2}},
		permissions: map[uint64][]string{2: {common.PermissionAlumniSensitiveRead}},
	}
	svc := NewAdminService(store, access)

	pager, err := svc.List(context.Background(), dto.AdminListRequest{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("expected list success, got %v", err)
	}
	if pager.Page != 1 || pager.PageSize != 20 || pager.Total != 1 {
		t.Fatalf("unexpected pager metadata: %+v", pager)
	}
	if len(pager.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(pager.Items))
	}
	if pager.Items[0].ID != 1 || pager.Items[0].Account != "admin" || pager.Items[0].Role != common.RoleSuperAdmin {
		t.Fatalf("unexpected admin item: %+v", pager.Items[0])
	}
}

func TestAdminServiceListReturnsDatabaseUnavailable(t *testing.T) {
	svc := NewAdminService(nil)
	_, err := svc.List(context.Background(), dto.AdminListRequest{})
	if err != common.ErrDatabaseUnavailable {
		t.Fatalf("expected database unavailable, got %v", err)
	}
}

func TestAdminServiceCreateHashesPasswordAndMapsDetail(t *testing.T) {
	realName := "管理员01"
	mobile := "13800000000"
	createdAt := time.Date(2026, 4, 29, 11, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 4, 29, 11, 0, 0, 0, time.UTC)
	store := &fakeUserStore{
		created: &model.User{
			ID:        2,
			Account:   "manager01",
			Role:      common.RoleAdmin,
			RealName:  &realName,
			Mobile:    &mobile,
			Status:    common.UserStatusActive,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}
	svc := NewAdminService(store, &fakeAdminAccessStore{
		domains:     activeAdminDomains(),
		domainsByID: map[uint64][]uint64{2: {2}},
		permissions: map[uint64][]string{2: {common.PermissionAlumniSensitiveRead}},
	})

	result, err := svc.Create(context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, dto.AdminCreateRequest{
		Account:     " manager01 ",
		Password:    "InitPass123",
		RealName:    &realName,
		Mobile:      &mobile,
		DomainIDs:   []uint64{2},
		Permissions: []string{common.PermissionAlumniSensitiveRead},
	})
	if err != nil {
		t.Fatalf("expected create success, got %v", err)
	}
	if store.createProfile.Account != "manager01" {
		t.Fatalf("expected trimmed account, got %q", store.createProfile.Account)
	}
	if store.createHash == "" || store.createHash == "InitPass123" {
		t.Fatal("expected password to be hashed before persisting")
	}
	if len(store.createDomainIDs) != 1 || store.createDomainIDs[0] != 2 || len(store.createPermissions) != 1 || store.createPermissions[0] != common.PermissionAlumniSensitiveRead {
		t.Fatalf("expected access to be persisted, got domains=%v permissions=%v", store.createDomainIDs, store.createPermissions)
	}
	if result.ID != 2 || result.Account != "manager01" || result.Role != common.RoleAdmin {
		t.Fatalf("unexpected create result: %+v", result)
	}
	if len(result.Domains) != 1 || result.Domains[0].ID != 2 || len(result.Permissions) != 1 || result.Permissions[0] != common.PermissionAlumniSensitiveRead {
		t.Fatalf("expected create response to include access, got %+v", result)
	}
}

func TestAdminServiceCreateReturnsAccountAlreadyExists(t *testing.T) {
	store := &fakeUserStore{createErr: common.ErrAccountAlreadyExists}
	svc := NewAdminService(store, &fakeAdminAccessStore{domains: activeAdminDomains()})

	_, err := svc.Create(context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, dto.AdminCreateRequest{
		Account:   "manager01",
		Password:  "InitPass123",
		DomainIDs: []uint64{1},
	})
	if err != common.ErrAccountAlreadyExists {
		t.Fatalf("expected account already exists, got %v", err)
	}
}

func TestAdminServiceCreateValidatesAccessAndOperator(t *testing.T) {
	store := &fakeUserStore{}
	svc := NewAdminService(store, &fakeAdminAccessStore{domains: activeAdminDomains()})
	superAdmin := common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}
	request := dto.AdminCreateRequest{Account: "manager01", Password: "InitPass123"}

	if _, err := svc.Create(context.Background(), common.AccessContext{Role: common.RoleAdmin}, request); err != common.ErrPermissionDenied {
		t.Fatalf("expected non-super-admin to be rejected, got %v", err)
	}
	if _, err := svc.Create(context.Background(), superAdmin, request); err != common.ErrInvalidDataDomain {
		t.Fatalf("expected empty domain list to be rejected, got %v", err)
	}
	request.DomainIDs = []uint64{99}
	if _, err := svc.Create(context.Background(), superAdmin, request); err != common.ErrInvalidDataDomain {
		t.Fatalf("expected unknown domain to be rejected, got %v", err)
	}
	request.DomainIDs = []uint64{1, 1}
	if _, err := svc.Create(context.Background(), superAdmin, request); err != common.ErrInvalidRequest {
		t.Fatalf("expected duplicate domains to be rejected, got %v", err)
	}
	request.DomainIDs = []uint64{1}
	request.Permissions = []string{"unknown.permission"}
	if _, err := svc.Create(context.Background(), superAdmin, request); err != common.ErrInvalidRequest {
		t.Fatalf("expected unknown permission to be rejected, got %v", err)
	}
}

func TestAdminServiceReplaceAccessAndListExposeCurrentAuthorization(t *testing.T) {
	store := &fakeUserStore{
		usersByID: map[uint64]*model.User{
			2: {ID: 2, Account: "mpa_admin", Role: common.RoleAdmin, Status: common.UserStatusActive},
			3: {ID: 3, Account: "root", Role: common.RoleSuperAdmin, Status: common.UserStatusActive},
		},
		users: []*model.User{{ID: 2, Account: "mpa_admin", Role: common.RoleAdmin, Status: common.UserStatusActive}},
		total: 1,
	}
	access := &fakeAdminAccessStore{
		domains:     activeAdminDomains(),
		domainsByID: map[uint64][]uint64{2: {2}},
		permissions: map[uint64][]string{2: {common.PermissionAlumniSensitiveRead}},
	}
	svc := NewAdminService(store, access)

	pager, err := svc.List(context.Background(), dto.AdminListRequest{})
	if err != nil {
		t.Fatalf("expected list success, got %v", err)
	}
	if len(pager.Items) != 1 || len(pager.Items[0].Domains) != 1 || pager.Items[0].Domains[0].ID != 2 || len(pager.Items[0].Permissions) != 1 {
		t.Fatalf("expected list to expose current access, got %+v", pager.Items)
	}

	updated, err := svc.ReplaceAccess(context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, 2, dto.AdminAccessUpdateRequest{
		DomainIDs:   []uint64{2, 1},
		Permissions: []string{common.PermissionAlumniSensitiveRead, common.PermissionAlumniFilesManage},
	})
	if err != nil {
		t.Fatalf("expected access replacement success, got %v", err)
	}
	if store.replaceID != 2 || len(store.replaceDomainIDs) != 2 || store.replaceDomainIDs[0] != 1 || store.replaceDomainIDs[1] != 2 {
		t.Fatalf("expected sorted replacement domains, got %v", store.replaceDomainIDs)
	}
	if len(store.replacePermissions) != 2 || store.replacePermissions[0] != common.PermissionAlumniFilesManage || store.replacePermissions[1] != common.PermissionAlumniSensitiveRead {
		t.Fatalf("expected sorted replacement permissions, got %v", store.replacePermissions)
	}
	if updated == nil || updated.ID != 2 {
		t.Fatalf("unexpected updated admin: %+v", updated)
	}

	_, err = svc.ReplaceAccess(context.Background(), common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, 3, dto.AdminAccessUpdateRequest{DomainIDs: []uint64{1}})
	if err != common.ErrCannotModifySuper {
		t.Fatalf("expected super-admin access change to be rejected, got %v", err)
	}
}

func TestAdminServiceDeleteSuccess(t *testing.T) {
	store := &fakeUserStore{
		usersByID: map[uint64]*model.User{
			2: {ID: 2, Role: common.RoleAdmin, Status: common.UserStatusActive},
		},
	}
	svc := NewAdminService(store)

	err := svc.Delete(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("expected delete success, got %v", err)
	}
	if store.deleteID != 2 {
		t.Fatalf("expected delete target id 2, got %d", store.deleteID)
	}
}

func TestAdminServiceDeleteRejectsSelfDelete(t *testing.T) {
	svc := NewAdminService(&fakeUserStore{})
	err := svc.Delete(context.Background(), 1, 1)
	if err != common.ErrCannotDeleteSelf {
		t.Fatalf("expected cannot delete self, got %v", err)
	}
}

func TestAdminServiceDeleteRejectsSuperAdmin(t *testing.T) {
	store := &fakeUserStore{
		usersByID: map[uint64]*model.User{
			2: {ID: 2, Role: common.RoleSuperAdmin, Status: common.UserStatusActive},
		},
	}
	svc := NewAdminService(store)

	err := svc.Delete(context.Background(), 1, 2)
	if err != common.ErrCannotDeleteSuper {
		t.Fatalf("expected cannot delete super admin, got %v", err)
	}
}
