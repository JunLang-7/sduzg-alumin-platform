package do

import (
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
)

// AuditQuery 操作历史查询条件。
type AuditQuery struct {
	Page            common.PageQuery
	StartAt         *time.Time
	EndAt           *time.Time
	ManagementScope string
	Action          string
	TargetID        *uint64
}

// Normalize 清理查询条件并设置分页默认值。
func (q AuditQuery) Normalize() AuditQuery {
	q.Page = q.Page.Normalize()
	q.ManagementScope = strings.TrimSpace(q.ManagementScope)
	q.Action = strings.TrimSpace(q.Action)
	return q
}
