SET NAMES utf8mb4;

USE sdu_alumni_db;

CREATE TABLE IF NOT EXISTS history_entries (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  title VARCHAR(200) NOT NULL,
  summary VARCHAR(1000) NOT NULL DEFAULT '',
  content MEDIUMTEXT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'published' COMMENT 'published / archived',
  current_version INT UNSIGNED NOT NULL DEFAULT 1,
  created_by BIGINT UNSIGNED NULL,
  updated_by BIGINT UNSIGNED NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  UNIQUE KEY uk_history_entries_title (title),
  INDEX idx_history_entries_status_updated (status, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='院史正式词条';

CREATE TABLE IF NOT EXISTS history_entry_versions (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  entry_id BIGINT UNSIGNED NOT NULL,
  version_number INT UNSIGNED NOT NULL,
  title VARCHAR(200) NOT NULL,
  summary VARCHAR(1000) NOT NULL DEFAULT '',
  content MEDIUMTEXT NOT NULL,
  source_note TEXT NOT NULL,
  contribution_id BIGINT UNSIGNED NULL,
  approved_by BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_history_entry_version (entry_id, version_number),
  INDEX idx_history_versions_entry_created (entry_id, created_at),
  CONSTRAINT fk_history_versions_entry FOREIGN KEY (entry_id) REFERENCES history_entries(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='院史词条正式版本';

CREATE TABLE IF NOT EXISTS history_contributions (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  entry_id BIGINT UNSIGNED NULL COMMENT '为空时表示新词条投稿',
  title VARCHAR(200) NOT NULL,
  section_name VARCHAR(100) NOT NULL DEFAULT '',
  content MEDIUMTEXT NOT NULL,
  source_note TEXT NOT NULL,
  change_note VARCHAR(1000) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'draft' COMMENT 'draft / pending / returned / approved / rejected',
  data_domain_id BIGINT UNSIGNED NULL COMMENT '由服务端根据投稿校友档案写入',
  author_user_id BIGINT UNSIGNED NOT NULL,
  author_alumni_id BIGINT UNSIGNED NOT NULL,
  reviewed_by BIGINT UNSIGNED NULL,
  review_comment TEXT NULL,
  submitted_at DATETIME NULL,
  reviewed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  INDEX idx_history_contrib_author_status (author_user_id, status, updated_at),
  INDEX idx_history_contrib_domain_status (data_domain_id, status, submitted_at),
  INDEX idx_history_contrib_entry (entry_id),
  CONSTRAINT fk_history_contrib_entry FOREIGN KEY (entry_id) REFERENCES history_entries(id),
  CONSTRAINT fk_history_contrib_domain FOREIGN KEY (data_domain_id) REFERENCES data_domains(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='院史校友投稿';

CREATE TABLE IF NOT EXISTS history_attachments (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  contribution_id BIGINT UNSIGNED NOT NULL,
  object_key VARCHAR(512) NOT NULL,
  original_name VARCHAR(255) NOT NULL,
  mime_type VARCHAR(128) NOT NULL,
  file_size BIGINT UNSIGNED NOT NULL DEFAULT 0,
  description VARCHAR(1000) NOT NULL,
  source_note TEXT NOT NULL,
  rights_note TEXT NOT NULL,
  consent_confirmed TINYINT(1) NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending / approved / rejected',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  INDEX idx_history_attachments_contribution (contribution_id, status),
  CONSTRAINT fk_history_attachments_contribution FOREIGN KEY (contribution_id) REFERENCES history_contributions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='院史投稿附件';
