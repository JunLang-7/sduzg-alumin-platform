package repository

import (
	"context"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	querypkg "github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	operationLogQuery := querypkg.Use(r.db).OperationLog.As("logs")
	var items []AuditEntry
	if err := query.
		Order(clause.OrderBy{
			Expression: clause.CommaExpression{
				Exprs: []clause.Expression{
					operationLogQuery.CreatedAt.Desc(),
					operationLogQuery.ID.Desc(),
				},
			},
		}).
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
	operationLogQuery := querypkg.Use(r.db).OperationLog.As("logs")
	result := r.baseQuery(ctx).
		Where(operationLogQuery.ID.Eq(id)).
		Where(operationLogQuery.TargetType.In(auditTargetAlumni, auditTargetBatch)).
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

func applyAuditFilters(db *gorm.DB, auditQuery do.AuditQuery) *gorm.DB {
	operationLogQuery := querypkg.Use(db).OperationLog.As("logs")
	alumniProfileQuery := querypkg.Use(db).AlumniProfile
	dataDomainQuery := querypkg.Use(db).DataDomain

	db = db.Where(operationLogQuery.TargetType.In(auditTargetAlumni, auditTargetBatch))
	if auditQuery.StartAt != nil {
		db = db.Where(operationLogQuery.CreatedAt.Gte(*auditQuery.StartAt))
	}
	if auditQuery.EndAt != nil {
		db = db.Where(operationLogQuery.CreatedAt.Lt(*auditQuery.EndAt))
	}
	if auditQuery.Action != "" {
		db = db.Where(operationLogQuery.Action.Eq(auditQuery.Action))
	}
	if auditQuery.TargetID != nil {
		db = db.Where(
			operationLogQuery.TargetType.Eq(auditTargetAlumni),
			operationLogQuery.TargetID.Eq(*auditQuery.TargetID),
		)
	}
	if auditQuery.ManagementScope != "" {
		db = db.Where(clause.Or(
			clause.And(
				operationLogQuery.TargetType.Eq(auditTargetAlumni),
				dataDomainQuery.Name.Eq(auditQuery.ManagementScope),
			),
			clause.And(
				operationLogQuery.TargetType.Neq(auditTargetAlumni),
				gorm.Expr(
					"JSON_UNQUOTE(JSON_EXTRACT(?, '$.management_scope')) = ?",
					operationLogQuery.Detail,
					auditQuery.ManagementScope,
				),
			),
		))
	}
	if auditQuery.RestrictToDataDomains {
		if len(auditQuery.DataDomainIDs) == 0 {
			return db.Where("1 = 0")
		}
		db = db.Where(
			operationLogQuery.TargetType.Eq(auditTargetAlumni),
			alumniProfileQuery.DataDomainID.In(auditQuery.DataDomainIDs...),
		)
	}
	return db
}

var _ AuditStore = (*AuditRepository)(nil)
