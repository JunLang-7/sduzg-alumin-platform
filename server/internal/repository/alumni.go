package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

type AlumniStore interface {
	List(ctx context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, int64, error)
	ListAll(ctx context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, error)
	CountActive(ctx context.Context) (int64, error)
	FindOnly(ctx context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, error)
	GetByID(ctx context.Context, id uint64) (*model.AlumniProfile, error)
	Create(ctx context.Context, profile *do.AlumniCreateProfile, operatorID uint64) (*model.AlumniProfile, error)
	BatchCreate(ctx context.Context, profiles []do.AlumniCreateProfile, operatorID uint64) error
	Update(ctx context.Context, id uint64, updaterID uint64, profile do.AlumniUpdateProfile) error
	Delete(ctx context.Context, id uint64, updaterID uint64) error
	UpdateEditableFields(ctx context.Context, id uint64, updaterID uint64, profile do.AlumniEditableProfile) error
	FindExistingByDedupKey(ctx context.Context, keys []do.AlumniDedupKey) (map[string]bool, error)
	FindByMobile(ctx context.Context, mobile string) (*model.AlumniProfile, error)
	FindByEmail(ctx context.Context, email string) (*model.AlumniProfile, error)
	UpdateMobile(ctx context.Context, id uint64, mobile string) error
	UpdateEmail(ctx context.Context, id uint64, email string) error
}

type AlumniRepository struct {
	db *gorm.DB
}

func NewAlumniRepository(db *gorm.DB) *AlumniRepository {
	return &AlumniRepository{db: db}
}

// applyFilters 构建带过滤条件的查询，供 List / ListAll / FindOnly 复用。
func applyFilters(db *gorm.DB, listQuery do.AlumniListQuery) *gorm.DB {
	qs := query.Use(db).AlumniProfile

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
	return db
}

// CountActive 统计当前活跃校友总数（不含过滤条件）。
func (r *AlumniRepository) CountActive(ctx context.Context) (int64, error) {
	if r.db == nil {
		return 0, common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AlumniProfile
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where(qs.Status.Eq(common.AlumniStatusActive)).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// FindOnly 仅查询数据行（不统计总数），用于已有缓存计数的场景。
func (r *AlumniRepository) FindOnly(ctx context.Context, listQuery do.AlumniListQuery) ([]*model.AlumniProfile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	listQuery = listQuery.Normalize()
	qs := query.Use(r.db).AlumniProfile
	db := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where(qs.Status.Eq(common.AlumniStatusActive))
	db = applyFilters(db, listQuery)

	var items []*model.AlumniProfile
	if err := db.
		Order(qs.ID.Desc()).
		Offset(listQuery.Page.Offset()).
		Limit(listQuery.Page.PageSize).
		Find(&items).
		Error; err != nil {
		return nil, err
	}
	return items, nil
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
		Where(qs.Status.Eq(common.AlumniStatusActive))
	db = applyFilters(db, listQuery)

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

// ListAll 根据查询条件获取所有校友记录（不分页），用于导出。
func (r *AlumniRepository) ListAll(ctx context.Context, listQuery do.AlumniListQuery) ([]*model.AlumniProfile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	listQuery = listQuery.Normalize()
	qs := query.Use(r.db).AlumniProfile
	db := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where(qs.Status.Eq(common.AlumniStatusActive))
	db = applyFilters(db, listQuery)

	var items []*model.AlumniProfile
	if err := db.Order(qs.ID.Desc()).Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

// GetByID 根据 ID 获取校友详情
func (r *AlumniRepository) GetByID(ctx context.Context, id uint64) (*model.AlumniProfile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AlumniProfile
	var item model.AlumniProfile
	err := r.db.WithContext(ctx).
		Where(qs.ID.Eq(id), qs.DeletedAt.IsNull(), qs.Status.Eq(common.AlumniStatusActive)).
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

// Create 新增校友档案。
func (r *AlumniRepository) Create(ctx context.Context, profile *do.AlumniCreateProfile, operatorID uint64) (*model.AlumniProfile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	if profile == nil {
		return nil, common.ErrInvalidRequest
	}

	item := &model.AlumniProfile{
		Name:           profile.Name,
		Grade:          profile.Grade,
		ClassName:      profile.ClassName,
		Cohort:         profile.Cohort,
		Counselor:      profile.Counselor,
		Mentor:         profile.Mentor,
		Major:          profile.Major,
		TrainingMode:   profile.TrainingMode,
		Industry:       profile.Industry,
		WorkUnit:       profile.WorkUnit,
		Position:       profile.Position,
		MailingAddress: profile.MailingAddress,
		Gender:         profile.Gender,
		Mobile:         profile.Mobile,
		Remark:         profile.Remark,
		Status:         profile.Status,
		CreatedBy:      &operatorID,
		UpdatedBy:      &operatorID,
	}

	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, err
	}

	return item, nil
}

// BatchCreate 批量新增校友档案。
func (r *AlumniRepository) BatchCreate(ctx context.Context, profiles []do.AlumniCreateProfile, operatorID uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}
	if len(profiles) == 0 {
		return nil
	}

	items := make([]*model.AlumniProfile, 0, len(profiles))
	for i := range profiles {
		p := profiles[i]
		items = append(items, &model.AlumniProfile{
			Name:           p.Name,
			Grade:          p.Grade,
			ClassName:      p.ClassName,
			Cohort:         p.Cohort,
			Counselor:      p.Counselor,
			Mentor:         p.Mentor,
			Major:          p.Major,
			TrainingMode:   p.TrainingMode,
			Industry:       p.Industry,
			WorkUnit:       p.WorkUnit,
			Position:       p.Position,
			MailingAddress: p.MailingAddress,
			Gender:         p.Gender,
			Mobile:         p.Mobile,
			Remark:         p.Remark,
			Status:         p.Status,
			CreatedBy:      &operatorID,
			UpdatedBy:      &operatorID,
		})
	}

	return r.db.WithContext(ctx).CreateInBatches(items, 100).Error
}

// FindExistingByDedupKey 批量查询已存在的 (姓名, 年级, 班级, 届数) 组合，返回 key 集合用于去重。
func (r *AlumniRepository) FindExistingByDedupKey(ctx context.Context, keys []do.AlumniDedupKey) (map[string]bool, error) {
	result := make(map[string]bool, len(keys))
	if r.db == nil || len(keys) == 0 {
		return result, nil
	}

	qs := query.Use(r.db).AlumniProfile
	db := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where(qs.DeletedAt.IsNull()).
		Where(qs.Status.Eq(common.AlumniStatusActive))

	cond := "(name = ? AND grade = ? AND COALESCE(class_name, '') = ? AND COALESCE(cohort, '') = ?)"
	var parts []string
	var args []interface{}
	for _, k := range keys {
		parts = append(parts, cond)
		args = append(args, k.Name, k.Grade, k.ClassName, k.Cohort)
	}
	db = db.Where("("+strings.Join(parts, " OR ")+")", args...)

	var existing []model.AlumniProfile
	if err := db.Select("name, grade, class_name, cohort").Find(&existing).Error; err != nil {
		return nil, err
	}

	for _, e := range existing {
		cn := ""
		if e.ClassName != nil {
			cn = *e.ClassName
		}
		ch := ""
		if e.Cohort != nil {
			ch = *e.Cohort
		}
		result[do.AlumniDedupKey{Name: e.Name, Grade: e.Grade, ClassName: cn, Cohort: ch}.Key()] = true
	}
	return result, nil
}

// Update 编辑管理员可维护的校友档案字段。
func (r *AlumniRepository) Update(ctx context.Context, id uint64, updaterID uint64, profile do.AlumniUpdateProfile) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	profile = profile.Normalize()
	qs := query.Use(r.db).AlumniProfile
	updates := map[string]any{
		qs.Name.ColumnName().String():      profile.Name,
		qs.Grade.ColumnName().String():     profile.Grade,
		qs.UpdatedBy.ColumnName().String(): updaterID,
	}
	if profile.ClassName != nil {
		updates[qs.ClassName.ColumnName().String()] = nullableString(*profile.ClassName)
	}
	if profile.Cohort != nil {
		updates[qs.Cohort.ColumnName().String()] = nullableString(*profile.Cohort)
	}
	if profile.Counselor != nil {
		updates[qs.Counselor.ColumnName().String()] = nullableString(*profile.Counselor)
	}
	if profile.Mentor != nil {
		updates[qs.Mentor.ColumnName().String()] = nullableString(*profile.Mentor)
	}
	if profile.Major != nil {
		updates[qs.Major.ColumnName().String()] = nullableString(*profile.Major)
	}
	if profile.TrainingMode != nil {
		updates[qs.TrainingMode.ColumnName().String()] = nullableString(*profile.TrainingMode)
	}
	if profile.Industry != nil {
		updates[qs.Industry.ColumnName().String()] = nullableString(*profile.Industry)
	}
	if profile.WorkUnit != nil {
		updates[qs.WorkUnit.ColumnName().String()] = nullableString(*profile.WorkUnit)
	}
	if profile.Position != nil {
		updates[qs.Position.ColumnName().String()] = nullableString(*profile.Position)
	}
	if profile.MailingAddress != nil {
		updates[qs.MailingAddress.ColumnName().String()] = nullableString(*profile.MailingAddress)
	}
	if profile.Gender != nil {
		updates[qs.Gender.ColumnName().String()] = nullableString(*profile.Gender)
	}
	if profile.Mobile != nil {
		updates[qs.Mobile.ColumnName().String()] = nullableString(*profile.Mobile)
	}
	if profile.Remark != nil {
		updates[qs.Remark.ColumnName().String()] = nullableString(*profile.Remark)
	}

	result := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where(qs.ID.Eq(id), qs.DeletedAt.IsNull(), qs.Status.Eq(common.AlumniStatusActive)).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrAlumniNotFound
	}

	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// Delete 软删除校友档案。
func (r *AlumniRepository) Delete(ctx context.Context, id uint64, updaterID uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AlumniProfile
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			qs.Status.ColumnName().String():    common.AlumniStatusDeleted,
			qs.UpdatedBy.ColumnName().String(): updaterID,
		}
		updateResult := tx.Model(&model.AlumniProfile{}).
			Where(qs.ID.Eq(id), qs.DeletedAt.IsNull(), qs.Status.Eq(common.AlumniStatusActive)).
			Updates(updates)
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected == 0 {
			return common.ErrAlumniNotFound
		}

		deleteResult := tx.Where(qs.ID.Eq(id), qs.DeletedAt.IsNull()).Delete(&model.AlumniProfile{})
		if deleteResult.Error != nil {
			return deleteResult.Error
		}
		if deleteResult.RowsAffected == 0 {
			return common.ErrAlumniNotFound
		}

		return nil
	})
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
		updates[qs.WorkUnit.ColumnName().String()] = nullableString(*profile.WorkUnit)
	}
	if profile.Position != nil {
		updates[qs.Position.ColumnName().String()] = nullableString(*profile.Position)
	}
	if profile.MailingAddress != nil {
		updates[qs.MailingAddress.ColumnName().String()] = nullableString(*profile.MailingAddress)
	}
	if profile.Mobile != nil {
		updates[qs.Mobile.ColumnName().String()] = nullableString(*profile.Mobile)
	}

	result := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where(qs.ID.Eq(id), qs.DeletedAt.IsNull(), qs.Status.Eq(common.AlumniStatusActive)).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrAlumniNotFound
	}

	return nil
}

// FindByMobile 通过手机号查找校友档案
func (r *AlumniRepository) FindByMobile(ctx context.Context, mobile string) (*model.AlumniProfile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var profile model.AlumniProfile
	qs := query.Use(r.db).AlumniProfile
	err := r.db.WithContext(ctx).
		Where(qs.Mobile.Eq(mobile), qs.DeletedAt.IsNull(), qs.Status.Eq(common.AlumniStatusActive)).
		First(&profile).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrAlumniNotFound
	}
	if err != nil {
		return nil, err
	}

	return &profile, nil
}

// FindByEmail 通过邮箱查找校友档案（大小写不敏感）
func (r *AlumniRepository) FindByEmail(ctx context.Context, email string) (*model.AlumniProfile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	lowerEmail := strings.ToLower(email)
	var profile model.AlumniProfile
	qs := query.Use(r.db).AlumniProfile
	err := r.db.WithContext(ctx).
		Where(qs.Email.Lower().Eq(lowerEmail), qs.Status.Eq(common.AlumniStatusActive)).
		First(&profile).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrAlumniNotFound
	}
	if err != nil {
		return nil, err
	}

	return &profile, nil
}


// UpdateMobile 更新校友手机号
func (r *AlumniRepository) UpdateMobile(ctx context.Context, id uint64, mobile string) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}
	qs := query.Use(r.db).AlumniProfile
	return r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where(qs.ID.Eq(id)).
		Update(qs.Mobile.ColumnName().String(), mobile).
		Error
}

// UpdateEmail 更新校友邮箱（小写）
func (r *AlumniRepository) UpdateEmail(ctx context.Context, id uint64, email string) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}
	qs := query.Use(r.db).AlumniProfile
	return r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where(qs.ID.Eq(id)).
		Update(qs.Email.ColumnName().String(), strings.ToLower(email)).
		Error
}
