package repository

import (
	"context"
	"errors"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func deleteHistoryEntry(tx *gorm.DB, entry *model.HistoryEntry, actorID uint64, comment string) error {
	if err := tx.Model(entry).Updates(map[string]any{"updated_by": actorID, "status": "archived"}).Error; err != nil {
		return err
	}
	if err := tx.Delete(entry).Error; err != nil {
		return err
	}
	return tx.Model(&model.HistoryEntryDeletionRequest{}).Where("entry_id = ? AND status = ?", entry.ID, "pending").Updates(map[string]any{
		"status": "approved", "pending_entry_id": nil, "reviewed_by": actorID,
		"review_comment": comment, "reviewed_at": time.Now(),
	}).Error
}

func (r *HistoryRepository) CreateDeletionRequest(ctx context.Context, access common.AccessContext, entryID uint64, reason string) (*model.HistoryEntryDeletionRequest, error) {
	if access.UserID == 0 || !access.IsAdministrator() {
		return nil, common.ErrPermissionDenied
	}
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	var result model.HistoryEntryDeletionRequest
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entry model.HistoryEntry
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", entryID, "published").First(&entry).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrHistoryEntryNotFound
			}
			return err
		}
		domainID, err := NewHistoryRepository(tx).EntryDataDomainID(ctx, entryID)
		if err != nil {
			return err
		}
		if !access.IsSuperAdmin() && (domainID == nil || !access.CanAccessDomain(*domainID)) {
			return common.ErrPermissionDenied
		}
		var pending int64
		if err := tx.Model(&model.HistoryEntryDeletionRequest{}).Where("pending_entry_id = ?", entryID).Count(&pending).Error; err != nil {
			return err
		}
		if pending != 0 {
			return common.ErrInvalidHistoryState
		}
		result = model.HistoryEntryDeletionRequest{EntryID: entryID, PendingEntryID: &entryID, Title: entry.Title, RequestedBy: access.UserID, DataDomainID: domainID, Reason: reason, Status: "pending"}
		return tx.Create(&result).Error
	})
	return &result, err
}

func (r *HistoryRepository) ListDeletionRequests(ctx context.Context, access common.AccessContext, mine bool) ([]model.HistoryEntryDeletionRequest, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	query := r.db.WithContext(ctx).Model(&model.HistoryEntryDeletionRequest{})
	if mine {
		query = query.Where("requested_by = ?", access.UserID)
	} else {
		query = query.Where("status = ?", "pending")
		if !access.IsSuperAdmin() {
			query = query.Where("requested_by <> ?", access.UserID)
			if len(access.DomainIDs) == 0 {
				return []model.HistoryEntryDeletionRequest{}, nil
			}
			query = query.Where("data_domain_id IN ?", access.DomainIDs)
		}
	}
	result := make([]model.HistoryEntryDeletionRequest, 0)
	err := query.Order("created_at DESC, id DESC").Find(&result).Error
	return result, err
}

func (r *HistoryRepository) ReviewDeletionRequest(ctx context.Context, access common.AccessContext, id uint64, action, comment string) (*model.HistoryEntryDeletionRequest, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	var result model.HistoryEntryDeletionRequest
	if err := r.db.WithContext(ctx).First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrHistoryEntryNotFound
		}
		return nil, err
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Always lock entry before request, matching direct deletion lock order.
		var entry model.HistoryEntry
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", result.EntryID, "published").First(&entry).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrHistoryEntryNotFound
			}
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result, id).Error; err != nil {
			return err
		}
		if result.Status != "pending" {
			return common.ErrInvalidHistoryState
		}
		if !access.IsAdministrator() || access.UserID == 0 {
			return common.ErrPermissionDenied
		}
		if !access.IsSuperAdmin() {
			if result.RequestedBy == access.UserID {
				return common.ErrPermissionDenied
			}
			domainID, err := NewHistoryRepository(tx).EntryDataDomainID(ctx, entry.ID)
			if err != nil {
				return err
			}
			if domainID == nil || !access.CanAccessDomain(*domainID) {
				return common.ErrPermissionDenied
			}
		}
		if action == "approve" {
			if err := deleteHistoryEntry(tx, &entry, access.UserID, comment); err != nil {
				return err
			}
		} else {
			if err := tx.Model(&result).Updates(map[string]any{"status": "rejected", "pending_entry_id": nil, "reviewed_by": access.UserID, "review_comment": comment, "reviewed_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		return tx.First(&result, id).Error
	})
	return &result, err
}
