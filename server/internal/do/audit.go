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
	// RestrictToDataDomains 表示查询是否必须限定在授权的数据域内。
	// 超级管理员不设置该标记；普通管理员即使没有分配数据域，也必须返回空结果。
	RestrictToDataDomains bool
	DataDomainIDs         []uint64
}

// Normalize 清理查询条件并设置分页默认值。
func (q AuditQuery) Normalize() AuditQuery {
	q.Page = q.Page.Normalize()
	q.ManagementScope = strings.TrimSpace(q.ManagementScope)
	q.Action = strings.TrimSpace(q.Action)
	q.DataDomainIDs = append([]uint64(nil), q.DataDomainIDs...)
	return q
}
