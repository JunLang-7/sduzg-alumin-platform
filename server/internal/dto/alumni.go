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

type AlumniExportRequest struct {
	Format       string `form:"format"`
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

func (r AlumniExportRequest) FormatOrDefault() string {
	if r.Format == "csv" {
		return "csv"
	}
	return "xlsx"
}

func (r AlumniExportRequest) ToQuery() do.AlumniListQuery {
	return do.AlumniListQuery{
		Page: common.PageQuery{
			Page:     1,
			PageSize: 0,
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

type AlumniProfileUpdateRequest struct {
	WorkUnit       *string `json:"work_unit"`
	Position       *string `json:"position"`
	MailingAddress *string `json:"mailing_address"`
	Mobile         *string `json:"mobile"`
}

func (r AlumniProfileUpdateRequest) ToProfile() do.AlumniEditableProfile {
	return do.AlumniEditableProfile{
		WorkUnit:       r.WorkUnit,
		Position:       r.Position,
		MailingAddress: r.MailingAddress,
		Mobile:         r.Mobile,
	}
}

type AdminAlumniCreateRequest struct {
	Name           string  `json:"name" binding:"required"`
	Grade          string  `json:"grade" binding:"required"`
	ClassName      *string `json:"class_name"`
	Cohort         *string `json:"cohort"`
	Counselor      *string `json:"counselor"`
	Mentor         *string `json:"mentor"`
	Major          *string `json:"major"`
	TrainingMode   *string `json:"training_mode"`
	Industry       *string `json:"industry"`
	WorkUnit       *string `json:"work_unit"`
	Position       *string `json:"position"`
	MailingAddress *string `json:"mailing_address"`
	Gender         *string `json:"gender"`
	Mobile         *string `json:"mobile"`
	Remark         *string `json:"remark"`
}

type AdminAlumniUpdateRequest struct {
	Name           string  `json:"name" binding:"required"`
	Grade          string  `json:"grade" binding:"required"`
	ClassName      *string `json:"class_name"`
	Cohort         *string `json:"cohort"`
	Counselor      *string `json:"counselor"`
	Mentor         *string `json:"mentor"`
	Major          *string `json:"major"`
	TrainingMode   *string `json:"training_mode"`
	Industry       *string `json:"industry"`
	WorkUnit       *string `json:"work_unit"`
	Position       *string `json:"position"`
	MailingAddress *string `json:"mailing_address"`
	Gender         *string `json:"gender"`
	Mobile         *string `json:"mobile"`
	Remark         *string `json:"remark"`
}

func (r AdminAlumniCreateRequest) ToProfile() do.AlumniCreateProfile {
	return do.AlumniCreateProfile{
		Name:           r.Name,
		Grade:          r.Grade,
		ClassName:      r.ClassName,
		Cohort:         r.Cohort,
		Counselor:      r.Counselor,
		Mentor:         r.Mentor,
		Major:          r.Major,
		TrainingMode:   r.TrainingMode,
		Industry:       r.Industry,
		WorkUnit:       r.WorkUnit,
		Position:       r.Position,
		MailingAddress: r.MailingAddress,
		Gender:         r.Gender,
		Mobile:         r.Mobile,
		Remark:         r.Remark,
	}
}

func (r AdminAlumniUpdateRequest) ToProfile() do.AlumniUpdateProfile {
	return do.AlumniUpdateProfile{
		Name:           r.Name,
		Grade:          r.Grade,
		ClassName:      r.ClassName,
		Cohort:         r.Cohort,
		Counselor:      r.Counselor,
		Mentor:         r.Mentor,
		Major:          r.Major,
		TrainingMode:   r.TrainingMode,
		Industry:       r.Industry,
		WorkUnit:       r.WorkUnit,
		Position:       r.Position,
		MailingAddress: r.MailingAddress,
		Gender:         r.Gender,
		Mobile:         r.Mobile,
		Remark:         r.Remark,
	}
}

type AlumniListItem struct {
	ID           uint64    `json:"id"`
	Name         string    `json:"name"`
	Grade        string    `json:"grade"`
	ClassName    *string   `json:"class_name"`
	Cohort       *string   `json:"cohort"`
	Counselor    *string   `json:"counselor"`
	Mentor       *string   `json:"mentor"`
	Major        *string   `json:"major"`
	TrainingMode *string   `json:"training_mode"`
	Industry     *string   `json:"industry"`
	WorkUnit     *string   `json:"work_unit"`
	Position     *string   `json:"position"`
	Gender       *string   `json:"gender"`
	Mobile       *string   `json:"mobile"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AlumniImportResult struct {
	Total   int              `json:"total"`
	Success int              `json:"success"`
	Errors  []AlumniRowError `json:"errors"`
}

type AlumniRowError struct {
	Row     int    `json:"row"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

type AlumniDetail struct {
	ID             uint64    `json:"id"`
	Name           string    `json:"name"`
	Grade          string    `json:"grade"`
	ClassName      *string   `json:"class_name"`
	Cohort         *string   `json:"cohort"`
	Counselor      *string   `json:"counselor"`
	Mentor         *string   `json:"mentor"`
	Major          *string   `json:"major"`
	TrainingMode   *string   `json:"training_mode"`
	Industry       *string   `json:"industry"`
	WorkUnit       *string   `json:"work_unit"`
	Position       *string   `json:"position"`
	MailingAddress *string   `json:"mailing_address"`
	Gender         *string   `json:"gender"`
	Mobile         *string   `json:"mobile"`
	Email          *string   `json:"email,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// UpdateContactRequest 修改校友手机号和/或邮箱
type UpdateContactRequest struct {
	Mobile *string `json:"mobile"`
	Email  *string `json:"email"`
	Code   string  `json:"code"`
}
