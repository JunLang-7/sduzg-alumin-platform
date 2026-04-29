package service

import (
	"context"
	"errors"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AdminService struct {
	users repository.UserStore
}

func NewAdminService(users repository.UserStore) *AdminService {
	return &AdminService{users: users}
}

// List 获取管理员账号分页列表。
func (s *AdminService) List(ctx context.Context, req dto.AdminListRequest) (common.Pager[dto.AdminListItem], error) {
	query := req.ToQuery().Normalize()
	if s.users == nil {
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

	return common.NewPager(mapAdminListItems(users), query.Page, total), nil
}

// Create 由超级管理员创建管理员账号。
func (s *AdminService) Create(ctx context.Context, req dto.AdminCreateRequest) (*dto.AdminDetail, error) {
	if s.users == nil {
		logger.Error("user repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	profile := req.ToProfile().Normalize()
	if profile.Account == "" || req.Password == "" {
		return nil, common.ErrInvalidRequest
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("failed to hash admin password", zap.Error(err))
		return nil, err
	}

	created, err := s.users.CreateAdmin(ctx, profile, string(passwordHash))
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

	return mapAdminDetail(created), nil
}

// mapAdminListItems 将 User 模型列表转换为 AdminListItem 列表
func mapAdminListItems(users []*model.User) []dto.AdminListItem {
	result := make([]dto.AdminListItem, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
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
		})
	}
	return result
}

func mapAdminDetail(user *model.User) *dto.AdminDetail {
	if user == nil {
		return nil
	}

	return &dto.AdminDetail{
		ID:        user.ID,
		Account:   user.Account,
		Role:      user.Role,
		RealName:  user.RealName,
		Mobile:    user.Mobile,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
