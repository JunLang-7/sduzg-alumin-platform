package service

import (
	"context"
	"errors"
	"slices"

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
func (s *DashboardService) Overview(ctx context.Context, operator common.AccessContext) (*dto.DashboardOverview, error) {
	if s.dashboard == nil {
		logger.Error("dashboard repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}
	dataDomainIDs, err := dashboardDataDomainIDs(operator)
	if err != nil {
		return nil, err
	}

	stats, err := s.dashboard.Overview(ctx, dataDomainIDs)
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

// Distribution 获取指定维度的校友分布统计。
func (s *DashboardService) Distribution(ctx context.Context, req dto.DashboardDistributionRequest, operator common.AccessContext) ([]dto.DashboardDistributionItem, error) {
	if s.dashboard == nil {
		logger.Error("dashboard repository is not initialized")
		return nil, common.ErrDatabaseUnavailable
	}

	query := req.ToQuery().Normalize()
	if !query.Valid() {
		return nil, common.ErrInvalidRequest
	}
	dataDomainIDs, err := dashboardDataDomainIDs(operator)
	if err != nil {
		return nil, err
	}
	query.DataDomainIDs = dataDomainIDs
	query = query.Normalize()

	items, err := s.dashboard.Distribution(ctx, query)
	if errors.Is(err, common.ErrDatabaseUnavailable) {
		logger.Error("database is unavailable", zap.String("dimension", query.Dimension), zap.Error(err))
		return nil, common.ErrDatabaseUnavailable
	}
	if errors.Is(err, common.ErrInvalidRequest) {
		return nil, common.ErrInvalidRequest
	}
	if err != nil {
		logger.Error("failed to get dashboard distribution", zap.String("dimension", query.Dimension), zap.Error(err))
		return nil, err
	}

	return mapDashboardDistributionItems(items), nil
}

// dashboardDataDomainIDs 将管理员授权上下文转换为统计查询范围。
// nil 表示超级管理员的全域范围；普通管理员必须至少拥有一个数据域。
func dashboardDataDomainIDs(operator common.AccessContext) ([]uint64, error) {
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	if operator.IsSuperAdmin() {
		return nil, nil
	}
	if len(operator.DomainIDs) == 0 {
		return nil, common.ErrPermissionDenied
	}
	domainIDs := slices.Clone(operator.DomainIDs)
	slices.Sort(domainIDs)
	return slices.Compact(domainIDs), nil
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

func mapDashboardDistributionItems(items []do.DashboardDistributionItem) []dto.DashboardDistributionItem {
	result := make([]dto.DashboardDistributionItem, 0, len(items))
	for _, item := range items {
		result = append(result, dto.DashboardDistributionItem{
			Name:  item.Name,
			Value: item.Value,
		})
	}
	return result
}
