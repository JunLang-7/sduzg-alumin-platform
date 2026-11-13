package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"go.uber.org/zap"
)

const (
	auditTargetAlumni = "alumni_profile"
	auditTargetBatch  = "alumni_batch"
	defaultAuditScope = "全部校友"

	AuditActionCreate = "create"
	AuditActionUpdate = "update"
	AuditActionDelete = "delete"
	AuditActionImport = "import"
)

// normalizeAuditAction 限制审计记录使用稳定的操作类型，避免查询端出现无法识别的自定义值。
func normalizeAuditAction(action string) string {
	switch action {
	case AuditActionCreate, AuditActionUpdate, AuditActionDelete, AuditActionImport:
		return action
	default:
		return ""
	}
}

// AuditChange 是写入 operation_logs.detail 的字段变化结构。
// 敏感字段的 OldValue/NewValue 只保存状态描述，不保存原始值。
type AuditChange struct {
	FieldName    string `json:"field_name"`
	FieldLabel   string `json:"field_label"`
	OldValue     string `json:"old_value"`
	NewValue     string `json:"new_value"`
	CurrentValue string `json:"current_value,omitempty"`
	Sensitive    bool   `json:"sensitive,omitempty"`
}

// auditBatchField 描述批量导入明细中的字段。字段元数据可以写入审计记录，字段值仍按权限单独处理。
type auditBatchField struct {
	FieldName  string `json:"field_name"`
	FieldLabel string `json:"field_label"`
	Sensitive  bool   `json:"sensitive,omitempty"`
}

// auditBatchRecord 只保存批量导入行的公开字段快照；敏感字段在授权查询详情时从校友档案读取。
type auditBatchRecord struct {
	ID          uint64            `json:"id"`
	Name        string            `json:"name"`
	TargetMeta  string            `json:"target_meta,omitempty"`
	FieldValues map[string]string `json:"field_values,omitempty"`
}

var auditBatchPublicFields = []auditBatchField{
	{FieldName: "name", FieldLabel: "姓名"},
	{FieldName: "training_level", FieldLabel: "培养层次"},
	{FieldName: "grade", FieldLabel: "年级"},
	{FieldName: "class_name", FieldLabel: "班级"},
	{FieldName: "major", FieldLabel: "专业"},
	{FieldName: "training_mode", FieldLabel: "培养方式"},
}

var auditBatchSensitiveFields = []auditBatchField{
	{FieldName: "mobile", FieldLabel: "手机号", Sensitive: true},
	{FieldName: "email", FieldLabel: "邮箱", Sensitive: true},
	{FieldName: "work_unit", FieldLabel: "工作单位", Sensitive: true},
	{FieldName: "position", FieldLabel: "职务", Sensitive: true},
	{FieldName: "mailing_address", FieldLabel: "通讯地址", Sensitive: true},
}

func auditBatchFields() []auditBatchField {
	fields := make([]auditBatchField, 0, len(auditBatchPublicFields)+len(auditBatchSensitiveFields))
	fields = append(fields, auditBatchPublicFields...)
	fields = append(fields, auditBatchSensitiveFields...)
	return fields
}

// AuditEvent 是一条业务操作审计事件。
type AuditEvent struct {
	Operator                  *model.User
	OperatorID                uint64
	Action                    string
	Source                    string
	ManagementScope           string
	Reason                    string
	TargetType                string
	TargetID                  *uint64
	TargetName                string
	TargetMeta                string
	Changes                   []AuditChange
	DataDomainIDs             []uint64
	EnrollmentID              *uint64
	EnrollmentLabel           string
	BatchImportFields         []auditBatchField
	BatchCreatedAlumni        []auditBatchRecord
	BatchHiddenSensitiveCount int
}

// AuditEventWriter 是业务服务依赖的最小审计写入契约，便于单元测试注入内存写入器。
type AuditEventWriter interface {
	WriteAudit(ctx context.Context, event AuditEvent) error
}

type auditLogDetail struct {
	SchemaVersion               int                `json:"schema_version"`
	OperatorName                string             `json:"operator_name,omitempty"`
	OperatorRoleLabel           string             `json:"operator_role_label,omitempty"`
	TargetID                    *uint64            `json:"target_id,omitempty"`
	TargetName                  string             `json:"target_name,omitempty"`
	TargetMeta                  string             `json:"target_meta,omitempty"`
	EnrollmentID                *uint64            `json:"enrollment_id,omitempty"`
	EnrollmentLabel             string             `json:"enrollment_label,omitempty"`
	ManagementScope             string             `json:"management_scope,omitempty"`
	Source                      string             `json:"source,omitempty"`
	Reason                      string             `json:"reason,omitempty"`
	Status                      string             `json:"status,omitempty"`
	Changes                     []AuditChange      `json:"changes,omitempty"`
	DataDomainIDs               []uint64           `json:"data_domain_ids,omitempty"`
	BatchImportFields           []auditBatchField  `json:"batch_import_fields,omitempty"`
	BatchCreatedAlumni          []auditBatchRecord `json:"batch_created_alumni,omitempty"`
	BatchCreatedAlumniTotal     int                `json:"batch_created_alumni_total,omitempty"`
	BatchCreatedAlumniTruncated bool               `json:"batch_created_alumni_truncated,omitempty"`
	BatchHiddenSensitiveCount   int                `json:"batch_hidden_sensitive_count,omitempty"`
}

// WriteAudit 写入结构化审计记录。审计写入失败不应覆盖已经成功的业务操作，调用方负责记录告警。
func (l *OperationLogger) WriteAudit(ctx context.Context, event AuditEvent) error {
	if l == nil || l.db == nil {
		return nil
	}
	action := normalizeAuditAction(event.Action)
	if action == "" {
		return fmt.Errorf("unsupported audit action %q", event.Action)
	}

	operatorID := event.OperatorID
	operatorRole := ""
	operatorName := "未知用户"
	operatorRoleLabel := ""
	if event.Operator != nil {
		operatorID = event.Operator.ID
		operatorRole = event.Operator.Role
		operatorName = userDisplayName(event.Operator)
		operatorRoleLabel = auditRoleLabel(event.Operator.Role)
	}
	if operatorID == 0 {
		return fmt.Errorf("audit operator id is required")
	}

	targetType := event.TargetType
	if targetType == "" {
		targetType = auditTargetAlumni
	}
	managementScope := event.ManagementScope
	if managementScope == "" {
		managementScope = defaultAuditScope
	}
	status := "applied"
	detail := auditLogDetail{
		SchemaVersion:             1,
		OperatorName:              operatorName,
		OperatorRoleLabel:         operatorRoleLabel,
		TargetID:                  event.TargetID,
		TargetName:                event.TargetName,
		TargetMeta:                event.TargetMeta,
		ManagementScope:           managementScope,
		Source:                    event.Source,
		Reason:                    event.Reason,
		Status:                    status,
		Changes:                   event.Changes,
		DataDomainIDs:             append([]uint64(nil), event.DataDomainIDs...),
		EnrollmentID:              event.EnrollmentID,
		EnrollmentLabel:           event.EnrollmentLabel,
		BatchImportFields:         append([]auditBatchField(nil), event.BatchImportFields...),
		BatchCreatedAlumni:        append([]auditBatchRecord(nil), event.BatchCreatedAlumni...),
		BatchHiddenSensitiveCount: event.BatchHiddenSensitiveCount,
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal audit detail: %w", err)
	}
	detailString := string(detailJSON)

	log := &model.OperationLog{
		OperatorID:   operatorID,
		OperatorRole: operatorRole,
		Action:       action,
		TargetType:   targetType,
		TargetID:     event.TargetID,
		Detail:       &detailString,
	}
	return l.Write(ctx, log)
}

func userDisplayName(user *model.User) string {
	if user == nil {
		return "未知用户"
	}
	if user.RealName != nil && *user.RealName != "" {
		return *user.RealName
	}
	if user.Account != "" {
		return user.Account
	}
	return "未知用户"
}

func auditRoleLabel(role string) string {
	switch role {
	case common.RoleSuperAdmin:
		return "超级管理员"
	case common.RoleAdmin:
		return "管理员"
	case common.RoleAlumni:
		return "校友本人"
	default:
		return ""
	}
}

type auditField struct {
	name      string
	label     string
	sensitive bool
	value     func(*model.AlumniProfile) string
}

var personalAuditFields = []auditField{
	{name: "name", label: "姓名", value: func(p *model.AlumniProfile) string { return p.Name }},
	{name: "industry", label: "行业", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Industry) }},
	{name: "work_unit", label: "工作单位", sensitive: true, value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.WorkUnit) }},
	{name: "position", label: "职务", sensitive: true, value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Position) }},
	{name: "mailing_address", label: "通讯地址", sensitive: true, value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.MailingAddress) }},
	{name: "gender", label: "性别", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Gender) }},
	{name: "mobile", label: "手机号", sensitive: true, value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Mobile) }},
	{name: "email", label: "邮箱", sensitive: true, value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Email) }},
}

type enrollmentAuditField struct {
	name      string
	label     string
	value     func(*model.AlumniEnrollment) string
	sensitive bool
}

var enrollmentAuditFields = []enrollmentAuditField{
	{name: "data_domain_id", label: "培养类别", value: func(e *model.AlumniEnrollment) string {
		return enrollmentDomainLabel(e.DataDomainID)
	}},
	{name: "training_level", label: "培养层次", value: func(e *model.AlumniEnrollment) string {
		return TrainingLevelLabel(e.TrainingLevel)
	}},
	{name: "grade", label: "年级", value: func(e *model.AlumniEnrollment) string { return e.Grade }},
	{name: "class_name", label: "班级", value: func(e *model.AlumniEnrollment) string { return optionalEnrollmentValue(e.ClassName) }},
	{name: "cohort", label: "届数", value: func(e *model.AlumniEnrollment) string { return optionalEnrollmentValue(e.Cohort) }},
	{name: "counselor", label: "辅导员", value: func(e *model.AlumniEnrollment) string { return optionalEnrollmentValue(e.Counselor) }},
	{name: "mentor", label: "导师", value: func(e *model.AlumniEnrollment) string { return optionalEnrollmentValue(e.Mentor) }},
	{name: "major", label: "专业", value: func(e *model.AlumniEnrollment) string { return optionalEnrollmentValue(e.Major) }},
	{name: "training_mode", label: "培养方式", value: func(e *model.AlumniEnrollment) string { return optionalEnrollmentValue(e.TrainingMode) }},
	{name: "remark", label: "管理员备注", sensitive: true, value: func(e *model.AlumniEnrollment) string { return optionalEnrollmentValue(e.Remark) }},
}

// enrollmentDomainLabel 供审计字段值使用；具体数据域名称由调用方的 target_meta 提供。
func domainDisplayValue(enrollment *model.AlumniEnrollment, domainNames map[uint64]string) string {
	if enrollment == nil {
		return "未填写"
	}
	if name := domainNames[enrollment.DataDomainID]; name != "" {
		return name
	}
	return enrollmentDomainLabel(enrollment.DataDomainID)
}

func enrollmentDomainLabel(id uint64) string {
	if id == 0 {
		return "未填写"
	}
	return fmt.Sprintf("数据域 #%d", id)
}

// TrainingLevelLabel 返回培养层次的稳定中文名称。
func TrainingLevelLabel(level string) string {
	switch level {
	case common.TrainingLevelUndergraduate:
		return "本科"
	case common.TrainingLevelMaster:
		return "硕士"
	case common.TrainingLevelDoctor:
		return "博士"
	case common.TrainingLevelUnverified:
		return "待核验"
	default:
		return "未填写"
	}
}

func optionalProfileValue(value *string) string {
	if value == nil || *value == "" {
		return "未填写"
	}
	return *value
}

func optionalEnrollmentValue(value *string) string {
	if value == nil || *value == "" {
		return "未填写"
	}
	return *value
}

func profileAuditChanges(action string, before, after *model.AlumniProfile) []AuditChange {
	if action == AuditActionDelete {
		return []AuditChange{{
			FieldName:    "status",
			FieldLabel:   "档案状态",
			OldValue:     common.AlumniStatusActive,
			NewValue:     common.AlumniStatusDeleted,
			CurrentValue: common.AlumniStatusDeleted,
		}}
	}

	changes := make([]AuditChange, 0)
	for _, field := range personalAuditFields {
		oldValue := "未填写"
		newValue := "未填写"
		if before != nil {
			oldValue = field.value(before)
		}
		if after != nil {
			newValue = field.value(after)
		}
		if oldValue == newValue {
			continue
		}
		changes = append(changes, buildAuditChange(field.name, field.label, field.sensitive, oldValue, newValue))
	}
	return changes
}

func enrollmentAuditChanges(action string, before, after *model.AlumniEnrollment, domainNames map[uint64]string) []AuditChange {
	if action == AuditActionDelete {
		return []AuditChange{{
			FieldName:    "status",
			FieldLabel:   "学籍状态",
			OldValue:     common.AlumniStatusActive,
			NewValue:     common.AlumniStatusDeleted,
			CurrentValue: common.AlumniStatusDeleted,
		}}
	}

	changes := make([]AuditChange, 0)
	for _, field := range enrollmentAuditFields {
		oldValue := "未填写"
		newValue := "未填写"
		if before != nil {
			oldValue = field.value(before)
		}
		if after != nil {
			newValue = field.value(after)
		}
		if oldValue == newValue {
			continue
		}
		if field.name == "data_domain_id" {
			if after != nil && domainNames[after.DataDomainID] != "" {
				oldValue = domainDisplayValue(before, domainNames)
				newValue = domainNames[after.DataDomainID]
			}
		}
		changes = append(changes, buildAuditChange(field.name, field.label, field.sensitive, oldValue, newValue))
	}
	return changes
}

func buildAuditChange(name, label string, sensitive bool, oldValue, newValue string) AuditChange {
	change := AuditChange{
		FieldName:  name,
		FieldLabel: label,
		Sensitive:  sensitive,
	}
	if sensitive {
		change.OldValue, change.NewValue = sensitiveTransition(oldValue, newValue)
		change.CurrentValue = change.NewValue
		return change
	}
	change.OldValue = oldValue
	change.NewValue = newValue
	change.CurrentValue = newValue
	return change
}

func sensitiveTransition(oldValue, newValue string) (string, string) {
	oldFilled := oldValue != "未填写"
	newFilled := newValue != "未填写"
	switch {
	case !oldFilled && newFilled:
		return "未填写", "首次填写"
	case oldFilled && !newFilled:
		return "已填写", "已清空"
	default:
		return "已填写", "已修改"
	}
}

// writeProfileAudit 记录个人共享字段变化。dataDomainIDs 用于按当前可访问学籍裁剪历史可见性。
func writeProfileAudit(
	ctx context.Context,
	writer AuditEventWriter,
	users repository.UserStore,
	operatorID uint64,
	action string,
	source string,
	reason string,
	managementScope string,
	dataDomainIDs []uint64,
	before *model.AlumniProfile,
	after *model.AlumniProfile,
) {
	if writer == nil || users == nil {
		return
	}

	operator, err := users.FindByID(ctx, operatorID)
	if err != nil {
		logger.Warn("failed to load audit operator", zap.Uint64("operator_id", operatorID), zap.Error(err))
		return
	}
	target := after
	if target == nil {
		target = before
	}
	if target == nil || target.ID == 0 {
		return
	}
	targetID := target.ID
	changes := profileAuditChanges(action, before, after)
	if len(changes) == 0 && action == AuditActionUpdate {
		return
	}
	event := AuditEvent{
		Operator:        operator,
		Action:          action,
		Source:          source,
		ManagementScope: managementScope,
		Reason:          reason,
		TargetType:      auditTargetAlumni,
		TargetID:        &targetID,
		TargetName:      target.Name,
		Changes:         changes,
		DataDomainIDs:   append([]uint64(nil), dataDomainIDs...),
	}
	if err := writer.WriteAudit(ctx, event); err != nil {
		logger.Warn("failed to write alumni audit", zap.Uint64("alumni_id", target.ID), zap.String("action", action), zap.Error(err))
	}
}

// writeEnrollmentAudit 记录具体学籍段的字段变化，日志仍以校友个人 ID 关联，
// 并附带学籍 ID 与标签，便于详情页定位到具体学籍。
func writeEnrollmentAudit(
	ctx context.Context,
	writer AuditEventWriter,
	users repository.UserStore,
	operatorID uint64,
	action string,
	source string,
	reason string,
	managementScope string,
	alumniID uint64,
	alumniName string,
	enrollmentLabel string,
	dataDomainID uint64,
	domainNames map[uint64]string,
	before *model.AlumniEnrollment,
	after *model.AlumniEnrollment,
) {
	if writer == nil || users == nil {
		return
	}

	var enrollmentID uint64
	if after != nil {
		enrollmentID = after.ID
	} else if before != nil {
		enrollmentID = before.ID
	}
	if enrollmentID == 0 {
		return
	}

	changes := enrollmentAuditChanges(action, before, after, domainNames)
	if len(changes) == 0 && action == AuditActionUpdate {
		return
	}
	operator, err := users.FindByID(ctx, operatorID)
	if err != nil {
		logger.Warn("failed to load audit operator", zap.Uint64("operator_id", operatorID), zap.Error(err))
		return
	}
	domainID := dataDomainID
	if after != nil {
		domainID = after.DataDomainID
	} else if before != nil {
		domainID = before.DataDomainID
	}
	targetID := alumniID
	event := AuditEvent{
		Operator:        operator,
		Action:          action,
		Source:          source,
		ManagementScope: managementScope,
		Reason:          reason,
		TargetType:      auditTargetAlumni,
		TargetID:        &targetID,
		TargetName:      alumniName,
		TargetMeta:      enrollmentLabel,
		Changes:         changes,
		DataDomainIDs:   []uint64{domainID},
		EnrollmentID:    &enrollmentID,
		EnrollmentLabel: enrollmentLabel,
	}
	if err := writer.WriteAudit(ctx, event); err != nil {
		logger.Warn("failed to write enrollment audit", zap.Uint64("alumni_id", alumniID), zap.Uint64("enrollment_id", enrollmentID), zap.String("action", action), zap.Error(err))
	}
}

func writeBatchAudit(
	ctx context.Context,
	writer AuditEventWriter,
	users repository.UserStore,
	operatorID uint64,
	success int,
	errorCount int,
) {
	if success == 0 || writer == nil || users == nil {
		return
	}
	operator, err := users.FindByID(ctx, operatorID)
	if err != nil {
		logger.Warn("failed to load batch audit operator", zap.Uint64("operator_id", operatorID), zap.Error(err))
		return
	}
	resultText := fmt.Sprintf("成功导入 %d 条", success)
	if errorCount > 0 {
		resultText += fmt.Sprintf("，%d 条未导入", errorCount)
	}
	if err := writer.WriteAudit(ctx, AuditEvent{
		Operator:        operator,
		Action:          "import",
		Source:          "admin_import",
		ManagementScope: defaultAuditScope,
		Reason:          "批量导入校友档案",
		TargetType:      auditTargetBatch,
		TargetName:      "校友档案批量导入",
		TargetMeta:      resultText,
		Changes: []AuditChange{{
			FieldName:    "alumni_profiles",
			FieldLabel:   "校友档案",
			OldValue:     "未导入",
			NewValue:     resultText,
			CurrentValue: resultText,
		}},
	}); err != nil {
		logger.Warn("failed to write batch alumni audit", zap.Uint64("operator_id", operatorID), zap.Error(err))
	}
}
