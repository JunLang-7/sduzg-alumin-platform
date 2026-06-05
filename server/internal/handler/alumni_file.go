package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

// AlumniFileHandler 校友档案文件处理器。
type AlumniFileHandler struct {
	fileService *service.AlumniFileService
	userRepo    UserRoleLookup
}

// UserRoleLookup 查询用户角色和绑定的校友 ID（用于权限控制）。
type UserRoleLookup interface {
	FindByID(ctx context.Context, userID uint64) (*model.User, error)
}

// NewAlumniFileHandler 创建文件处理器实例。
func NewAlumniFileHandler(fileService *service.AlumniFileService, userRepo UserRoleLookup) *AlumniFileHandler {
	return &AlumniFileHandler{
		fileService: fileService,
		userRepo:    userRepo,
	}
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
	if fileType != common.FileTypeDegreeArchive && fileType != common.FileTypeAcademicRecord {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "file_type 必须为 degree_archive 或 academic_record")
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

// ListFiles 查看文件列表 GET /alumni/:id/files
// 校友只能查看自己的文件，管理员可查看所有人。
func (h *AlumniFileHandler) ListFiles(c *gin.Context) {
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

	// 权限控制：校友只能查看自己的文件
	if allowed := h.checkAlumniAccess(c, userID, alumniID); !allowed {
		return
	}

	result, err := h.fileService.ListFiles(c.Request.Context(), alumniID)
	if err == nil {
		response.Success(c, result)
		return
	}
	response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "获取文件列表失败")
}

// Download 下载文件 GET /alumni/:id/files/:fileId/download
func (h *AlumniFileHandler) Download(c *gin.Context) {
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

	// 权限控制
	if allowed := h.checkAlumniAccess(c, userID, alumniID); !allowed {
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

// checkAlumniAccess 权限控制：校友只能访问自己绑定的档案文件，管理员可访问所有人。
func (h *AlumniFileHandler) checkAlumniAccess(c *gin.Context, userID uint64, alumniID uint64) bool {
	if h.userRepo == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
		return false
	}

	user, err := h.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return false
	}

	// 管理员可以查看/下载所有人的文件
	if user.Role == common.RoleAdmin || user.Role == common.RoleSuperAdmin {
		return true
	}

	// 校友只能访问自己绑定的档案
	if user.Role == common.RoleAlumni && user.AlumniID != nil && *user.AlumniID == alumniID {
		return true
	}

	response.Fail(c, http.StatusForbidden, response.CodeForbidden, "只能访问本人的档案文件")
	return false
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
