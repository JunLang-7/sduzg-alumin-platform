package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/logger"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/storage"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// 允许上传的文件类型
var allowedMimeTypes = map[string]string{
	"application/pdf":    ".pdf",
	"application/msword": ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
	"image/jpeg": ".jpg",
	"image/png":  ".png",
}

const maxFileSize int64 = 50 * 1024 * 1024 // 50MB

// AlumniFileService 校友档案文件业务逻辑。
type AlumniFileService struct {
	files    repository.AlumniFileStore
	alumni   repository.AlumniStore
	store    *storage.Client
	opLogger *OperationLogger
}

// NewAlumniFileService 创建文件服务实例。
func NewAlumniFileService(
	files repository.AlumniFileStore,
	alumni repository.AlumniStore,
	store *storage.Client,
	opLogger *OperationLogger,
) *AlumniFileService {
	return &AlumniFileService{
		files:    files,
		alumni:   alumni,
		store:    store,
		opLogger: opLogger,
	}
}

const presignedURLExpiry = 15 * time.Minute

// GenerateUploadURL 生成预签名上传 URL：校验 → 创建 pending 记录 → 返回 Presigned PUT URL。
// 客户端拿到 URL 后直传 MinIO，完成后调用 ConfirmUpload 确认。
func (s *AlumniFileService) GenerateUploadURL(ctx context.Context, operatorID uint64, alumniID uint64, fileType, originalName, mimeType string) (*dto.AlumniFileUploadURLResponse, error) {
	// 1. 校验 file_type
	if fileType != common.FileTypeDegreeArchive && fileType != common.FileTypeAcademicRecord {
		return nil, common.ErrFileTypeNotAllowed
	}

	// 2. 校验 MIME 类型
	if _, allowed := allowedMimeTypes[mimeType]; !allowed {
		return nil, common.ErrFileTypeNotAllowed
	}

	// 3. 校验校友存在
	alumni, err := s.alumni.GetByID(ctx, alumniID)
	if err != nil {
		return nil, err
	}

	// 4. 构建 MinIO 对象路径
	objectKey := buildObjectKey(alumniID, alumni.Name, fileType, originalName)

	// 5. 生成预签名 PUT URL
	uploadURL, err := s.store.PresignedPutURL(ctx, objectKey, presignedURLExpiry)
	if err != nil {
		logger.Error("failed to generate presigned put url",
			zap.Uint64("alumni_id", alumniID),
			zap.String("file_type", fileType),
			zap.Error(err),
		)
		return nil, common.ErrStorageUnavailable
	}

	// 6. 写入 DB 元数据（pending 状态）
	record := &model.AlumniFile{
		AlumniID:     alumniID,
		FileType:     fileType,
		ObjectKey:    objectKey,
		OriginalName: originalName,
		FileSize:     0,
		MimeType:     mimeType,
		UploadedBy:   &operatorID,
		Status:       common.FileStatusPending,
	}

	saved, err := s.files.Create(ctx, record)
	if err != nil {
		logger.Error("failed to save pending file record", zap.Error(err))
		return nil, err
	}

	s.writeOpLog(ctx, operatorID, "request_upload_url", saved.ID, alumni, fileType, originalName)

	return &dto.AlumniFileUploadURLResponse{
		FileID:    saved.ID,
		UploadURL: uploadURL,
		ExpiresIn: int(presignedURLExpiry.Seconds()),
		ObjectKey: objectKey,
	}, nil
}

// ConfirmUpload 确认直传完成：验证 MinIO 对象存在 → 替换旧文件 → 标记 active → 记录日志。
func (s *AlumniFileService) ConfirmUpload(ctx context.Context, operatorID uint64, alumniID uint64, fileID uint64) (*dto.AlumniFileUploadResponse, error) {
	// 1. 查找 pending 记录
	record, err := s.files.GetByIDAnyStatus(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if record.AlumniID != alumniID {
		return nil, common.ErrFileNotFound
	}
	if record.Status != common.FileStatusPending {
		return nil, common.ErrFileNotFound
	}

	// 2. 验证 MinIO 对象确实存在
	stat, err := s.store.StatObject(ctx, record.ObjectKey)
	if err != nil || stat.Size == 0 {
		logger.Error("upload confirm failed: object not in minio",
			zap.Uint64("file_id", fileID),
			zap.String("object_key", record.ObjectKey),
			zap.Error(err),
		)
		return nil, common.ErrFileNotFound
	}

	// 2b. 校验实际文件大小
	if stat.Size > maxFileSize {
		// 删除超限的 MinIO 对象
		if delErr := s.store.DeleteFile(ctx, record.ObjectKey); delErr != nil {
			logger.Warn("failed to delete oversized minio object", zap.Error(delErr))
		}
		return nil, common.ErrFileTooLarge
	}

	// 3. 替换旧文件：软删除同类型的旧 active 记录
	if err := s.replaceOldFiles(ctx, alumniID, record.FileType); err != nil {
		logger.Error("failed to replace old files on confirm", zap.Error(err))
	}

	// 4. 标记 active、更新文件大小
	if err := s.files.MarkActive(ctx, fileID, uint64(stat.Size)); err != nil {
		logger.Error("failed to mark file active", zap.Uint64("file_id", fileID), zap.Error(err))
		return nil, err
	}

	// 5. 记录操作日志
	alumni, _ := s.alumni.GetByID(ctx, alumniID)
	s.writeOpLog(ctx, operatorID, "confirm_upload", fileID, alumni, record.FileType, record.OriginalName)

	return &dto.AlumniFileUploadResponse{
		ID:           record.ID,
		AlumniID:     record.AlumniID,
		FileType:     record.FileType,
		OriginalName: record.OriginalName,
		FileSize:     uint64(stat.Size),
		MimeType:     record.MimeType,
	}, nil
}

// GenerateDownloadURL 生成预签名下载 URL：校验权限 → 返回 Presigned GET URL。
// 客户端凭该 URL 直连 MinIO 下载，数据不经过 API Server。
func (s *AlumniFileService) GenerateDownloadURL(ctx context.Context, alumniID uint64, fileID uint64) (*dto.AlumniFileDownloadURLResponse, error) {
	record, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}

	if record.AlumniID != alumniID {
		return nil, common.ErrFileNotFound
	}

	downloadURL, err := s.store.PresignedGetURL(ctx, record.ObjectKey, presignedURLExpiry)
	if err != nil {
		logger.Error("failed to generate presigned get url",
			zap.Uint64("file_id", fileID),
			zap.String("object_key", record.ObjectKey),
			zap.Error(err),
		)
		return nil, common.ErrStorageUnavailable
	}

	return &dto.AlumniFileDownloadURLResponse{
		DownloadURL:  downloadURL,
		ExpiresIn:    int(presignedURLExpiry.Seconds()),
		OriginalName: record.OriginalName,
	}, nil
}

// ListFiles 获取校友的文件列表，按 file_type 分组。
func (s *AlumniFileService) ListFiles(ctx context.Context, alumniID uint64) (*dto.AlumniFileListResponse, error) {
	records, err := s.files.ListByAlumniID(ctx, alumniID)
	if err != nil {
		return nil, err
	}

	result := &dto.AlumniFileListResponse{
		AlumniID:       alumniID,
		DegreeArchive:  make([]dto.AlumniFileItem, 0),
		AcademicRecord: make([]dto.AlumniFileItem, 0),
	}

	for _, rec := range records {
		item := dto.AlumniFileItem{
			ID:           rec.ID,
			FileType:     rec.FileType,
			OriginalName: rec.OriginalName,
			FileSize:     rec.FileSize,
			MimeType:     rec.MimeType,
			CreatedAt:    rec.CreatedAt,
		}

		switch rec.FileType {
		case common.FileTypeDegreeArchive:
			result.DegreeArchive = append(result.DegreeArchive, item)
		case common.FileTypeAcademicRecord:
			result.AcademicRecord = append(result.AcademicRecord, item)
		}
	}

	return result, nil
}

// DeleteFile 软删除文件记录 + 尽力清理 MinIO 对象 + 记录日志。
func (s *AlumniFileService) DeleteFile(ctx context.Context, operatorID uint64, alumniID uint64, fileID uint64) error {
	record, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return err
	}

	if record.AlumniID != alumniID {
		return common.ErrFileNotFound
	}

	if err := s.files.SoftDelete(ctx, fileID); err != nil {
		return err
	}

	// 尽力清理 MinIO 对象，失败不影响主流程
	if err := s.store.DeleteFile(ctx, record.ObjectKey); err != nil {
		logger.Warn("failed to delete file from minio",
			zap.Uint64("file_id", fileID),
			zap.String("object_key", record.ObjectKey),
			zap.Error(err),
		)
	}

	// 记录操作日志
	alumni, _ := s.alumni.GetByID(ctx, alumniID)
	s.writeOpLog(ctx, operatorID, "delete_alumni_file", fileID, alumni, record.FileType, record.OriginalName)

	return nil
}

// CascadeSoftDelete 校友删除时级联软删除所有文件记录。
// MinIO 对象清理为 best-effort。
func (s *AlumniFileService) CascadeSoftDelete(ctx context.Context, alumniID uint64) error {
	records, err := s.files.ListByAlumniID(ctx, alumniID)
	if err != nil {
		return err
	}

	if err := s.files.SoftDeleteByAlumniID(ctx, alumniID); err != nil {
		return err
	}

	// 异步清理 MinIO 对象（best-effort）
	for _, rec := range records {
		if err := s.store.DeleteFile(ctx, rec.ObjectKey); err != nil {
			logger.Warn("failed to cleanup minio object during cascade delete",
				zap.Uint64("file_id", rec.ID),
				zap.String("object_key", rec.ObjectKey),
				zap.Error(err),
			)
		}
	}

	return nil
}

// replaceOldFiles 软删除同 alumni_id + file_type 的活跃文件，并清理 MinIO 对象。
func (s *AlumniFileService) replaceOldFiles(ctx context.Context, alumniID uint64, fileType string) error {
	oldRecords, err := s.files.ListByAlumniID(ctx, alumniID)
	if err != nil {
		return err
	}

	var oldObjectKeys []string
	for _, rec := range oldRecords {
		if rec.FileType == fileType {
			oldObjectKeys = append(oldObjectKeys, rec.ObjectKey)
		}
	}

	if err := s.files.SoftDeleteByAlumniIDAndType(ctx, alumniID, fileType); err != nil {
		return err
	}

	for _, key := range oldObjectKeys {
		if err := s.store.DeleteFile(ctx, key); err != nil {
			logger.Warn("failed to delete old minio object during replacement",
				zap.String("object_key", key),
				zap.Error(err),
			)
		}
	}

	return nil
}

// writeOpLog 记录文件操作审计日志。
func (s *AlumniFileService) writeOpLog(ctx context.Context, operatorID uint64, action string, fileID uint64, alumni *model.AlumniProfile, fileType, originalName string) {
	if s.opLogger == nil {
		return
	}

	detail := map[string]any{
		"alumni_id":     alumni.ID,
		"alumni_name":   alumni.Name,
		"file_type":     fileType,
		"original_name": originalName,
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		logger.Warn("failed to marshal op log detail", zap.Error(err))
		return
	}

	detailStr := string(detailJSON)
	log := model.OperationLog{
		OperatorID: operatorID,
		Action:     action,
		TargetType: "alumni_file",
		TargetID:   &fileID,
		Detail:     &detailStr,
	}
	if err := s.opLogger.Write(ctx, &log); err != nil {
		logger.Warn("failed to write operation log", zap.String("action", action), zap.Error(err))
	}
}

// buildObjectKey 构建 MinIO 对象路径：alumni/{id}_{name}/{type}/{uuid}_{filename}
// 路径中包含校友名和原始文件名，在 MinIO Console 中可直接辨识，无需依赖 metadata。
// alumniName 和 originalName 中的不安全字符会被替换为下划线，防止路径穿越。
func buildObjectKey(alumniID uint64, alumniName string, fileType string, originalName string) string {
	safeName := sanitizePathSegment(alumniName)
	safeFile := sanitizePathSegment(originalName)
	return fmt.Sprintf("alumni/%d_%s/%s/%s_%s", alumniID, safeName, fileType, uuid.NewString(), safeFile)
}

// sanitizePathSegment 移除可能用于路径穿越的字符（/、\、..），替换为下划线。
func sanitizePathSegment(s string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "_",
	)
	return replacer.Replace(s)
}
