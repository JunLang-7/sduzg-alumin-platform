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

// AuditEvent 是一条业务操作审计事件。
type AuditEvent struct {
	Operator        *model.User
	OperatorID      uint64
	Action          string
	Source          string
	ManagementScope string
	Reason          string
	TargetType      string
	TargetID        *uint64
	TargetName      string
	TargetMeta      string
	Changes         []AuditChange
}

type auditLogDetail struct {
	SchemaVersion     int           `json:"schema_version"`
	OperatorName      string        `json:"operator_name,omitempty"`
	OperatorRoleLabel string        `json:"operator_role_label,omitempty"`
	TargetID          *uint64       `json:"target_id,omitempty"`
	TargetName        string        `json:"target_name,omitempty"`
	TargetMeta        string        `json:"target_meta,omitempty"`
	ManagementScope   string        `json:"management_scope,omitempty"`
	Source            string        `json:"source,omitempty"`
	Reason            string        `json:"reason,omitempty"`
	Status            string        `json:"status,omitempty"`
	Changes           []AuditChange `json:"changes,omitempty"`
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
		SchemaVersion:     1,
		OperatorName:      operatorName,
		OperatorRoleLabel: operatorRoleLabel,
		TargetID:          event.TargetID,
		TargetName:        event.TargetName,
		TargetMeta:        event.TargetMeta,
		ManagementScope:   managementScope,
		Source:            event.Source,
		Reason:            event.Reason,
		Status:            status,
		Changes:           event.Changes,
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

var alumniAuditFields = []auditField{
	{name: "name", label: "姓名", value: func(p *model.AlumniProfile) string { return p.Name }},
	{name: "grade", label: "年级", value: func(p *model.AlumniProfile) string { return p.Grade }},
	{name: "class_name", label: "班级", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.ClassName) }},
	{name: "cohort", label: "届数", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Cohort) }},
	{name: "counselor", label: "辅导员", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Counselor) }},
	{name: "mentor", label: "导师", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Mentor) }},
	{name: "major", label: "专业", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Major) }},
	{name: "training_mode", label: "培养方式", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.TrainingMode) }},
	{name: "industry", label: "行业", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Industry) }},
	{name: "work_unit", label: "工作单位", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.WorkUnit) }},
	{name: "position", label: "职务", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Position) }},
	{name: "mailing_address", label: "通讯地址", sensitive: true, value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.MailingAddress) }},
	{name: "gender", label: "性别", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Gender) }},
	{name: "mobile", label: "手机号", sensitive: true, value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Mobile) }},
	{name: "email", label: "邮箱", sensitive: true, value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Email) }},
	{name: "remark", label: "管理员备注", value: func(p *model.AlumniProfile) string { return optionalProfileValue(p.Remark) }},
}

func optionalProfileValue(value *string) string {
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
	for _, field := range alumniAuditFields {
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

		change := AuditChange{
			FieldName:  field.name,
			FieldLabel: field.label,
			Sensitive:  field.sensitive,
		}
		if field.sensitive {
			change.OldValue, change.NewValue = sensitiveTransition(oldValue, newValue)
			change.CurrentValue = change.NewValue
		} else {
			change.OldValue = oldValue
			change.NewValue = newValue
			change.CurrentValue = newValue
		}
		changes = append(changes, change)
	}
	return changes
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

func profileAuditMeta(profile *model.AlumniProfile) string {
	if profile == nil {
		return ""
	}
	meta := profile.Grade
	if profile.ClassName != nil && *profile.ClassName != "" {
		meta += " · " + *profile.ClassName
	}
	return meta
}

func writeProfileAudit(
	ctx context.Context,
	opLogger *OperationLogger,
	users repository.UserStore,
	operatorID uint64,
	action string,
	source string,
	reason string,
	managementScope string,
	before *model.AlumniProfile,
	after *model.AlumniProfile,
) {
	if opLogger == nil || opLogger.db == nil || users == nil {
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
		TargetMeta:      profileAuditMeta(target),
		Changes:         changes,
	}
	if err := opLogger.WriteAudit(ctx, event); err != nil {
		logger.Warn("failed to write alumni audit", zap.Uint64("alumni_id", target.ID), zap.String("action", action), zap.Error(err))
	}
}

func writeBatchAudit(
	ctx context.Context,
	opLogger *OperationLogger,
	users repository.UserStore,
	operatorID uint64,
	success int,
	errorCount int,
) {
	if success == 0 || opLogger == nil || opLogger.db == nil || users == nil {
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
	if err := opLogger.WriteAudit(ctx, AuditEvent{
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
