package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
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

	result, err := h.alumni.List(c.Request.Context(), req)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, service.ErrDatabaseUnavailable):
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

	result, err := h.alumni.GetByID(c.Request.Context(), id)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, service.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应校友不存在")
	case errors.Is(err, service.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
