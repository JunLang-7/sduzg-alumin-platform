package service

import (
	"context"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"strings"
	"unicode/utf8"
)

func (s *HistoryService) RequestEntryDeletion(ctx context.Context, access common.AccessContext, id uint64, req dto.HistoryDeletionRequest) (*model.HistoryEntryDeletionRequest, error) {
	if access.UserID == 0 || !access.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 2000 {
		return nil, common.ErrInvalidRequest
	}
	return s.repository.CreateDeletionRequest(ctx, access, id, reason)
}

func (s *HistoryService) ListDeletionRequests(ctx context.Context, access common.AccessContext, mine bool) ([]model.HistoryEntryDeletionRequest, error) {
	if access.UserID == 0 || !access.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	return s.repository.ListDeletionRequests(ctx, access, mine)
}

func (s *HistoryService) ReviewEntryDeletion(ctx context.Context, access common.AccessContext, id uint64, req dto.HistoryDeletionReviewRequest) (*model.HistoryEntryDeletionRequest, error) {
	if access.UserID == 0 || !access.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	comment := strings.TrimSpace(req.ReviewComment)
	if (req.Action != "approve" && req.Action != "reject") || (req.Action == "reject" && comment == "") || utf8.RuneCountInString(comment) > 2000 {
		return nil, common.ErrInvalidRequest
	}
	return s.repository.ReviewDeletionRequest(ctx, access, id, req.Action, comment)
}
