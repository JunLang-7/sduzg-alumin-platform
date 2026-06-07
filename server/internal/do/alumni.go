package do

import (
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
)

type AlumniListQuery struct {
	Page         common.PageQuery
	Keyword      string
	Grade        string
	ClassName    string
	Cohort       string
	Counselor    string
	Mentor       string
	Major        string
	TrainingMode string
	Industry     string
	WorkUnit     string
	Position     string
	Mobile       string
}

type AlumniEditableProfile struct {
	WorkUnit       *string
	Position       *string
	MailingAddress *string
	Mobile         *string
}

type AlumniUpdateProfile struct {
	Name           string
	Grade          string
	ClassName      *string
	Cohort         *string
	Counselor      *string
	Mentor         *string
	Major          *string
	TrainingMode   *string
	Industry       *string
	WorkUnit       *string
	Position       *string
	MailingAddress *string
	Gender         *string
	Mobile         *string
	Remark         *string
}

type AlumniCreateProfile struct {
	Name           string
	Grade          string
	ClassName      *string
	Cohort         *string
	Counselor      *string
	Mentor         *string
	Major          *string
	TrainingMode   *string
	Industry       *string
	WorkUnit       *string
	Position       *string
	MailingAddress *string
	Gender         *string
	Mobile         *string
	Remark         *string
	Status         string
}

// Normalize 对管理端新增校友字段做基础清理，并设置默认状态。
func (p AlumniCreateProfile) Normalize() AlumniCreateProfile {
	p.Name = strings.TrimSpace(p.Name)
	p.Grade = strings.TrimSpace(p.Grade)
	p.ClassName = trimEmptyStringPointer(p.ClassName)
	p.Cohort = trimEmptyStringPointer(p.Cohort)
	p.Counselor = trimEmptyStringPointer(p.Counselor)
	p.Mentor = trimEmptyStringPointer(p.Mentor)
	p.Major = trimEmptyStringPointer(p.Major)
	p.TrainingMode = trimEmptyStringPointer(p.TrainingMode)
	p.Industry = trimEmptyStringPointer(p.Industry)
	p.WorkUnit = trimEmptyStringPointer(p.WorkUnit)
	p.Position = trimEmptyStringPointer(p.Position)
	p.MailingAddress = trimEmptyStringPointer(p.MailingAddress)
	p.Gender = trimEmptyStringPointer(p.Gender)
	p.Mobile = trimEmptyStringPointer(p.Mobile)
	p.Remark = trimEmptyStringPointer(p.Remark)
	p.Status = strings.TrimSpace(p.Status)
	if p.Status == "" {
		p.Status = common.AlumniStatusActive
	}
	return p
}

// Normalize 对校友本人可编辑字段去除首尾空格，保留 nil 以区分未提交字段。
func (p AlumniEditableProfile) Normalize() AlumniEditableProfile {
	p.WorkUnit = trimStringPointer(p.WorkUnit)
	p.Position = trimStringPointer(p.Position)
	p.MailingAddress = trimStringPointer(p.MailingAddress)
	p.Mobile = trimStringPointer(p.Mobile)
	return p
}

// Normalize 对管理员编辑校友字段做基础清理，并保留已提交但清空的字段语义。
func (p AlumniUpdateProfile) Normalize() AlumniUpdateProfile {
	p.Name = strings.TrimSpace(p.Name)
	p.Grade = strings.TrimSpace(p.Grade)
	p.ClassName = trimStringPointer(p.ClassName)
	p.Cohort = trimStringPointer(p.Cohort)
	p.Counselor = trimStringPointer(p.Counselor)
	p.Mentor = trimStringPointer(p.Mentor)
	p.Major = trimStringPointer(p.Major)
	p.TrainingMode = trimStringPointer(p.TrainingMode)
	p.Industry = trimStringPointer(p.Industry)
	p.WorkUnit = trimStringPointer(p.WorkUnit)
	p.Position = trimStringPointer(p.Position)
	p.MailingAddress = trimStringPointer(p.MailingAddress)
	p.Gender = trimStringPointer(p.Gender)
	p.Mobile = trimStringPointer(p.Mobile)
	p.Remark = trimStringPointer(p.Remark)
	return p
}

func (p AlumniEditableProfile) IsEmpty() bool {
	return p.WorkUnit == nil && p.Position == nil && p.MailingAddress == nil && p.Mobile == nil
}

// Normalize 对查询参数进行规范化处理，例如去除多余的空格等
func (q AlumniListQuery) Normalize() AlumniListQuery {
	q.Page = q.Page.Normalize()
	q.Keyword = strings.TrimSpace(q.Keyword)
	q.Grade = strings.TrimSpace(q.Grade)
	q.ClassName = strings.TrimSpace(q.ClassName)
	q.Cohort = strings.TrimSpace(q.Cohort)
	q.Counselor = strings.TrimSpace(q.Counselor)
	q.Mentor = strings.TrimSpace(q.Mentor)
	q.Major = strings.TrimSpace(q.Major)
	q.TrainingMode = strings.TrimSpace(q.TrainingMode)
	q.Industry = strings.TrimSpace(q.Industry)
	q.WorkUnit = strings.TrimSpace(q.WorkUnit)
	q.Position = strings.TrimSpace(q.Position)
	q.Mobile = strings.TrimSpace(q.Mobile)
	return q
}

// IsUnfiltered 无过滤条件时返回 true，此时可安全使用缓存计数。
func (q AlumniListQuery) IsUnfiltered() bool {
	return q.Keyword == "" && q.Grade == "" && q.ClassName == "" && q.Cohort == "" &&
		q.Counselor == "" && q.Mentor == "" && q.Major == "" && q.TrainingMode == "" &&
		q.Industry == "" && q.WorkUnit == "" && q.Position == "" && q.Mobile == ""
}

func trimStringPointer(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

type AlumniDedupKey struct {
	Name      string
	Grade     string
	ClassName string
	Cohort    string
}

func (k AlumniDedupKey) Key() string {
	return k.Name + "|" + k.Grade + "|" + k.ClassName + "|" + k.Cohort
}

func trimEmptyStringPointer(value *string) *string {
	value = trimStringPointer(value)
	if value == nil || *value == "" {
		return nil
	}
	return value
}
