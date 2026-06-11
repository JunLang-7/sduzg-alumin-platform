package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

type AlumniHandler struct {
	alumni *service.AlumniService
}

func NewAlumniHandler(alumni *service.AlumniService) *AlumniHandler {
	return &AlumniHandler{alumni: alumni}
}

func (h *AlumniHandler) List(c *gin.Context) {
	var req dto.AlumniListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	viewerID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	result, err := h.alumni.List(c.Request.Context(), req, viewerID)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

func (h *AlumniHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	viewerID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	result, err := h.alumni.GetByID(c.Request.Context(), id, viewerID)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

func (h *AlumniHandler) Create(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	var req dto.AdminAlumniCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.alumni.Create(c.Request.Context(), userID, req)
	if err == nil {
		response.JSON(c, http.StatusCreated, response.CodeSuccess, "success", result)
		return
	}

	switch {
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

func (h *AlumniHandler) Update(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	var req dto.AdminAlumniUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.alumni.Update(c.Request.Context(), userID, id, req)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

func (h *AlumniHandler) Delete(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid alumni id")
		return
	}

	err = h.alumni.Delete(c.Request.Context(), userID, id)
	if err == nil {
		response.Success(c, gin.H{"deleted": true})
		return
	}

	switch {
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

func (h *AlumniHandler) Export(c *gin.Context) {
	var req dto.AlumniExportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.alumni.Export(c.Request.Context(), req)
	if err == nil {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", result.Filename))
		c.Data(http.StatusOK, result.ContentType, result.Data)
		return
	}

	switch {
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

func (h *AlumniHandler) ExportTemplate(c *gin.Context) {
	result, err := h.alumni.ExportTemplate(c.Request.Context())
	if err == nil {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", result.Filename))
		c.Data(http.StatusOK, result.ContentType, result.Data)
		return
	}

	response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
}

func (h *AlumniHandler) Import(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "请选择要上传的 Excel 文件")
		return
	}

	const maxSize = 20 << 20 // 20 MB
	if fileHeader.Size > maxSize {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "文件过大，请上传小于 20MB 的文件")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "无法打开上传文件")
		return
	}
	defer file.Close()

	result, err := h.alumni.Import(c.Request.Context(), userID, file)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "文件格式不正确，请使用导出的模板文件")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "服务器内部错误")
	}
}

func (h *AlumniHandler) Me(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	result, err := h.alumni.GetMe(c.Request.Context(), userID)
	if err == nil {
		response.Success(c, result)
		return
	}

	h.writeMeError(c, err)
}

func (h *AlumniHandler) UpdateMe(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	var req dto.AlumniProfileUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.alumni.UpdateMe(c.Request.Context(), userID, req)
	if err == nil {
		response.Success(c, result)
		return
	}

	h.writeMeError(c, err)
}

func (h *AlumniHandler) writeMeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrUserNotFound):
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
	case errors.Is(err, common.ErrAccountDisabled):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "account is disabled")
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "仅校友账号可访问本人资料")
	case errors.Is(err, common.ErrAlumniProfileUnbound):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "当前账号未绑定校友档案")
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
