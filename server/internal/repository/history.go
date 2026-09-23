package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/gen/field"
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
	qs := query.Use(r.db).HistoryEntry
	db := r.db.WithContext(ctx).Where(qs.Status.Eq("published")).Order(qs.UpdatedAt.Desc())
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where(field.Or(qs.Title.Like(like), qs.Summary.Like(like), qs.Content.Like(like)))
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
	qs := query.Use(r.db).HistoryEntry
	var entry model.HistoryEntry
	if err := r.db.WithContext(ctx).Where(qs.ID.Eq(id), qs.Status.Eq("published")).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrHistoryEntryNotFound
		}
		return nil, err
	}
	return &entry, nil
}

func (r *HistoryRepository) LatestSourceNote(ctx context.Context, entryID uint64) (string, error) {
	if r == nil || r.db == nil {
		return "", common.ErrDatabaseUnavailable
	}
	versions := query.Use(r.db).HistoryEntryVersion
	version, err := versions.WithContext(ctx).Where(versions.EntryID.Eq(entryID)).Order(versions.VersionNumber.Desc()).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return version.SourceNote, nil
}

func (r *HistoryRepository) EntryDataDomainID(ctx context.Context, entryID uint64) (*uint64, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	versions := query.Use(r.db).HistoryEntryVersion
	version, err := versions.WithContext(ctx).Where(versions.EntryID.Eq(entryID)).Order(versions.VersionNumber.Desc()).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if version.ContributionID == nil {
		return nil, nil
	}
	contribution, err := r.GetContribution(ctx, *version.ContributionID)
	if errors.Is(err, common.ErrHistoryContributionNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return contribution.DataDomainID, nil
}

func (r *HistoryRepository) UpdatePublished(ctx context.Context, id, editorID uint64, req dto.HistoryEntryUpdateRequest) (*model.HistoryEntry, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	var result model.HistoryEntry
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entries := query.Use(tx).HistoryEntry
		versions := query.Use(tx).HistoryEntryVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(entries.ID.Eq(id), entries.Status.Eq("published")).First(&result).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.ErrHistoryEntryNotFound
			}
			return err
		}
		latestVersion, err := versions.WithContext(ctx).Where(versions.EntryID.Eq(id)).Order(versions.VersionNumber.Desc()).First()
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var contributionID *uint64
		if err == nil {
			contributionID = latestVersion.ContributionID
		}
		result.Title = strings.TrimSpace(req.Title)
		result.Summary = strings.TrimSpace(req.ChangeNote)
		result.Content = strings.TrimSpace(req.Content)
		result.CurrentVersion++
		result.UpdatedBy = &editorID
		if err := tx.Save(&result).Error; err != nil {
			return err
		}
		return tx.Create(&model.HistoryEntryVersion{
			EntryID: result.ID, VersionNumber: result.CurrentVersion, Title: result.Title, Summary: result.Summary,
			Content: result.Content, SourceNote: strings.TrimSpace(req.SourceNote), ContributionID: contributionID, ApprovedBy: editorID,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *HistoryRepository) AlumniDomainID(ctx context.Context, alumniID uint64) (uint64, error) {
	if r == nil || r.db == nil {
		return 0, common.ErrDatabaseUnavailable
	}
	qs := query.Use(r.db).AlumniProfile
	profile, err := qs.WithContext(ctx).Select(qs.DataDomainID).Where(qs.ID.Eq(alumniID)).First()
	if err != nil {
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

func (r *HistoryRepository) UpdateContribution(ctx context.Context, id, userID uint64, req dto.HistoryContributionRequest) (*model.HistoryContribution, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	var result *model.HistoryContribution
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.HistoryContribution
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"id = ? AND author_user_id = ? AND status IN ?", id, userID,
			[]string{HistoryContributionDraft, HistoryContributionPending, HistoryContributionReturned},
		).First(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.ErrInvalidHistoryState
		}
		if err != nil {
			return err
		}
		updates := map[string]any{
			"title":       strings.TrimSpace(req.Title),
			"content":     strings.TrimSpace(req.Content),
			"source_note": strings.TrimSpace(req.SourceNote),
			"change_note": strings.TrimSpace(req.ChangeNote),
		}
		if sectionName := strings.TrimSpace(req.SectionName); sectionName != "" {
			updates["section_name"] = sectionName
		}
		if err := tx.Model(&model.HistoryContribution{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
			return err
		}
		item.Title = updates["title"].(string)
		item.Content = updates["content"].(string)
		item.SourceNote = updates["source_note"].(string)
		item.ChangeNote = updates["change_note"].(string)
		if sectionName, ok := updates["section_name"].(string); ok {
			item.SectionName = sectionName
		}
		result = &item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteDraft soft-deletes a draft and its attachments together. The status and
// author are checked again in the transaction so a concurrent submission cannot
// cause a pending contribution to be deleted.
func (r *HistoryRepository) DeleteDraft(ctx context.Context, id, userID uint64) error {
	if r == nil || r.db == nil {
		return common.ErrDatabaseUnavailable
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		contributions := query.Use(tx).HistoryContribution
		attachments := query.Use(tx).HistoryAttachment
		item, err := contributions.WithContext(ctx).
			Where(contributions.ID.Eq(id), contributions.AuthorUserID.Eq(userID), contributions.Status.Eq(HistoryContributionDraft)).
			First()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.ErrInvalidHistoryState
		}
		if err != nil {
			return err
		}
		if _, err := attachments.WithContext(ctx).Where(attachments.ContributionID.Eq(item.ID)).Delete(); err != nil {
			return err
		}
		_, err = contributions.WithContext(ctx).Where(contributions.ID.Eq(item.ID)).Delete()
		return err
	})
}

func (r *HistoryRepository) GetContribution(ctx context.Context, id uint64) (*model.HistoryContribution, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	qs := query.Use(r.db).HistoryContribution
	var item model.HistoryContribution
	if err := r.db.WithContext(ctx).Where(qs.ID.Eq(id)).First(&item).Error; err != nil {
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
	qs := query.Use(r.db).HistoryContribution
	var items []*model.HistoryContribution
	if err := r.db.WithContext(ctx).Where(qs.AuthorUserID.Eq(userID)).Order(qs.UpdatedAt.Desc()).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *HistoryRepository) ListPending(ctx context.Context, domainIDs []uint64, allDomains bool) ([]*model.HistoryContribution, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	qs := query.Use(r.db).HistoryContribution
	db := r.db.WithContext(ctx).Where(qs.Status.Eq(HistoryContributionPending)).Order(qs.SubmittedAt)
	if !allDomains {
		if len(domainIDs) == 0 {
			return []*model.HistoryContribution{}, nil
		}
		db = db.Where(qs.DataDomainID.In(domainIDs...))
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
	qs := query.Use(r.db).HistoryContribution
	if err := r.db.WithContext(ctx).Model(&model.HistoryContribution{}).Where(qs.ID.Eq(id)).Updates(map[string]any{"status": HistoryContributionPending, "submitted_at": now, "review_comment": nil}).Error; err != nil {
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
		contributions := query.Use(tx).HistoryContribution
		entries := query.Use(tx).HistoryEntry
		attachments := query.Use(tx).HistoryAttachment
		var item model.HistoryContribution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(contributions.ID.Eq(id)).First(&item).Error; err != nil {
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
				// Defend the domain boundary again at publication time.  The
				// service validates it when a draft is created, but the target
				// entry can only safely be trusted while this transaction holds
				// the relevant rows.
				var latestVersion model.HistoryEntryVersion
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("entry_id = ?", *entryID).Order("version_number DESC").First(&latestVersion).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return common.ErrPermissionDenied
					}
					return err
				}
				if latestVersion.ContributionID == nil || item.DataDomainID == nil {
					return common.ErrPermissionDenied
				}
				var latestContribution model.HistoryContribution
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", *latestVersion.ContributionID).First(&latestContribution).Error; err != nil {
					return err
				}
				if latestContribution.DataDomainID == nil || *latestContribution.DataDomainID != *item.DataDomainID {
					return common.ErrPermissionDenied
				}
				var entry model.HistoryEntry
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(entries.ID.Eq(*entryID)).First(&entry).Error; err != nil {
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
			if err := tx.Where(entries.ID.Eq(*entryID)).First(&entry).Error; err != nil {
				return err
			}
			version := &model.HistoryEntryVersion{EntryID: entry.ID, VersionNumber: entry.CurrentVersion, Title: entry.Title, Summary: entry.Summary, Content: entry.Content, SourceNote: item.SourceNote, ContributionID: &item.ID, ApprovedBy: reviewerID}
			if err := tx.Create(version).Error; err != nil {
				return err
			}
			item.EntryID = entryID
			if err := tx.Model(&model.HistoryAttachment{}).Where(attachments.ContributionID.Eq(item.ID), attachments.Status.Eq("pending")).Update("status", "approved").Error; err != nil {
				return err
			}
		} else if action == "reject" {
			if err := tx.Model(&model.HistoryAttachment{}).Where(attachments.ContributionID.Eq(item.ID), attachments.Status.Eq("pending")).Update("status", "rejected").Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.HistoryContribution{}).Where(contributions.ID.Eq(id)).Updates(map[string]any{"entry_id": item.EntryID, "status": status, "reviewed_by": reviewerID, "review_comment": comment, "reviewed_at": now}).Error; err != nil {
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
	qs := query.Use(r.db).HistoryAttachment
	if err := r.db.WithContext(ctx).Model(&model.HistoryAttachment{}).Where(qs.ContributionID.Eq(contributionID)).Count(&count).Error; err != nil {
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
	qs := query.Use(r.db).HistoryAttachment
	var item model.HistoryAttachment
	if err := r.db.WithContext(ctx).Where(qs.ID.Eq(id)).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrHistoryAttachmentNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *HistoryRepository) ListAttachments(ctx context.Context, contributionID uint64) ([]*model.HistoryAttachment, error) {
	if r == nil || r.db == nil {
		return nil, common.ErrDatabaseUnavailable
	}
	qs := query.Use(r.db).HistoryAttachment
	var items []*model.HistoryAttachment
	if err := r.db.WithContext(ctx).Where(qs.ContributionID.Eq(contributionID)).Order(qs.ID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *HistoryRepository) ConfirmAttachment(ctx context.Context, id, fileSize uint64) error {
	if r == nil || r.db == nil {
		return common.ErrDatabaseUnavailable
	}
	qs := query.Use(r.db).HistoryAttachment
	return r.db.WithContext(ctx).Model(&model.HistoryAttachment{}).Where(qs.ID.Eq(id)).Update("file_size", fileSize).Error
}
