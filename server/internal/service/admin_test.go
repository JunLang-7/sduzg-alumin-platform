package service

import (
	"context"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

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
	svc := NewAdminService(store)

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
	svc := NewAdminService(store)

	result, err := svc.Create(context.Background(), dto.AdminCreateRequest{
		Account:  " manager01 ",
		Password: "InitPass123",
		RealName: &realName,
		Mobile:   &mobile,
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
	if result.ID != 2 || result.Account != "manager01" || result.Role != common.RoleAdmin {
		t.Fatalf("unexpected create result: %+v", result)
	}
}

func TestAdminServiceCreateReturnsAccountAlreadyExists(t *testing.T) {
	store := &fakeUserStore{createErr: common.ErrAccountAlreadyExists}
	svc := NewAdminService(store)

	_, err := svc.Create(context.Background(), dto.AdminCreateRequest{
		Account:  "manager01",
		Password: "InitPass123",
	})
	if err != common.ErrAccountAlreadyExists {
		t.Fatalf("expected account already exists, got %v", err)
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
