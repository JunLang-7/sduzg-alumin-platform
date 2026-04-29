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
}

var ErrAlumniNotFound = errors.New("alumni not found")

func NewAlumniService(alumni repository.AlumniStore) *AlumniService {
	return &AlumniService{alumni: alumni}
}

// List 根据查询条件分页获取校友列表
func (s *AlumniService) List(ctx context.Context, req dto.AlumniListRequest) (common.Pager[dto.AlumniListItem], error) {
	query := req.ToQuery().Normalize()
	if s.alumni == nil {
		return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), ErrDatabaseUnavailable
	}

	items, total, err := s.alumni.List(ctx, query)
	if errors.Is(err, repository.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Error(err))
		return common.NewPager[dto.AlumniListItem](nil, query.Page, 0), ErrDatabaseUnavailable
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
		return nil, ErrDatabaseUnavailable
	}

	item, err := s.alumni.GetByID(ctx, id)
	if errors.Is(err, repository.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Uint64("alumni_id", id), zap.Error(err))
		return nil, ErrDatabaseUnavailable
	}
	if errors.Is(err, repository.ErrAlumniNotFound) {
		logger.Warn("alumni not found", zap.Uint64("alumni_id", id))
		return nil, ErrAlumniNotFound
	}
	if err != nil {
		logger.Error("failed to get alumni", zap.Uint64("alumni_id", id), zap.Error(err))
		return nil, err
	}

	return mapAlumniDetail(item), nil
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
