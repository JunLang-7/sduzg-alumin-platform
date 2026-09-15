package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	HistoryContributionDraft    = "draft"
	HistoryContributionPending  = "pending"
	HistoryContributionReturned = "returned"
	HistoryContributionApproved = "approved"
	HistoryContributionRejected = "rejected"
)

type HistoryRepository struct{ db *gorm.DB }

func NewHistoryRepository(db *gorm.DB) *HistoryRepository { return &HistoryRepository{db: db} }

func (r *HistoryRepository) ListPublished(ctx context.Context, keyword string) ([]*model.HistoryEntry, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	db := r.db.WithContext(ctx).Where("status = ?", "published").Order("updated_at DESC")
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("title LIKE ? OR summary LIKE ? OR content LIKE ?", like, like, like)
	}
	var entries []*model.HistoryEntry
	if err := db.Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *HistoryRepository) GetPublished(ctx context.Context, id uint64) (*model.HistoryEntry, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	var entry model.HistoryEntry
	if err := r.db.WithContext(ctx).Where("id = ? AND status = ?", id, "published").First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrHistoryEntryNotFound
		}
		return nil, err
	}
	return &entry, nil
}

func (r *HistoryRepository) AlumniDomainID(ctx context.Context, alumniID uint64) (uint64, error) {
	if r == nil || r.db == nil {
		return 0, common.ErrDatabaseUnavailable
	}
	var profile model.AlumniProfile
	if err := r.db.WithContext(ctx).Select("data_domain_id").First(&profile, alumniID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, common.ErrAlumniNotFound
		}
		return 0, err
	}
	return profile.DataDomainID, nil
}

func (r *HistoryRepository) CreateContribution(ctx context.Context, item *model.HistoryContribution) (*model.HistoryContribution, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

func (r *HistoryRepository) GetContribution(ctx context.Context, id uint64) (*model.HistoryContribution, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	var item model.HistoryContribution
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrHistoryContributionNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *HistoryRepository) ListMine(ctx context.Context, userID uint64) ([]*model.HistoryContribution, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	var items []*model.HistoryContribution
	if err := r.db.WithContext(ctx).Where("author_user_id = ?", userID).Order("updated_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *HistoryRepository) ListPending(ctx context.Context, domainIDs []uint64, allDomains bool) ([]*model.HistoryContribution, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	db := r.db.WithContext(ctx).Where("status = ?", HistoryContributionPending).Order("submitted_at ASC")
	if !allDomains {
		if len(domainIDs) == 0 {
			return []*model.HistoryContribution{}, nil
		}
		db = db.Where("data_domain_id IN ?", domainIDs)
	}
	var items []*model.HistoryContribution
	if err := db.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *HistoryRepository) Submit(ctx context.Context, id, userID uint64) (*model.HistoryContribution, error) {
	item, err := r.GetContribution(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.AuthorUserID != userID {
		return nil, common.ErrPermissionDenied
	}
	if item.Status != HistoryContributionDraft && item.Status != HistoryContributionReturned {
		return nil, common.ErrInvalidHistoryState
	}
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&model.HistoryContribution{}).Where("id = ?", id).Updates(map[string]any{"status": HistoryContributionPending, "submitted_at": now, "review_comment": nil}).Error; err != nil {
		return nil, err
	}
	return r.GetContribution(ctx, id)
}

func (r *HistoryRepository) Review(ctx context.Context, id, reviewerID uint64, action, comment string) (*model.HistoryContribution, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	var result *model.HistoryContribution
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.HistoryContribution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrHistoryContributionNotFound
			}
			return err
		}
		if item.Status != HistoryContributionPending {
			return common.ErrInvalidHistoryState
		}
		now := time.Now()
		status := HistoryContributionRejected
		if action == "return" {
			status = HistoryContributionReturned
		}
		if action == "approve" {
			status = HistoryContributionApproved
			entryID := item.EntryID
			if entryID == nil {
				entry := &model.HistoryEntry{Title: item.Title, Summary: item.ChangeNote, Content: item.Content, Status: "published", CurrentVersion: 1, CreatedBy: &item.AuthorUserID, UpdatedBy: &reviewerID}
				if err := tx.Create(entry).Error; err != nil {
					return err
				}
				entryID = &entry.ID
			} else {
				var entry model.HistoryEntry
				if err := tx.First(&entry, *entryID).Error; err != nil {
					return err
				}
				entry.Title, entry.Summary, entry.Content = item.Title, item.ChangeNote, item.Content
				entry.CurrentVersion++
				entry.UpdatedBy = &reviewerID
				if err := tx.Save(&entry).Error; err != nil {
					return err
				}
			}
			var entry model.HistoryEntry
			if err := tx.First(&entry, *entryID).Error; err != nil {
				return err
			}
			version := &model.HistoryEntryVersion{EntryID: entry.ID, VersionNumber: entry.CurrentVersion, Title: entry.Title, Summary: entry.Summary, Content: entry.Content, SourceNote: item.SourceNote, ContributionID: &item.ID, ApprovedBy: reviewerID}
			if err := tx.Create(version).Error; err != nil {
				return err
			}
			item.EntryID = entryID
			if err := tx.Model(&model.HistoryAttachment{}).Where("contribution_id = ? AND status = ?", item.ID, "pending").Update("status", "approved").Error; err != nil {
				return err
			}
		} else if action == "reject" {
			if err := tx.Model(&model.HistoryAttachment{}).Where("contribution_id = ? AND status = ?", item.ID, "pending").Update("status", "rejected").Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.HistoryContribution{}).Where("id = ?", id).Updates(map[string]any{"entry_id": item.EntryID, "status": status, "reviewed_by": reviewerID, "review_comment": comment, "reviewed_at": now}).Error; err != nil {
			return err
		}
		result = &item
		result.Status, result.ReviewedBy, result.ReviewedAt = status, &reviewerID, &now
		result.ReviewComment = &comment
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *HistoryRepository) CountAttachments(ctx context.Context, contributionID uint64) (int64, error) {
	if r == nil || r.db == nil {
		return 0, common.ErrDatabaseUnavailable
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.HistoryAttachment{}).Where("contribution_id = ?", contributionID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *HistoryRepository) CreateAttachment(ctx context.Context, item *model.HistoryAttachment) (*model.HistoryAttachment, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

func (r *HistoryRepository) GetAttachment(ctx context.Context, id uint64) (*model.HistoryAttachment, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	var item model.HistoryAttachment
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrHistoryAttachmentNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *HistoryRepository) ConfirmAttachment(ctx context.Context, id, fileSize uint64) error {
	if r == nil || r.db == nil {
		return common.ErrDatabaseUnavailable
	}
	return r.db.WithContext(ctx).Model(&model.HistoryAttachment{}).Where("id = ?", id).Update("file_size", fileSize).Error
}
