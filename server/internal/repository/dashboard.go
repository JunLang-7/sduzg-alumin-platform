package repository

import (
	"context"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
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

// visibleAlumniCondition 限定至少有一段可访问学籍的活跃个人档案。
// 校友总人数按人去重，不会因多段学籍被重复计入。
func visibleAlumniCondition(dataDomainIDs []uint64) (string, []any) {
	if len(dataDomainIDs) == 0 {
		return "EXISTS (SELECT 1 FROM alumni_enrollments AS e WHERE e.alumni_id = p.id AND e.status = ? AND e.deleted_at IS NULL)",
			[]any{common.AlumniStatusActive}
	}
	return "EXISTS (SELECT 1 FROM alumni_enrollments AS e WHERE e.alumni_id = p.id AND e.status = ? AND e.deleted_at IS NULL AND e.data_domain_id IN (" + placeholders(len(dataDomainIDs)) + "))",
		append([]any{common.AlumniStatusActive}, uint64Args(dataDomainIDs)...)
}

// Overview 获取数据大屏总览统计。
func (r *DashboardRepository) Overview(ctx context.Context, dataDomainIDs []uint64) (do.DashboardOverviewStats, error) {
	if r.db == nil {
		return do.DashboardOverviewStats{}, common.ErrDatabaseUnavailable
	}

	visible, visibleArgs := visibleAlumniCondition(dataDomainIDs)

	var stats do.DashboardOverviewStats
	if err := r.db.WithContext(ctx).
		Table("alumni_profiles AS p").
		Select(`
			COUNT(*) AS total_alumni,
			COALESCE(SUM(CASE WHEN p.mobile IS NOT NULL AND TRIM(p.mobile) <> '' THEN 1 ELSE 0 END), 0) AS mobile_complete,
			COALESCE(SUM(CASE WHEN p.work_unit IS NOT NULL AND TRIM(p.work_unit) <> '' THEN 1 ELSE 0 END), 0) AS work_unit_complete
		`).
		Where("p.status = ? AND p.deleted_at IS NULL", common.AlumniStatusActive).
		Where(visible, visibleArgs...).
		Scan(&stats).Error; err != nil {
		return do.DashboardOverviewStats{}, err
	}

	mentorQuery := r.db.WithContext(ctx).
		Table("alumni_profiles AS p").
		Joins("JOIN alumni_enrollments AS e ON e.alumni_id = p.id AND e.status = ? AND e.deleted_at IS NULL", common.AlumniStatusActive).
		Where("p.status = ? AND p.deleted_at IS NULL", common.AlumniStatusActive).
		Where("e.mentor IS NOT NULL AND TRIM(e.mentor) <> ''")
	if len(dataDomainIDs) > 0 {
		mentorQuery = mentorQuery.Where("e.data_domain_id IN ("+placeholders(len(dataDomainIDs))+")", uint64Args(dataDomainIDs)...)
	}
	if err := mentorQuery.Distinct("p.id").Count(&stats.MentorComplete).Error; err != nil {
		return do.DashboardOverviewStats{}, err
	}

	accountQuery := r.db.WithContext(ctx).
		Table("alumni_profiles AS p").
		Joins("JOIN users AS u ON u.alumni_id = p.id AND u.deleted_at IS NULL AND u.status = ? AND u.role = ?", common.UserStatusActive, common.RoleAlumni).
		Where("p.status = ? AND p.deleted_at IS NULL", common.AlumniStatusActive).
		Where(visible, visibleArgs...)
	if err := accountQuery.Distinct("p.id").Count(&stats.TotalAccounts).Error; err != nil {
		return do.DashboardOverviewStats{}, err
	}

	return stats, nil
}

// Distribution 获取指定维度的校友分布统计。
// 学籍维度按人去重计数：同一人在不同年级各计一次，但同一值内不重复。
func (r *DashboardRepository) Distribution(ctx context.Context, dashboardQuery do.DashboardDistributionQuery) ([]do.DashboardDistributionItem, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	dashboardQuery = dashboardQuery.Normalize()
	column, enrollmentDimension, ok := dashboardDistributionColumn(dashboardQuery.Dimension)
	if !ok {
		return nil, common.ErrInvalidRequest
	}

	var items []do.DashboardDistributionItem
	query := r.db.WithContext(ctx)
	if enrollmentDimension {
		query = query.
			Table("alumni_profiles AS p").
			Select("COALESCE(NULLIF(TRIM(e."+column+"), ''), '未填') AS name, COUNT(DISTINCT p.id) AS value").
			Joins("JOIN alumni_enrollments AS e ON e.alumni_id = p.id AND e.status = ? AND e.deleted_at IS NULL", common.AlumniStatusActive).
			Where("p.status = ? AND p.deleted_at IS NULL", common.AlumniStatusActive)
		if len(dashboardQuery.DataDomainIDs) > 0 {
			query = query.Where("e.data_domain_id IN ("+placeholders(len(dashboardQuery.DataDomainIDs))+")", uint64Args(dashboardQuery.DataDomainIDs)...)
		}
		query = query.Group("COALESCE(NULLIF(TRIM(e." + column + "), ''), '未填')")
	} else {
		visible, visibleArgs := visibleAlumniCondition(dashboardQuery.DataDomainIDs)
		nameExpr := "COALESCE(NULLIF(TRIM(p." + column + "), ''), '未填')"
		query = query.
			Table("alumni_profiles AS p").
			Select(nameExpr+" AS name, COUNT(*) AS value").
			Where("p.status = ? AND p.deleted_at IS NULL", common.AlumniStatusActive).
			Where(visible, visibleArgs...).
			Group(nameExpr)
	}
	if err := query.Order("value DESC").Order("name ASC").Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// dashboardDistributionColumn 返回统计列及该列是否属于学籍维度。
func dashboardDistributionColumn(dimension string) (string, bool, bool) {
	switch dimension {
	case do.DashboardDistributionDimensionGrade:
		return "grade", true, true
	case do.DashboardDistributionDimensionClassName:
		return "class_name", true, true
	case do.DashboardDistributionDimensionCohort:
		return "cohort", true, true
	case do.DashboardDistributionDimensionMajor:
		return "major", true, true
	case do.DashboardDistributionDimensionTrainingMode:
		return "training_mode", true, true
	case do.DashboardDistributionDimensionTrainingLevel:
		return "training_level", true, true
	case do.DashboardDistributionDimensionGender:
		return "gender", false, true
	case do.DashboardDistributionDimensionIndustry:
		return "industry", false, true
	default:
		return "", false, false
	}
}
