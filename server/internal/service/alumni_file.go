package service

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"

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

// Upload 上传文件：校验 → 替换旧文件 → 上传 MinIO → 写入 DB 元数据 → 记录日志。
func (s *AlumniFileService) Upload(ctx context.Context, operatorID uint64, alumniID uint64, fileType string, fileHeader *multipart.FileHeader) (*dto.AlumniFileUploadResponse, error) {
	// 1. 校验文件类型
	mimeType := fileHeader.Header.Get("Content-Type")
	if _, allowed := allowedMimeTypes[mimeType]; !allowed {
		return nil, common.ErrFileTypeNotAllowed
	}

	// 2. 校验文件大小
	if fileHeader.Size > maxFileSize {
		return nil, common.ErrFileTooLarge
	}

	// 3. 校验校友存在
	alumni, err := s.alumni.GetByID(ctx, alumniID)
	if err != nil {
		return nil, err
	}

	// 4. 构建 MinIO 对象路径（路径中含校友名，Console 中可直接辨识）
	objectKey := buildObjectKey(alumniID, alumni.Name, fileType, fileHeader.Filename)

	// 5. 替换旧文件：先软删除同类型的旧记录
	if err := s.replaceOldFiles(ctx, alumniID, fileType); err != nil {
		logger.Error("failed to replace old files", zap.Error(err))
		// 非致命，继续上传
	}

	// 6. 上传到 MinIO
	uploadInfo, err := s.store.UploadFile(ctx, objectKey, fileHeader, nil)
	if err != nil {
		logger.Error("failed to upload file to minio",
			zap.Uint64("alumni_id", alumniID),
			zap.String("file_type", fileType),
			zap.Error(err),
		)
		return nil, common.ErrStorageUnavailable
	}

	// 7. 写入 DB 元数据
	record := &model.AlumniFile{
		AlumniID:     alumniID,
		FileType:     fileType,
		ObjectKey:    objectKey,
		OriginalName: fileHeader.Filename,
		FileSize:     uint64(uploadInfo.Size),
		MimeType:     mimeType,
		UploadedBy:   &operatorID,
		Status:       common.FileStatusActive,
	}

	saved, err := s.files.Create(ctx, record)
	if err != nil {
		// DB 写入失败，清除已上传的 MinIO 对象
		if delErr := s.store.DeleteFile(ctx, objectKey); delErr != nil {
			logger.Warn("failed to clean up minio object after db error",
				zap.String("object_key", objectKey),
				zap.Error(delErr),
			)
		}
		logger.Error("failed to save file metadata", zap.Error(err))
		return nil, err
	}

	// 8. 记录操作日志
	s.writeOpLog(ctx, operatorID, "upload_alumni_file", saved.ID, alumni, fileType, fileHeader.Filename)

	return &dto.AlumniFileUploadResponse{
		ID:           saved.ID,
		AlumniID:     saved.AlumniID,
		FileType:     saved.FileType,
		OriginalName: saved.OriginalName,
		FileSize:     saved.FileSize,
		MimeType:     saved.MimeType,
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

// Download 获取文件流和文件信息，供 handler 流式输出。
func (s *AlumniFileService) Download(ctx context.Context, id uint64, alumniID uint64) (*storage.FileDownload, error) {
	record, err := s.files.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if record.AlumniID != alumniID {
		return nil, common.ErrFileNotFound
	}

	reader, stat, err := s.store.GetFile(ctx, record.ObjectKey)
	if err != nil {
		logger.Error("failed to get file from minio",
			zap.Uint64("file_id", id),
			zap.String("object_key", record.ObjectKey),
			zap.Error(err),
		)
		return nil, common.ErrStorageUnavailable
	}

	return &storage.FileDownload{
		Reader:       reader,
		ContentType:  stat.ContentType,
		Size:         stat.Size,
		OriginalName: record.OriginalName,
	}, nil
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
func buildObjectKey(alumniID uint64, alumniName string, fileType string, originalName string) string {
	return fmt.Sprintf("alumni/%d_%s/%s/%s_%s", alumniID, alumniName, fileType, uuid.NewString(), originalName)
}
