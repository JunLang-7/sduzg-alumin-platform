package dto

import "time"

type HistoryEntryListRequest struct {
	Keyword string `form:"keyword"`
}

type HistoryEntryItem struct {
	ID             uint64    `json:"id"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	Content        string    `json:"content"`
	SourceNote     string    `json:"source_note"`
	CurrentVersion uint      `json:"current_version"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type HistoryEntryUpdateRequest struct {
	Title      string `json:"title" binding:"required,max=200"`
	Content    string `json:"content" binding:"required,max=20000"`
	SourceNote string `json:"source_note" binding:"required,max=5000"`
	ChangeNote string `json:"change_note" binding:"max=1000"`
}

type HistoryContributionRequest struct {
	EntryID     *uint64 `json:"entry_id"`
	Title       string  `json:"title" binding:"required,max=200"`
	SectionName string  `json:"section_name" binding:"max=100"`
	Content     string  `json:"content" binding:"required,max=20000"`
	SourceNote  string  `json:"source_note" binding:"required,max=5000"`
	ChangeNote  string  `json:"change_note" binding:"max=1000"`
}

type HistoryReviewRequest struct {
	Action        string `json:"action" binding:"required,oneof=approve return reject"`
	ReviewComment string `json:"review_comment" binding:"max=5000"`
}

type HistoryAttachmentUploadRequest struct {
	OriginalName     string `json:"original_name" binding:"required,max=255"`
	MimeType         string `json:"mime_type" binding:"required,max=128"`
	Description      string `json:"description" binding:"required,max=1000"`
	SourceNote       string `json:"source_note" binding:"required,max=5000"`
	RightsNote       string `json:"rights_note" binding:"required,max=5000"`
	ConsentConfirmed bool   `json:"consent_confirmed"`
}

type HistoryAttachmentUploadResult struct {
	ID        uint64 `json:"id"`
	UploadURL string `json:"upload_url"`
	ExpiresIn int    `json:"expires_in"`
}

type HistoryAttachmentDownloadResult struct {
	DownloadURL string `json:"download_url"`
	ExpiresIn   int    `json:"expires_in"`
}

type HistoryAttachmentItem struct {
	ID               uint64 `json:"id"`
	OriginalName     string `json:"original_name"`
	MimeType         string `json:"mime_type"`
	FileSize         uint64 `json:"file_size"`
	Description      string `json:"description"`
	SourceNote       string `json:"source_note"`
	RightsNote       string `json:"rights_note"`
	ConsentConfirmed bool   `json:"consent_confirmed"`
}

type HistoryContributionItem struct {
	ID            uint64     `json:"id"`
	EntryID       *uint64    `json:"entry_id,omitempty"`
	Title         string     `json:"title"`
	SectionName   string     `json:"section_name"`
	Content       string     `json:"content"`
	SourceNote    string     `json:"source_note"`
	ChangeNote    string     `json:"change_note"`
	Status        string     `json:"status"`
	DataDomainID  *uint64    `json:"data_domain_id,omitempty"`
	ReviewComment *string    `json:"review_comment,omitempty"`
	SubmittedAt   *time.Time `json:"submitted_at,omitempty"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
