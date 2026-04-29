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
