package repository

import (
	"context"
	"errors"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

const AlumniStatusActive = "active"

type AlumniStore interface {
	List(ctx context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, int64, error)
	GetByID(ctx context.Context, id uint64) (*model.AlumniProfile, error)
	UpdateEditableFields(ctx context.Context, id uint64, updaterID uint64, profile do.AlumniEditableProfile) error
}

type AlumniRepository struct {
	db *gorm.DB
}

func NewAlumniRepository(db *gorm.DB) *AlumniRepository {
	return &AlumniRepository{db: db}
}

// List 根据查询条件分页获取校友列表
func (r *AlumniRepository) List(ctx context.Context, listQuery do.AlumniListQuery) ([]*model.AlumniProfile, int64, error) {
	if r.db == nil {
		return nil, 0, common.ErrDatabaseUnavailable
	}

	listQuery = listQuery.Normalize()
	qs := query.Use(r.db).AlumniProfile
	db := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where(qs.DeletedAt.IsNull()).
		Where(qs.Status.Eq(AlumniStatusActive))

	if listQuery.Keyword != "" {
		like := "%" + listQuery.Keyword + "%"
		db = db.Where(field.Or(
			qs.Name.Like(like),
			qs.WorkUnit.Like(like),
			qs.Position.Like(like),
			qs.Mentor.Like(like),
			qs.Counselor.Like(like),
			qs.Mobile.Like(like),
		))
	}
	if listQuery.Grade != "" {
		db = db.Where(qs.Grade.Eq(listQuery.Grade))
	}
	if listQuery.ClassName != "" {
		db = db.Where(qs.ClassName.Eq(listQuery.ClassName))
	}
	if listQuery.Cohort != "" {
		db = db.Where(qs.Cohort.Eq(listQuery.Cohort))
	}
	if listQuery.Counselor != "" {
		db = db.Where(qs.Counselor.Eq(listQuery.Counselor))
	}
	if listQuery.Mentor != "" {
		db = db.Where(qs.Mentor.Eq(listQuery.Mentor))
	}
	if listQuery.Major != "" {
		db = db.Where(qs.Major.Eq(listQuery.Major))
	}
	if listQuery.TrainingMode != "" {
		db = db.Where(qs.TrainingMode.Eq(listQuery.TrainingMode))
	}
	if listQuery.Industry != "" {
		db = db.Where(qs.Industry.Eq(listQuery.Industry))
	}
	if listQuery.WorkUnit != "" {
		db = db.Where(qs.WorkUnit.Like("%" + listQuery.WorkUnit + "%"))
	}
	if listQuery.Position != "" {
		db = db.Where(qs.Position.Like("%" + listQuery.Position + "%"))
	}
	if listQuery.Mobile != "" {
		db = db.Where(qs.Mobile.Eq(listQuery.Mobile))
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.AlumniProfile
	if err := db.
		Order(qs.ID.Desc()).
		Offset(listQuery.Page.Offset()).
		Limit(listQuery.Page.PageSize).
		Find(&items).
		Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetByID 根据 ID 获取校友详情
func (r *AlumniRepository) GetByID(ctx context.Context, id uint64) (*model.AlumniProfile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AlumniProfile
	var item model.AlumniProfile
	err := r.db.WithContext(ctx).
		Where(qs.ID.Eq(id), qs.DeletedAt.IsNull(), qs.Status.Eq(AlumniStatusActive)).
		First(&item).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrAlumniNotFound
	}
	if err != nil {
		return nil, err
	}

	return &item, nil
}

// UpdateEditableFields 更新校友本人允许维护的四个字段。
func (r *AlumniRepository) UpdateEditableFields(ctx context.Context, id uint64, updaterID uint64, profile do.AlumniEditableProfile) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	profile = profile.Normalize()
	if profile.IsEmpty() {
		return nil
	}

	qs := query.Use(r.db).AlumniProfile
	updates := map[string]any{
		qs.UpdatedBy.ColumnName().String(): updaterID,
	}
	if profile.WorkUnit != nil {
		updates[qs.WorkUnit.ColumnName().String()] = *profile.WorkUnit
	}
	if profile.Position != nil {
		updates[qs.Position.ColumnName().String()] = *profile.Position
	}
	if profile.MailingAddress != nil {
		updates[qs.MailingAddress.ColumnName().String()] = *profile.MailingAddress
	}
	if profile.Mobile != nil {
		updates[qs.Mobile.ColumnName().String()] = *profile.Mobile
	}

	result := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where(qs.ID.Eq(id), qs.DeletedAt.IsNull(), qs.Status.Eq(AlumniStatusActive)).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrAlumniNotFound
	}

	return nil
}
