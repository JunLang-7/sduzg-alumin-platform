package handler

import (
	"errors"
	"net/http"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	dashboard *service.DashboardService
}

func NewDashboardHandler(dashboard *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboard: dashboard}
}

func (h *DashboardHandler) Overview(c *gin.Context) {
	result, err := h.dashboard.Overview(c.Request.Context())
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

func (h *DashboardHandler) Distribution(c *gin.Context) {
	var req dto.DashboardDistributionRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.dashboard.Distribution(c.Request.Context(), req)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid dimension")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
