package repository

import (
	"context"
	"encoding/json"
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
	FindByMobile(ctx context.Context, mobile string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByAlumniID(ctx context.Context, alumniID uint64) (*model.User, error)
	FindByID(ctx context.Context, id uint64) (*model.User, error)
	ListAdmins(ctx context.Context, listQuery do.AdminListQuery) ([]*model.User, int64, error)
	CreateAdmin(ctx context.Context, profile do.AdminCreateProfile, passwordHash string) (*model.User, error)
	CreateAdminWithAccess(ctx context.Context, profile do.AdminCreateProfile, passwordHash string, domainIDs []uint64, permissions []string, operatorID uint64) (*model.User, error)
	ReplaceAdminAccess(ctx context.Context, id uint64, domainIDs []uint64, permissions []string, operatorID uint64) (*model.User, error)
	DeleteAdmin(ctx context.Context, id uint64) error
	UpdateLastLoginAt(ctx context.Context, id uint64, loggedInAt time.Time) error
	UpdatePasswordHash(ctx context.Context, id uint64, passwordHash string) error
	UpdateMobile(ctx context.Context, id uint64, mobile string) error
	UpdateEmail(ctx context.Context, id uint64, email string) error
	CreateUser(ctx context.Context, user *model.User) error
}

type adminAccessAuditDetail struct {
	Before *adminAccessSnapshot `json:"before,omitempty"`
	After  adminAccessSnapshot  `json:"after"`
}

type adminAccessSnapshot struct {
	DomainIDs   []uint64 `json:"domain_ids"`
	Permissions []string `json:"permissions"`
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

// FindByMobile 通过手机号查找用户（忽略软删除）
func (r *UserRepository) FindByMobile(ctx context.Context, mobile string) (*model.User, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var user model.User
	qs := query.Use(r.db).User
	err := r.db.WithContext(ctx).
		Where(qs.Mobile.Eq(mobile), qs.DeletedAt.IsNull()).
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

// FindByEmail 通过邮箱查找用户（忽略软删除）。
// Email 在写入时已统一转为小写，查询时直接精确匹配以利用索引。
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	lowerEmail := strings.ToLower(email)
	var user model.User
	qs := query.Use(r.db).User
	err := r.db.WithContext(ctx).
		Where(qs.Email.Eq(lowerEmail), qs.DeletedAt.IsNull()).
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

// CreateAdminWithAccess 在同一事务中创建管理员、写入授权映射和审计日志。
func (r *UserRepository) CreateAdminWithAccess(ctx context.Context, profile do.AdminCreateProfile, passwordHash string, domainIDs []uint64, permissions []string, operatorID uint64) (*model.User, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var created *model.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateActiveDataDomains(tx, domainIDs); err != nil {
			return err
		}
		item := &model.User{
			Account:      profile.Account,
			PasswordHash: passwordHash,
			Role:         common.RoleAdmin,
			RealName:     profile.RealName,
			Mobile:       profile.Mobile,
			Status:       common.UserStatusActive,
		}
		if err := tx.Create(item).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				return common.ErrAccountAlreadyExists
			}
			return err
		}
		if err := replaceAdminAccessRecords(tx, item.ID, domainIDs, permissions); err != nil {
			return err
		}
		if err := writeAdminAccessAuditLog(tx, operatorID, "create_admin_access", item.ID, nil, adminAccessSnapshot{DomainIDs: domainIDs, Permissions: permissions}); err != nil {
			return err
		}
		created = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// ReplaceAdminAccess 整体替换普通管理员的数据域和权限，并记录变更前后快照。
func (r *UserRepository) ReplaceAdminAccess(ctx context.Context, id uint64, domainIDs []uint64, permissions []string, operatorID uint64) (*model.User, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var updated *model.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateActiveDataDomains(tx, domainIDs); err != nil {
			return err
		}
		var user model.User
		if err := tx.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrUserNotFound
			}
			return err
		}
		if user.Role == common.RoleSuperAdmin {
			return common.ErrCannotModifySuper
		}
		if user.Role != common.RoleAdmin {
			return common.ErrUserNotFound
		}

		before, err := loadAdminAccessSnapshot(tx, id)
		if err != nil {
			return err
		}
		if err := replaceAdminAccessRecords(tx, id, domainIDs, permissions); err != nil {
			return err
		}
		if err := writeAdminAccessAuditLog(tx, operatorID, "replace_admin_access", id, &before, adminAccessSnapshot{DomainIDs: domainIDs, Permissions: permissions}); err != nil {
			return err
		}
		updated = &user
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func validateActiveDataDomains(tx *gorm.DB, domainIDs []uint64) error {
	if len(domainIDs) == 0 {
		return common.ErrInvalidDataDomain
	}
	var count int64
	if err := tx.Model(&model.DataDomain{}).
		Where("id IN ? AND status = ?", domainIDs, common.DataDomainStatusActive).
		Count(&count).
		Error; err != nil {
		return err
	}
	if count != int64(len(domainIDs)) {
		return common.ErrInvalidDataDomain
	}
	return nil
}

func replaceAdminAccessRecords(tx *gorm.DB, userID uint64, domainIDs []uint64, permissions []string) error {
	if err := tx.Where("user_id = ?", userID).Delete(&model.AdminDataScope{}).Error; err != nil {
		return err
	}
	if err := tx.Where("user_id = ?", userID).Delete(&model.AdminPermission{}).Error; err != nil {
		return err
	}
	for _, domainID := range domainIDs {
		if err := tx.Create(&model.AdminDataScope{UserID: userID, DataDomainID: domainID}).Error; err != nil {
			return err
		}
	}
	for _, permission := range permissions {
		if err := tx.Create(&model.AdminPermission{UserID: userID, PermissionCode: permission}).Error; err != nil {
			return err
		}
	}
	return nil
}

func loadAdminAccessSnapshot(tx *gorm.DB, userID uint64) (adminAccessSnapshot, error) {
	snapshot := adminAccessSnapshot{DomainIDs: []uint64{}, Permissions: []string{}}
	var scopes []model.AdminDataScope
	if err := tx.Where("user_id = ?", userID).Order("data_domain_id ASC").Find(&scopes).Error; err != nil {
		return snapshot, err
	}
	for _, scope := range scopes {
		snapshot.DomainIDs = append(snapshot.DomainIDs, scope.DataDomainID)
	}
	var permissions []model.AdminPermission
	if err := tx.Where("user_id = ?", userID).Order("permission_code ASC").Find(&permissions).Error; err != nil {
		return snapshot, err
	}
	for _, permission := range permissions {
		snapshot.Permissions = append(snapshot.Permissions, permission.PermissionCode)
	}
	return snapshot, nil
}

func writeAdminAccessAuditLog(tx *gorm.DB, operatorID uint64, action string, targetID uint64, before *adminAccessSnapshot, after adminAccessSnapshot) error {
	detail, err := json.Marshal(adminAccessAuditDetail{Before: before, After: after})
	if err != nil {
		return err
	}
	detailText := string(detail)
	return tx.Create(&model.OperationLog{
		OperatorID:   operatorID,
		OperatorRole: common.RoleSuperAdmin,
		Action:       action,
		TargetType:   "admin_access",
		TargetID:     &targetID,
		Detail:       &detailText,
	}).Error
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

// FindByAlumniID 通过校友ID查找关联用户
func (r *UserRepository) FindByAlumniID(ctx context.Context, alumniID uint64) (*model.User, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var user model.User
	qs := query.Use(r.db).User
	err := r.db.WithContext(ctx).
		Where(qs.AlumniID.Eq(alumniID), qs.DeletedAt.IsNull()).
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

// CreateUser 创建新用户记录
func (r *UserRepository) CreateUser(ctx context.Context, user *model.User) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}
	return r.db.WithContext(ctx).Create(user).Error
}

// UpdateMobile 更新用户手机号
func (r *UserRepository) UpdateMobile(ctx context.Context, id uint64, mobile string) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}
	qs := query.Use(r.db).User
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where(qs.ID.Eq(id)).
		Update(qs.Mobile.ColumnName().String(), mobile).
		Error
}

// UpdateEmail 更新用户邮箱（小写）
func (r *UserRepository) UpdateEmail(ctx context.Context, id uint64, email string) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}
	qs := query.Use(r.db).User
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where(qs.ID.Eq(id)).
		Update(qs.Email.ColumnName().String(), strings.ToLower(email)).
		Error
}
