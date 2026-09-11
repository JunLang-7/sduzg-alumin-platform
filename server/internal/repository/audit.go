package repository

import (
	"context"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"gorm.io/gorm"
)

const (
	auditTargetAlumni = "alumni_profile"
	auditTargetBatch  = "alumni_batch"
)

// AuditEntry 是操作日志及其当前关联展示信息的查询结果。
type AuditEntry struct {
	ID               uint64    `gorm:"column:id"`
	OperatorID       uint64    `gorm:"column:operator_id"`
	OperatorRole     string    `gorm:"column:operator_role"`
	Action           string    `gorm:"column:action"`
	TargetType       string    `gorm:"column:target_type"`
	TargetID         *uint64   `gorm:"column:target_id"`
	Detail           *string   `gorm:"column:detail"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	OperatorName     *string   `gorm:"column:operator_name"`
	OperatorAccount  *string   `gorm:"column:operator_account"`
	TargetName       *string   `gorm:"column:target_name"`
	TargetGrade      *string   `gorm:"column:target_grade"`
	TargetClass      *string   `gorm:"column:target_class"`
	TargetCohort     *string   `gorm:"column:target_cohort"`
	TargetDomainName *string   `gorm:"column:target_domain_name"`
}

// AuditStore 提供操作历史的只读查询能力。
type AuditStore interface {
	List(ctx context.Context, query do.AuditQuery) ([]AuditEntry, int64, error)
	GetByID(ctx context.Context, id uint64, query do.AuditQuery) (*AuditEntry, error)
}

type AuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) List(ctx context.Context, listQuery do.AuditQuery) ([]AuditEntry, int64, error) {
	if r.db == nil {
		return nil, 0, common.ErrDatabaseUnavailable
	}

	listQuery = listQuery.Normalize()
	countDB := r.scopeQuery(ctx)
	countDB = applyAuditFilters(countDB, listQuery)
	var total int64
	if err := countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := r.baseQuery(ctx)
	query = applyAuditFilters(query, listQuery)
	var items []AuditEntry
	if err := query.
		Order("logs.created_at DESC, logs.id DESC").
		Offset(listQuery.Page.Offset()).
		Limit(listQuery.Page.PageSize).
		Scan(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *AuditRepository) GetByID(ctx context.Context, id uint64, listQuery do.AuditQuery) (*AuditEntry, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	if id == 0 {
		return nil, common.ErrAuditNotFound
	}

	var item AuditEntry
	listQuery = listQuery.Normalize()
	result := r.baseQuery(ctx).
		Where("logs.id = ?", id).
		Where("logs.target_type IN ?", []string{auditTargetAlumni, auditTargetBatch}).
		Scopes(func(db *gorm.DB) *gorm.DB { return applyAuditFilters(db, listQuery) }).
		Limit(1).
		Scan(&item)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, common.ErrAuditNotFound
	}

	return &item, nil
}

func (r *AuditRepository) baseQuery(ctx context.Context) *gorm.DB {
	return r.scopeQuery(ctx).
		Select(`
			logs.id,
			logs.operator_id,
			logs.operator_role,
			logs.action,
			logs.target_type,
			logs.target_id,
			logs.detail,
			logs.created_at,
			COALESCE(NULLIF(users.real_name, ''), users.account) AS operator_name,
			users.account AS operator_account,
			alumni_profiles.name AS target_name,
			alumni_profiles.grade AS target_grade,
			alumni_profiles.class_name AS target_class,
			alumni_profiles.cohort AS target_cohort,
			data_domains.name AS target_domain_name`)
}

func (r *AuditRepository) scopeQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("operation_logs AS logs").
		Joins("LEFT JOIN users ON users.id = logs.operator_id").
		Joins("LEFT JOIN alumni_profiles ON logs.target_type = ? AND alumni_profiles.id = logs.target_id", auditTargetAlumni).
		Joins("LEFT JOIN data_domains ON data_domains.id = alumni_profiles.data_domain_id")
}

func applyAuditFilters(db *gorm.DB, query do.AuditQuery) *gorm.DB {
	db = db.Where("logs.target_type IN ?", []string{auditTargetAlumni, auditTargetBatch})
	if query.StartAt != nil {
		db = db.Where("logs.created_at >= ?", *query.StartAt)
	}
	if query.EndAt != nil {
		db = db.Where("logs.created_at < ?", *query.EndAt)
	}
	if query.Action != "" {
		db = db.Where("logs.action = ?", query.Action)
	}
	if query.TargetID != nil {
		db = db.Where("logs.target_type = ? AND logs.target_id = ?", auditTargetAlumni, *query.TargetID)
	}
	if query.ManagementScope != "" {
		db = db.Where(`(
			(logs.target_type = ? AND data_domains.name = ?)
			OR (logs.target_type <> ? AND JSON_UNQUOTE(JSON_EXTRACT(logs.detail, '$.management_scope')) = ?)
		)`, auditTargetAlumni, query.ManagementScope, auditTargetAlumni, query.ManagementScope)
	}
	if query.RestrictToDataDomains {
		if len(query.DataDomainIDs) == 0 {
			return db.Where("1 = 0")
		}
		db = db.Where("logs.target_type = ? AND alumni_profiles.data_domain_id IN ?", auditTargetAlumni, query.DataDomainIDs)
	}
	return db
}

var _ AuditStore = (*AuditRepository)(nil)
