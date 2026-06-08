package repository

import (
	"context"
	"errors"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gorm"
)

// AlumniFileStore 校友档案文件仓库接口。
type AlumniFileStore interface {
	Create(ctx context.Context, record *model.AlumniFile) (*model.AlumniFile, error)
	ListByAlumniID(ctx context.Context, alumniID uint64) ([]*model.AlumniFile, error)
	GetByID(ctx context.Context, id uint64) (*model.AlumniFile, error)
	GetByIDAnyStatus(ctx context.Context, id uint64) (*model.AlumniFile, error)
	MarkActive(ctx context.Context, id uint64, fileSize uint64) error
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

	qs := query.Use(r.db).AlumniFile
	var items []*model.AlumniFile
	if err := r.db.WithContext(ctx).
		Where(qs.AlumniID.Eq(alumniID), qs.Status.Eq(common.FileStatusActive), qs.DeletedAt.IsNull()).
		Order(qs.CreatedAt.Desc()).
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

	qs := query.Use(r.db).AlumniFile
	var item model.AlumniFile
	err := r.db.WithContext(ctx).
		Where(qs.ID.Eq(id), qs.Status.Eq(common.FileStatusActive), qs.DeletedAt.IsNull()).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetByIDAnyStatus 根据 ID 获取文件记录（不限制 status），用于确认 pending 上传。
func (r *AlumniFileRepository) GetByIDAnyStatus(ctx context.Context, id uint64) (*model.AlumniFile, error) {
	if r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AlumniFile
	var item model.AlumniFile
	err := r.db.WithContext(ctx).
		Where(qs.ID.Eq(id), qs.DeletedAt.IsNull()).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// MarkActive 将文件记录标记为 active 并更新文件大小（确认直传完成后调用）。
func (r *AlumniFileRepository) MarkActive(ctx context.Context, id uint64, fileSize uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AlumniFile
	result := r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where(qs.ID.Eq(id), qs.Status.Eq(common.FileStatusPending), qs.DeletedAt.IsNull()).
		Updates(map[string]any{
			qs.Status.ColumnName().String():   common.FileStatusActive,
			qs.FileSize.ColumnName().String(): fileSize,
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

	qs := query.Use(r.db).AlumniFile
	result := r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where(qs.ID.Eq(id), qs.Status.Eq(common.FileStatusActive), qs.DeletedAt.IsNull()).
		Updates(map[string]any{
			qs.Status.ColumnName().String():    common.FileStatusDeleted,
			qs.DeletedAt.ColumnName().String(): time.Now(),
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

	qs := query.Use(r.db).AlumniFile
	return r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where(qs.AlumniID.Eq(alumniID), qs.FileType.Eq(fileType), qs.Status.Eq(common.FileStatusActive), qs.DeletedAt.IsNull()).
		Updates(map[string]any{
			qs.Status.ColumnName().String():    common.FileStatusDeleted,
			qs.DeletedAt.ColumnName().String(): time.Now(),
		}).Error
}

// SoftDeleteByAlumniID 级联软删除某校友的所有文件记录（校友删除时调用）。
func (r *AlumniFileRepository) SoftDeleteByAlumniID(ctx context.Context, alumniID uint64) error {
	if r.db == nil {
		return common.ErrDatabaseUnavailable
	}

	qs := query.Use(r.db).AlumniFile
	return r.db.WithContext(ctx).
		Model(&model.AlumniFile{}).
		Where(qs.AlumniID.Eq(alumniID), qs.Status.Eq(common.FileStatusActive), qs.DeletedAt.IsNull()).
		Updates(map[string]any{
			qs.Status.ColumnName().String():    common.FileStatusDeleted,
			qs.DeletedAt.ColumnName().String(): time.Now(),
		}).Error
}
