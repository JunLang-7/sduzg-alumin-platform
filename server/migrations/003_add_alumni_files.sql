USE sdu_alumni_db;

CREATE TABLE IF NOT EXISTS alumni_files (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  alumni_id BIGINT UNSIGNED NOT NULL COMMENT '校友档案 ID',
  file_type VARCHAR(32) NOT NULL COMMENT 'degree_archive / academic_record',
  object_key VARCHAR(512) NOT NULL COMMENT 'MinIO 对象路径',
  original_name VARCHAR(255) NOT NULL COMMENT '原始文件名',
  file_size BIGINT UNSIGNED NOT NULL COMMENT '文件大小（字节）',
  mime_type VARCHAR(128) NOT NULL COMMENT 'MIME 类型',
  uploaded_by BIGINT UNSIGNED NULL COMMENT '上传者用户 ID',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active / deleted',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  INDEX idx_af_alumni_id (alumni_id),
  INDEX idx_af_file_type (file_type),
  INDEX idx_af_alumni_type (alumni_id, file_type),
  INDEX idx_af_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='校友档案文件';
