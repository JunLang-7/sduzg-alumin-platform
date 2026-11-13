package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AlumniStore 校友个人主档案与学籍段的持久化接口。
// 个人档案保存稳定的身份与账号绑定；学籍段保存培养信息、管理数据域与内部备注。
type AlumniStore interface {
	List(ctx context.Context, query do.AlumniListQuery) ([]do.AlumniRecord, int64, error)
	FindOnly(ctx context.Context, query do.AlumniListQuery) ([]do.AlumniRecord, error)
	ListAll(ctx context.Context, query do.AlumniListQuery) ([]do.AlumniExportRow, error)
	CountActive(ctx context.Context) (int64, error)
	GetDetail(ctx context.Context, id uint64, dataDomainIDs []uint64) (*do.AlumniRecord, error)
	GetPersonalProfile(ctx context.Context, id uint64) (*model.AlumniProfile, error)
	GetEnrollment(ctx context.Context, alumniID uint64, enrollmentID uint64, dataDomainIDs []uint64) (*model.AlumniEnrollment, error)
	Create(ctx context.Context, input do.AlumniCreateInput, operatorID uint64) (*do.AlumniRecord, error)
	BatchCreate(ctx context.Context, inputs []do.AlumniCreateInput, operatorID uint64) ([]uint64, error)
	Save(ctx context.Context, id uint64, input do.AlumniSaveInput, operatorID uint64, dataDomainIDs []uint64) (*do.AlumniSaveResult, error)
	Delete(ctx context.Context, id uint64, updaterID uint64, dataDomainIDs []uint64) error
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

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

// enrollmentFilterSQL 构建单段学籍的过滤条件。
// 所有学籍条件（含数据域范围）都必须命中同一条 alumni_enrollments 记录。
func enrollmentFilterSQL(listQuery do.AlumniListQuery, alias string, includeKeyword bool) (string, []any) {
	conditions := []string{
		alias + ".alumni_id = alumni_profiles.id",
		alias + ".status = ?",
		alias + ".deleted_at IS NULL",
	}
	args := []any{common.AlumniStatusActive}
	if len(listQuery.DataDomainIDs) > 0 {
		conditions = append(conditions, alias+".data_domain_id IN ("+placeholders(len(listQuery.DataDomainIDs))+")")
		for _, id := range listQuery.DataDomainIDs {
			args = append(args, id)
		}
	}
	exact := func(column, value string) {
		if value == "" {
			return
		}
		conditions = append(conditions, alias+"."+column+" = ?")
		args = append(args, value)
	}
	exact("grade", listQuery.Grade)
	exact("class_name", listQuery.ClassName)
	exact("cohort", listQuery.Cohort)
	exact("counselor", listQuery.Counselor)
	exact("mentor", listQuery.Mentor)
	exact("major", listQuery.Major)
	exact("training_mode", listQuery.TrainingMode)
	exact("training_level", listQuery.TrainingLevel)
	if includeKeyword && listQuery.Keyword != "" {
		like := "%" + listQuery.Keyword + "%"
		conditions = append(conditions, "("+alias+".mentor LIKE ? OR "+alias+".counselor LIKE ?)")
		args = append(args, like, like)
	}
	return strings.Join(conditions, " AND "), args
}

// applyFilters 构建带过滤条件的查询，供 List / ListAll / FindOnly 复用。
// 个人信息条件作用在个人档案上；学籍条件通过 EXISTS 子查询要求同一段学籍满足。
func applyFilters(db *gorm.DB, listQuery do.AlumniListQuery) *gorm.DB {
	if listQuery.Keyword != "" {
		like := "%" + listQuery.Keyword + "%"
		personConditions := []string{"alumni_profiles.name LIKE ?", "alumni_profiles.industry LIKE ?"}
		args := []any{like, like}
		if listQuery.CanReadSensitive {
			personConditions = append(personConditions,
				"alumni_profiles.work_unit LIKE ?",
				"alumni_profiles.position LIKE ?",
				"alumni_profiles.mobile LIKE ?",
			)
			args = append(args, like, like, like)
		}
		enrollmentCondition, enrollmentArgs := enrollmentFilterSQL(listQuery, "e", true)
		sql := "(" + strings.Join(personConditions, " OR ") +
			" OR EXISTS (SELECT 1 FROM alumni_enrollments AS e WHERE " + enrollmentCondition + "))"
		db = db.Where(sql, append(args, enrollmentArgs...)...)
	}
	if listQuery.Industry != "" {
		db = db.Where("alumni_profiles.industry = ?", listQuery.Industry)
	}
	if listQuery.CanReadSensitive && listQuery.WorkUnit != "" {
		db = db.Where("alumni_profiles.work_unit LIKE ?", "%"+listQuery.WorkUnit+"%")
	}
	if listQuery.CanReadSensitive && listQuery.Position != "" {
		db = db.Where("alumni_profiles.position LIKE ?", "%"+listQuery.Position+"%")
	}
	if listQuery.CanReadSensitive && listQuery.Mobile != "" {
		db = db.Where("alumni_profiles.mobile = ?", listQuery.Mobile)
	}
	enrollmentCondition, enrollmentArgs := enrollmentFilterSQL(listQuery, "e", false)
	db = db.Where("EXISTS (SELECT 1 FROM alumni_enrollments AS e WHERE "+enrollmentCondition+")", enrollmentArgs...)
	return db
}

func activeAlumniQuery(db *gorm.DB) *gorm.DB {
	return db.Model(&model.AlumniProfile{}).
		Where("alumni_profiles.status = ?", common.AlumniStatusActive).
		Where("alumni_profiles.deleted_at IS NULL")
}

// CountActive 统计当前活跃校友总数（不含过滤条件，按人去重）。
func (r *AlumniRepository) CountActive(ctx context.Context) (int64, error) {
	if r.db == nil {
		return 0, common.ErrDatabaseUnavailable
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where("status = ?", common.AlumniStatusActive).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// FindOnly 仅查询数据行（不统计总数），用于已有缓存计数的场景。
func (r *AlumniRepository) FindOnly(ctx context.Context, listQuery do.AlumniListQuery) ([]do.AlumniRecord, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	listQuery = listQuery.Normalize()
	base := activeAlumniQuery(r.db.WithContext(ctx))
	base = applyFilters(base, listQuery)

	var profiles []*model.AlumniProfile
	if err := base.
		Order("alumni_profiles.id DESC").
		Offset(listQuery.Page.Offset()).
		Limit(listQuery.Page.PageSize).
		Find(&profiles).
		Error; err != nil {
		return nil, err
	}
	return r.attachEnrollments(ctx, profiles, listQuery.DataDomainIDs)
}

// List 根据查询条件分页获取校友列表。
func (r *AlumniRepository) List(ctx context.Context, listQuery do.AlumniListQuery) ([]do.AlumniRecord, int64, error) {
	if r.db == nil {
		return nil, 0, common.ErrDatabaseUnavailable
	}

	listQuery = listQuery.Normalize()
	base := activeAlumniQuery(r.db.WithContext(ctx))
	base = applyFilters(base, listQuery)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var profiles []*model.AlumniProfile
	if err := base.
		Order("alumni_profiles.id DESC").
		Offset(listQuery.Page.Offset()).
		Limit(listQuery.Page.PageSize).
		Find(&profiles).
		Error; err != nil {
		return nil, 0, err
	}
	records, err := r.attachEnrollments(ctx, profiles, listQuery.DataDomainIDs)
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// ListAll 返回导出的平铺行：匹配到的每位校友的每一段可访问学籍各占一行。
// 学籍筛选条件（不含关键字）用于裁剪导出行，关键字按个人匹配处理。
func (r *AlumniRepository) ListAll(ctx context.Context, listQuery do.AlumniListQuery) ([]do.AlumniExportRow, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	listQuery = listQuery.Normalize()
	base := activeAlumniQuery(r.db.WithContext(ctx))
	base = applyFilters(base, listQuery)

	var profiles []*model.AlumniProfile
	if err := base.Order("alumni_profiles.id DESC").Find(&profiles).Error; err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, nil
	}

	profileByID := make(map[uint64]*model.AlumniProfile, len(profiles))
	profileIDs := make([]uint64, 0, len(profiles))
	for _, profile := range profiles {
		profileByID[profile.ID] = profile
		profileIDs = append(profileIDs, profile.ID)
	}

	// 导出行仍要满足学籍筛选条件；关键字已在个人查询中处理，避免只导出“姓名命中”的空行。
	exportFilter := listQuery
	exportFilter.Keyword = ""
	enrollments, err := r.enrollmentsFor(ctx, profileIDs, exportFilter)
	if err != nil {
		return nil, err
	}

	rows := make([]do.AlumniExportRow, 0, len(enrollments))
	for _, enrollment := range enrollments {
		profile := profileByID[enrollment.AlumniID]
		if profile == nil {
			continue
		}
		rows = append(rows, do.AlumniExportRow{Profile: profile, Enrollment: enrollment})
	}
	return rows, nil
}

// GetDetail 根据 ID 和可访问数据域获取校友详情。
// 普通管理员必须至少有一段可访问学籍，否则按不存在处理，避免泄露隐藏身份。
func (r *AlumniRepository) GetDetail(ctx context.Context, id uint64, dataDomainIDs []uint64) (*do.AlumniRecord, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	db := activeAlumniQuery(r.db.WithContext(ctx)).
		Where("alumni_profiles.id = ?", id)
	if len(dataDomainIDs) > 0 {
		db = db.Where("EXISTS (SELECT 1 FROM alumni_enrollments AS e WHERE e.alumni_id = alumni_profiles.id AND e.status = ? AND e.deleted_at IS NULL AND e.data_domain_id IN ("+placeholders(len(dataDomainIDs))+"))", append([]any{common.AlumniStatusActive}, uint64Args(dataDomainIDs)...)...)
	}

	var profile model.AlumniProfile
	err := db.First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrAlumniNotFound
	}
	if err != nil {
		return nil, err
	}

	records, err := r.attachEnrollments(ctx, []*model.AlumniProfile{&profile}, dataDomainIDs)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 || len(records[0].Enrollments) == 0 {
		return nil, common.ErrAlumniNotFound
	}
	return &records[0], nil
}

// GetPersonalProfile 读取个人主档案，不校验数据域，供认证与本人资料流程使用。
func (r *AlumniRepository) GetPersonalProfile(ctx context.Context, id uint64) (*model.AlumniProfile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var profile model.AlumniProfile
	err := activeAlumniQuery(r.db.WithContext(ctx)).
		Where("alumni_profiles.id = ?", id).
		First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrAlumniNotFound
	}
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetEnrollment 读取单段学籍，并校验所属校友可用、学籍在可访问数据域内。
func (r *AlumniRepository) GetEnrollment(ctx context.Context, alumniID uint64, enrollmentID uint64, dataDomainIDs []uint64) (*model.AlumniEnrollment, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	db := r.db.WithContext(ctx).
		Table("alumni_enrollments AS e").
		Select("e.*").
		Joins("JOIN alumni_profiles AS p ON p.id = e.alumni_id AND p.status = ? AND p.deleted_at IS NULL", common.AlumniStatusActive).
		Where("e.id = ? AND e.alumni_id = ? AND e.status = ? AND e.deleted_at IS NULL", enrollmentID, alumniID, common.AlumniStatusActive)
	if len(dataDomainIDs) > 0 {
		db = db.Where("e.data_domain_id IN ("+placeholders(len(dataDomainIDs))+")", uint64Args(dataDomainIDs)...)
	}

	var enrollment model.AlumniEnrollment
	if err := db.First(&enrollment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrEnrollmentNotFound
		}
		return nil, err
	}
	return &enrollment, nil
}

// attachEnrollments 为已查询的个人档案补充可访问学籍段。
func (r *AlumniRepository) attachEnrollments(ctx context.Context, profiles []*model.AlumniProfile, dataDomainIDs []uint64) ([]do.AlumniRecord, error) {
	records := make([]do.AlumniRecord, 0, len(profiles))
	if len(profiles) == 0 {
		return records, nil
	}
	ids := make([]uint64, 0, len(profiles))
	for _, profile := range profiles {
		if profile != nil {
			ids = append(ids, profile.ID)
		}
	}
	filter := do.AlumniListQuery{DataDomainIDs: dataDomainIDs}
	enrollments, err := r.enrollmentsFor(ctx, ids, filter)
	if err != nil {
		return nil, err
	}
	byAlumni := make(map[uint64][]*model.AlumniEnrollment, len(ids))
	for _, enrollment := range enrollments {
		byAlumni[enrollment.AlumniID] = append(byAlumni[enrollment.AlumniID], enrollment)
	}
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		items := byAlumni[profile.ID]
		if items == nil {
			items = []*model.AlumniEnrollment{}
		}
		records = append(records, do.AlumniRecord{Profile: profile, Enrollments: items})
	}
	return records, nil
}

// enrollmentsFor 查询指定校友的可访问学籍段，并按查询条件裁剪。
func (r *AlumniRepository) enrollmentsFor(ctx context.Context, alumniIDs []uint64, listQuery do.AlumniListQuery) ([]*model.AlumniEnrollment, error) {
	if len(alumniIDs) == 0 {
		return nil, nil
	}

	conditions := []string{
		"e.alumni_id IN (" + placeholders(len(alumniIDs)) + ")",
		"e.status = ?",
		"e.deleted_at IS NULL",
	}
	args := make([]any, 0, len(alumniIDs)+8)
	for _, id := range alumniIDs {
		args = append(args, id)
	}
	args = append(args, common.AlumniStatusActive)
	if len(listQuery.DataDomainIDs) > 0 {
		conditions = append(conditions, "e.data_domain_id IN ("+placeholders(len(listQuery.DataDomainIDs))+")")
		args = append(args, uint64Args(listQuery.DataDomainIDs)...)
	}
	exact := func(column, value string) {
		if value == "" {
			return
		}
		conditions = append(conditions, "e."+column+" = ?")
		args = append(args, value)
	}
	exact("grade", listQuery.Grade)
	exact("class_name", listQuery.ClassName)
	exact("cohort", listQuery.Cohort)
	exact("counselor", listQuery.Counselor)
	exact("mentor", listQuery.Mentor)
	exact("major", listQuery.Major)
	exact("training_mode", listQuery.TrainingMode)
	exact("training_level", listQuery.TrainingLevel)

	var enrollments []*model.AlumniEnrollment
	if err := r.db.WithContext(ctx).
		Table("alumni_enrollments AS e").
		Select("e.*").
		Where(strings.Join(conditions, " AND "), args...).
		Order("e.alumni_id ASC, e.id ASC").
		Find(&enrollments).Error; err != nil {
		return nil, err
	}
	return enrollments, nil
}

func uint64Args(values []uint64) []any {
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return args
}

// Create 新增校友：一份个人档案 + 至少一段学籍，写在同一事务内。
func (r *AlumniRepository) Create(ctx context.Context, input do.AlumniCreateInput, operatorID uint64) (*do.AlumniRecord, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	input = input.Normalize()
	if input.Personal.Name == "" || len(input.Enrollments) == 0 {
		return nil, common.ErrInvalidRequest
	}

	domainIDs := make([]uint64, 0, len(input.Enrollments))
	for _, enrollment := range input.Enrollments {
		if enrollment.DataDomainID == 0 {
			return nil, common.ErrInvalidDataDomain
		}
		domainIDs = append(domainIDs, enrollment.DataDomainID)
	}
	if err := r.validateActiveDataDomainIDs(ctx, domainIDs); err != nil {
		return nil, err
	}

	var profileID uint64
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		profile := &model.AlumniProfile{
			Name:           input.Personal.Name,
			Industry:       input.Personal.Industry,
			WorkUnit:       input.Personal.WorkUnit,
			Position:       input.Personal.Position,
			MailingAddress: input.Personal.MailingAddress,
			Gender:         input.Personal.Gender,
			Mobile:         input.Personal.Mobile,
			Email:          input.Personal.Email,
			Status:         input.Status,
			Version:        1,
			CreatedBy:      &operatorID,
			UpdatedBy:      &operatorID,
		}
		if err := tx.Create(profile).Error; err != nil {
			return err
		}
		profileID = profile.ID

		enrollments := make([]*model.AlumniEnrollment, 0, len(input.Enrollments))
		for _, item := range input.Enrollments {
			enrollments = append(enrollments, &model.AlumniEnrollment{
				AlumniID:      profileID,
				DataDomainID:  item.DataDomainID,
				TrainingLevel: item.TrainingLevel,
				Grade:         item.Grade,
				ClassName:     nilIfEmptyPointer(item.ClassName),
				Cohort:        nilIfEmptyPointer(item.Cohort),
				Counselor:     nilIfEmptyPointer(item.Counselor),
				Mentor:        nilIfEmptyPointer(item.Mentor),
				Major:         nilIfEmptyPointer(item.Major),
				TrainingMode:  nilIfEmptyPointer(item.TrainingMode),
				Remark:        nilIfEmptyPointer(item.Remark),
				Status:        common.AlumniStatusActive,
				Version:       1,
				CreatedBy:     &operatorID,
				UpdatedBy:     &operatorID,
			})
		}
		return tx.Create(&enrollments).Error
	}); err != nil {
		return nil, err
	}

	return r.GetDetail(ctx, profileID, nil)
}

// BatchCreate 批量新增校友（每份输入生成一份个人档案与至少一段学籍）。
func (r *AlumniRepository) BatchCreate(ctx context.Context, inputs []do.AlumniCreateInput, operatorID uint64) ([]uint64, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	if len(inputs) == 0 {
		return nil, nil
	}

	domainIDs := make([]uint64, 0)
	for _, input := range inputs {
		for _, enrollment := range input.Enrollments {
			if enrollment.DataDomainID == 0 {
				return nil, common.ErrInvalidDataDomain
			}
			domainIDs = append(domainIDs, enrollment.DataDomainID)
		}
	}
	if err := r.validateActiveDataDomainIDs(ctx, domainIDs); err != nil {
		return nil, err
	}

	createdIDs := make([]uint64, 0, len(inputs))
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, input := range inputs {
			input = input.Normalize()
			profile := &model.AlumniProfile{
				Name:           input.Personal.Name,
				Industry:       input.Personal.Industry,
				WorkUnit:       input.Personal.WorkUnit,
				Position:       input.Personal.Position,
				MailingAddress: input.Personal.MailingAddress,
				Gender:         input.Personal.Gender,
				Mobile:         input.Personal.Mobile,
				Email:          input.Personal.Email,
				Status:         input.Status,
				Version:        1,
				CreatedBy:      &operatorID,
				UpdatedBy:      &operatorID,
			}
			if err := tx.Create(profile).Error; err != nil {
				return err
			}
			enrollments := make([]*model.AlumniEnrollment, 0, len(input.Enrollments))
			for _, item := range input.Enrollments {
				enrollments = append(enrollments, &model.AlumniEnrollment{
					AlumniID:      profile.ID,
					DataDomainID:  item.DataDomainID,
					TrainingLevel: item.TrainingLevel,
					Grade:         item.Grade,
					ClassName:     nilIfEmptyPointer(item.ClassName),
					Cohort:        nilIfEmptyPointer(item.Cohort),
					Counselor:     nilIfEmptyPointer(item.Counselor),
					Mentor:        nilIfEmptyPointer(item.Mentor),
					Major:         nilIfEmptyPointer(item.Major),
					TrainingMode:  nilIfEmptyPointer(item.TrainingMode),
					Remark:        nilIfEmptyPointer(item.Remark),
					Status:        common.AlumniStatusActive,
					Version:       1,
					CreatedBy:     &operatorID,
					UpdatedBy:     &operatorID,
				})
			}
			if err := tx.Create(&enrollments).Error; err != nil {
				return err
			}
			createdIDs = append(createdIDs, profile.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return createdIDs, nil
}

// Save 在一个事务内保存个人共享字段、提交的已有学籍、本次新增学籍及文件变更。
// 任一校验或写入失败都会整体回滚，不会产生部分更新。
func (r *AlumniRepository) Save(ctx context.Context, id uint64, input do.AlumniSaveInput, operatorID uint64, dataDomainIDs []uint64) (*do.AlumniSaveResult, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	input = input.Normalize()
	if input.Personal.Name == "" {
		return nil, common.ErrInvalidRequest
	}

	var removedObjectKeys []string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var profile model.AlumniProfile
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ? AND deleted_at IS NULL", id, common.AlumniStatusActive).
			First(&profile).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.ErrAlumniNotFound
		}
		if err != nil {
			return err
		}
		if len(dataDomainIDs) > 0 {
			var accessible int64
			if err := tx.Model(&model.AlumniEnrollment{}).
				Where("alumni_id = ? AND status = ? AND deleted_at IS NULL AND data_domain_id IN ("+placeholders(len(dataDomainIDs))+")", append([]any{id, common.AlumniStatusActive}, uint64Args(dataDomainIDs)...)...).
				Count(&accessible).Error; err != nil {
				return err
			}
			if accessible == 0 {
				return common.ErrAlumniNotFound
			}
		}
		if input.Version == 0 || input.Version != profile.Version {
			return common.ErrVersionConflict
		}

		updates := map[string]any{
			"name":       input.Personal.Name,
			"updated_by": operatorID,
			"version":    gorm.Expr("version + 1"),
			"updated_at": time.Now(),
		}
		if input.Personal.Gender != nil {
			updates["gender"] = nullableString(*input.Personal.Gender)
		}
		if input.Personal.Mobile != nil {
			updates["mobile"] = nullableString(*input.Personal.Mobile)
		}
		if input.Personal.Email != nil {
			updates["email"] = nullableString(*input.Personal.Email)
		}
		if input.Personal.Industry != nil {
			updates["industry"] = nullableString(*input.Personal.Industry)
		}
		if input.Personal.WorkUnit != nil {
			updates["work_unit"] = nullableString(*input.Personal.WorkUnit)
		}
		if input.Personal.Position != nil {
			updates["position"] = nullableString(*input.Personal.Position)
		}
		if input.Personal.MailingAddress != nil {
			updates["mailing_address"] = nullableString(*input.Personal.MailingAddress)
		}
		result := tx.Model(&model.AlumniProfile{}).
			Where("id = ? AND version = ? AND status = ?", id, input.Version, common.AlumniStatusActive).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return common.ErrVersionConflict
		}

		for _, enrollmentInput := range input.Enrollments {
			if enrollmentInput.ID == 0 {
				if err := r.createEnrollment(tx, id, enrollmentInput, operatorID); err != nil {
					return err
				}
				continue
			}
			keys, err := r.updateEnrollment(tx, id, enrollmentInput, operatorID, dataDomainIDs)
			if err != nil {
				return err
			}
			removedObjectKeys = append(removedObjectKeys, keys...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	record, err := r.GetDetail(ctx, id, dataDomainIDs)
	if err != nil {
		return nil, err
	}
	return &do.AlumniSaveResult{Record: *record, RemovedObjectKeys: removedObjectKeys}, nil
}

func (r *AlumniRepository) createEnrollment(tx *gorm.DB, alumniID uint64, input do.AlumniEnrollmentInput, operatorID uint64) error {
	enrollment := &model.AlumniEnrollment{
		AlumniID:      alumniID,
		DataDomainID:  input.DataDomainID,
		TrainingLevel: input.TrainingLevel,
		Grade:         input.Grade,
		ClassName:     input.ClassName,
		Cohort:        input.Cohort,
		Counselor:     input.Counselor,
		Mentor:        input.Mentor,
		Major:         input.Major,
		TrainingMode:  input.TrainingMode,
		Remark:        input.Remark,
		Status:        common.AlumniStatusActive,
		Version:       1,
		CreatedBy:     &operatorID,
		UpdatedBy:     &operatorID,
	}
	if err := tx.Create(enrollment).Error; err != nil {
		return err
	}
	_, err := r.applyFileChanges(tx, enrollment.ID, input, nil, operatorID)
	return err
}

func (r *AlumniRepository) updateEnrollment(tx *gorm.DB, alumniID uint64, input do.AlumniEnrollmentInput, operatorID uint64, dataDomainIDs []uint64) ([]string, error) {
	var enrollment model.AlumniEnrollment
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND alumni_id = ? AND status = ? AND deleted_at IS NULL", input.ID, alumniID, common.AlumniStatusActive).
		First(&enrollment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrEnrollmentNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(dataDomainIDs) > 0 && !containsUint64(dataDomainIDs, enrollment.DataDomainID) {
		// 不区分“无权”和“不存在”，避免通过学籍 ID 探测隐藏身份。
		return nil, common.ErrEnrollmentNotFound
	}
	if input.DataDomainID != enrollment.DataDomainID {
		return nil, common.ErrEnrollmentDomainFixed
	}
	if input.Version == 0 || input.Version != enrollment.Version {
		return nil, common.ErrVersionConflict
	}

	updates := map[string]any{
		"training_level": input.TrainingLevel,
		"grade":          input.Grade,
		"updated_by":     operatorID,
		"version":        gorm.Expr("version + 1"),
		"updated_at":     time.Now(),
	}
	if input.ClassName != nil {
		updates["class_name"] = nullableString(*input.ClassName)
	}
	if input.Cohort != nil {
		updates["cohort"] = nullableString(*input.Cohort)
	}
	if input.Counselor != nil {
		updates["counselor"] = nullableString(*input.Counselor)
	}
	if input.Mentor != nil {
		updates["mentor"] = nullableString(*input.Mentor)
	}
	if input.Major != nil {
		updates["major"] = nullableString(*input.Major)
	}
	if input.TrainingMode != nil {
		updates["training_mode"] = nullableString(*input.TrainingMode)
	}
	if input.Remark != nil {
		updates["remark"] = nullableString(*input.Remark)
	}

	result := tx.Model(&model.AlumniEnrollment{}).
		Where("id = ? AND version = ? AND status = ?", input.ID, input.Version, common.AlumniStatusActive).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, common.ErrVersionConflict
	}
	return r.applyFileChanges(tx, enrollment.ID, input, &enrollment, operatorID)
}

// applyFileChanges 在同一事务内确认草稿文件与移除文件。
// 激活前必须是 draft（已直传完成但尚未保存）；移除只改元数据，MinIO 对象在提交后清理。
func (r *AlumniRepository) applyFileChanges(tx *gorm.DB, enrollmentID uint64, input do.AlumniEnrollmentInput, current *model.AlumniEnrollment, operatorID uint64) ([]string, error) {
	if len(input.ActivateFileIDs) > 0 {
		var files []model.AlumniFile
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ? AND enrollment_id = ? AND status = ? AND uploaded_by = ? AND deleted_at IS NULL", input.ActivateFileIDs, enrollmentID, common.FileStatusDraft, operatorID).
			Find(&files).Error; err != nil {
			return nil, err
		}
		if len(files) != len(input.ActivateFileIDs) {
			return nil, common.ErrFileNotFound
		}
		if err := tx.Model(&model.AlumniFile{}).
			Where("id IN ?", input.ActivateFileIDs).
			Updates(map[string]any{"status": common.FileStatusActive}).Error; err != nil {
			return nil, err
		}
	}

	if len(input.RemoveFileIDs) == 0 {
		return nil, nil
	}
	var files []model.AlumniFile
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ? AND enrollment_id = ? AND status IN ? AND deleted_at IS NULL", input.RemoveFileIDs, enrollmentID, []string{common.FileStatusActive, common.FileStatusDraft}).
		Find(&files).Error; err != nil {
		return nil, err
	}
	if len(files) != len(input.RemoveFileIDs) {
		return nil, common.ErrFileNotFound
	}
	for _, file := range files {
		// 草稿只允许上传者本人清理；正式附件由任何有权维护该学籍的管理员移除。
		if file.Status == common.FileStatusDraft && (file.UploadedBy == nil || *file.UploadedBy != operatorID) {
			return nil, common.ErrFileNotFound
		}
	}
	now := time.Now()
	if err := tx.Model(&model.AlumniFile{}).
		Where("id IN ?", input.RemoveFileIDs).
		Updates(map[string]any{"status": common.FileStatusDeleted, "deleted_at": now}).Error; err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(files))
	for _, file := range files {
		keys = append(keys, file.ObjectKey)
	}
	return keys, nil
}

func containsUint64(values []uint64, target uint64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// Delete 软删除只有一段学籍的校友；多段校友的整人删除语义尚未定义，直接拒绝。
func (r *AlumniRepository) Delete(ctx context.Context, id uint64, updaterID uint64, dataDomainIDs []uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var enrollments []model.AlumniEnrollment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("alumni_id = ? AND status = ? AND deleted_at IS NULL", id, common.AlumniStatusActive).
			Find(&enrollments).Error; err != nil {
			return err
		}
		if len(enrollments) == 0 {
			return common.ErrAlumniNotFound
		}
		if len(enrollments) > 1 {
			return common.ErrAlumniMultiEnrollment
		}
		if len(dataDomainIDs) > 0 && !containsUint64(dataDomainIDs, enrollments[0].DataDomainID) {
			return common.ErrAlumniNotFound
		}

		now := time.Now()
		result := tx.Model(&model.AlumniProfile{}).
			Where("id = ? AND status = ? AND deleted_at IS NULL", id, common.AlumniStatusActive).
			Updates(map[string]any{
				"status":     common.AlumniStatusDeleted,
				"updated_by": updaterID,
				"deleted_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return common.ErrAlumniNotFound
		}

		if err := tx.Model(&model.AlumniEnrollment{}).
			Where("alumni_id = ? AND status = ? AND deleted_at IS NULL", id, common.AlumniStatusActive).
			Updates(map[string]any{
				"status":     common.AlumniStatusDeleted,
				"updated_by": updaterID,
				"deleted_at": now,
			}).Error; err != nil {
			return err
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

	updates := map[string]any{
		"updated_by": updaterID,
		"version":    gorm.Expr("version + 1"),
	}
	if profile.WorkUnit != nil {
		updates["work_unit"] = nullableString(*profile.WorkUnit)
	}
	if profile.Position != nil {
		updates["position"] = nullableString(*profile.Position)
	}
	if profile.MailingAddress != nil {
		updates["mailing_address"] = nullableString(*profile.MailingAddress)
	}
	if profile.Mobile != nil {
		updates["mobile"] = nullableString(*profile.Mobile)
	}

	result := r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", id, common.AlumniStatusActive).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrAlumniNotFound
	}
	return nil
}

// FindExistingByDedupKey 批量查询已存在的 (姓名, 手机号, 数据域, 年级, 班级, 届数) 组合。
func (r *AlumniRepository) FindExistingByDedupKey(ctx context.Context, keys []do.AlumniDedupKey) (map[string]bool, error) {
	result := make(map[string]bool, len(keys))
	if r.db == nil || len(keys) == 0 {
		return result, nil
	}

	condition := "(p.name = ? AND COALESCE(p.mobile, '') = ? AND e.data_domain_id = ? AND e.grade = ? AND COALESCE(e.class_name, '') = ? AND COALESCE(e.cohort, '') = ?)"
	parts := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)*6)
	for _, key := range keys {
		parts = append(parts, condition)
		args = append(args, key.Name, strings.TrimSpace(key.Mobile), key.DataDomainID, key.Grade, key.ClassName, key.Cohort)
	}

	type dedupRow struct {
		Name         string
		Mobile       string
		DataDomainID uint64
		Grade        string
		ClassName    string
		Cohort       string
	}
	var rows []dedupRow
	err := r.db.WithContext(ctx).
		Table("alumni_profiles AS p").
		Select("p.name, COALESCE(p.mobile, '') AS mobile, e.data_domain_id, e.grade, COALESCE(e.class_name, '') AS class_name, COALESCE(e.cohort, '') AS cohort").
		Joins("JOIN alumni_enrollments AS e ON e.alumni_id = p.id AND e.status = ? AND e.deleted_at IS NULL", common.AlumniStatusActive).
		Where("p.status = ? AND p.deleted_at IS NULL", common.AlumniStatusActive).
		Where("("+strings.Join(parts, " OR ")+")", args...).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[do.AlumniDedupKey{
			Name:         row.Name,
			Mobile:       row.Mobile,
			DataDomainID: row.DataDomainID,
			Grade:        row.Grade,
			ClassName:    row.ClassName,
			Cohort:       row.Cohort,
		}.Key()] = true
	}
	return result, nil
}

// validateActiveDataDomainIDs 校验目标数据域存在且处于可用状态。
func (r *AlumniRepository) validateActiveDataDomainIDs(ctx context.Context, ids []uint64) error {
	uniqueIDs := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			return common.ErrInvalidDataDomain
		}
		uniqueIDs[id] = struct{}{}
	}
	if len(uniqueIDs) == 0 {
		return common.ErrInvalidDataDomain
	}

	uniqueIDList := make([]uint64, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		uniqueIDList = append(uniqueIDList, id)
	}
	qs := query.Use(r.db).DataDomain
	var domains []model.DataDomain
	if err := r.db.WithContext(ctx).
		Where(qs.ID.In(uniqueIDList...), qs.Status.Eq(common.DataDomainStatusActive)).
		Find(&domains).
		Error; err != nil {
		return err
	}
	if len(domains) != len(uniqueIDList) {
		return common.ErrInvalidDataDomain
	}
	return nil
}

// mapDataDomainLookupError 将不可用的数据域转换为明确的系统配置错误。
func mapDataDomainLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.ErrDataDomainUnavailable
	}
	return err
}

// FindByMobile 通过手机号查找校友个人档案
func (r *AlumniRepository) FindByMobile(ctx context.Context, mobile string) (*model.AlumniProfile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var profile model.AlumniProfile
	err := activeAlumniQuery(r.db.WithContext(ctx)).
		Where("alumni_profiles.mobile = ?", mobile).
		First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrAlumniNotFound
	}
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// FindByEmail 通过邮箱查找校友个人档案（大小写不敏感）
func (r *AlumniRepository) FindByEmail(ctx context.Context, email string) (*model.AlumniProfile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var profile model.AlumniProfile
	err := activeAlumniQuery(r.db.WithContext(ctx)).
		Where("LOWER(alumni_profiles.email) = ?", strings.ToLower(email)).
		First(&profile).Error
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
	return r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where("id = ?", id).
		Update("mobile", mobile).Error
}

// UpdateEmail 更新校友邮箱（小写）
func (r *AlumniRepository) UpdateEmail(ctx context.Context, id uint64, email string) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}
	return r.db.WithContext(ctx).
		Model(&model.AlumniProfile{}).
		Where("id = ?", id).
		Update("email", strings.ToLower(email)).Error
}

func nilIfEmptyPointer(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	return value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
