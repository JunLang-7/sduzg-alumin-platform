package dto

import (
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
)

type AlumniListRequest struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
	Keyword      string `form:"keyword"`
	Grade        string `form:"grade"`
	ClassName    string `form:"class_name"`
	Cohort       string `form:"cohort"`
	Counselor    string `form:"counselor"`
	Mentor       string `form:"mentor"`
	Major        string `form:"major"`
	TrainingMode string `form:"training_mode"`
	Industry     string `form:"industry"`
	WorkUnit     string `form:"work_unit"`
	Position     string `form:"position"`
	Mobile       string `form:"mobile"`
}

func (r AlumniListRequest) ToQuery() do.AlumniListQuery {
	return do.AlumniListQuery{
		Page: common.PageQuery{
			Page:     r.Page,
			PageSize: r.PageSize,
		},
		Keyword:      r.Keyword,
		Grade:        r.Grade,
		ClassName:    r.ClassName,
		Cohort:       r.Cohort,
		Counselor:    r.Counselor,
		Mentor:       r.Mentor,
		Major:        r.Major,
		TrainingMode: r.TrainingMode,
		Industry:     r.Industry,
		WorkUnit:     r.WorkUnit,
		Position:     r.Position,
		Mobile:       r.Mobile,
	}
}

type AlumniListItem struct {
	ID           uint64    `json:"id"`
	Name         string    `json:"name"`
	Grade        string    `json:"grade"`
	ClassName    *string   `json:"class_name"`
	Cohort       *string   `json:"cohort"`
	Major        *string   `json:"major"`
	TrainingMode *string   `json:"training_mode"`
	Industry     *string   `json:"industry"`
	WorkUnit     *string   `json:"work_unit"`
	Position     *string   `json:"position"`
	Mobile       *string   `json:"mobile"`
	UpdatedAt    time.Time `json:"updated_at"`
}
