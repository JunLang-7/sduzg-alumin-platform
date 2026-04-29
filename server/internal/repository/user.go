package repository

import (
	"context"
	"errors"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gorm"
)

var (
	ErrDatabaseUnavailable = errors.New("database unavailable")
	ErrCacheUnavailable    = errors.New("cache unavailable")
	ErrUserNotFound        = errors.New("user not found")
)

type UserStore interface {
	FindByAccount(ctx context.Context, account string) (*model.User, error)
	FindByID(ctx context.Context, id uint64) (*model.User, error)
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
		return nil, ErrDatabaseUnavailable
	}

	var user model.User
	qs := query.Use(r.db).User
	err := r.db.WithContext(ctx).
		Where(qs.Account.Eq(account), qs.DeletedAt.IsNull()).
		First(&user).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdateLastLoginAt 更新用户最后登录时间
func (r *UserRepository) UpdateLastLoginAt(ctx context.Context, id uint64, loggedInAt time.Time) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
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
		return nil, ErrDatabaseUnavailable
	}

	var user model.User
	qs := query.Use(r.db).User
	err := r.db.WithContext(ctx).
		Where(qs.ID.Eq(id), qs.DeletedAt.IsNull()).
		First(&user).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdatePasswordHash 更新用户密码哈希
func (r *UserRepository) UpdatePasswordHash(ctx context.Context, id uint64, passwordHash string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).User
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where(qs.ID.Eq(id)).
		Update(qs.PasswordHash.ColumnName().String(), passwordHash).
		Error
}
