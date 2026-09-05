-- 第四阶段：新增校友数据域与管理员授权映射。

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS data_domains (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  code VARCHAR(64) NOT NULL COMMENT '稳定数据域编码',
  name VARCHAR(100) NOT NULL COMMENT '展示名称',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/disabled',
  sort_order INT NOT NULL DEFAULT 0 COMMENT '展示顺序',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_data_domains_code (code),
  INDEX idx_data_domains_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='校友数据域';

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
  ADD COLUMN data_domain_id BIGINT UNSIGNED NULL COMMENT '校友数据域 ID' AFTER id;

-- 平台历史数据属于 MPA 试点；先完成回填，再收紧为必填字段，保证升级后仍可访问。
UPDATE alumni_profiles
SET data_domain_id = (
  SELECT id FROM data_domains WHERE code = 'mpa'
)
WHERE data_domain_id IS NULL;

ALTER TABLE alumni_profiles
  MODIFY COLUMN data_domain_id BIGINT UNSIGNED NOT NULL COMMENT '校友数据域 ID',
  ADD INDEX idx_alumni_data_domain_id (data_domain_id),
  ADD CONSTRAINT fk_alumni_profiles_data_domain
    FOREIGN KEY (data_domain_id) REFERENCES data_domains(id);

CREATE TABLE IF NOT EXISTS admin_data_scopes (
  user_id BIGINT UNSIGNED NOT NULL COMMENT '管理员用户 ID',
  data_domain_id BIGINT UNSIGNED NOT NULL COMMENT '允许访问的校友数据域 ID',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, data_domain_id),
  CONSTRAINT fk_admin_data_scopes_user
    FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_admin_data_scopes_data_domain
    FOREIGN KEY (data_domain_id) REFERENCES data_domains(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员校友数据范围';

CREATE TABLE IF NOT EXISTS admin_permissions (
  user_id BIGINT UNSIGNED NOT NULL COMMENT '管理员用户 ID',
  permission_code VARCHAR(100) NOT NULL COMMENT '权限编码',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, permission_code),
  CONSTRAINT fk_admin_permissions_user
    FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员权限编码';

-- 为历史普通管理员补充分配 MPA 数据域，确保升级后仍可访问原有数据。
INSERT IGNORE INTO admin_data_scopes (user_id, data_domain_id)
SELECT u.id, d.id
FROM users AS u
JOIN data_domains AS d ON d.code = 'mpa'
WHERE u.role = 'admin'
  AND u.status = 'active'
  AND u.deleted_at IS NULL;
