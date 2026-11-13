package dto

import (
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
)

// AlumniListRequest 校友列表查询参数。
// 个人信息条件与学籍条件分开传递，服务端要求学籍条件由同一段学籍满足。
type AlumniListRequest struct {
	Page          int     `form:"page"`
	PageSize      int     `form:"page_size"`
	Keyword       string  `form:"keyword"`
	Grade         string  `form:"grade"`
	ClassName     string  `form:"class_name"`
	Cohort        string  `form:"cohort"`
	Counselor     string  `form:"counselor"`
	Mentor        string  `form:"mentor"`
	Major         string  `form:"major"`
	TrainingMode  string  `form:"training_mode"`
	TrainingLevel string  `form:"training_level"`
	Industry      string  `form:"industry"`
	WorkUnit      string  `form:"work_unit"`
	Position      string  `form:"position"`
	Mobile        string  `form:"mobile"`
	DataDomainID  *uint64 `form:"data_domain_id"`
}

type AlumniExportRequest struct {
	Format        string  `form:"format"`
	Keyword       string  `form:"keyword"`
	Grade         string  `form:"grade"`
	ClassName     string  `form:"class_name"`
	Cohort        string  `form:"cohort"`
	Counselor     string  `form:"counselor"`
	Mentor        string  `form:"mentor"`
	Major         string  `form:"major"`
	TrainingMode  string  `form:"training_mode"`
	TrainingLevel string  `form:"training_level"`
	Industry      string  `form:"industry"`
	WorkUnit      string  `form:"work_unit"`
	Position      string  `form:"position"`
	Mobile        string  `form:"mobile"`
	DataDomainID  *uint64 `form:"data_domain_id"`
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
		Keyword:       r.Keyword,
		Grade:         r.Grade,
		ClassName:     r.ClassName,
		Cohort:        r.Cohort,
		Counselor:     r.Counselor,
		Mentor:        r.Mentor,
		Major:         r.Major,
		TrainingMode:  r.TrainingMode,
		TrainingLevel: r.TrainingLevel,
		Industry:      r.Industry,
		WorkUnit:      r.WorkUnit,
		Position:      r.Position,
		Mobile:        r.Mobile,
		DataDomainID:  r.DataDomainID,
	}
}

func (r AlumniListRequest) ToQuery() do.AlumniListQuery {
	return do.AlumniListQuery{
		Page: common.PageQuery{
			Page:     r.Page,
			PageSize: r.PageSize,
		},
		Keyword:       r.Keyword,
		Grade:         r.Grade,
		ClassName:     r.ClassName,
		Cohort:        r.Cohort,
		Counselor:     r.Counselor,
		Mentor:        r.Mentor,
		Major:         r.Major,
		TrainingMode:  r.TrainingMode,
		TrainingLevel: r.TrainingLevel,
		Industry:      r.Industry,
		WorkUnit:      r.WorkUnit,
		Position:      r.Position,
		Mobile:        r.Mobile,
		DataDomainID:  r.DataDomainID,
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

// AlumniEnrollmentPayload 是编辑弹窗中单段学籍的提交内容。
// id 为空表示本次新增；已有学籍必须提交 version。
type AlumniEnrollmentPayload struct {
	ID              *uint64  `json:"id"`
	Version         *uint32  `json:"version"`
	DataDomainID    uint64   `json:"data_domain_id"`
	TrainingLevel   string   `json:"training_level"`
	Grade           string   `json:"grade"`
	ClassName       *string  `json:"class_name"`
	Cohort          *string  `json:"cohort"`
	Counselor       *string  `json:"counselor"`
	Mentor          *string  `json:"mentor"`
	Major           *string  `json:"major"`
	TrainingMode    *string  `json:"training_mode"`
	Remark          *string  `json:"remark"`
	ActivateFileIDs []uint64 `json:"activate_file_ids"`
	RemoveFileIDs   []uint64 `json:"remove_file_ids"`
}

func (p AlumniEnrollmentPayload) ToInput() do.AlumniEnrollmentInput {
	input := do.AlumniEnrollmentInput{
		DataDomainID:    p.DataDomainID,
		TrainingLevel:   p.TrainingLevel,
		Grade:           p.Grade,
		ClassName:       p.ClassName,
		Cohort:          p.Cohort,
		Counselor:       p.Counselor,
		Mentor:          p.Mentor,
		Major:           p.Major,
		TrainingMode:    p.TrainingMode,
		Remark:          p.Remark,
		ActivateFileIDs: p.ActivateFileIDs,
		RemoveFileIDs:   p.RemoveFileIDs,
	}
	if p.ID != nil {
		input.ID = *p.ID
	}
	if p.Version != nil {
		input.Version = *p.Version
	}
	return input
}

func enrollmentInputsFromPayload(payloads []AlumniEnrollmentPayload) []do.AlumniEnrollmentInput {
	inputs := make([]do.AlumniEnrollmentInput, 0, len(payloads))
	for _, payload := range payloads {
		inputs = append(inputs, payload.ToInput())
	}
	return inputs
}

type AdminAlumniCreateRequest struct {
	Name           string                    `json:"name" binding:"required"`
	Gender         *string                   `json:"gender"`
	Mobile         *string                   `json:"mobile"`
	Email          *string                   `json:"email"`
	Industry       *string                   `json:"industry"`
	WorkUnit       *string                   `json:"work_unit"`
	Position       *string                   `json:"position"`
	MailingAddress *string                   `json:"mailing_address"`
	Enrollments    []AlumniEnrollmentPayload `json:"enrollments" binding:"required,min=1,dive"`
}

func (r AdminAlumniCreateRequest) ToInput() do.AlumniCreateInput {
	return do.AlumniCreateInput{
		Personal: do.AlumniPersonalInput{
			Name:           r.Name,
			Gender:         r.Gender,
			Mobile:         r.Mobile,
			Email:          r.Email,
			Industry:       r.Industry,
			WorkUnit:       r.WorkUnit,
			Position:       r.Position,
			MailingAddress: r.MailingAddress,
		},
		Enrollments: enrollmentInputsFromPayload(r.Enrollments),
	}
}

type AdminAlumniUpdateRequest struct {
	Version        uint32                    `json:"version"`
	Name           string                    `json:"name" binding:"required"`
	Gender         *string                   `json:"gender"`
	Mobile         *string                   `json:"mobile"`
	Email          *string                   `json:"email"`
	Industry       *string                   `json:"industry"`
	WorkUnit       *string                   `json:"work_unit"`
	Position       *string                   `json:"position"`
	MailingAddress *string                   `json:"mailing_address"`
	Enrollments    []AlumniEnrollmentPayload `json:"enrollments" binding:"dive"`
}

func (r AdminAlumniUpdateRequest) ToInput() do.AlumniSaveInput {
	return do.AlumniSaveInput{
		Version: r.Version,
		Personal: do.AlumniPersonalInput{
			Name:           r.Name,
			Gender:         r.Gender,
			Mobile:         r.Mobile,
			Email:          r.Email,
			Industry:       r.Industry,
			WorkUnit:       r.WorkUnit,
			Position:       r.Position,
			MailingAddress: r.MailingAddress,
		},
		Enrollments: enrollmentInputsFromPayload(r.Enrollments),
	}
}

type AlumniEnrollmentSummary struct {
	ID             uint64  `json:"id"`
	DataDomainID   uint64  `json:"data_domain_id"`
	DataDomainName string  `json:"data_domain_name"`
	TrainingLevel  string  `json:"training_level"`
	Grade          string  `json:"grade"`
	ClassName      *string `json:"class_name"`
	Cohort         *string `json:"cohort"`
	Counselor      *string `json:"counselor"`
	Mentor         *string `json:"mentor"`
	Major          *string `json:"major"`
	TrainingMode   *string `json:"training_mode"`
}

type AlumniEnrollmentDetail struct {
	ID             uint64    `json:"id"`
	DataDomainID   uint64    `json:"data_domain_id"`
	DataDomainName string    `json:"data_domain_name"`
	TrainingLevel  string    `json:"training_level"`
	Grade          string    `json:"grade"`
	ClassName      *string   `json:"class_name"`
	Cohort         *string   `json:"cohort"`
	Counselor      *string   `json:"counselor"`
	Mentor         *string   `json:"mentor"`
	Major          *string   `json:"major"`
	TrainingMode   *string   `json:"training_mode"`
	Remark         *string   `json:"remark,omitempty"`
	Version        uint32    `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AlumniListItem struct {
	ID          uint64                    `json:"id"`
	Name        string                    `json:"name"`
	Gender      *string                   `json:"gender"`
	Industry    *string                   `json:"industry"`
	WorkUnit    *string                   `json:"work_unit"`
	Position    *string                   `json:"position"`
	Mobile      *string                   `json:"mobile"`
	Email       *string                   `json:"email"`
	Version     uint32                    `json:"version"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	Enrollments []AlumniEnrollmentSummary `json:"enrollments"`
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

// AlumniDetail 是个人主档案详情。学籍段仅包含当前查看者可访问的部分。
type AlumniDetail struct {
	ID             uint64                   `json:"id"`
	Name           string                   `json:"name"`
	Gender         *string                  `json:"gender"`
	Mobile         *string                  `json:"mobile"`
	Email          *string                  `json:"email"`
	Industry       *string                  `json:"industry"`
	WorkUnit       *string                  `json:"work_unit"`
	Position       *string                  `json:"position"`
	MailingAddress *string                  `json:"mailing_address"`
	Status         string                   `json:"status"`
	Version        uint32                   `json:"version"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
	Enrollments    []AlumniEnrollmentDetail `json:"enrollments"`
}

// UpdateContactRequest 修改校友手机号和/或邮箱
type UpdateContactRequest struct {
	Mobile *string `json:"mobile"`
	Email  *string `json:"email"`
	Code   string  `json:"code"`
}
