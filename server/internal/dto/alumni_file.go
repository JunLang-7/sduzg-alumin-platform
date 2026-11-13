package dto

import "time"

// AlumniFileUploadResponse 文件上传成功后的响应。
type AlumniFileUploadResponse struct {
	ID           uint64 `json:"id"`
	AlumniID     uint64 `json:"alumni_id"`
	EnrollmentID uint64 `json:"enrollment_id"`
	FileType     string `json:"file_type"`
	OriginalName string `json:"original_name"`
	FileSize     uint64 `json:"file_size"`
	MimeType     string `json:"mime_type"`
	Status       string `json:"status"`
}

// AlumniFileItem 文件列表中的单条记录。
type AlumniFileItem struct {
	ID           uint64    `json:"id"`
	EnrollmentID uint64    `json:"enrollment_id"`
	FileType     string    `json:"file_type"`
	OriginalName string    `json:"original_name"`
	FileSize     uint64    `json:"file_size"`
	MimeType     string    `json:"mime_type"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// AlumniEnrollmentFileGroup 是单段学籍的两类档案文件。
type AlumniEnrollmentFileGroup struct {
	EnrollmentID   uint64           `json:"enrollment_id"`
	DegreeArchive  []AlumniFileItem `json:"degree_archive"`
	AcademicRecord []AlumniFileItem `json:"academic_record"`
}

// AlumniFileListResponse 按学籍段和类型分组后的文件列表，只包含可访问学籍。
type AlumniFileListResponse struct {
	AlumniID    uint64                      `json:"alumni_id"`
	Enrollments []AlumniEnrollmentFileGroup `json:"enrollments"`
}

// AlumniFileUploadURLResponse 预签名上传 URL 响应，客户端用该 URL 直传 MinIO。
type AlumniFileUploadURLResponse struct {
	FileID       uint64 `json:"file_id"`
	EnrollmentID uint64 `json:"enrollment_id"`
	UploadURL    string `json:"upload_url"`
	ExpiresIn    int    `json:"expires_in"`
	ObjectKey    string `json:"-"`
}

// AlumniFileDownloadURLResponse 预签名下载 URL 响应，客户端用该 URL 直连 MinIO 下载。
type AlumniFileDownloadURLResponse struct {
	DownloadURL  string `json:"download_url"`
	ExpiresIn    int    `json:"expires_in"`
	OriginalName string `json:"original_name"`
}

// AlumniFileUploadURLRequest 是申请预签名上传 URL 的请求体。
type AlumniFileUploadURLRequest struct {
	FileType     string `json:"file_type" binding:"required"`
	OriginalName string `json:"original_name" binding:"required"`
	MimeType     string `json:"mime_type" binding:"required"`
}
