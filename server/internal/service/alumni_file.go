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

// AlumniFileService 校友学籍档案文件业务逻辑。
type AlumniFileService struct {
	files    repository.AlumniFileStore
	alumni   repository.AlumniStore
	store    *storage.Client
	opLogger OperationLogWriter
}

// NewAlumniFileService 创建文件服务实例。
func NewAlumniFileService(
	files repository.AlumniFileStore,
	alumni repository.AlumniStore,
	store *storage.Client,
	opLogger OperationLogWriter,
) *AlumniFileService {
	return &AlumniFileService{
		files:    files,
		alumni:   alumni,
		store:    store,
		opLogger: opLogger,
	}
}

const presignedURLExpiry = 15 * time.Minute

// GenerateUploadURL 为指定学籍段生成预签名上传 URL：校验 → 创建 pending 记录 → 返回 Presigned PUT URL。
// 客户端拿到 URL 后直传 MinIO，完成后调用 ConfirmUpload；保存全部修改前文件保持隔离草稿状态。
func (s *AlumniFileService) GenerateUploadURL(ctx context.Context, operator common.AccessContext, alumniID uint64, enrollmentID uint64, fileType, originalName, mimeType string) (*dto.AlumniFileUploadURLResponse, error) {
	if fileType != common.FileTypeDegreeArchive && fileType != common.FileTypeAcademicRecord {
		return nil, common.ErrFileTypeNotAllowed
	}
	if _, allowed := allowedMimeTypes[mimeType]; !allowed {
		return nil, common.ErrFileTypeNotAllowed
	}

	enrollment, err := s.getAccessibleEnrollment(ctx, operator, alumniID, enrollmentID)
	if err != nil {
		return nil, err
	}
	alumni, err := s.alumni.GetPersonalProfile(ctx, alumniID)
	if err != nil {
		return nil, err
	}

	objectKey := buildObjectKey(alumniID, alumni.Name, enrollmentID, fileType, originalName)
	uploadURL, err := s.store.PresignedPutURL(ctx, objectKey, presignedURLExpiry)
	if err != nil {
		logger.Error("failed to generate presigned put url",
			zap.Uint64("alumni_id", alumniID),
			zap.Uint64("enrollment_id", enrollmentID),
			zap.String("file_type", fileType),
			zap.Error(err),
		)
		return nil, common.ErrStorageUnavailable
	}

	record := &model.AlumniFile{
		EnrollmentID: &enrollmentID,
		FileType:     fileType,
		ObjectKey:    objectKey,
		OriginalName: originalName,
		FileSize:     0,
		MimeType:     mimeType,
		UploadedBy:   &operator.UserID,
		Status:       common.FileStatusPending,
	}
	saved, err := s.files.Create(ctx, record)
	if err != nil {
		logger.Error("failed to save pending file record", zap.Error(err))
		return nil, err
	}

	s.writeOpLog(ctx, operator, "request_upload_url", saved.ID, alumni, enrollment, fileType, originalName)

	return &dto.AlumniFileUploadURLResponse{
		FileID:       saved.ID,
		EnrollmentID: enrollmentID,
		UploadURL:    uploadURL,
		ExpiresIn:    int(presignedURLExpiry.Seconds()),
		ObjectKey:    objectKey,
	}, nil
}

// ConfirmUpload 确认直传完成：验证 MinIO 对象存在 → 标记为隔离草稿。
// 正式附件在「保存全部修改」时才生效，取消编辑不会影响已有附件。
func (s *AlumniFileService) ConfirmUpload(ctx context.Context, operator common.AccessContext, alumniID uint64, enrollmentID uint64, fileID uint64) (*dto.AlumniFileUploadResponse, error) {
	enrollment, err := s.getAccessibleEnrollment(ctx, operator, alumniID, enrollmentID)
	if err != nil {
		return nil, err
	}

	record, err := s.files.GetByIDAnyStatus(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if record.EnrollmentID == nil || *record.EnrollmentID != enrollmentID {
		return nil, common.ErrFileNotFound
	}
	if record.Status != common.FileStatusPending {
		return nil, common.ErrFileNotFound
	}
	if record.UploadedBy == nil || *record.UploadedBy != operator.UserID {
		return nil, common.ErrFileNotFound
	}

	stat, err := s.store.StatObject(ctx, record.ObjectKey)
	if err != nil || stat.Size == 0 {
		logger.Error("upload confirm failed: object not in minio",
			zap.Uint64("file_id", fileID),
			zap.String("object_key", record.ObjectKey),
			zap.Error(err),
		)
		return nil, common.ErrFileNotFound
	}
	if stat.Size > maxFileSize {
		if delErr := s.store.DeleteFile(ctx, record.ObjectKey); delErr != nil {
			logger.Warn("failed to delete oversized minio object", zap.Error(delErr))
		}
		return nil, common.ErrFileTooLarge
	}

	if err := s.files.MarkDraft(ctx, fileID, uint64(stat.Size)); err != nil {
		logger.Error("failed to mark file draft", zap.Uint64("file_id", fileID), zap.Error(err))
		return nil, err
	}

	alumni, err := s.alumni.GetPersonalProfile(ctx, alumniID)
	if err != nil {
		return nil, err
	}
	s.writeOpLog(ctx, operator, "confirm_upload", fileID, alumni, enrollment, record.FileType, record.OriginalName)

	return &dto.AlumniFileUploadResponse{
		ID:           record.ID,
		AlumniID:     alumniID,
		EnrollmentID: enrollmentID,
		FileType:     record.FileType,
		OriginalName: record.OriginalName,
		FileSize:     uint64(stat.Size),
		MimeType:     record.MimeType,
		Status:       common.FileStatusDraft,
	}, nil
}

// GenerateDownloadURL 生成预签名下载 URL：校验权限与学籍归属 → 返回 Presigned GET URL。
func (s *AlumniFileService) GenerateDownloadURL(ctx context.Context, operator common.AccessContext, alumniID uint64, enrollmentID uint64, fileID uint64) (*dto.AlumniFileDownloadURLResponse, error) {
	enrollment, err := s.getAccessibleEnrollment(ctx, operator, alumniID, enrollmentID)
	if err != nil {
		return nil, err
	}

	record, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if record.EnrollmentID == nil || *record.EnrollmentID != enrollmentID {
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
	alumni, err := s.alumni.GetPersonalProfile(ctx, alumniID)
	if err != nil {
		return nil, err
	}
	s.writeOpLog(ctx, operator, "download_alumni_file", fileID, alumni, enrollment, record.FileType, record.OriginalName)

	return &dto.AlumniFileDownloadURLResponse{
		DownloadURL:  downloadURL,
		ExpiresIn:    int(presignedURLExpiry.Seconds()),
		OriginalName: record.OriginalName,
	}, nil
}

// ListFiles 获取校友各段可访问学籍的文件，按学籍段与文件类型分组。
// includeDrafts 仅用于管理端编辑弹窗，查看页不展示未保存的草稿。
func (s *AlumniFileService) ListFiles(ctx context.Context, operator common.AccessContext, alumniID uint64, includeDrafts bool) (*dto.AlumniFileListResponse, error) {
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	if !operator.HasPermission(common.PermissionAlumniFilesManage) {
		return nil, common.ErrPermissionDenied
	}
	domainIDs, restricted := scopedDataDomainIDs(operator)
	if restricted && len(domainIDs) == 0 {
		return nil, common.ErrPermissionDenied
	}
	record, err := s.alumni.GetDetail(ctx, alumniID, domainIDs)
	if err != nil {
		return nil, err
	}

	enrollmentIDs := make([]uint64, 0, len(record.Enrollments))
	for _, enrollment := range record.Enrollments {
		enrollmentIDs = append(enrollmentIDs, enrollment.ID)
	}
	files, err := s.files.ListByEnrollmentIDs(ctx, enrollmentIDs, includeDrafts)
	if err != nil {
		return nil, err
	}

	result := &dto.AlumniFileListResponse{
		AlumniID:    alumniID,
		Enrollments: make([]dto.AlumniEnrollmentFileGroup, 0, len(record.Enrollments)),
	}
	groupIndex := make(map[uint64]int, len(record.Enrollments))
	for _, enrollment := range record.Enrollments {
		groupIndex[enrollment.ID] = len(result.Enrollments)
		result.Enrollments = append(result.Enrollments, dto.AlumniEnrollmentFileGroup{
			EnrollmentID:   enrollment.ID,
			DegreeArchive:  make([]dto.AlumniFileItem, 0),
			AcademicRecord: make([]dto.AlumniFileItem, 0),
		})
	}
	for _, rec := range files {
		if rec.EnrollmentID == nil || rec.Status == common.FileStatusPending {
			// pending 仅用于直传确认前的临时记录，列表不展示；级联删除仍会清理其对象。
			continue
		}
		if rec.Status == common.FileStatusDraft && (!includeDrafts || rec.UploadedBy == nil || *rec.UploadedBy != operator.UserID) {
			continue
		}
		index, ok := groupIndex[*rec.EnrollmentID]
		if !ok {
			continue
		}
		item := dto.AlumniFileItem{
			ID:           rec.ID,
			EnrollmentID: *rec.EnrollmentID,
			FileType:     rec.FileType,
			OriginalName: rec.OriginalName,
			FileSize:     rec.FileSize,
			MimeType:     rec.MimeType,
			Status:       rec.Status,
			CreatedAt:    rec.CreatedAt,
		}
		switch rec.FileType {
		case common.FileTypeDegreeArchive:
			result.Enrollments[index].DegreeArchive = append(result.Enrollments[index].DegreeArchive, item)
		case common.FileTypeAcademicRecord:
			result.Enrollments[index].AcademicRecord = append(result.Enrollments[index].AcademicRecord, item)
		}
	}
	return result, nil
}

// DeleteDraftFile 删除尚未保存的草稿文件（取消编辑时的清理），正式附件必须通过保存全部修改移除。
func (s *AlumniFileService) DeleteDraftFile(ctx context.Context, operator common.AccessContext, alumniID uint64, enrollmentID uint64, fileID uint64) error {
	enrollment, err := s.getAccessibleEnrollment(ctx, operator, alumniID, enrollmentID)
	if err != nil {
		return err
	}
	record, err := s.files.GetByIDAnyStatus(ctx, fileID)
	if err != nil {
		return err
	}
	if record.EnrollmentID == nil || *record.EnrollmentID != enrollmentID {
		return common.ErrFileNotFound
	}
	if record.Status != common.FileStatusDraft && record.Status != common.FileStatusPending {
		return common.ErrFileNotDraft
	}
	if record.UploadedBy == nil || *record.UploadedBy != operator.UserID {
		return common.ErrFileNotFound
	}
	if err := s.files.DeleteDraft(ctx, fileID, enrollmentID, operator.UserID); err != nil {
		return err
	}
	if err := s.store.DeleteFile(ctx, record.ObjectKey); err != nil {
		logger.Warn("failed to delete draft file from minio",
			zap.Uint64("file_id", fileID),
			zap.String("object_key", record.ObjectKey),
			zap.Error(err),
		)
	}
	alumni, err := s.alumni.GetPersonalProfile(ctx, alumniID)
	if err != nil {
		return err
	}
	s.writeOpLog(ctx, operator, "delete_draft_alumni_file", fileID, alumni, enrollment, record.FileType, record.OriginalName)
	return nil
}

// getAccessibleEnrollment 在任何文件读写前校验管理员角色、功能权限、学籍归属与数据域范围。
func (s *AlumniFileService) getAccessibleEnrollment(ctx context.Context, operator common.AccessContext, alumniID uint64, enrollmentID uint64) (*model.AlumniEnrollment, error) {
	if !operator.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	if !operator.HasPermission(common.PermissionAlumniFilesManage) {
		return nil, common.ErrPermissionDenied
	}
	domainIDs, restricted := scopedDataDomainIDs(operator)
	if restricted && len(domainIDs) == 0 {
		return nil, common.ErrPermissionDenied
	}
	return s.alumni.GetEnrollment(ctx, alumniID, enrollmentID, domainIDs)
}

// CascadeSoftDelete 校友删除时级联软删除所有学籍段的文件记录。
func (s *AlumniFileService) CascadeSoftDelete(ctx context.Context, alumniID uint64) error {
	record, err := s.alumni.GetDetail(ctx, alumniID, nil)
	if err != nil {
		return err
	}
	enrollmentIDs := make([]uint64, 0, len(record.Enrollments))
	for _, enrollment := range record.Enrollments {
		enrollmentIDs = append(enrollmentIDs, enrollment.ID)
	}
	files, err := s.files.ListByEnrollmentIDs(ctx, enrollmentIDs, true)
	if err != nil {
		return err
	}
	if err := s.files.SoftDeleteByAlumniID(ctx, alumniID); err != nil {
		return err
	}
	for _, rec := range files {
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

// RemoveObjects 在保存事务提交后清理被移除文件的 MinIO 对象（best-effort）。
func (s *AlumniFileService) RemoveObjects(ctx context.Context, objectKeys []string) {
	for _, key := range objectKeys {
		if err := s.store.DeleteFile(ctx, key); err != nil {
			logger.Warn("failed to delete removed file object", zap.String("object_key", key), zap.Error(err))
		}
	}
}

// writeOpLog 记录文件操作审计日志。
func (s *AlumniFileService) writeOpLog(ctx context.Context, operator common.AccessContext, action string, fileID uint64, alumni *model.AlumniProfile, enrollment *model.AlumniEnrollment, fileType, originalName string) {
	if s.opLogger == nil || alumni == nil || enrollment == nil {
		return
	}

	detail := map[string]any{
		"alumni_id":                alumni.ID,
		"alumni_name":              alumni.Name,
		"enrollment_id":            enrollment.ID,
		"target_data_domain_id":    enrollment.DataDomainID,
		"operator_data_domain_ids": operator.DomainIDs,
		"file_type":                fileType,
		"original_name":            originalName,
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		logger.Warn("failed to marshal op log detail", zap.Error(err))
		return
	}

	detailStr := string(detailJSON)
	log := model.OperationLog{
		OperatorID:   operator.UserID,
		OperatorRole: operator.Role,
		Action:       action,
		TargetType:   "alumni_file",
		TargetID:     &fileID,
		Detail:       &detailStr,
	}
	if err := s.opLogger.Write(ctx, &log); err != nil {
		logger.Warn("failed to write operation log", zap.String("action", action), zap.Error(err))
	}
}

// buildObjectKey 构建 MinIO 对象路径：alumni/{id}_{name}/enrollments/{eid}/{type}/{uuid}_{filename}
func buildObjectKey(alumniID uint64, alumniName string, enrollmentID uint64, fileType string, originalName string) string {
	safeName := sanitizePathSegment(alumniName)
	safeFile := sanitizePathSegment(originalName)
	return fmt.Sprintf("alumni/%d_%s/enrollments/%d/%s/%s_%s", alumniID, safeName, enrollmentID, fileType, uuid.NewString(), safeFile)
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
