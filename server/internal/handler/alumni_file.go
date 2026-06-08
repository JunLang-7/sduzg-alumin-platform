package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

// AlumniFileHandler 校友档案文件处理器。
type AlumniFileHandler struct {
	fileService *service.AlumniFileService
}

// NewAlumniFileHandler 创建文件处理器实例。
func NewAlumniFileHandler(fileService *service.AlumniFileService) *AlumniFileHandler {
	return &AlumniFileHandler{fileService: fileService}
}

// uploadRequest 客户端请求预签名上传 URL 的 JSON body。
type uploadRequest struct {
	FileType     string `json:"file_type"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
}

// RequestUpload 请求预签名上传 URL POST /admin/alumni/:id/files/upload-url
// 返回 Presigned PUT URL，客户端直传 MinIO 后需调用 ConfirmUpload 确认。
func (h *AlumniFileHandler) RequestUpload(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	alumniID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || alumniID == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	var req uploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "参数错误：需要 file_type, original_name, mime_type")
		return
	}
	if req.FileType == "" || req.OriginalName == "" || req.MimeType == "" {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "file_type, original_name, mime_type 不能为空")
		return
	}

	result, err := h.fileService.GenerateUploadURL(c.Request.Context(), userID, alumniID, req.FileType, req.OriginalName, req.MimeType)
	if err == nil {
		response.JSON(c, http.StatusOK, response.CodeSuccess, "success", result)
		return
	}

	h.writeUploadError(c, err)
}

// ConfirmUpload 确认直传完成 POST /admin/alumni/:id/files/:fileId/confirm
// 验证 MinIO 对象存在后将记录标记为 active。
func (h *AlumniFileHandler) ConfirmUpload(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	alumniID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || alumniID == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	fileID, err := strconv.ParseUint(c.Param("fileId"), 10, 64)
	if err != nil || fileID == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid file id")
		return
	}

	result, err := h.fileService.ConfirmUpload(c.Request.Context(), userID, alumniID, fileID)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrFileNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeFileNotFound, "文件记录不存在或未完成上传")
	case errors.Is(err, common.ErrFileTooLarge):
		response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeFileTooLarge, "文件大小超过限制（最大 50MB）")
	case errors.Is(err, common.ErrStorageUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "存储服务不可用")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "确认上传失败")
	}
}

// ListFiles 查看文件列表 GET /admin/alumni/:id/files（管理员专用）
func (h *AlumniFileHandler) ListFiles(c *gin.Context) {
	alumniID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || alumniID == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	result, err := h.fileService.ListFiles(c.Request.Context(), alumniID)
	if err == nil {
		response.Success(c, result)
		return
	}
	response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "获取文件列表失败")
}

// DownloadURL 获取预签名下载 URL GET /admin/alumni/:id/files/:fileId/download
// 返回 Presigned GET URL，客户端直连 MinIO 下载，数据不经过 API Server。
func (h *AlumniFileHandler) DownloadURL(c *gin.Context) {
	alumniID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || alumniID == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	fileID, err := strconv.ParseUint(c.Param("fileId"), 10, 64)
	if err != nil || fileID == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid file id")
		return
	}

	result, err := h.fileService.GenerateDownloadURL(c.Request.Context(), alumniID, fileID)
	if err != nil {
		if errors.Is(err, common.ErrFileNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeFileNotFound, "文件不存在")
			return
		}
		if errors.Is(err, common.ErrStorageUnavailable) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "存储服务不可用")
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "获取下载链接失败")
		return
	}

	response.Success(c, result)
}

// Delete 删除文件 DELETE /admin/alumni/:id/files/:fileId
func (h *AlumniFileHandler) Delete(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	alumniID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || alumniID == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	fileID, err := strconv.ParseUint(c.Param("fileId"), 10, 64)
	if err != nil || fileID == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid file id")
		return
	}

	err = h.fileService.DeleteFile(c.Request.Context(), userID, alumniID, fileID)
	if err == nil {
		response.Success(c, gin.H{"deleted": true})
		return
	}

	switch {
	case errors.Is(err, common.ErrFileNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeFileNotFound, "文件不存在")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "删除失败")
	}
}

func (h *AlumniFileHandler) writeUploadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrFileTypeNotAllowed):
		response.Fail(c, http.StatusUnsupportedMediaType, response.CodeFileTypeNotAllowed, "不支持的文件类型，仅允许 PDF、Word、JPG、PNG")
	case errors.Is(err, common.ErrFileTooLarge):
		response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeFileTooLarge, "文件大小超过限制（最大 50MB）")
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrStorageUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "存储服务不可用")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "文件上传失败")
	}
}
