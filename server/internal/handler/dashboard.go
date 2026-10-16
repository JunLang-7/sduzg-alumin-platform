package handler

import (
	"errors"
	"net/http"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
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

// Overview 返回数据大屏概览指标。
//
//	@Summary		数据大屏概览
//	@Description	统计总数与字段完备率，范围限定在调用者可访问的数据域内；普通管理员未分配数据域时返回 403。
//	@Tags			数据大屏
//	@Produce		json
//	@Success		200	{object}	response.Body{data=dto.DashboardOverview}
//	@Failure		403	{object}	response.Body	"非管理员，或未分配任何数据域"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/dashboard/overview [get]
func (h *DashboardHandler) Overview(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	result, err := h.dashboard.Overview(c.Request.Context(), *access)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

// Distribution 按指定维度返回校友分布统计。
//
//	@Summary		校友分布统计
//	@Description	返回指定维度的分组统计，直接返回数组而不套分页结构。
//	@Tags			数据大屏
//	@Produce		json
//	@Param			dimension	query		string	true	"统计维度"	Enums(grade,class_name,cohort,gender,major,training_mode,industry)
//	@Success		200			{object}	response.Body{data=[]dto.DashboardDistributionItem}
//	@Failure		400			{object}	response.Body	"维度不受支持"
//	@Failure		403			{object}	response.Body	"非管理员，或未分配任何数据域"
//	@Failure		503			{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/dashboard/distribution [get]
func (h *DashboardHandler) Distribution(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	var req dto.DashboardDistributionRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.dashboard.Distribution(c.Request.Context(), req, *access)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid dimension")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
