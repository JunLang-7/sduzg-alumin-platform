package repository

import (
	"context"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gorm"
)

// AccessControlStore reads the data ranges and permission codes assigned to administrators.
// Mutation methods are deliberately added with the administrator-management feature.
type AccessControlStore interface {
	ListActiveDataDomains(ctx context.Context) ([]*model.DataDomain, error)
	ListAdminDataDomainIDs(ctx context.Context, userID uint64) ([]uint64, error)
	ListAdminPermissionCodes(ctx context.Context, userID uint64) ([]string, error)
}

// AccessControlRepository persists administrator data scopes and permission codes.
type AccessControlRepository struct {
	db *gorm.DB
}

func NewAccessControlRepository(db *gorm.DB) *AccessControlRepository {
	return &AccessControlRepository{db: db}
}

// ListActiveDataDomains returns assignable alumni data domains in display order.
func (r *AccessControlRepository) ListActiveDataDomains(ctx context.Context) ([]*model.DataDomain, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).DataDomain
	var domains []*model.DataDomain
	if err := r.db.WithContext(ctx).
		Where(qs.Status.Eq(common.DataDomainStatusActive)).
		Order(qs.SortOrder.Asc()).
		Order(qs.ID.Asc()).
		Find(&domains).
		Error; err != nil {
		return nil, err
	}
	return domains, nil
}

// ListAdminDataDomainIDs returns every domain assigned to one administrator.
func (r *AccessControlRepository) ListAdminDataDomainIDs(ctx context.Context, userID uint64) ([]uint64, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AdminDataScope
	var scopes []*model.AdminDataScope
	if err := r.db.WithContext(ctx).
		Where(qs.UserID.Eq(userID)).
		Order(qs.DataDomainID.Asc()).
		Find(&scopes).
		Error; err != nil {
		return nil, err
	}

	ids := make([]uint64, 0, len(scopes))
	for _, scope := range scopes {
		if scope != nil {
			ids = append(ids, scope.DataDomainID)
		}
	}
	return ids, nil
}

// ListAdminPermissionCodes returns every permission code assigned to one administrator.
func (r *AccessControlRepository) ListAdminPermissionCodes(ctx context.Context, userID uint64) ([]string, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AdminPermission
	var permissions []*model.AdminPermission
	if err := r.db.WithContext(ctx).
		Where(qs.UserID.Eq(userID)).
		Order(qs.PermissionCode.Asc()).
		Find(&permissions).
		Error; err != nil {
		return nil, err
	}

	codes := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if permission != nil {
			codes = append(codes, permission.PermissionCode)
		}
	}
	return codes, nil
}
