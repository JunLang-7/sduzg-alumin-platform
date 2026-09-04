-- Phase 4: Add alumni data domains and administrator access-control mappings.

CREATE TABLE IF NOT EXISTS data_domains (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  code VARCHAR(64) NOT NULL COMMENT 'stable domain code',
  name VARCHAR(100) NOT NULL COMMENT 'display name',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/disabled',
  sort_order INT NOT NULL DEFAULT 0 COMMENT 'display order',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_data_domains_code (code),
  INDEX idx_data_domains_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='alumni data domains';

INSERT INTO data_domains (code, name, status, sort_order)
VALUES
  ('undergraduate', '本科生', 'active', 10),
  ('academic_graduate', '学术学位研究生', 'active', 20),
  ('mpa', 'MPA专业学位研究生', 'active', 30)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  status = VALUES(status),
  sort_order = VALUES(sort_order);

ALTER TABLE alumni_profiles
  ADD COLUMN data_domain_id BIGINT UNSIGNED NULL COMMENT 'alumni data domain id' AFTER id;

-- The platform has historically served the MPA pilot. Backfill existing records before
-- making the new field mandatory so upgrade deployments retain access to all records.
UPDATE alumni_profiles
SET data_domain_id = (
  SELECT id FROM data_domains WHERE code = 'mpa'
)
WHERE data_domain_id IS NULL;

ALTER TABLE alumni_profiles
  MODIFY COLUMN data_domain_id BIGINT UNSIGNED NOT NULL COMMENT 'alumni data domain id',
  ADD INDEX idx_alumni_data_domain_id (data_domain_id),
  ADD CONSTRAINT fk_alumni_profiles_data_domain
    FOREIGN KEY (data_domain_id) REFERENCES data_domains(id);

CREATE TABLE IF NOT EXISTS admin_data_scopes (
  user_id BIGINT UNSIGNED NOT NULL COMMENT 'administrator user id',
  data_domain_id BIGINT UNSIGNED NOT NULL COMMENT 'allowed alumni data domain id',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, data_domain_id),
  CONSTRAINT fk_admin_data_scopes_user
    FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_admin_data_scopes_data_domain
    FOREIGN KEY (data_domain_id) REFERENCES data_domains(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='administrator alumni data scopes';

CREATE TABLE IF NOT EXISTS admin_permissions (
  user_id BIGINT UNSIGNED NOT NULL COMMENT 'administrator user id',
  permission_code VARCHAR(100) NOT NULL COMMENT 'permission code',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, permission_code),
  CONSTRAINT fk_admin_permissions_user
    FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='administrator permission codes';

-- Existing non-super administrators remain responsible for the historic MPA data.
INSERT IGNORE INTO admin_data_scopes (user_id, data_domain_id)
SELECT u.id, d.id
FROM users AS u
JOIN data_domains AS d ON d.code = 'mpa'
WHERE u.role = 'admin'
  AND u.status = 'active'
  AND u.deleted_at IS NULL;
