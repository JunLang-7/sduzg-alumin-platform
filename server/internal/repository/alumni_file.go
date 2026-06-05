package repository

import (
	"context"
	"errors"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"gorm.io/gorm"
)

// AlumniFileStore 校友档案文件仓库接口。
type AlumniFileStore interface {
	Create(ctx context.Context, record *model.AlumniFile) (*model.AlumniFile, error)
	ListByAlumniID(ctx context.Context, alumniID uint64) ([]*model.AlumniFile, error)
	GetByID(ctx context.Context, id uint64) (*model.AlumniFile, error)
	SoftDelete(ctx context.Context, id uint64) error
	SoftDeleteByAlumniIDAndType(ctx context.Context, alumniID uint64, fileType string) error
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

// ListByAlumniID 查询某校友的所有活跃文件。
func (r *AlumniFileRepository) ListByAlumniID(ctx context.Context, alumniID uint64) ([]*model.AlumniFile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var items []*model.AlumniFile
	if err := r.db.WithContext(ctx).
		Where("alumni_id = ?", alumniID).
		Where("status = ?", common.FileStatusActive).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 根据 ID 获取单条文件记录（仅活跃记录）。
func (r *AlumniFileRepository) GetByID(ctx context.Context, id uint64) (*model.AlumniFile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	var item model.AlumniFile
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		Where("status = ?", common.FileStatusActive).
		Where("deleted_at IS NULL").
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// SoftDelete 软删除单条文件记录。
func (r *AlumniFileRepository) SoftDelete(ctx context.Context, id uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	result := r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", id, common.FileStatusActive).
		Updates(map[string]any{
			"status":     common.FileStatusDeleted,
			"deleted_at": gorm.Expr("NOW()"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrFileNotFound
	}
	return nil
}

// SoftDeleteByAlumniIDAndType 软删除指定校友的某类型所有活跃文件（文件替换策略使用）。
func (r *AlumniFileRepository) SoftDeleteByAlumniIDAndType(ctx context.Context, alumniID uint64, fileType string) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	return r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where("alumni_id = ? AND file_type = ? AND status = ? AND deleted_at IS NULL",
			alumniID, fileType, common.FileStatusActive).
		Updates(map[string]any{
			"status":     common.FileStatusDeleted,
			"deleted_at": gorm.Expr("NOW()"),
		}).Error
}

// SoftDeleteByAlumniID 级联软删除某校友的所有文件记录（校友删除时调用）。
func (r *AlumniFileRepository) SoftDeleteByAlumniID(ctx context.Context, alumniID uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	return r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where("alumni_id = ? AND status = ? AND deleted_at IS NULL",
			alumniID, common.FileStatusActive).
		Updates(map[string]any{
			"status":     common.FileStatusDeleted,
			"deleted_at": gorm.Expr("NOW()"),
		}).Error
}
