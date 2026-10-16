package handler

import (
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/gin-gonic/gin"
	"net/http"
)

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

func (h *HistoryHandler) ListMyDeletionRequests(c *gin.Context) { h.listDeletionRequests(c, true) }
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
