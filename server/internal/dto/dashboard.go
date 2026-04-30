package dto

import "github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"

type DashboardOverview struct {
	TotalAlumni          int64   `json:"total_alumni"`
	TotalAccounts        int64   `json:"total_accounts"`
	MobileCompleteRate   float64 `json:"mobile_complete_rate"`
	WorkUnitCompleteRate float64 `json:"work_unit_complete_rate"`
	MentorCompleteRate   float64 `json:"mentor_complete_rate"`
}

type DashboardDistributionRequest struct {
	Dimension string `form:"dimension" binding:"required"`
}

func (r DashboardDistributionRequest) ToQuery() do.DashboardDistributionQuery {
	return do.DashboardDistributionQuery{
		Dimension: r.Dimension,
	}
}

type DashboardDistributionItem struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}
