SET NAMES utf8mb4;
-- migrate: requires history_entries
-- migrate: proves history_entry_deletion_requests
USE sdu_alumni_db;

CREATE TABLE IF NOT EXISTS history_entry_deletion_requests (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  entry_id BIGINT UNSIGNED NOT NULL,
  pending_entry_id BIGINT UNSIGNED NULL,
  title VARCHAR(200) NOT NULL,
  requested_by BIGINT UNSIGNED NOT NULL,
  data_domain_id BIGINT UNSIGNED NULL,
  reason VARCHAR(2000) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  reviewed_by BIGINT UNSIGNED NULL,
  review_comment VARCHAR(2000) NOT NULL DEFAULT '',
  reviewed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_history_deletion_pending_entry (pending_entry_id),
  INDEX idx_history_deletion_author (requested_by, created_at),
  INDEX idx_history_deletion_domain_status (data_domain_id, status, created_at),
  CONSTRAINT fk_history_deletion_entry FOREIGN KEY (entry_id) REFERENCES history_entries(id),
  CONSTRAINT chk_history_deletion_pending CHECK (
    (status = 'pending' AND pending_entry_id IS NOT NULL AND pending_entry_id = entry_id)
    OR (status IN ('approved', 'rejected') AND pending_entry_id IS NULL)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='院史词条删除申请及审核记录';
