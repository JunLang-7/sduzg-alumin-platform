package service

import (
	"context"
	"errors"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"go.uber.org/zap"
)

type DashboardService struct {
	dashboard repository.DashboardStore
}

func NewDashboardService(dashboard repository.DashboardStore) *DashboardService {
	return &DashboardService{dashboard: dashboard}
}

// Overview 获取数据大屏总览指标。
func (s *DashboardService) Overview(ctx context.Context) (*dto.DashboardOverview, error) {
	if s.dashboard == nil {
		logger.Error("dashboard repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	stats, err := s.dashboard.Overview(ctx)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.Error(err))
		return nil, common.ErrDatabaseUnavailable
	}
	if err != nil {
		logger.Error("failed to get dashboard overview", zap.Error(err))
		return nil, err
	}

	return mapDashboardOverview(stats), nil
}

func mapDashboardOverview(stats do.DashboardOverviewStats) *dto.DashboardOverview {
	return &dto.DashboardOverview{
		TotalAlumni:          stats.TotalAlumni,
		TotalAccounts:        stats.TotalAccounts,
		MobileCompleteRate:   completionRate(stats.MobileComplete, stats.TotalAlumni),
		WorkUnitCompleteRate: completionRate(stats.WorkUnitComplete, stats.TotalAlumni),
		MentorCompleteRate:   completionRate(stats.MentorComplete, stats.TotalAlumni),
	}
}

func completionRate(completed int64, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(completed) / float64(total)
}
