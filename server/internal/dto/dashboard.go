package dto

type DashboardOverview struct {
	TotalAlumni          int64   `json:"total_alumni"`
	TotalAccounts        int64   `json:"total_accounts"`
	MobileCompleteRate   float64 `json:"mobile_complete_rate"`
	WorkUnitCompleteRate float64 `json:"work_unit_complete_rate"`
	MentorCompleteRate   float64 `json:"mentor_complete_rate"`
}
