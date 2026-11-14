package do

import (
	"sort"
	"strconv"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

// AlumniListQuery 是校友列表/导出查询条件。
// 个人信息筛选直接作用于个人档案；年级、班级、届数、专业、培养方式、导师、
// 辅导员、培养层次和数据域筛选必须由同一段学籍满足，避免跨段组合误命中。
type AlumniListQuery struct {
	Page             common.PageQuery
	Keyword          string
	Grade            string
	ClassName        string
	Cohort           string
	Counselor        string
	Mentor           string
	Major            string
	TrainingMode     string
	TrainingLevel    string
	Industry         string
	WorkUnit         string
	Position         string
	Mobile           string
	DataDomainID     *uint64
	DataDomainIDs    []uint64
	CanReadSensitive bool
}

// AlumniRecord 是一份个人主档案与查询者可访问学籍段的组合。
type AlumniRecord struct {
	Profile     *model.AlumniProfile
	Enrollments []*model.AlumniEnrollment
}

// AlumniExportRow 是导出的平铺行：一行一段有权限的学籍，个人信息重复填充。
type AlumniExportRow struct {
	Profile    *model.AlumniProfile
	Enrollment *model.AlumniEnrollment
}

// AlumniPersonalInput 是个人主档案中由管理员维护的共享字段。
// nil 表示不修改，空字符串表示显式清空。
type AlumniPersonalInput struct {
	Name           string
	Gender         *string
	Mobile         *string
	Email          *string
	Industry       *string
	WorkUnit       *string
	Position       *string
	MailingAddress *string
}

// AlumniEnrollmentInput 是一次保存中提交的单段学籍。
// ID 为 0 表示新增；ID 非 0 时必须同时提交当前 version 用于并发冲突检查。
type AlumniEnrollmentInput struct {
	ID              uint64
	Version         uint32
	DataDomainID    uint64
	TrainingLevel   string
	Grade           string
	ClassName       *string
	Cohort          *string
	Counselor       *string
	Mentor          *string
	Major           *string
	TrainingMode    *string
	Remark          *string
	ActivateFileIDs []uint64
	RemoveFileIDs   []uint64
}

// AlumniSaveInput 是「保存全部修改」的提交内容。
// 未提交的已有学籍保持不变，不会被当作删除。
type AlumniSaveInput struct {
	Version     uint32
	Personal    AlumniPersonalInput
	Enrollments []AlumniEnrollmentInput
}

// AlumniCreateInput 是新增校友：一份个人档案 + 至少一段初始学籍。
type AlumniCreateInput struct {
	Personal    AlumniPersonalInput
	Enrollments []AlumniEnrollmentInput
	Status      string
}

// AlumniSaveResult 是保存成功后的完整详情与被移除文件的 MinIO 对象键。
// 对象清理在数据库事务提交后执行，避免事务回滚时误删正式附件。
type AlumniSaveResult struct {
	Record            AlumniRecord
	RemovedObjectKeys []string
	FileChanges       []AlumniFileChange
}

// AlumniFileChange 仅在统一保存事务成功后交给服务层审计，不包含存储对象键。
type AlumniFileChange struct {
	EnrollmentID uint64
	FileID       uint64
	FileType     string
	OriginalName string
	Removed      bool
}

// AlumniFileChanges 是事务内附件变更与提交后的对象清理清单。
type AlumniFileChanges struct {
	RemovedObjectKeys []string
	Changes           []AlumniFileChange
}

type AlumniEditableProfile struct {
	WorkUnit       *string
	Position       *string
	MailingAddress *string
	Mobile         *string
}

// Normalize 对管理员提交的个人共享字段做基础清理。
func (p AlumniPersonalInput) Normalize() AlumniPersonalInput {
	p.Name = strings.TrimSpace(p.Name)
	p.Gender = trimEmptyStringPointer(p.Gender)
	p.Mobile = trimEmptyStringPointer(p.Mobile)
	p.Email = trimEmptyStringPointer(p.Email)
	p.Industry = trimEmptyStringPointer(p.Industry)
	p.WorkUnit = trimEmptyStringPointer(p.WorkUnit)
	p.Position = trimEmptyStringPointer(p.Position)
	p.MailingAddress = trimEmptyStringPointer(p.MailingAddress)
	return p
}

// Normalize 清理单段学籍字段。Remark 支持显式清空，因此只做去空格。
func (e AlumniEnrollmentInput) Normalize() AlumniEnrollmentInput {
	e.TrainingLevel = strings.TrimSpace(e.TrainingLevel)
	e.Grade = strings.TrimSpace(e.Grade)
	e.ClassName = trimStringPointer(e.ClassName)
	e.Cohort = trimStringPointer(e.Cohort)
	e.Counselor = trimStringPointer(e.Counselor)
	e.Mentor = trimStringPointer(e.Mentor)
	e.Major = trimStringPointer(e.Major)
	e.TrainingMode = trimStringPointer(e.TrainingMode)
	e.Remark = trimStringPointer(e.Remark)
	e.ActivateFileIDs = uniqueUint64(e.ActivateFileIDs)
	e.RemoveFileIDs = uniqueUint64(e.RemoveFileIDs)
	return e
}

// Normalize 清理整次保存提交，并保留指针语义。
func (s AlumniSaveInput) Normalize() AlumniSaveInput {
	s.Personal = s.Personal.Normalize()
	s.Enrollments = normalizeEnrollmentInputs(s.Enrollments)
	return s
}

// Normalize 清理新增校友的提交内容。
func (c AlumniCreateInput) Normalize() AlumniCreateInput {
	c.Personal = c.Personal.Normalize()
	c.Enrollments = normalizeEnrollmentInputs(c.Enrollments)
	for i := range c.Enrollments {
		c.Enrollments[i].blankToNil()
	}
	c.Status = strings.TrimSpace(c.Status)
	if c.Status == "" {
		c.Status = common.AlumniStatusActive
	}
	return c
}

func normalizeEnrollmentInputs(inputs []AlumniEnrollmentInput) []AlumniEnrollmentInput {
	result := make([]AlumniEnrollmentInput, 0, len(inputs))
	for _, item := range inputs {
		result = append(result, item.Normalize())
	}
	return result
}

// blankToNil 用于新增记录：空字符串应落库为 NULL，而不是空串。
func blankToNil(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	return value
}

func (e *AlumniEnrollmentInput) blankToNil() {
	e.ClassName = blankToNil(e.ClassName)
	e.Cohort = blankToNil(e.Cohort)
	e.Counselor = blankToNil(e.Counselor)
	e.Mentor = blankToNil(e.Mentor)
	e.Major = blankToNil(e.Major)
	e.TrainingMode = blankToNil(e.TrainingMode)
	e.Remark = blankToNil(e.Remark)
}

// Normalize 对校友本人可编辑字段去除首尾空格，保留 nil 以区分未提交字段。
func (p AlumniEditableProfile) Normalize() AlumniEditableProfile {
	p.WorkUnit = trimStringPointer(p.WorkUnit)
	p.Position = trimStringPointer(p.Position)
	p.MailingAddress = trimStringPointer(p.MailingAddress)
	p.Mobile = trimStringPointer(p.Mobile)
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
	q.TrainingLevel = strings.TrimSpace(q.TrainingLevel)
	q.Industry = strings.TrimSpace(q.Industry)
	q.WorkUnit = strings.TrimSpace(q.WorkUnit)
	q.Position = strings.TrimSpace(q.Position)
	q.Mobile = strings.TrimSpace(q.Mobile)
	q.DataDomainIDs = uniqueSortedUint64(q.DataDomainIDs)
	return q
}

// IsUnfiltered 无过滤条件时返回 true，此时可安全使用缓存计数。
func (q AlumniListQuery) IsUnfiltered() bool {
	return q.Keyword == "" && q.Grade == "" && q.ClassName == "" && q.Cohort == "" &&
		q.Counselor == "" && q.Mentor == "" && q.Major == "" && q.TrainingMode == "" &&
		q.TrainingLevel == "" && q.Industry == "" && q.WorkUnit == "" && q.Position == "" && q.Mobile == ""
}

type AlumniDedupKey struct {
	Name         string
	Mobile       string
	DataDomainID uint64
	Grade        string
	ClassName    string
	Cohort       string
}

func (k AlumniDedupKey) Key() string {
	return strings.Join([]string{
		strings.TrimSpace(k.Name),
		strings.TrimSpace(k.Mobile),
		strconv.FormatUint(k.DataDomainID, 10),
		strings.TrimSpace(k.Grade),
		strings.TrimSpace(k.ClassName),
		strings.TrimSpace(k.Cohort),
	}, "|")
}

func trimStringPointer(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func trimEmptyStringPointer(value *string) *string {
	value = trimStringPointer(value)
	if value == nil || *value == "" {
		return nil
	}
	return value
}

func uniqueUint64(values []uint64) []uint64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(values))
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueSortedUint64(values []uint64) []uint64 {
	result := uniqueUint64(values)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
