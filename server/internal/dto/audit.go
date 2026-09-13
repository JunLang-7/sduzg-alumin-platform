package dto

import (
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
)

const auditDateFormat = "2006-01-02"

// AuditListRequest 操作历史列表查询参数。
type AuditListRequest struct {
	Page            int    `form:"page"`
	PageSize        int    `form:"page_size"`
	StartDate       string `form:"start_date"`
	EndDate         string `form:"end_date"`
	ManagementScope string `form:"management_scope"`
	Action          string `form:"action"`
	TargetID        uint64 `form:"target_id"`
}

// ToQuery 将日期边界转换为左闭右开时间范围。
func (r AuditListRequest) ToQuery() (do.AuditQuery, error) {
	query := do.AuditQuery{
		Page: common.PageQuery{
			Page:     r.Page,
			PageSize: r.PageSize,
		},
		ManagementScope: r.ManagementScope,
		Action:          r.Action,
	}

	if r.TargetID > 0 {
		query.TargetID = &r.TargetID
	}

	if r.StartDate != "" {
		start, err := time.Parse(auditDateFormat, r.StartDate)
		if err != nil {
			return do.AuditQuery{}, common.ErrInvalidRequest
		}
		query.StartAt = &start
	}
	if r.EndDate != "" {
		end, err := time.Parse(auditDateFormat, r.EndDate)
		if err != nil {
			return do.AuditQuery{}, common.ErrInvalidRequest
		}
		end = end.AddDate(0, 0, 1)
		query.EndAt = &end
	}

	return query.Normalize(), nil
}

// AuditChange 字段变化。敏感字段只返回脱敏状态，不返回原始值。
type AuditChange struct {
	FieldName    string `json:"field_name"`
	FieldLabel   string `json:"field_label"`
	OldValue     string `json:"old_value"`
	NewValue     string `json:"new_value"`
	CurrentValue string `json:"current_value,omitempty"`
	Sensitive    bool   `json:"sensitive,omitempty"`
}

// AuditOperation 一次业务操作对应一条记录。
type AuditOperation struct {
	ID                uint64        `json:"id"`
	CreatedAt         time.Time     `json:"created_at"`
	Operator          string        `json:"operator"`
	OperatorRole      string        `json:"operator_role"`
	OperatorRoleLabel string        `json:"operator_role_label"`
	TargetID          *uint64       `json:"target_id,omitempty"`
	TargetType        string        `json:"target_type"`
	TargetName        string        `json:"target_name"`
	TargetMeta        string        `json:"target_meta,omitempty"`
	ManagementScope   string        `json:"management_scope"`
	Action            string        `json:"action"`
	Source            string        `json:"source"`
	Reason            string        `json:"reason,omitempty"`
	Status            string        `json:"status"`
	Changes           []AuditChange `json:"changes,omitempty"`
}
