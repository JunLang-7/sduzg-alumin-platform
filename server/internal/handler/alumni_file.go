package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
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

// RequestUpload 请求预签名上传 URL POST /admin/alumni/:id/files/upload-url
// 返回 Presigned PUT URL，客户端直传 MinIO 后需调用 ConfirmUpload 确认。
//
//	@Summary		申请档案文件上传地址
//	@Description	生成预签名 PUT 地址，客户端直传对象存储后必须调用确认接口。需要 alumni.files.manage 功能权限，且档案必须在可访问数据域内；该权限与敏感字段权限互相独立。
//	@Tags			校友档案文件
//	@Accept			json
//	@Produce		json
//	@Param			id		path		integer							true	"校友档案 ID"
//	@Param			request	body		dto.AlumniFileUploadURLRequest	true	"文件类型与元信息"
//	@Success		200		{object}	response.Body{data=dto.AlumniFileUploadURLResponse}
//	@Failure		400		{object}	response.Body	"校友 ID 无效，或 file_type、original_name、mime_type 缺失"
//	@Failure		403		{object}	response.Body	"权限不足，或档案不在可访问数据域内"
//	@Failure		404		{object}	response.Body	"对应校友不存在"
//	@Failure		413		{object}	response.Body	"文件大小超过限制（最大 50MB）"
//	@Failure		415		{object}	response.Body	"不支持的文件类型，仅允许 PDF、Word、JPG、PNG"
//	@Failure		503		{object}	response.Body	"存储或数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/alumni/{id}/files/upload-url [post]
func (h *AlumniFileHandler) RequestUpload(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	alumniID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || alumniID == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	var req dto.AlumniFileUploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "参数错误：需要 file_type, original_name, mime_type")
		return
	}

	result, err := h.fileService.GenerateUploadURL(c.Request.Context(), *access, alumniID, req.FileType, req.OriginalName, req.MimeType)
	if err == nil {
		response.JSON(c, http.StatusOK, response.CodeSuccess, "success", result)
		return
	}

	h.writeUploadError(c, err)
}

// ConfirmUpload 确认直传完成 POST /admin/alumni/:id/files/:fileId/confirm
// 验证 MinIO 对象存在后将记录标记为 active。
//
//	@Summary		确认档案文件上传完成
//	@Description	校验对象存储中的对象存在后，将文件记录标记为 active。需要 alumni.files.manage 功能权限。
//	@Tags			校友档案文件
//	@Produce		json
//	@Param			id		path		integer	true	"校友档案 ID"
//	@Param			fileId	path		integer	true	"文件记录 ID"
//	@Success		200		{object}	response.Body{data=dto.AlumniFileUploadResponse}
//	@Failure		403		{object}	response.Body	"权限不足"
//	@Failure		404		{object}	response.Body	"文件记录不存在或未完成上传，或对应校友不存在"
//	@Failure		413		{object}	response.Body	"文件大小超过限制（最大 50MB）"
//	@Failure		503		{object}	response.Body	"存储或数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/alumni/{id}/files/{fileId}/confirm [post]
func (h *AlumniFileHandler) ConfirmUpload(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
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

	result, err := h.fileService.ConfirmUpload(c.Request.Context(), *access, alumniID, fileID)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrFileNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeFileNotFound, "文件记录不存在或未完成上传")
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrFileTooLarge):
		response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeFileTooLarge, "文件大小超过限制（最大 50MB）")
	case errors.Is(err, common.ErrStorageUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "存储服务不可用")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "数据库服务不可用")
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "确认上传失败")
	}
}

// ListFiles 查看文件列表 GET /admin/alumni/:id/files（管理员专用）
//
//	@Summary		档案文件列表
//	@Description	按类型分组返回该校友的档案文件。需要 alumni.files.manage 功能权限，该权限与敏感字段权限互相独立。
//	@Tags			校友档案文件
//	@Produce		json
//	@Param			id	path		integer	true	"校友档案 ID"
//	@Success		200	{object}	response.Body{data=dto.AlumniFileListResponse}
//	@Failure		403	{object}	response.Body	"权限不足"
//	@Failure		404	{object}	response.Body	"对应校友不存在"
//	@Failure		503	{object}	response.Body	"存储或数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/alumni/{id}/files [get]
func (h *AlumniFileHandler) ListFiles(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	alumniID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || alumniID == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	result, err := h.fileService.ListFiles(c.Request.Context(), *access, alumniID)
	if err == nil {
		response.Success(c, result)
		return
	}
	h.writeFileError(c, err, "获取文件列表失败")
}

// DownloadURL 获取预签名下载 URL GET /admin/alumni/:id/files/:fileId/download
// 返回 Presigned GET URL，客户端直连 MinIO 下载，数据不经过 API Server。
//
//	@Summary		获取档案文件下载地址
//	@Description	返回预签名 GET 地址，客户端直连对象存储下载，文件流不经过 API 服务。需要 alumni.files.manage 功能权限。
//	@Tags			校友档案文件
//	@Produce		json
//	@Param			id		path		integer	true	"校友档案 ID"
//	@Param			fileId	path		integer	true	"文件记录 ID"
//	@Success		200		{object}	response.Body{data=dto.AlumniFileDownloadURLResponse}
//	@Failure		403		{object}	response.Body	"权限不足"
//	@Failure		404		{object}	response.Body	"文件不存在"
//	@Failure		503		{object}	response.Body	"存储或数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/alumni/{id}/files/{fileId}/download [get]
func (h *AlumniFileHandler) DownloadURL(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
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

	result, err := h.fileService.GenerateDownloadURL(c.Request.Context(), *access, alumniID, fileID)
	if err != nil {
		if errors.Is(err, common.ErrFileNotFound) || errors.Is(err, common.ErrAlumniNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeFileNotFound, "文件不存在")
			return
		}
		if errors.Is(err, common.ErrStorageUnavailable) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "存储服务不可用")
			return
		}
		if errors.Is(err, common.ErrDatabaseUnavailable) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "数据库服务不可用")
			return
		}
		if errors.Is(err, common.ErrPermissionDenied) {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "获取下载链接失败")
		return
	}

	response.Success(c, result)
}

// Delete 删除文件 DELETE /admin/alumni/:id/files/:fileId
//
//	@Summary		删除档案文件
//	@Description	同时删除对象存储中的对象与文件记录。需要 alumni.files.manage 功能权限。
//	@Tags			校友档案文件
//	@Produce		json
//	@Param			id		path		integer	true	"校友档案 ID"
//	@Param			fileId	path		integer	true	"文件记录 ID"
//	@Success		200		{object}	response.Body{data=dto.DeletedResult}
//	@Failure		403		{object}	response.Body	"权限不足"
//	@Failure		404		{object}	response.Body	"文件不存在"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/alumni/{id}/files/{fileId} [delete]
func (h *AlumniFileHandler) Delete(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
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

	err = h.fileService.DeleteFile(c.Request.Context(), *access, alumniID, fileID)
	if err == nil {
		response.Success(c, dto.DeletedResult{Deleted: true})
		return
	}

	switch {
	case errors.Is(err, common.ErrFileNotFound), errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeFileNotFound, "文件不存在")
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "数据库服务不可用")
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
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrStorageUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "存储服务不可用")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "数据库服务不可用")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "文件上传失败")
	}
}

func (h *AlumniFileHandler) writeFileError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, common.ErrAlumniNotFound), errors.Is(err, common.ErrFileNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrStorageUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "存储服务不可用")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "数据库服务不可用")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, fallback)
	}
}
