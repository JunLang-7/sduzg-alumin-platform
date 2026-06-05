package handler

import (
	"errors"
	"fmt"
	"io"
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

// Upload 上传文件 POST /admin/alumni/:id/files
func (h *AlumniFileHandler) Upload(c *gin.Context) {
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

	fileType := c.PostForm("file_type")
	if fileType == "" {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "file_type 不能为空")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "缺少上传文件")
		return
	}

	result, err := h.fileService.Upload(c.Request.Context(), userID, alumniID, fileType, file)
	if err == nil {
		response.JSON(c, http.StatusCreated, response.CodeSuccess, "上传成功", result)
		return
	}

	h.writeUploadError(c, err)
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

// Download 下载文件 GET /admin/alumni/:id/files/:fileId/download（管理员专用）
func (h *AlumniFileHandler) Download(c *gin.Context) {
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

	dl, err := h.fileService.Download(c.Request.Context(), fileID, alumniID)
	if err != nil {
		if errors.Is(err, common.ErrFileNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeFileNotFound, "文件不存在")
			return
		}
		if errors.Is(err, common.ErrStorageUnavailable) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "存储服务不可用")
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "下载失败")
		return
	}
	defer dl.Reader.Close()

	// 设置强制下载响应头
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, dl.OriginalName))
	c.Header("Content-Type", dl.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", dl.Size))

	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, dl.Reader); err != nil {
		c.Abort()
		return
	}
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
