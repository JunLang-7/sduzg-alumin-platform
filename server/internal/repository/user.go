package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gorm"
)

type UserStore interface {
	FindByAccount(ctx context.Context, account string) (*model.User, error)
	FindByID(ctx context.Context, id uint64) (*model.User, error)
	ListAdmins(ctx context.Context, listQuery do.AdminListQuery) ([]*model.User, int64, error)
	CreateAdmin(ctx context.Context, profile do.AdminCreateProfile, passwordHash string) (*model.User, error)
	DeleteAdmin(ctx context.Context, id uint64) error
	UpdateLastLoginAt(ctx context.Context, id uint64, loggedInAt time.Time) error
	UpdatePasswordHash(ctx context.Context, id uint64, passwordHash string) error
}

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByAccount 根据账户查找用户
func (r *UserRepository) FindByAccount(ctx context.Context, account string) (*model.User, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var user model.User
	qs := query.Use(r.db).User
	err := r.db.WithContext(ctx).
		Where(qs.Account.Eq(account), qs.DeletedAt.IsNull()).
		First(&user).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdateLastLoginAt 更新用户最后登录时间
func (r *UserRepository) UpdateLastLoginAt(ctx context.Context, id uint64, loggedInAt time.Time) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).User
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where(qs.ID.Eq(id)).
		Update(qs.LastLoginAt.ColumnName().String(), loggedInAt).
		Error
}

// FindByID 根据 ID 查找用户
func (r *UserRepository) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var user model.User
	qs := query.Use(r.db).User
	err := r.db.WithContext(ctx).
		Where(qs.ID.Eq(id), qs.DeletedAt.IsNull()).
		First(&user).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// ListAdmins 分页查询管理员账号列表（admin 与 super_admin）。
func (r *UserRepository) ListAdmins(ctx context.Context, listQuery do.AdminListQuery) ([]*model.User, int64, error) {
	if r.db == nil {
		return nil, 0, common.ErrDatabaseUnavailable
	}

	listQuery = listQuery.Normalize()
	qs := query.Use(r.db).User
	db := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where(qs.DeletedAt.IsNull()).
		Where(qs.Status.Neq(common.UserStatusDeleted)).
		Where(qs.Role.In(common.RoleAdmin, common.RoleSuperAdmin))

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.User
	if err := db.
		Order(qs.ID.Asc()).
		Offset(listQuery.Page.Offset()).
		Limit(listQuery.Page.PageSize).
		Find(&items).
		Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// CreateAdmin 创建管理员账号。
func (r *UserRepository) CreateAdmin(ctx context.Context, profile do.AdminCreateProfile, passwordHash string) (*model.User, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	item := &model.User{
		Account:      profile.Account,
		PasswordHash: passwordHash,
		Role:         common.RoleAdmin,
		RealName:     profile.RealName,
		Mobile:       profile.Mobile,
		Status:       common.UserStatusActive,
	}
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, common.ErrAccountAlreadyExists
		}
		return nil, err
	}

	return item, nil
}

// DeleteAdmin 软删除管理员账号。
func (r *UserRepository) DeleteAdmin(ctx context.Context, id uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).User
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateResult := tx.Model(&model.User{}).
			Where(qs.ID.Eq(id), qs.DeletedAt.IsNull(), qs.Status.Neq(common.UserStatusDeleted), qs.Role.Eq(common.RoleAdmin)).
			Update(qs.Status.ColumnName().String(), common.UserStatusDeleted)
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected == 0 {
			return common.ErrUserNotFound
		}

		deleteResult := tx.Where(qs.ID.Eq(id), qs.DeletedAt.IsNull()).Delete(&model.User{})
		if deleteResult.Error != nil {
			return deleteResult.Error
		}
		if deleteResult.RowsAffected == 0 {
			return common.ErrUserNotFound
		}
		return nil
	})
}

// UpdatePasswordHash 更新用户密码哈希
func (r *UserRepository) UpdatePasswordHash(ctx context.Context, id uint64, passwordHash string) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).User
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where(qs.ID.Eq(id)).
		Update(qs.PasswordHash.ColumnName().String(), passwordHash).
		Error
}
