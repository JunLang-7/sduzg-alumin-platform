package repository

import (
	"context"
	"errors"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"gorm.io/gorm"
)

// AlumniFileStore 校友学籍档案文件仓库接口。
type AlumniFileStore interface {
	Create(ctx context.Context, record *model.AlumniFile) (*model.AlumniFile, error)
	ListByEnrollmentID(ctx context.Context, enrollmentID uint64, includeDrafts bool) ([]*model.AlumniFile, error)
	ListByEnrollmentIDs(ctx context.Context, enrollmentIDs []uint64, includeDrafts bool) ([]*model.AlumniFile, error)
	GetByID(ctx context.Context, id uint64) (*model.AlumniFile, error)
	GetByIDAnyStatus(ctx context.Context, id uint64) (*model.AlumniFile, error)
	MarkDraft(ctx context.Context, id uint64, fileSize uint64) error
	SoftDelete(ctx context.Context, id uint64) error
	SoftDeleteByEnrollmentIDAndType(ctx context.Context, enrollmentID uint64, fileType string) error
	SoftDeleteByAlumniID(ctx context.Context, alumniID uint64) error
}

// AlumniFileRepository 文件元数据的 MySQL 存储实现。
type AlumniFileRepository struct {
	db *gorm.DB
}

// NewAlumniFileRepository 创建文件仓库实例。
func NewAlumniFileRepository(db *gorm.DB) *AlumniFileRepository {
	return &AlumniFileRepository{db: db}
}

// Create 写入文件元数据。
func (r *AlumniFileRepository) Create(ctx context.Context, record *model.AlumniFile) (*model.AlumniFile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// ListByEnrollmentID 查询某学籍段的所有文件（默认只含正式文件）。
func (r *AlumniFileRepository) ListByEnrollmentID(ctx context.Context, enrollmentID uint64, includeDrafts bool) ([]*model.AlumniFile, error) {
	return r.ListByEnrollmentIDs(ctx, []uint64{enrollmentID}, includeDrafts)
}

// ListByEnrollmentIDs 批量查询多个学籍段的文件。
func (r *AlumniFileRepository) ListByEnrollmentIDs(ctx context.Context, enrollmentIDs []uint64, includeDrafts bool) ([]*model.AlumniFile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	if len(enrollmentIDs) == 0 {
		return nil, nil
	}

	statuses := []string{common.FileStatusActive}
	if includeDrafts {
		statuses = append(statuses, common.FileStatusDraft, common.FileStatusPending)
	}
	var items []*model.AlumniFile
	if err := r.db.WithContext(ctx).
		Where("enrollment_id IN ? AND status IN ? AND deleted_at IS NULL", enrollmentIDs, statuses).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 根据 ID 获取单条文件记录（仅正式文件）。
func (r *AlumniFileRepository) GetByID(ctx context.Context, id uint64) (*model.AlumniFile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var item model.AlumniFile
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ? AND deleted_at IS NULL", id, common.FileStatusActive).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetByIDAnyStatus 根据 ID 获取文件记录（不限制 status），用于确认上传草稿。
func (r *AlumniFileRepository) GetByIDAnyStatus(ctx context.Context, id uint64) (*model.AlumniFile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var item model.AlumniFile
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// MarkDraft 将直传完成的 pending 文件标记为隔离草稿，保存全部修改时才转正式文件。
func (r *AlumniFileRepository) MarkDraft(ctx context.Context, id uint64, fileSize uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	result := r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", id, common.FileStatusPending).
		Updates(map[string]any{
			"status":    common.FileStatusDraft,
			"file_size": fileSize,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrFileNotFound
	}
	return nil
}

// SoftDelete 软删除单条文件记录。
func (r *AlumniFileRepository) SoftDelete(ctx context.Context, id uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	result := r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where("id = ? AND status IN ? AND deleted_at IS NULL", id, []string{common.FileStatusActive, common.FileStatusDraft}).
		Updates(map[string]any{
			"status":     common.FileStatusDeleted,
			"deleted_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrFileNotFound
	}
	return nil
}

// SoftDeleteByEnrollmentIDAndType 软删除指定学籍某类型的所有活跃文件。
func (r *AlumniFileRepository) SoftDeleteByEnrollmentIDAndType(ctx context.Context, enrollmentID uint64, fileType string) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	return r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where("enrollment_id = ? AND file_type = ? AND status IN ? AND deleted_at IS NULL", enrollmentID, fileType, []string{common.FileStatusActive, common.FileStatusDraft}).
		Updates(map[string]any{
			"status":     common.FileStatusDeleted,
			"deleted_at": time.Now(),
		}).Error
}

// SoftDeleteByAlumniID 级联软删除某校友所有学籍段的文件记录（整人删除时调用）。
func (r *AlumniFileRepository) SoftDeleteByAlumniID(ctx context.Context, alumniID uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	subQuery := r.db.WithContext(ctx).
		Model(&model.AlumniEnrollment{}).
		Select("id").
		Where("alumni_id = ?", alumniID)
	return r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where("enrollment_id IN (?) AND status IN ? AND deleted_at IS NULL", subQuery, []string{common.FileStatusActive, common.FileStatusDraft, common.FileStatusPending}).
		Updates(map[string]any{
			"status":     common.FileStatusDeleted,
			"deleted_at": time.Now(),
		}).Error
}
