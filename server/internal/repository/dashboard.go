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
	Overview(ctx context.Context) (do.DashboardOverviewStats, error)
	Distribution(ctx context.Context, query do.DashboardDistributionQuery) ([]do.DashboardDistributionItem, error)
}

type DashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

// Overview 获取数据大屏总览统计。
func (r *DashboardRepository) Overview(ctx context.Context) (do.DashboardOverviewStats, error) {
	if r.db == nil {
		return do.DashboardOverviewStats{}, common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AlumniProfile
	var stats do.DashboardOverviewStats
	if err := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Select(`
			COUNT(*) AS total_alumni,
			COALESCE(SUM(CASE WHEN mobile IS NOT NULL AND TRIM(mobile) <> '' THEN 1 ELSE 0 END), 0) AS mobile_complete,
			COALESCE(SUM(CASE WHEN work_unit IS NOT NULL AND TRIM(work_unit) <> '' THEN 1 ELSE 0 END), 0) AS work_unit_complete,
			COALESCE(SUM(CASE WHEN mentor IS NOT NULL AND TRIM(mentor) <> '' THEN 1 ELSE 0 END), 0) AS mentor_complete
		`).
		Where(qs.DeletedAt.IsNull()).
		Where(qs.Status.Eq(common.AlumniStatusActive)).
		Scan(&stats).
		Error; err != nil {
		return do.DashboardOverviewStats{}, err
	}

	if err := r.db.WithContext(ctx).
		Table("users AS u").
		Joins(
			"JOIN alumni_profiles AS a ON a.id = u.alumni_id AND a.deleted_at IS NULL AND a.status = ?",
			common.AlumniStatusActive,
		).
		Where("u.deleted_at IS NULL").
		Where("u.status = ?", common.UserStatusActive).
		Where("u.role = ?", common.RoleAlumni).
		Where("u.alumni_id IS NOT NULL").
		Distinct("u.alumni_id").
		Count(&stats.TotalAccounts).
		Error; err != nil {
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
	if err := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Select(nameExpr + " AS name, COUNT(*) AS value").
		Where(qs.DeletedAt.IsNull()).
		Where(qs.Status.Eq(common.AlumniStatusActive)).
		Group(nameExpr).
		Order("value DESC").
		Order("name ASC").
		Scan(&items).
		Error; err != nil {
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
