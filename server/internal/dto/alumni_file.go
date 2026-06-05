package dto

import "time"

// AlumniFileUploadResponse 文件上传成功后的响应。
type AlumniFileUploadResponse struct {
	ID           uint64 `json:"id"`
	AlumniID     uint64 `json:"alumni_id"`
	FileType     string `json:"file_type"`
	OriginalName string `json:"original_name"`
	FileSize     uint64 `json:"file_size"`
	MimeType     string `json:"mime_type"`
}

// AlumniFileItem 文件列表中的单条记录。
type AlumniFileItem struct {
	ID           uint64    `json:"id"`
	FileType     string    `json:"file_type"`
	OriginalName string    `json:"original_name"`
	FileSize     uint64    `json:"file_size"`
	MimeType     string    `json:"mime_type"`
	CreatedAt    time.Time `json:"created_at"`
}

// AlumniFileListResponse 按类型分组后的文件列表。
type AlumniFileListResponse struct {
	AlumniID       uint64           `json:"alumni_id"`
	DegreeArchive  []AlumniFileItem `json:"degree_archive"`
	AcademicRecord []AlumniFileItem `json:"academic_record"`
}
