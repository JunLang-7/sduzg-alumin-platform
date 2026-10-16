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

type AdminHandler struct {
	admin *service.AdminService
}

func NewAdminHandler(admin *service.AdminService) *AdminHandler {
	return &AdminHandler{admin: admin}
}

// List 分页列出管理员账号（仅超级管理员）。
//
//	@Summary		管理员账号列表
//	@Description	仅超级管理员可调用。返回账号、角色、状态、已分配数据域与功能权限。
//	@Tags			管理员账号与授权
//	@Produce		json
//	@Param			page		query		int	false	"页码，默认 1"
//	@Param			page_size	query		int	false	"每页条数，默认 20，最大 100"
//	@Success		200			{object}	dto.AdminListItemPage
//	@Failure		401			{object}	response.Body	"未登录或令牌失效"
//	@Failure		403			{object}	response.Body	"非超级管理员"
//	@Failure		503			{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/super-admin/admins [get]
func (h *AdminHandler) List(c *gin.Context) {
	var req dto.AdminListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.admin.List(c.Request.Context(), req)
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

// Create 新增管理员账号并分配数据域与功能权限（仅超级管理员），成功返回 HTTP 201。
//
//	@Summary		新增管理员账号
//	@Description	仅超级管理员可调用。账号须为 4-32 位、以字母开头，仅可包含字母、数字、下划线和连字符；新建账号默认密码登录可用。
//	@Tags			管理员账号与授权
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.AdminCreateRequest	true	"账号、初始密码、数据域与功能权限"
//	@Success		201		{object}	response.Body{data=dto.AdminDetail}
//	@Failure		400		{object}	response.Body	"账号或手机号格式不正确、数据域无效、账号已存在"
//	@Failure		403		{object}	response.Body	"非超级管理员"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/super-admin/admins [post]
func (h *AdminHandler) Create(c *gin.Context) {
	operator, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	var req dto.AdminCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.admin.Create(c.Request.Context(), *operator, req)
	if err == nil {
		response.JSON(c, http.StatusCreated, response.CodeSuccess, "success", result)
		return
	}

	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrInvalidAccountFormat):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "账号须为 4-32 位，以字母开头，仅可包含字母、数字、下划线和连字符")
	case errors.Is(err, common.ErrInvalidMobileFormat):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "手机号格式不正确")
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
	case errors.Is(err, common.ErrInvalidDataDomain):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "数据域无效")
	case errors.Is(err, common.ErrAccountAlreadyExists):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "账号已存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

// Detail 查看指定管理员的数据域与功能权限（仅超级管理员）。
//
//	@Summary	管理员详情
//	@Tags		管理员账号与授权
//	@Produce	json
//	@Param		id	path		integer	true	"管理员账号 ID"
//	@Success	200	{object}	response.Body{data=dto.AdminDetail}
//	@Failure	400	{object}	response.Body	"管理员 ID 无效"
//	@Failure	403	{object}	response.Body	"非超级管理员"
//	@Failure	404	{object}	response.Body	"对应管理员不存在"
//	@Failure	503	{object}	response.Body	"数据库不可用"
//	@Security	BearerAuth
//	@Router		/super-admin/admins/{id} [get]
func (h *AdminHandler) Detail(c *gin.Context) {
	operator, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid admin id")
		return
	}

	result, err := h.admin.Get(c.Request.Context(), *operator, id)
	if err == nil {
		response.Success(c, result)
		return
	}
	h.writeAccessError(c, err)
}

// ReplaceAccess 整体替换管理员的数据域与功能权限（仅超级管理员）。
//
//	@Summary		替换管理员授权
//	@Description	整体替换而非合并，传入空数组即收回全部数据域与权限。不能修改超级管理员本人的授权。
//	@Tags			管理员账号与授权
//	@Accept			json
//	@Produce		json
//	@Param			id		path		integer							true	"管理员账号 ID"
//	@Param			request	body		dto.AdminAccessUpdateRequest	true	"新的数据域与功能权限"
//	@Success		200		{object}	response.Body{data=dto.AdminDetail}
//	@Failure		400		{object}	response.Body	"参数不正确、数据域无效，或尝试修改超级管理员授权"
//	@Failure		403		{object}	response.Body	"非超级管理员"
//	@Failure		404		{object}	response.Body	"对应管理员不存在"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/super-admin/admins/{id}/access [put]
func (h *AdminHandler) ReplaceAccess(c *gin.Context) {
	operator, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid admin id")
		return
	}
	var req dto.AdminAccessUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}

	result, err := h.admin.ReplaceAccess(c.Request.Context(), *operator, id, req)
	if err == nil {
		response.Success(c, result)
		return
	}
	h.writeAccessError(c, err)
}

// ListDataDomains 列出全部有效数据域（仅超级管理员）。
//
//	@Summary		数据域列表
//	@Description	返回当前有效的数据域，用于分配授权。不要硬编码数据库 ID。
//	@Tags			管理员账号与授权
//	@Produce		json
//	@Success		200	{object}	response.Body{data=[]dto.AdminDataDomain}
//	@Failure		403	{object}	response.Body	"非超级管理员"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/super-admin/data-domains [get]
func (h *AdminHandler) ListDataDomains(c *gin.Context) {
	result, err := h.admin.ListDataDomains(c.Request.Context())
	if err == nil {
		response.Success(c, result)
		return
	}
	h.writeAccessError(c, err)
}

// Delete 删除管理员账号（仅超级管理员）。
//
//	@Summary		删除管理员账号
//	@Description	不能删除自己，也不能删除超级管理员账号。
//	@Tags			管理员账号与授权
//	@Produce		json
//	@Param			id	path		integer	true	"管理员账号 ID"
//	@Success		200	{object}	response.Body{data=dto.DeletedResult}
//	@Failure		400	{object}	response.Body	"管理员 ID 无效、尝试删除自己或尝试删除超级管理员"
//	@Failure		403	{object}	response.Body	"非超级管理员"
//	@Failure		404	{object}	response.Body	"对应管理员不存在"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/super-admin/admins/{id} [delete]
func (h *AdminHandler) Delete(c *gin.Context) {
	operator, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid admin id")
		return
	}

	err = h.admin.Delete(c.Request.Context(), *operator, id)
	if err == nil {
		response.Success(c, dto.DeletedResult{Deleted: true})
		return
	}

	h.writeAccessError(c, err)
}

func (h *AdminHandler) writeAccessError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case errors.Is(err, common.ErrInvalidDataDomain):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "数据域无效")
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
	case errors.Is(err, common.ErrCannotModifySuper):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "不能修改超级管理员授权")
	case errors.Is(err, common.ErrCannotDeleteSelf):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "不能删除自己")
	case errors.Is(err, common.ErrCannotDeleteSuper):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "不能删除超级管理员账号")
	case errors.Is(err, common.ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "对应管理员不存在")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
