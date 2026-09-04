package repository

import (
	"context"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gorm"
)

// AccessControlStore 查询管理员已分配的数据范围和权限码。
// 写入能力将在管理员授权管理功能中补充。
type AccessControlStore interface {
	ListActiveDataDomains(ctx context.Context) ([]*model.DataDomain, error)
	ListAdminDataDomainIDs(ctx context.Context, userID uint64) ([]uint64, error)
	ListAdminPermissionCodes(ctx context.Context, userID uint64) ([]string, error)
}

// AccessControlRepository 负责管理员数据范围和权限码的数据访问。
type AccessControlRepository struct {
	db *gorm.DB
}

func NewAccessControlRepository(db *gorm.DB) *AccessControlRepository {
	return &AccessControlRepository{db: db}
}

// ListActiveDataDomains 按展示顺序返回可分配的校友数据域。
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

// ListAdminDataDomainIDs 返回某管理员被分配的全部数据域 ID。
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

// ListAdminPermissionCodes 返回某管理员被分配的全部权限码。
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
