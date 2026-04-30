package do

import "strings"

const (
	DashboardDistributionDimensionGrade        = "grade"
	DashboardDistributionDimensionClassName    = "class_name"
	DashboardDistributionDimensionCohort       = "cohort"
	DashboardDistributionDimensionGender       = "gender"
	DashboardDistributionDimensionMajor        = "major"
	DashboardDistributionDimensionTrainingMode = "training_mode"
	DashboardDistributionDimensionIndustry     = "industry"
)

type DashboardOverviewStats struct {
	TotalAlumni      int64
	TotalAccounts    int64
	MobileComplete   int64
	WorkUnitComplete int64
	MentorComplete   int64
}

type DashboardDistributionQuery struct {
	Dimension string
}

func (q DashboardDistributionQuery) Normalize() DashboardDistributionQuery {
	q.Dimension = strings.ToLower(strings.TrimSpace(q.Dimension))
	return q
}

func (q DashboardDistributionQuery) Valid() bool {
	switch q.Dimension {
	case DashboardDistributionDimensionGrade,
		DashboardDistributionDimensionClassName,
		DashboardDistributionDimensionCohort,
		DashboardDistributionDimensionGender,
		DashboardDistributionDimensionMajor,
		DashboardDistributionDimensionTrainingMode,
		DashboardDistributionDimensionIndustry:
		return true
	default:
		return false
	}
}

type DashboardDistributionItem struct {
	Name  string
	Value int64
}
