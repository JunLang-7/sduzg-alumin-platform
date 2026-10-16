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

type AuditHandler struct {
	audit *service.AuditService
}

func NewAuditHandler(audit *service.AuditService) *AuditHandler {
	return &AuditHandler{audit: audit}
}

// List 分页查询操作历史，仅管理员及以上可访问。
//
//	@Summary		操作历史列表
//	@Description	管理员按已分配的有效数据域查询操作记录；超级管理员可查全域。普通管理员即使未分配数据域也只能得到空结果。
//	@Tags			操作历史
//	@Produce		json
//	@Param			page				query		int		false	"页码，默认 1"
//	@Param			page_size			query		int		false	"每页条数，默认 20，最大 100"
//	@Param			start_date			query		string	false	"开始日期（YYYY-MM-DD，含当天）"
//	@Param			end_date			query		string	false	"结束日期（YYYY-MM-DD，含当天）"
//	@Param			management_scope	query		string	false	"管理范围，取数据域中文名，如 本科生、学术学位研究生、MPA专业学位研究生"
//	@Param			action				query		string	false	"操作类型"	Enums(create,update,delete,import)
//	@Param			target_id			query		integer	false	"目标校友档案 ID"
//	@Success		200					{object}	response.Body{data=dto.AuditOperationPage}
//	@Failure		400					{object}	response.Body	"筛选条件不正确，含日期格式错误"
//	@Failure		403					{object}	response.Body	"权限不足"
//	@Failure		503					{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/audit/operations [get]
func (h *AuditHandler) List(c *gin.Context) {
	var req dto.AuditListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	result, err := h.audit.List(c.Request.Context(), *access, req)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "操作历史筛选条件不正确")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "操作历史加载失败")
	}
}

// Detail 查询单条操作详情，敏感字段只返回脱敏状态。
//
//	@Summary		操作历史详情
//	@Description	返回字段变化、批量导入明细等。无 alumni.sensitive.read 权限时，敏感字段只返回脱敏状态而不返回原值。
//	@Tags			操作历史
//	@Produce		json
//	@Param			id	path		integer	true	"操作记录 ID"
//	@Success		200	{object}	response.Body{data=dto.AuditOperation}
//	@Failure		400	{object}	response.Body	"操作记录 ID 无效"
//	@Failure		403	{object}	response.Body	"权限不足，或记录不在可访问数据域内"
//	@Failure		404	{object}	response.Body	"操作记录不存在"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/admin/audit/operations/{id} [get]
func (h *AuditHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid audit id")
		return
	}
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	result, err := h.audit.Detail(c.Request.Context(), *access, id)
	if err == nil {
		response.Success(c, result)
		return
	}

	switch {
	case errors.Is(err, common.ErrAuditNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "操作记录不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "操作详情加载失败")
	}
}
