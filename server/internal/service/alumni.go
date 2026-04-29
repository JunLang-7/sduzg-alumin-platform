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
)

type AlumniService struct {
	alumni repository.AlumniStore
	users  repository.UserStore
}

func NewAlumniService(alumni repository.AlumniStore, users repository.UserStore) *AlumniService {
	return &AlumniService{alumni: alumni, users: users}
}

// List 根据查询条件分页获取校友列表
func (s *AlumniService) List(ctx context.Context, req dto.AlumniListRequest) (common.Pager[dto.AlumniListItem], error) {
	query := req.ToQuery().Normalize()
	if s.alumni == nil {
		return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), common.ErrDatabaseUnavailable
	}

	items, total, err := s.alumni.List(ctx, query)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Error(err))
		return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), common.ErrDatabaseUnavailable
	}
	if err != nil {
		logger.Error("failed to list alumni", zap.Error(err))
		return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), err
	}

	return common.NewPager(mapAlumniListItems(items), query.Page, total), nil
}

// GetByID 根据 ID 获取校友详情
func (s *AlumniService) GetByID(ctx context.Context, id uint64) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	item, err := s.alumni.GetByID(ctx, id)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("alumni_id", id), zap.Error(err))
		return nil, common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrAlumniNotFound) {
		logger.Warn("alumni not found", zap.Uint64("alumni_id", id))
		return nil, common.ErrAlumniNotFound
	}
	if err != nil {
		logger.Error("failed to get alumni", zap.Uint64("alumni_id", id), zap.Error(err))
		return nil, err
	}

	return mapAlumniDetail(item), nil
}

// GetMe 获取当前登录校友绑定的本人资料。
func (s *AlumniService) GetMe(ctx context.Context, userID uint64) (*dto.AlumniDetail, error) {
	alumniID, err := s.currentAlumniID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.GetByID(ctx, alumniID)
}

// UpdateMe 更新当前登录校友本人允许维护的字段，并返回更新后的资料。
func (s *AlumniService) UpdateMe(ctx context.Context, userID uint64, req dto.AlumniProfileUpdateRequest) (*dto.AlumniDetail, error) {
	if s.alumni == nil {
		logger.Error("alumni repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	alumniID, err := s.currentAlumniID(ctx, userID)
	if err != nil {
		return nil, err
	}

	profile := req.ToProfile().Normalize()
	if !profile.IsEmpty() {
		if err := s.alumni.UpdateEditableFields(ctx, alumniID, userID, profile); err != nil {
			if errors.Is(err, common.ErrDatabaseUnavailable) {
				logger.Error("database is unavailable", zap.Uint64("alumni_id", alumniID), zap.Uint64("user_id", userID), zap.Error(err))
				return nil, common.ErrDatabaseUnavailable
			}
			if errors.Is(err, common.ErrAlumniNotFound) {
				logger.Warn("alumni not found", zap.Uint64("alumni_id", alumniID), zap.Uint64("user_id", userID))
				return nil, common.ErrAlumniNotFound
			}
			logger.Error("failed to update alumni profile", zap.Uint64("alumni_id", alumniID), zap.Uint64("user_id", userID), zap.Error(err))
			return nil, err
		}
	}

	return s.GetByID(ctx, alumniID)
}

// currentAlumniID 获取当前用户绑定的校友 ID。如果用户不存在、不是校友、或未绑定校友资料，返回相应错误。
func (s *AlumniService) currentAlumniID(ctx context.Context, userID uint64) (uint64, error) {
	if s.users == nil {
		logger.Error("user repository is not initialized")
		return 0, common.ErrDatabaseUnavailable
	}

	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("user_id", userID), zap.Error(err))
		return 0, common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrUserNotFound) {
		logger.Warn("current user not found", zap.Uint64("user_id", userID))
		return 0, common.ErrUserNotFound
	}
	if err != nil {
		logger.Error("failed to find current user", zap.Uint64("user_id", userID), zap.Error(err))
		return 0, err
	}
	if user.Role != "alumni" {
		logger.Warn("current user is not alumni", zap.Uint64("user_id", userID), zap.String("role", user.Role))
		return 0, common.ErrPermissionDenied
	}
	if user.AlumniID == nil || *user.AlumniID == 0 {
		logger.Warn("current user has no bound alumni profile", zap.Uint64("user_id", userID))
		return 0, common.ErrAlumniProfileUnbound
	}

	return *user.AlumniID, nil
}

// mapAlumniListItems 将 AlumniProfile 列表转换为 AlumniListItem 列表
func mapAlumniListItems(items []*model.AlumniProfile) []dto.AlumniListItem {
	result := make([]dto.AlumniListItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, dto.AlumniListItem{
			ID:           item.ID,
			Name:         item.Name,
			Grade:        item.Grade,
			ClassName:    item.ClassName,
			Cohort:       item.Cohort,
			Major:        item.Major,
			TrainingMode: item.TrainingMode,
			Industry:     item.Industry,
			WorkUnit:     item.WorkUnit,
			Position:     item.Position,
			Mobile:       item.Mobile,
			UpdatedAt:    item.UpdatedAt,
		})
	}
	return result
}

// mapAlumniDetail 将 AlumniProfile 转换为详情响应
func mapAlumniDetail(item *model.AlumniProfile) *dto.AlumniDetail {
	if item == nil {
		return nil
	}

	return &dto.AlumniDetail{
		ID:             item.ID,
		Name:           item.Name,
		Grade:          item.Grade,
		ClassName:      item.ClassName,
		Cohort:         item.Cohort,
		Counselor:      item.Counselor,
		Mentor:         item.Mentor,
		Major:          item.Major,
		TrainingMode:   item.TrainingMode,
		Industry:       item.Industry,
		WorkUnit:       item.WorkUnit,
		Position:       item.Position,
		MailingAddress: item.MailingAddress,
		Gender:         item.Gender,
		Mobile:         item.Mobile,
		Status:         item.Status,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}
