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
