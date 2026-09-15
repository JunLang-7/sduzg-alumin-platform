package model

import (
	"time"

	"gorm.io/gorm"
)

type HistoryEntry struct {
	ID             uint64 `gorm:"primaryKey"`
	Title          string
	Summary        string
	Content        string
	Status         string
	CurrentVersion uint
	CreatedBy      *uint64
	UpdatedBy      *uint64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (HistoryEntry) TableName() string { return "history_entries" }

type HistoryEntryVersion struct {
	ID             uint64 `gorm:"primaryKey"`
	EntryID        uint64
	VersionNumber  uint
	Title          string
	Summary        string
	Content        string
	SourceNote     string
	ContributionID *uint64
	ApprovedBy     uint64
	CreatedAt      time.Time
}

func (HistoryEntryVersion) TableName() string { return "history_entry_versions" }

type HistoryContribution struct {
	ID             uint64 `gorm:"primaryKey"`
	EntryID        *uint64
	Title          string
	SectionName    string
	Content        string
	SourceNote     string
	ChangeNote     string
	Status         string
	DataDomainID   *uint64
	AuthorUserID   uint64
	AuthorAlumniID uint64
	ReviewedBy     *uint64
	ReviewComment  *string
	SubmittedAt    *time.Time
	ReviewedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (HistoryContribution) TableName() string { return "history_contributions" }

type HistoryAttachment struct {
	ID               uint64 `gorm:"primaryKey"`
	ContributionID   uint64
	ObjectKey        string
	OriginalName     string
	MimeType         string
	FileSize         uint64
	Description      string
	SourceNote       string
	RightsNote       string
	ConsentConfirmed bool
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`
}

func (HistoryAttachment) TableName() string { return "history_attachments" }
