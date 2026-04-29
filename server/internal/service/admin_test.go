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
