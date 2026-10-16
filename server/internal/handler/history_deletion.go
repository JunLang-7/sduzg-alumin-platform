package handler

import (
	"net/http"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/gin-gonic/gin"
)

// RequestEntryDeletion 就已有院史词条提交删除申请，成功后进入交叉审核，返回 HTTP 201。
//
//	@Summary		提交院史词条删除申请
//	@Description	不直接删除词条，而是创建待其他管理员交叉审核的删除申请；申请人本人不能审核自己的申请。
//	@Tags			院史共编
//	@Accept			json
//	@Produce		json
//	@Param			id		path		integer						true	"院史词条 ID"
//	@Param			request	body		dto.HistoryDeletionRequest	true	"删除原因，最多 2000 字"
//	@Success		201		{object}	response.Body{data=model.HistoryEntryDeletionRequest}
//	@Failure		400		{object}	response.Body	"词条 ID 无效，或未填写删除原因"
//	@Failure		403		{object}	response.Body	"权限不足"
//	@Failure		404		{object}	response.Body	"院史内容不存在"
//	@Failure		409		{object}	response.Body	"已存在待审核的删除申请或当前状态不允许此操作"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/entries/{id}/deletion-requests [post]
func (h *HistoryHandler) RequestEntryDeletion(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	var req dto.HistoryDeletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "请填写删除原因（最多2000字）")
		return
	}
	result, err := h.history.RequestEntryDeletion(c.Request.Context(), *access, id, req)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, response.CodeSuccess, "success", result)
}

// ListMyDeletionRequests 列出本人提交的院史词条删除申请。
//
//	@Summary	本人删除申请列表
//	@Tags		院史共编
//	@Produce	json
//	@Success	200	{object}	response.Body{data=[]model.HistoryEntryDeletionRequest}
//	@Failure	401	{object}	response.Body	"未登录或令牌失效"
//	@Failure	503	{object}	response.Body	"数据库不可用"
//	@Security	BearerAuth
//	@Router		/history/deletion-requests/me [get]
func (h *HistoryHandler) ListMyDeletionRequests(c *gin.Context) { h.listDeletionRequests(c, true) }

// ListPendingDeletionRequests 列出待审核的院史词条删除申请。
//
//	@Summary		待审核删除申请列表
//	@Description	管理员及以上可调用，返回权限范围内待交叉审核的删除申请，直接返回数组而不套分页结构。
//	@Tags			院史共编
//	@Produce		json
//	@Success		200	{object}	response.Body{data=[]model.HistoryEntryDeletionRequest}
//	@Failure		403	{object}	response.Body	"权限不足，或未分配任何数据域"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/deletion-requests [get]
func (h *HistoryHandler) ListPendingDeletionRequests(c *gin.Context) {
	h.listDeletionRequests(c, false)
}
func (h *HistoryHandler) listDeletionRequests(c *gin.Context, mine bool) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	items, err := h.history.ListDeletionRequests(c.Request.Context(), *access, mine)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, items)
}

// ReviewEntryDeletion 审核院史词条删除申请。
//
//	@Summary		审核删除申请
//	@Description	action 为 approve 通过并删除词条，或 reject 驳回。申请人不能审核自己的申请。
//	@Tags			院史共编
//	@Accept			json
//	@Produce		json
//	@Param			id		path		integer								true	"删除申请 ID"
//	@Param			request	body		dto.HistoryDeletionReviewRequest	true	"审核动作与意见"
//	@Success		200		{object}	response.Body{data=model.HistoryEntryDeletionRequest}
//	@Failure		400		{object}	response.Body	"参数不正确，action 不是 approve/reject"
//	@Failure		403		{object}	response.Body	"权限不足，或尝试审核本人提交的申请"
//	@Failure		404		{object}	response.Body	"院史内容不存在"
//	@Failure		409		{object}	response.Body	"申请当前状态不允许此操作"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/deletion-requests/{id}/review [post]
func (h *HistoryHandler) ReviewEntryDeletion(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	var req dto.HistoryDeletionReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不正确")
		return
	}
	item, err := h.history.ReviewEntryDeletion(c.Request.Context(), *access, id, req)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, item)
}
