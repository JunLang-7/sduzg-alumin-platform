package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/storage"
	"github.com/google/uuid"
)

type HistoryService struct {
	repository *repository.HistoryRepository
	storage    *storage.Client
}

func NewHistoryService(repository *repository.HistoryRepository, storageClient *storage.Client) *HistoryService {
	return &HistoryService{repository: repository, storage: storageClient}
}

// ListEntries returns only published entries. Route authentication is enforced
// globally; this method deliberately never exposes a draft or pending entry.
func (s *HistoryService) ListEntries(ctx context.Context, keyword string) ([]dto.HistoryEntryItem, error) {
	entries, err := s.repository.ListPublished(ctx, keyword)
	if err != nil {
		return nil, err
	}
	result := make([]dto.HistoryEntryItem, 0, len(entries))
	for _, entry := range entries {
		sourceNote, err := s.repository.LatestSourceNote(ctx, entry.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, historyEntryItem(entry, sourceNote))
	}
	return result, nil
}

func (s *HistoryService) GetEntry(ctx context.Context, id uint64) (*dto.HistoryEntryItem, error) {
	entry, err := s.repository.GetPublished(ctx, id)
	if err != nil {
		return nil, err
	}
	sourceNote, err := s.repository.LatestSourceNote(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	result := historyEntryItem(entry, sourceNote)
	return &result, nil
}

func (s *HistoryService) UpdateEntry(ctx context.Context, access common.AccessContext, id uint64, req dto.HistoryEntryUpdateRequest) (*dto.HistoryEntryItem, error) {
	if !access.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	if !access.IsSuperAdmin() {
		domainID, err := s.repository.EntryDataDomainID(ctx, id)
		if err != nil {
			return nil, err
		}
		if domainID == nil || !access.CanAccessDomain(*domainID) {
			return nil, common.ErrPermissionDenied
		}
	}
	entry, err := s.repository.UpdatePublished(ctx, id, access.UserID, req)
	if err != nil {
		return nil, err
	}
	result := historyEntryItem(entry, strings.TrimSpace(req.SourceNote))
	return &result, nil
}

func (s *HistoryService) CreateDraft(ctx context.Context, access common.AccessContext, req dto.HistoryContributionRequest) (*dto.HistoryContributionItem, error) {
	if !access.IsAdministrator() && (access.Role != common.RoleAlumni || access.AlumniID == nil) {
		return nil, common.ErrPermissionDenied
	}
	var (
		domainID       uint64
		authorAlumniID uint64
	)
	if access.Role == common.RoleAlumni {
		var err error
		domainID, err = s.repository.AlumniDomainID(ctx, *access.AlumniID)
		if err != nil {
			return nil, err
		}
		authorAlumniID = *access.AlumniID
	} else {
		if req.DataDomainID == nil || !access.CanAccessDomain(*req.DataDomainID) {
			return nil, common.ErrPermissionDenied
		}
		domainID = *req.DataDomainID
	}
	// A contribution that changes an existing entry must stay in that entry's
	// established domain.  The entry itself has no domain column, so its domain
	// is derived from the contribution that produced its latest version.
	if req.EntryID != nil {
		entryDomainID, err := s.repository.EntryDataDomainID(ctx, *req.EntryID)
		if err != nil {
			return nil, err
		}
		if entryDomainID == nil || *entryDomainID != domainID {
			return nil, common.ErrPermissionDenied
		}
	}
	item, err := s.repository.CreateContribution(ctx, &model.HistoryContribution{
		EntryID: req.EntryID, Title: strings.TrimSpace(req.Title), SectionName: strings.TrimSpace(req.SectionName),
		Content: strings.TrimSpace(req.Content), SourceNote: strings.TrimSpace(req.SourceNote), ChangeNote: strings.TrimSpace(req.ChangeNote),
		Status: repository.HistoryContributionDraft, DataDomainID: &domainID, AuthorUserID: access.UserID, AuthorAlumniID: authorAlumniID,
	})
	if err != nil {
		return nil, err
	}
	result := historyContributionItem(item)
	return &result, nil
}

func (s *HistoryService) GetContribution(ctx context.Context, access common.AccessContext, id uint64) (*dto.HistoryContributionItem, error) {
	item, err := s.repository.GetContribution(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canManageOwnContribution(access, item) {
		return nil, common.ErrPermissionDenied
	}
	result := historyContributionItem(item)
	return &result, nil
}

func (s *HistoryService) UpdateContribution(ctx context.Context, access common.AccessContext, id uint64, req dto.HistoryContributionRequest) (*dto.HistoryContributionItem, error) {
	if !access.IsAdministrator() && access.Role != common.RoleAlumni {
		return nil, common.ErrPermissionDenied
	}
	item, err := s.repository.GetContribution(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canManageOwnContribution(access, item) {
		return nil, common.ErrPermissionDenied
	}
	if !isEditableContributionStatus(item.Status) {
		return nil, common.ErrInvalidHistoryState
	}
	item, err = s.repository.UpdateContribution(ctx, id, access.UserID, req)
	if err != nil {
		return nil, err
	}
	result := historyContributionItem(item)
	return &result, nil
}

func (s *HistoryService) DeleteDraft(ctx context.Context, access common.AccessContext, id uint64) error {
	if !access.IsAdministrator() && access.Role != common.RoleAlumni {
		return common.ErrPermissionDenied
	}
	item, err := s.repository.GetContribution(ctx, id)
	if err != nil {
		return err
	}
	if !canManageOwnContribution(access, item) {
		return common.ErrPermissionDenied
	}
	if item.Status != repository.HistoryContributionDraft {
		return common.ErrInvalidHistoryState
	}
	return s.repository.DeleteDraft(ctx, id, access.UserID)
}

func (s *HistoryService) Submit(ctx context.Context, access common.AccessContext, id uint64) (*dto.HistoryContributionItem, error) {
	if !access.IsAdministrator() && access.Role != common.RoleAlumni {
		return nil, common.ErrPermissionDenied
	}
	item, err := s.repository.GetContribution(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canManageOwnContribution(access, item) {
		return nil, common.ErrPermissionDenied
	}
	item, err = s.repository.Submit(ctx, id, access.UserID)
	if err != nil {
		return nil, err
	}
	result := historyContributionItem(item)
	return &result, nil
}

func (s *HistoryService) ListMine(ctx context.Context, access common.AccessContext) ([]dto.HistoryContributionItem, error) {
	if !access.IsAdministrator() && access.Role != common.RoleAlumni {
		return nil, common.ErrPermissionDenied
	}
	items, err := s.repository.ListMine(ctx, access.UserID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.HistoryContributionItem, 0, len(items))
	for _, item := range items {
		result = append(result, historyContributionItem(item))
	}
	return result, nil
}

func (s *HistoryService) ListPending(ctx context.Context, access common.AccessContext) ([]dto.HistoryContributionItem, error) {
	if !access.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	items, err := s.repository.ListPending(ctx, access.DomainIDs, access.IsSuperAdmin())
	if err != nil {
		return nil, err
	}
	result := make([]dto.HistoryContributionItem, 0, len(items))
	for _, item := range items {
		result = append(result, historyContributionItem(item))
	}
	return result, nil
}

func (s *HistoryService) ListAttachments(ctx context.Context, access common.AccessContext, contributionID uint64) ([]dto.HistoryAttachmentItem, error) {
	item, err := s.repository.GetContribution(ctx, contributionID)
	if err != nil {
		return nil, err
	}
	if item.AuthorUserID == access.UserID && (access.Role == common.RoleAlumni || access.IsAdministrator()) {
		// Authors may inspect the metadata of their own attachments before review.
	} else if !access.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	} else if item.DataDomainID == nil {
		if !access.IsSuperAdmin() {
			return nil, common.ErrPermissionDenied
		}
	} else if !access.CanAccessDomain(*item.DataDomainID) {
		return nil, common.ErrPermissionDenied
	}
	attachments, err := s.repository.ListAttachments(ctx, contributionID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.HistoryAttachmentItem, 0, len(attachments))
	for _, attachment := range attachments {
		result = append(result, dto.HistoryAttachmentItem{ID: attachment.ID, OriginalName: attachment.OriginalName, MimeType: attachment.MimeType, FileSize: attachment.FileSize, Description: attachment.Description, SourceNote: attachment.SourceNote, RightsNote: attachment.RightsNote, ConsentConfirmed: attachment.ConsentConfirmed})
	}
	return result, nil
}

func (s *HistoryService) Review(ctx context.Context, access common.AccessContext, id uint64, req dto.HistoryReviewRequest) (*dto.HistoryContributionItem, error) {
	if !access.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	if (req.Action == "return" || req.Action == "reject") && strings.TrimSpace(req.ReviewComment) == "" {
		return nil, common.ErrInvalidRequest
	}
	item, err := s.repository.GetContribution(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.DataDomainID == nil {
		if !access.IsSuperAdmin() {
			return nil, common.ErrPermissionDenied
		}
	} else if !access.CanAccessDomain(*item.DataDomainID) {
		return nil, common.ErrPermissionDenied
	}
	item, err = s.repository.Review(ctx, id, access.UserID, req.Action, strings.TrimSpace(req.ReviewComment))
	if err != nil {
		return nil, err
	}
	result := historyContributionItem(item)
	return &result, nil
}

func (s *HistoryService) RequestAttachmentUpload(ctx context.Context, access common.AccessContext, contributionID uint64, req dto.HistoryAttachmentUploadRequest) (*dto.HistoryAttachmentUploadResult, error) {
	if !access.IsAdministrator() && access.Role != common.RoleAlumni {
		return nil, common.ErrPermissionDenied
	}
	if s.storage == nil {
		return nil, common.ErrStorageUnavailable
	}
	contribution, err := s.repository.GetContribution(ctx, contributionID)
	if err != nil {
		return nil, err
	}
	if !canManageOwnContribution(access, contribution) {
		return nil, common.ErrPermissionDenied
	}
	if !isEditableContributionStatus(contribution.Status) {
		return nil, common.ErrInvalidHistoryState
	}
	if !allowedHistoryMime(req.MimeType) || strings.TrimSpace(req.OriginalName) == "" {
		return nil, common.ErrFileTypeNotAllowed
	}
	if strings.HasPrefix(strings.ToLower(req.MimeType), "image/") && !req.ConsentConfirmed {
		return nil, common.ErrInvalidRequest
	}
	count, err := s.repository.CountAttachments(ctx, contributionID)
	if err != nil {
		return nil, err
	}
	if count >= 6 {
		return nil, common.ErrFileTooLarge
	}
	objectKey := fmt.Sprintf("history/contributions/%d/%s%s", contributionID, uuid.NewString(), filepath.Ext(req.OriginalName))
	attachment, err := s.repository.CreateAttachment(ctx, &model.HistoryAttachment{ContributionID: contributionID, ObjectKey: objectKey, OriginalName: strings.TrimSpace(req.OriginalName), MimeType: strings.ToLower(req.MimeType), Description: strings.TrimSpace(req.Description), SourceNote: strings.TrimSpace(req.SourceNote), RightsNote: strings.TrimSpace(req.RightsNote), ConsentConfirmed: req.ConsentConfirmed, Status: "pending"})
	if err != nil {
		return nil, err
	}
	uploadURL, err := s.storage.PresignedPutURL(ctx, objectKey, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	return &dto.HistoryAttachmentUploadResult{ID: attachment.ID, UploadURL: uploadURL, ExpiresIn: 600}, nil
}

func (s *HistoryService) ConfirmAttachmentUpload(ctx context.Context, access common.AccessContext, contributionID, attachmentID uint64) error {
	if !access.IsAdministrator() && access.Role != common.RoleAlumni {
		return common.ErrPermissionDenied
	}
	attachment, contribution, err := s.attachmentForContribution(ctx, contributionID, attachmentID)
	if err != nil {
		return err
	}
	if !canManageOwnContribution(access, contribution) {
		return common.ErrPermissionDenied
	}
	if !isEditableContributionStatus(contribution.Status) {
		return common.ErrInvalidHistoryState
	}
	if s.storage == nil {
		return common.ErrStorageUnavailable
	}
	info, err := s.storage.StatObject(ctx, attachment.ObjectKey)
	if err != nil {
		return common.ErrStorageUnavailable
	}
	if info.Size <= 0 {
		return common.ErrInvalidRequest
	}
	if info.Size > 10<<20 {
		return common.ErrFileTooLarge
	}
	return s.repository.ConfirmAttachment(ctx, attachmentID, uint64(info.Size))
}

func (s *HistoryService) AttachmentDownloadURL(ctx context.Context, access common.AccessContext, contributionID, attachmentID uint64) (*dto.HistoryAttachmentDownloadResult, error) {
	attachment, contribution, err := s.attachmentForContribution(ctx, contributionID, attachmentID)
	if err != nil {
		return nil, err
	}
	allowed := access.Role == common.RoleAlumni && contribution.AuthorUserID == access.UserID
	if access.IsAdministrator() && contribution.DataDomainID != nil && access.CanAccessDomain(*contribution.DataDomainID) {
		allowed = true
	}
	if contribution.Status == repository.HistoryContributionApproved && attachment.Status == "approved" {
		allowed = true
	}
	if !allowed {
		return nil, common.ErrPermissionDenied
	}
	if s.storage == nil {
		return nil, common.ErrStorageUnavailable
	}
	downloadURL, err := s.storage.PresignedGetURL(ctx, attachment.ObjectKey, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	return &dto.HistoryAttachmentDownloadResult{DownloadURL: downloadURL, ExpiresIn: 600}, nil
}

func (s *HistoryService) attachmentForContribution(ctx context.Context, contributionID, attachmentID uint64) (*model.HistoryAttachment, *model.HistoryContribution, error) {
	attachment, err := s.repository.GetAttachment(ctx, attachmentID)
	if err != nil {
		return nil, nil, err
	}
	if attachment.ContributionID != contributionID {
		return nil, nil, common.ErrHistoryAttachmentNotFound
	}
	contribution, err := s.repository.GetContribution(ctx, contributionID)
	if err != nil {
		return nil, nil, err
	}
	return attachment, contribution, nil
}

func allowedHistoryMime(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/png", "image/webp", "application/pdf":
		return true
	default:
		return false
	}
}

func isEditableContributionStatus(status string) bool {
	return status == repository.HistoryContributionDraft ||
		status == repository.HistoryContributionPending ||
		status == repository.HistoryContributionReturned
}

func canManageOwnContribution(access common.AccessContext, contribution *model.HistoryContribution) bool {
	if contribution == nil || contribution.AuthorUserID != access.UserID {
		return false
	}
	if access.Role == common.RoleAlumni {
		return true
	}
	return access.IsAdministrator() && contribution.DataDomainID != nil && access.CanAccessDomain(*contribution.DataDomainID)
}

func canViewContribution(access common.AccessContext, contribution *model.HistoryContribution) bool {
	if contribution == nil {
		return false
	}
	if access.Role == common.RoleAlumni && contribution.AuthorUserID == access.UserID {
		return true
	}
	if !access.IsAdministrator() {
		return false
	}
	if contribution.DataDomainID == nil {
		return access.IsSuperAdmin()
	}
	return access.CanAccessDomain(*contribution.DataDomainID)
}

func historyEntryItem(entry *model.HistoryEntry, sourceNote string) dto.HistoryEntryItem {
	return dto.HistoryEntryItem{ID: entry.ID, Title: entry.Title, Summary: entry.Summary, Content: entry.Content, SourceNote: sourceNote, CurrentVersion: uint(entry.CurrentVersion), UpdatedAt: entry.UpdatedAt}
}

func historyContributionItem(item *model.HistoryContribution) dto.HistoryContributionItem {
	return dto.HistoryContributionItem{ID: item.ID, EntryID: item.EntryID, Title: item.Title, SectionName: item.SectionName, Content: item.Content, SourceNote: item.SourceNote, ChangeNote: item.ChangeNote, Status: item.Status, DataDomainID: item.DataDomainID, ReviewComment: item.ReviewComment, SubmittedAt: item.SubmittedAt, ReviewedAt: item.ReviewedAt, UpdatedAt: item.UpdatedAt}
}

func IsHistoryNotFound(err error) bool {
	return errors.Is(err, common.ErrHistoryEntryNotFound) || errors.Is(err, common.ErrHistoryContributionNotFound)
}
