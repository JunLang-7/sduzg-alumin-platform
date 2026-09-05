package service

import (
	"context"
	"errors"
	"sort"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AdminService struct {
	users  repository.UserStore
	access repository.AccessControlStore
}

func NewAdminService(users repository.UserStore, access ...repository.AccessControlStore) *AdminService {
	service := &AdminService{users: users}
	if len(access) > 0 {
		service.access = access[0]
	}
	return service
}

// List 获取管理员账号分页列表。
func (s *AdminService) List(ctx context.Context, req dto.AdminListRequest) (common.Pager[dto.AdminListItem], error) {
	query := req.ToQuery().Normalize()
	if s.users == nil || s.access == nil {
		logger.Error("user repository is not initialized")
		return common.NewPager[dto.AdminListItem](nil, query.Page, 0), common.ErrDatabaseUnavailable
	}

	users, total, err := s.users.ListAdmins(ctx, query)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Error(err))
		return common.NewPager[dto.AdminListItem](nil, query.Page, 0), common.ErrDatabaseUnavailable
	}
	if err != nil {
		logger.Error("failed to list admins", zap.Error(err))
		return common.NewPager[dto.AdminListItem](nil, query.Page, 0), err
	}

	items, err := s.mapAdminListItems(ctx, users)
	if err != nil {
		return common.NewPager[dto.AdminListItem](nil, query.Page, 0), err
	}
	return common.NewPager(items, query.Page, total), nil
}

// Create 由超级管理员创建管理员账号。
func (s *AdminService) Create(ctx context.Context, operator common.AccessContext, req dto.AdminCreateRequest) (*dto.AdminDetail, error) {
	if !operator.IsSuperAdmin() {
		return nil, common.ErrPermissionDenied
	}
	if s.users == nil || s.access == nil {
		logger.Error("user repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	profile := req.ToProfile().Normalize()
	if profile.Account == "" || req.Password == "" {
		return nil, common.ErrInvalidRequest
	}
	domainIDs, permissions, err := s.normalizeAccess(ctx, req.DomainIDs, req.Permissions)
	if err != nil {
		return nil, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("failed to hash admin password", zap.Error(err))
		return nil, err
	}

	created, err := s.users.CreateAdminWithAccess(ctx, profile, string(passwordHash), domainIDs, permissions, operator.UserID)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Error(err))
		return nil, common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrAccountAlreadyExists) {
		logger.Warn("admin account already exists", zap.String("account", profile.Account))
		return nil, common.ErrAccountAlreadyExists
	}
	if err != nil {
		logger.Error("failed to create admin account", zap.String("account", profile.Account), zap.Error(err))
		return nil, err
	}

	return s.mapAdminDetail(ctx, created)
}

// Get 获取单个管理员及其当前数据域和权限。
func (s *AdminService) Get(ctx context.Context, id uint64) (*dto.AdminDetail, error) {
	if s.users == nil || s.access == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user.Role != common.RoleAdmin && user.Role != common.RoleSuperAdmin {
		return nil, common.ErrUserNotFound
	}
	return s.mapAdminDetail(ctx, user)
}

// ReplaceAccess 由超级管理员整体替换普通管理员的数据域与权限。
func (s *AdminService) ReplaceAccess(ctx context.Context, operator common.AccessContext, id uint64, req dto.AdminAccessUpdateRequest) (*dto.AdminDetail, error) {
	if !operator.IsSuperAdmin() {
		return nil, common.ErrPermissionDenied
	}
	if s.users == nil || s.access == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	domainIDs, permissions, err := s.normalizeAccess(ctx, req.DomainIDs, req.Permissions)
	if err != nil {
		return nil, err
	}
	updated, err := s.users.ReplaceAdminAccess(ctx, id, domainIDs, permissions, operator.UserID)
	if err != nil {
		return nil, err
	}
	return s.mapAdminDetail(ctx, updated)
}

// ListDataDomains 返回可分配的有效数据域。
func (s *AdminService) ListDataDomains(ctx context.Context) ([]dto.AdminDataDomain, error) {
	if s.access == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	domains, err := s.access.ListActiveDataDomains(ctx)
	if err != nil {
		return nil, err
	}
	return mapAdminDataDomains(domains), nil
}

// Delete 由超级管理员删除管理员账号。
func (s *AdminService) Delete(ctx context.Context, operatorID uint64, id uint64) error {
	if s.users == nil {
		logger.Error("user repository is not initialized")
		return common.ErrDatabaseUnavailable
	}
	if operatorID == id {
		return common.ErrCannotDeleteSelf
	}

	target, err := s.users.FindByID(ctx, id)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("target_user_id", id), zap.Error(err))
		return common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrUserNotFound) {
		logger.Warn("target user not found", zap.Uint64("target_user_id", id))
		return common.ErrUserNotFound
	}
	if err != nil {
		logger.Error("failed to find target user", zap.Uint64("target_user_id", id), zap.Error(err))
		return err
	}
	if target.Role == common.RoleSuperAdmin {
		return common.ErrCannotDeleteSuper
	}
	if target.Role != common.RoleAdmin {
		return common.ErrUserNotFound
	}

	if err := s.users.DeleteAdmin(ctx, id); err != nil {
		if errors.Is(err, common.ErrDatabaseUnavailable) {
			logger.Error("database is unavailable", zap.Uint64("target_user_id", id), zap.Error(err))
			return common.ErrDatabaseUnavailable
		}
		if errors.Is(err, common.ErrUserNotFound) {
			logger.Warn("target admin not found when deleting", zap.Uint64("target_user_id", id))
			return common.ErrUserNotFound
		}
		logger.Error("failed to delete admin", zap.Uint64("target_user_id", id), zap.Error(err))
		return err
	}

	return nil
}

// mapAdminListItems 将 User 模型列表转换为 AdminListItem 列表
func (s *AdminService) mapAdminListItems(ctx context.Context, users []*model.User) ([]dto.AdminListItem, error) {
	result := make([]dto.AdminListItem, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		domains, permissions, err := s.resolveAccess(ctx, user)
		if err != nil {
			return nil, err
		}
		result = append(result, dto.AdminListItem{
			ID:          user.ID,
			Account:     user.Account,
			Role:        user.Role,
			RealName:    user.RealName,
			Mobile:      user.Mobile,
			Status:      user.Status,
			LastLoginAt: user.LastLoginAt,
			CreatedAt:   user.CreatedAt,
			Domains:     domains,
			Permissions: permissions,
		})
	}
	return result, nil
}

func (s *AdminService) mapAdminDetail(ctx context.Context, user *model.User) (*dto.AdminDetail, error) {
	if user == nil {
		return nil, nil
	}
	domains, permissions, err := s.resolveAccess(ctx, user)
	if err != nil {
		return nil, err
	}

	return &dto.AdminDetail{
		ID:          user.ID,
		Account:     user.Account,
		Role:        user.Role,
		RealName:    user.RealName,
		Mobile:      user.Mobile,
		Status:      user.Status,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Domains:     domains,
		Permissions: permissions,
	}, nil
}

func (s *AdminService) resolveAccess(ctx context.Context, user *model.User) ([]dto.AdminDataDomain, []string, error) {
	if s.access == nil {
		return []dto.AdminDataDomain{}, []string{}, nil
	}
	activeDomains, err := s.access.ListActiveDataDomains(ctx)
	if err != nil {
		return nil, nil, err
	}
	if user.Role == common.RoleSuperAdmin {
		return mapAdminDataDomains(activeDomains), allAdminPermissions(), nil
	}
	domainIDs, err := s.access.ListAdminDataDomainIDs(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	permissions, err := s.access.ListAdminPermissionCodes(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	domainByID := make(map[uint64]*model.DataDomain, len(activeDomains))
	for _, domain := range activeDomains {
		if domain != nil {
			domainByID[domain.ID] = domain
		}
	}
	domains := make([]dto.AdminDataDomain, 0, len(domainIDs))
	for _, id := range domainIDs {
		if domain, ok := domainByID[id]; ok {
			domains = append(domains, dto.AdminDataDomain{ID: domain.ID, Code: domain.Code, Name: domain.Name})
		}
	}
	sort.Strings(permissions)
	return domains, permissions, nil
}

func (s *AdminService) normalizeAccess(ctx context.Context, domainIDs []uint64, permissions []string) ([]uint64, []string, error) {
	if s.access == nil {
		return nil, nil, common.ErrDatabaseUnavailable
	}
	if len(domainIDs) == 0 {
		return nil, nil, common.ErrInvalidDataDomain
	}
	activeDomains, err := s.access.ListActiveDataDomains(ctx)
	if err != nil {
		return nil, nil, err
	}
	activeIDs := make(map[uint64]struct{}, len(activeDomains))
	for _, domain := range activeDomains {
		if domain != nil {
			activeIDs[domain.ID] = struct{}{}
		}
	}
	seenDomains := make(map[uint64]struct{}, len(domainIDs))
	for _, id := range domainIDs {
		if _, exists := seenDomains[id]; exists {
			return nil, nil, common.ErrInvalidRequest
		}
		if _, active := activeIDs[id]; !active {
			return nil, nil, common.ErrInvalidDataDomain
		}
		seenDomains[id] = struct{}{}
	}
	seenPermissions := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if !common.IsKnownAdminPermission(permission) {
			return nil, nil, common.ErrInvalidRequest
		}
		if _, exists := seenPermissions[permission]; exists {
			return nil, nil, common.ErrInvalidRequest
		}
		seenPermissions[permission] = struct{}{}
	}

	normalizedDomains := append([]uint64(nil), domainIDs...)
	normalizedPermissions := append([]string(nil), permissions...)
	sort.Slice(normalizedDomains, func(i, j int) bool { return normalizedDomains[i] < normalizedDomains[j] })
	sort.Strings(normalizedPermissions)
	return normalizedDomains, normalizedPermissions, nil
}

func mapAdminDataDomains(domains []*model.DataDomain) []dto.AdminDataDomain {
	result := make([]dto.AdminDataDomain, 0, len(domains))
	for _, domain := range domains {
		if domain != nil {
			result = append(result, dto.AdminDataDomain{ID: domain.ID, Code: domain.Code, Name: domain.Name})
		}
	}
	return result
}

func allAdminPermissions() []string {
	return []string{common.PermissionAlumniFilesManage, common.PermissionAlumniSensitiveRead}
}
