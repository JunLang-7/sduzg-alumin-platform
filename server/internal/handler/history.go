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

func (h *HistoryHandler) ListEntries(c *gin.Context) {
	var req dto.HistoryEntryListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request")
		return
	}
	result, err := h.history.ListEntries(c.Request.Context(), req.Keyword)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *HistoryHandler) GetEntry(c *gin.Context) {
	id, ok := historyID(c, "id")
	if !ok {
		return
	}
	result, err := h.history.GetEntry(c.Request.Context(), id)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, result)
}

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
	response.Success(c, gin.H{"confirmed": true})
}

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
