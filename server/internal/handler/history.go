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

type HistoryHandler struct{ history *service.HistoryService }

func NewHistoryHandler(history *service.HistoryService) *HistoryHandler {
	return &HistoryHandler{history: history}
}

// ListEntries 列出当前用户可见的院史词条。
//
//	@Summary		院史词条列表
//	@Description	返回当前用户可见范围内的院史词条，直接返回数组而不套分页结构。can_request_deletion、can_delete 表示当前用户对该词条可执行的操作。
//	@Tags			院史共编
//	@Produce		json
//	@Param			keyword	query		string	false	"按标题模糊搜索"
//	@Success		200		{object}	response.Body{data=[]dto.HistoryEntryItem}
//	@Failure		401		{object}	response.Body	"未登录或令牌失效"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/entries [get]
func (h *HistoryHandler) ListEntries(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	var req dto.HistoryEntryListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}
	result, err := h.history.ListEntries(c.Request.Context(), *access, req.Keyword)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// GetEntry 返回单个院史词条详情。
//
//	@Summary	院史词条详情
//	@Tags		院史共编
//	@Produce	json
//	@Param		id	path		integer	true	"院史词条 ID"
//	@Success	200	{object}	response.Body{data=dto.HistoryEntryItem}
//	@Failure	400	{object}	response.Body	"词条 ID 无效"
//	@Failure	403	{object}	response.Body	"词条不在可访问范围内"
//	@Failure	404	{object}	response.Body	"院史内容不存在"
//	@Failure	503	{object}	response.Body	"数据库不可用"
//	@Security	BearerAuth
//	@Router		/history/entries/{id} [get]
func (h *HistoryHandler) GetEntry(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	result, err := h.history.GetEntry(c.Request.Context(), *access, id)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// UpdateEntry 更新院史词条，由服务层校验角色与数据域。
//
//	@Summary		更新院史词条
//	@Description	更新成功后生成新版本。需要对这些词条有编辑权限，由服务层按角色与数据域校验。
//	@Tags			院史共编
//	@Accept			json
//	@Produce		json
//	@Param			id		path		integer							true	"院史词条 ID"
//	@Param			request	body		dto.HistoryEntryUpdateRequest	true	"词条内容；title、content、source_note 必填"
//	@Success		200		{object}	response.Body{data=dto.HistoryEntryItem}
//	@Failure		400		{object}	response.Body	"参数不正确"
//	@Failure		403		{object}	response.Body	"权限不足"
//	@Failure		404		{object}	response.Body	"院史内容不存在"
//	@Failure		409		{object}	response.Body	"当前状态不允许此操作"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/entries/{id} [put]
func (h *HistoryHandler) UpdateEntry(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	var req dto.HistoryEntryUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}
	result, err := h.history.UpdateEntry(c.Request.Context(), *access, id, req)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// DeleteEntry 删除院史词条，由服务层校验角色与数据域。
//
//	@Summary		删除院史词条
//	@Description	管理员及以上可调用，由服务层按角色与数据域校验。
//	@Tags			院史共编
//	@Produce		json
//	@Param			id	path		integer	true	"院史词条 ID"
//	@Success		200	{object}	response.Body{data=dto.DeletedResult}
//	@Failure		400	{object}	response.Body	"词条 ID 无效"
//	@Failure		403	{object}	response.Body	"权限不足"
//	@Failure		404	{object}	response.Body	"院史内容不存在"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/entries/{id} [delete]
func (h *HistoryHandler) DeleteEntry(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	if err := h.history.DeleteEntry(c.Request.Context(), *access, id); err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, dto.DeletedResult{Deleted: true})
}

// CreateDraft 新建院史投稿草稿，成功返回 HTTP 201。
//
//	@Summary		新建院史投稿草稿
//	@Description	校友与管理员均可投稿。entry_id 为空表示新建独立词条，非空表示补充到已有词条。
//	@Tags			院史共编
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.HistoryContributionRequest	true	"投稿内容；title、content、source_note 必填"
//	@Success		201		{object}	response.Body{data=dto.HistoryContributionItem}
//	@Failure		400		{object}	response.Body	"参数不正确"
//	@Failure		403		{object}	response.Body	"权限不足"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/contributions [post]
func (h *HistoryHandler) CreateDraft(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	var req dto.HistoryContributionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}
	result, err := h.history.CreateDraft(c.Request.Context(), *access, req)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, response.CodeSuccess, "success", result)
}

// GetContribution 返回单条院史投稿详情。
//
//	@Summary		院史投稿详情
//	@Description	仅投稿作者本人可查看（管理员作者限本人数据域内）；审核人查看待审内容请使用 /history/reviews 系列接口。
//	@Tags			院史共编
//	@Produce		json
//	@Param			id	path		integer	true	"投稿 ID"
//	@Success		200	{object}	response.Body{data=dto.HistoryContributionItem}
//	@Failure		400	{object}	response.Body	"投稿 ID 无效"
//	@Failure		403	{object}	response.Body	"投稿不在可访问范围内"
//	@Failure		404	{object}	response.Body	"院史内容不存在"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/contributions/{id} [get]
func (h *HistoryHandler) GetContribution(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	result, err := h.history.GetContribution(c.Request.Context(), *access, id)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// UpdateContribution 修改院史投稿草稿，仅草稿或被退回状态可修改。
//
//	@Summary	修改院史投稿
//	@Tags		院史共编
//	@Accept		json
//	@Produce	json
//	@Param		id		path		integer							true	"投稿 ID"
//	@Param		request	body		dto.HistoryContributionRequest	true	"投稿内容；title、content、source_note 必填"
//	@Success	200		{object}	response.Body{data=dto.HistoryContributionItem}
//	@Failure	400		{object}	response.Body	"参数不正确"
//	@Failure	403		{object}	response.Body	"权限不足"
//	@Failure	404		{object}	response.Body	"院史内容不存在"
//	@Failure	409		{object}	response.Body	"投稿当前状态不允许此操作"
//	@Failure	503		{object}	response.Body	"数据库不可用"
//	@Security	BearerAuth
//	@Router		/history/contributions/{id} [put]
func (h *HistoryHandler) UpdateContribution(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	var req dto.HistoryContributionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}
	result, err := h.history.UpdateContribution(c.Request.Context(), *access, id, req)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// DeleteDraft 删除本人院史投稿草稿。
//
//	@Summary		删除院史投稿草稿
//	@Description	只允许删除本人尚未提交的草稿。
//	@Tags			院史共编
//	@Produce		json
//	@Param			id	path		integer	true	"投稿 ID"
//	@Success		200	{object}	response.Body{data=dto.DeletedResult}
//	@Failure		403	{object}	response.Body	"权限不足"
//	@Failure		404	{object}	response.Body	"院史内容不存在"
//	@Failure		409	{object}	response.Body	"投稿当前状态不允许此操作"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/contributions/{id} [delete]
func (h *HistoryHandler) DeleteDraft(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	if err := h.history.DeleteDraft(c.Request.Context(), *access, id); err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, dto.DeletedResult{Deleted: true})
}

// Submit 提交院史投稿进入审核流程。
//
//	@Summary		提交院史投稿
//	@Description	提交后状态转为待审核，将生成或更新院史词条。
//	@Tags			院史共编
//	@Produce		json
//	@Param			id	path		integer	true	"投稿 ID"
//	@Success		200	{object}	response.Body{data=dto.HistoryContributionItem}
//	@Failure		403	{object}	response.Body	"权限不足"
//	@Failure		404	{object}	response.Body	"院史内容不存在"
//	@Failure		409	{object}	response.Body	"投稿当前状态不允许此操作"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/contributions/{id}/submit [post]
func (h *HistoryHandler) Submit(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	result, err := h.history.Submit(c.Request.Context(), *access, id)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// ListMine 列出本人提交的院史投稿。
//
//	@Summary	本人院史投稿列表
//	@Tags		院史共编
//	@Produce	json
//	@Success	200	{object}	response.Body{data=[]dto.HistoryContributionItem}
//	@Failure	401	{object}	response.Body	"未登录或令牌失效"
//	@Failure	503	{object}	response.Body	"数据库不可用"
//	@Security	BearerAuth
//	@Router		/history/contributions/me [get]
func (h *HistoryHandler) ListMine(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	result, err := h.history.ListMine(c.Request.Context(), *access)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// ListPending 列出待审核的院史投稿。
//
//	@Summary		待审核投稿列表
//	@Description	仅审核人可调用，返回权限范围内的待审核投稿。
//	@Tags			院史共编
//	@Produce		json
//	@Success		200	{object}	response.Body{data=[]dto.HistoryContributionItem}
//	@Failure		403	{object}	response.Body	"非审核人"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/reviews [get]
func (h *HistoryHandler) ListPending(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	result, err := h.history.ListPending(c.Request.Context(), *access)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// Review 审核院史投稿。
//
//	@Summary		审核院史投稿
//	@Description	action 为 approve 通过、return 退回、reject 拒绝。仅审核人可调用，且投稿必须在可审核的数据域内。
//	@Tags			院史共编
//	@Accept			json
//	@Produce		json
//	@Param			id		path		integer						true	"投稿 ID"
//	@Param			request	body		dto.HistoryReviewRequest	true	"审核动作与意见"
//	@Success		200		{object}	response.Body{data=dto.HistoryContributionItem}
//	@Failure		400		{object}	response.Body	"参数不正确，action 不是 approve/return/reject"
//	@Failure		403		{object}	response.Body	"非审核人，或投稿不在可审核数据域内"
//	@Failure		404		{object}	response.Body	"院史内容不存在"
//	@Failure		409		{object}	response.Body	"投稿当前状态不允许此操作"
//	@Failure		503		{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/reviews/{id} [post]
func (h *HistoryHandler) Review(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	var req dto.HistoryReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}
	result, err := h.history.Review(c.Request.Context(), *access, id, req)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// ListAttachments 列出投稿的院史附件，投稿人与审核人共用。
//
//	@Summary		院史附件列表
//	@Description	投稿人与审核人共用同一实现，路径不同但权限校验一致。直接返回数组而不套分页结构。
//	@Tags			院史共编
//	@Produce		json
//	@Param			id	path		integer	true	"投稿 ID"
//	@Success		200	{object}	response.Body{data=[]dto.HistoryAttachmentItem}
//	@Failure		403	{object}	response.Body	"投稿不在可访问范围内"
//	@Failure		404	{object}	response.Body	"院史内容不存在"
//	@Failure		503	{object}	response.Body	"数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/contributions/{id}/attachments [get]
//	@Router			/history/reviews/{id}/attachments [get]
func (h *HistoryHandler) ListAttachments(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	result, err := h.history.ListAttachments(c.Request.Context(), *access, id)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// RequestAttachmentUpload 申请院史附件的预签名上传地址。
//
//	@Summary		申请院史附件上传地址
//	@Description	返回预签名 PUT 地址，客户端直传对象存储后必须调用确认接口。仅支持 JPG、PNG、WebP、PDF，单个不超过 10 MB，单次投稿最多 6 个附件。
//	@Tags			院史共编
//	@Accept			json
//	@Produce		json
//	@Param			id		path		integer								true	"投稿 ID"
//	@Param			request	body		dto.HistoryAttachmentUploadRequest	true	"附件元信息与权属说明"
//	@Success		200		{object}	response.Body{data=dto.HistoryAttachmentUploadResult}
//	@Failure		400		{object}	response.Body	"参数不正确"
//	@Failure		403		{object}	response.Body	"权限不足"
//	@Failure		404		{object}	response.Body	"院史内容不存在"
//	@Failure		409		{object}	response.Body	"投稿当前状态不允许此操作"
//	@Failure		413		{object}	response.Body	"附件超过 10 MB 或单次投稿超过 6 个附件"
//	@Failure		415		{object}	response.Body	"附件类型不支持，仅支持 JPG、PNG、WebP、PDF"
//	@Failure		503		{object}	response.Body	"存储或数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/contributions/{id}/attachments/upload-url [post]
func (h *HistoryHandler) RequestAttachmentUpload(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	contributionID, ok := historyID(c, "id")
	if !ok {
		return
	}
	var req dto.HistoryAttachmentUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}
	result, err := h.history.RequestAttachmentUpload(c.Request.Context(), *access, contributionID, req)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

// ConfirmAttachmentUpload 确认附件直传完成。
//
//	@Summary		确认院史附件上传完成
//	@Description	校验对象存储中的对象存在后将附件记录置为可用。
//	@Tags			院史共编
//	@Produce		json
//	@Param			id				path		integer	true	"投稿 ID"
//	@Param			attachmentId	path		integer	true	"附件 ID"
//	@Success		200				{object}	response.Body{data=dto.ConfirmedResult}
//	@Failure		403				{object}	response.Body	"权限不足"
//	@Failure		404				{object}	response.Body	"院史内容不存在或附件不存在"
//	@Failure		503				{object}	response.Body	"存储或数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/contributions/{id}/attachments/{attachmentId}/confirm [post]
func (h *HistoryHandler) ConfirmAttachmentUpload(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	contributionID, ok := historyID(c, "id")
	if !ok {
		return
	}
	attachmentID, ok := historyID(c, "attachmentId")
	if !ok {
		return
	}
	if err := h.history.ConfirmAttachmentUpload(c.Request.Context(), *access, contributionID, attachmentID); err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, dto.ConfirmedResult{Confirmed: true})
}

// DeleteAttachment 删除院史附件。
//
//	@Summary	删除院史附件
//	@Tags		院史共编
//	@Produce	json
//	@Param		id				path		integer	true	"投稿 ID"
//	@Param		attachmentId	path		integer	true	"附件 ID"
//	@Success	200				{object}	response.Body{data=dto.DeletedResult}
//	@Failure	403				{object}	response.Body	"权限不足"
//	@Failure	404				{object}	response.Body	"院史内容不存在或附件不存在"
//	@Failure	409				{object}	response.Body	"投稿当前状态不允许此操作"
//	@Failure	503				{object}	response.Body	"存储或数据库不可用"
//	@Security	BearerAuth
//	@Router		/history/contributions/{id}/attachments/{attachmentId} [delete]
func (h *HistoryHandler) DeleteAttachment(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	contributionID, ok := historyID(c, "id")
	if !ok {
		return
	}
	attachmentID, ok := historyID(c, "attachmentId")
	if !ok {
		return
	}
	if err := h.history.DeleteAttachment(c.Request.Context(), *access, contributionID, attachmentID); err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, dto.DeletedResult{Deleted: true})
}

// UpdateAttachment 修改院史附件的描述与权属说明。
//
//	@Summary	修改院史附件信息
//	@Tags		院史共编
//	@Accept		json
//	@Produce	json
//	@Param		id				path		integer								true	"投稿 ID"
//	@Param		attachmentId	path		integer								true	"附件 ID"
//	@Param		request			body		dto.HistoryAttachmentUpdateRequest	true	"附件描述与权属说明"
//	@Success	200				{object}	response.Body{data=dto.UpdatedResult}
//	@Failure	400				{object}	response.Body	"参数不正确"
//	@Failure	403				{object}	response.Body	"权限不足"
//	@Failure	404				{object}	response.Body	"院史内容不存在或附件不存在"
//	@Failure	409				{object}	response.Body	"投稿当前状态不允许此操作"
//	@Failure	503				{object}	response.Body	"数据库不可用"
//	@Security	BearerAuth
//	@Router		/history/contributions/{id}/attachments/{attachmentId} [put]
func (h *HistoryHandler) UpdateAttachment(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	contributionID, ok := historyID(c, "id")
	if !ok {
		return
	}
	attachmentID, ok := historyID(c, "attachmentId")
	if !ok {
		return
	}
	var req dto.HistoryAttachmentUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}
	if err := h.history.UpdateAttachment(c.Request.Context(), *access, contributionID, attachmentID, req); err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, dto.UpdatedResult{Updated: true})
}

// AttachmentDownloadURL 获取院史附件的预签名下载地址。
//
//	@Summary		获取院史附件下载地址
//	@Description	返回预签名 GET 地址，客户端直连对象存储下载。
//	@Tags			院史共编
//	@Produce		json
//	@Param			id				path		integer	true	"投稿 ID"
//	@Param			attachmentId	path		integer	true	"附件 ID"
//	@Success		200				{object}	response.Body{data=dto.HistoryAttachmentDownloadResult}
//	@Failure		403				{object}	response.Body	"权限不足"
//	@Failure		404				{object}	response.Body	"院史内容不存在或附件不存在"
//	@Failure		503				{object}	response.Body	"存储或数据库不可用"
//	@Security		BearerAuth
//	@Router			/history/contributions/{id}/attachments/{attachmentId}/download [get]
func (h *HistoryHandler) AttachmentDownloadURL(c *gin.Context) {
	access, ok := middleware.CurrentAccessContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	contributionID, ok := historyID(c, "id")
	if !ok {
		return
	}
	attachmentID, ok := historyID(c, "attachmentId")
	if !ok {
		return
	}
	result, err := h.history.AttachmentDownloadURL(c.Request.Context(), *access, contributionID, attachmentID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

func historyID(c *gin.Context, key string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func (h *HistoryHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrPermissionDenied):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "权限不足")
	case service.IsHistoryNotFound(err), errors.Is(err, common.ErrHistoryAttachmentNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "院史内容不存在")
	case errors.Is(err, common.ErrInvalidHistoryState):
		response.Fail(c, http.StatusConflict, response.CodeBadRequest, "投稿当前状态不允许此操作")
	case errors.Is(err, common.ErrInvalidRequest):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "请求参数不正确")
	case errors.Is(err, common.ErrAlumniNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "未找到校友档案")
	case errors.Is(err, common.ErrDatabaseUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "database is unavailable")
	case errors.Is(err, common.ErrStorageUnavailable):
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "storage is unavailable")
	case errors.Is(err, common.ErrFileTypeNotAllowed):
		response.Fail(c, http.StatusUnsupportedMediaType, response.CodeFileTypeNotAllowed, "仅支持 JPG、PNG、WebP、PDF")
	case errors.Is(err, common.ErrFileTooLarge):
		response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeFileTooLarge, "附件超过 10 MB 或单次投稿超过 6 个附件")
	default:
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
