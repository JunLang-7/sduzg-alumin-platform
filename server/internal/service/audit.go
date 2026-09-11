package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
)

// AuditService 提供操作历史查询，不提供历史记录修改、删除或恢复能力。
type AuditService struct {
	audits repository.AuditStore
}

func NewAuditService(audits repository.AuditStore) *AuditService {
	return &AuditService{audits: audits}
}

// List 查询全局或指定校友的操作历史。
func (s *AuditService) List(ctx context.Context, access common.AccessContext, req dto.AuditListRequest) (common.Pager[dto.AuditOperation], error) {
	query, err := req.ToQuery()
	if err != nil {
		return common.NewPager[dto.AuditOperation](nil, query.Page, 0), err
	}
	if s == nil || s.audits == nil {
		return common.NewPager[dto.AuditOperation](nil, query.Page, 0), common.ErrDatabaseUnavailable
	}
	if !access.IsAdministrator() {
		return common.NewPager[dto.AuditOperation](nil, query.Page, 0), common.ErrPermissionDenied
	}
	query = applyAuditAccess(query, access)

	items, total, err := s.audits.List(ctx, query)
	if err != nil {
		return common.NewPager[dto.AuditOperation](nil, query.Page, 0), err
	}

	result := make([]dto.AuditOperation, 0, len(items))
	for _, item := range items {
		result = append(result, mapAuditOperation(item, false))
	}
	return common.NewPager(result, query.Page, total), nil
}

// Detail 查询单条操作历史及字段差异。
func (s *AuditService) Detail(ctx context.Context, access common.AccessContext, id uint64) (*dto.AuditOperation, error) {
	if s == nil || s.audits == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	if !access.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	item, err := s.audits.GetByID(ctx, id, applyAuditAccess(do.AuditQuery{}, access))
	if err != nil {
		return nil, err
	}
	result := mapAuditOperation(*item, true)
	return &result, nil
}

func applyAuditAccess(query do.AuditQuery, access common.AccessContext) do.AuditQuery {
	if access.IsSuperAdmin() {
		return query
	}
	query.RestrictToDataDomains = true
	query.DataDomainIDs = append([]uint64(nil), access.DomainIDs...)
	return query
}

func mapAuditOperation(item repository.AuditEntry, includeChanges bool) dto.AuditOperation {
	detail := auditLogDetail{}
	if item.Detail != nil && strings.TrimSpace(*item.Detail) != "" {
		_ = json.Unmarshal([]byte(*item.Detail), &detail)
	}

	operator := detail.OperatorName
	if operator == "" && item.OperatorName != nil {
		operator = *item.OperatorName
	}
	if operator == "" && item.OperatorAccount != nil {
		operator = *item.OperatorAccount
	}
	if operator == "" {
		operator = "未知用户"
	}

	operatorRole := item.OperatorRole
	if operatorRole == "" {
		operatorRole = roleFromDetail(detail.OperatorRoleLabel)
	}
	roleLabel := detail.OperatorRoleLabel
	if roleLabel == "" {
		roleLabel = auditRoleLabel(operatorRole)
	}

	targetID := item.TargetID
	if targetID == nil {
		targetID = detail.TargetID
	}
	targetName := detail.TargetName
	if targetName == "" && item.TargetName != nil {
		targetName = *item.TargetName
	}
	if targetName == "" && item.TargetType == auditTargetBatch {
		targetName = "校友档案批量导入"
	}
	if targetName == "" {
		targetName = "未命名对象"
	}

	targetMeta := detail.TargetMeta
	if targetMeta == "" {
		targetMeta = joinedTargetMeta(item.TargetGrade, item.TargetClass, item.TargetCohort)
	}
	managementScope := detail.ManagementScope
	if (managementScope == "" || managementScope == defaultAuditScope) && item.TargetDomainName != nil && *item.TargetDomainName != "" {
		managementScope = *item.TargetDomainName
	}
	if managementScope == "" {
		managementScope = defaultAuditScope
	}
	source := detail.Source
	if source == "" {
		if operatorRole == common.RoleAlumni {
			source = "alumni_self"
		} else {
			source = "admin"
		}
	}
	status := detail.Status
	if status == "" {
		status = "applied"
	}

	result := dto.AuditOperation{
		ID:                item.ID,
		CreatedAt:         item.CreatedAt,
		Operator:          operator,
		OperatorRole:      operatorRole,
		OperatorRoleLabel: roleLabel,
		TargetID:          targetID,
		TargetType:        item.TargetType,
		TargetName:        targetName,
		TargetMeta:        targetMeta,
		ManagementScope:   managementScope,
		Action:            item.Action,
		Source:            source,
		Reason:            detail.Reason,
		Status:            status,
	}
	if includeChanges {
		result.Changes = make([]dto.AuditChange, 0, len(detail.Changes))
		for _, change := range detail.Changes {
			result.Changes = append(result.Changes, mapAuditChange(change))
		}
	}
	return result
}

var sensitiveAuditFields = map[string]bool{
	"mobile":          true,
	"email":           true,
	"mailing_address": true,
}

func mapAuditChange(change AuditChange) dto.AuditChange {
	if !change.Sensitive && !sensitiveAuditFields[change.FieldName] {
		return dto.AuditChange{
			FieldName:    change.FieldName,
			FieldLabel:   change.FieldLabel,
			OldValue:     change.OldValue,
			NewValue:     change.NewValue,
			CurrentValue: change.CurrentValue,
		}
	}

	oldValue, newValue := safeSensitiveTransition(change.OldValue, change.NewValue)
	return dto.AuditChange{
		FieldName:    change.FieldName,
		FieldLabel:   change.FieldLabel,
		OldValue:     oldValue,
		NewValue:     newValue,
		CurrentValue: newValue,
		Sensitive:    true,
	}
}

func safeSensitiveTransition(oldValue, newValue string) (string, string) {
	oldFilled := sensitiveValueFilled(oldValue)
	newFilled := sensitiveValueFilled(newValue)
	switch {
	case !oldFilled && newFilled:
		return "未填写", "首次填写"
	case oldFilled && !newFilled:
		return "已填写", "已清空"
	case oldFilled && newFilled:
		return "已填写", "已修改"
	default:
		return "未填写", "未填写"
	}
}

func sensitiveValueFilled(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "未填写", "已清空":
		return false
	default:
		return true
	}
}

func roleFromDetail(roleLabel string) string {
	switch roleLabel {
	case "超级管理员":
		return common.RoleSuperAdmin
	case "管理员", "主管理员", "子管理员":
		return common.RoleAdmin
	case "校友本人":
		return common.RoleAlumni
	default:
		return ""
	}
}

func joinedTargetMeta(grade, className, cohort *string) string {
	parts := make([]string, 0, 2)
	if grade != nil && *grade != "" {
		parts = append(parts, *grade)
	}
	if className != nil && *className != "" {
		parts = append(parts, *className)
	}
	if len(parts) == 0 && cohort != nil && *cohort != "" {
		return *cohort
	}
	return strings.Join(parts, " · ")
}
