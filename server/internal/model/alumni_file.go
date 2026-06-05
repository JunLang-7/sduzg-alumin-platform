package model

import (
	"time"

	"gorm.io/gorm"
)

const TableNameAlumniFile = "alumni_files"

// AlumniFile 校友档案文件元数据
type AlumniFile struct {
	ID           uint64         `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement:true" json:"id"`
	AlumniID     uint64         `gorm:"column:alumni_id;type:bigint unsigned;not null;index:idx_af_alumni_id,priority:1;index:idx_af_alumni_type,priority:1" json:"alumni_id"`
	FileType     string         `gorm:"column:file_type;type:varchar(32);not null;index:idx_af_file_type,priority:1;index:idx_af_alumni_type,priority:2" json:"file_type"`
	ObjectKey    string         `gorm:"column:object_key;type:varchar(512);not null" json:"object_key"`
	OriginalName string         `gorm:"column:original_name;type:varchar(255);not null" json:"original_name"`
	FileSize     uint64         `gorm:"column:file_size;type:bigint unsigned;not null" json:"file_size"`
	MimeType     string         `gorm:"column:mime_type;type:varchar(128);not null" json:"mime_type"`
	UploadedBy   *uint64        `gorm:"column:uploaded_by;type:bigint unsigned" json:"uploaded_by"`
	Status       string         `gorm:"column:status;type:varchar(32);not null;default:active;index:idx_af_status,priority:1" json:"status"`
	CreatedAt    time.Time      `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;type:datetime" json:"deleted_at"`
}

func (*AlumniFile) TableName() string {
	return TableNameAlumniFile
}
