package repository

import (
	"context"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gorm"
)

type DashboardStore interface {
	Overview(ctx context.Context, dataDomainIDs []uint64) (do.DashboardOverviewStats, error)
	Distribution(ctx context.Context, query do.DashboardDistributionQuery) ([]do.DashboardDistributionItem, error)
}

type DashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

// Overview 获取数据大屏总览统计。
func (r *DashboardRepository) Overview(ctx context.Context, dataDomainIDs []uint64) (do.DashboardOverviewStats, error) {
	if r.db == nil {
		return do.DashboardOverviewStats{}, common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AlumniProfile
	var stats do.DashboardOverviewStats
	alumniStatsQuery := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Select(`
			COUNT(*) AS total_alumni,
			COALESCE(SUM(CASE WHEN mobile IS NOT NULL AND TRIM(mobile) <> '' THEN 1 ELSE 0 END), 0) AS mobile_complete,
			COALESCE(SUM(CASE WHEN work_unit IS NOT NULL AND TRIM(work_unit) <> '' THEN 1 ELSE 0 END), 0) AS work_unit_complete,
			COALESCE(SUM(CASE WHEN mentor IS NOT NULL AND TRIM(mentor) <> '' THEN 1 ELSE 0 END), 0) AS mentor_complete
		`).
		Where(qs.DeletedAt.IsNull()).
		Where(qs.Status.Eq(common.AlumniStatusActive))
	if len(dataDomainIDs) > 0 {
		alumniStatsQuery = alumniStatsQuery.Where(qs.DataDomainID.In(dataDomainIDs...))
	}
	if err := alumniStatsQuery.Scan(&stats).Error; err != nil {
		return do.DashboardOverviewStats{}, err
	}

	accountStatsQuery := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Joins(
			"JOIN users AS u ON u.alumni_id = alumni_profiles.id AND u.deleted_at IS NULL AND u.status = ? AND u.role = ?",
			common.UserStatusActive,
			common.RoleAlumni,
		).
		Where(qs.DeletedAt.IsNull()).
		Where(qs.Status.Eq(common.AlumniStatusActive)).
		Where("u.alumni_id IS NOT NULL").
		Distinct("u.alumni_id")
	if len(dataDomainIDs) > 0 {
		accountStatsQuery = accountStatsQuery.Where(qs.DataDomainID.In(dataDomainIDs...))
	}
	if err := accountStatsQuery.Count(&stats.TotalAccounts).Error; err != nil {
		return do.DashboardOverviewStats{}, err
	}

	return stats, nil
}

// Distribution 获取指定维度的校友分布统计。
func (r *DashboardRepository) Distribution(ctx context.Context, dashboardQuery do.DashboardDistributionQuery) ([]do.DashboardDistributionItem, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	dashboardQuery = dashboardQuery.Normalize()
	column, ok := dashboardDistributionColumn(dashboardQuery.Dimension)
	if !ok {
		return nil, common.ErrInvalidRequest
	}

	nameExpr := "COALESCE(NULLIF(TRIM(" + column + "), ''), '未填')"
	qs := query.Use(r.db).AlumniProfile
	var items []do.DashboardDistributionItem
	distributionQuery := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Select(nameExpr + " AS name, COUNT(*) AS value").
		Where(qs.DeletedAt.IsNull()).
		Where(qs.Status.Eq(common.AlumniStatusActive)).
		Group(nameExpr).
		Order("value DESC").
		Order("name ASC")
	if len(dashboardQuery.DataDomainIDs) > 0 {
		distributionQuery = distributionQuery.Where(qs.DataDomainID.In(dashboardQuery.DataDomainIDs...))
	}
	if err := distributionQuery.Scan(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func dashboardDistributionColumn(dimension string) (string, bool) {
	switch dimension {
	case do.DashboardDistributionDimensionGrade:
		return "grade", true
	case do.DashboardDistributionDimensionClassName:
		return "class_name", true
	case do.DashboardDistributionDimensionCohort:
		return "cohort", true
	case do.DashboardDistributionDimensionGender:
		return "gender", true
	case do.DashboardDistributionDimensionMajor:
		return "major", true
	case do.DashboardDistributionDimensionTrainingMode:
		return "training_mode", true
	case do.DashboardDistributionDimensionIndustry:
		return "industry", true
	default:
		return "", false
	}
}
