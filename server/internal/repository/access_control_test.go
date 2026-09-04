package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
)

func TestAccessControlRepositoryReturnsDatabaseUnavailableWithoutDB(t *testing.T) {
	t.Parallel()

	repo := NewAccessControlRepository(nil)
	ctx := context.Background()

	if _, err := repo.ListActiveDataDomains(ctx); !errors.Is(err, common.ErrDatabaseUnavailable) {
		t.Fatalf("ListActiveDataDomains() error = %v, want database unavailable", err)
	}
	if _, err := repo.ListAdminDataDomainIDs(ctx, 1); !errors.Is(err, common.ErrDatabaseUnavailable) {
		t.Fatalf("ListAdminDataDomainIDs() error = %v, want database unavailable", err)
	}
	if _, err := repo.ListAdminPermissionCodes(ctx, 1); !errors.Is(err, common.ErrDatabaseUnavailable) {
		t.Fatalf("ListAdminPermissionCodes() error = %v, want database unavailable", err)
	}
}
