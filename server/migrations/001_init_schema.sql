CREATE DATABASE IF NOT EXISTS sdu_alumni_db
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE sdu_alumni_db;

CREATE TABLE IF NOT EXISTS users (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  account VARCHAR(100) NOT NULL UNIQUE COMMENT 'login account',
  password_hash VARCHAR(255) NOT NULL COMMENT 'password hash',
  role VARCHAR(32) NOT NULL COMMENT 'alumni/admin/super_admin',
  alumni_id BIGINT UNSIGNED NULL COMMENT 'linked alumni profile id',
  real_name VARCHAR(100) NULL COMMENT 'real name',
  mobile VARCHAR(30) NULL COMMENT 'mobile or backup contact',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/disabled/deleted',
  last_login_at DATETIME NULL COMMENT 'last login time',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  INDEX idx_users_role (role),
  INDEX idx_users_alumni_id (alumni_id),
  INDEX idx_users_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='user accounts';

CREATE TABLE IF NOT EXISTS alumni_profiles (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(100) NOT NULL COMMENT 'name',
  grade VARCHAR(50) NOT NULL COMMENT 'grade',
  class_name VARCHAR(100) NULL COMMENT 'class',
  cohort VARCHAR(50) NULL COMMENT 'cohort',
  counselor VARCHAR(100) NULL COMMENT 'counselor',
  mentor VARCHAR(100) NULL COMMENT 'mentor',
  major VARCHAR(100) NULL COMMENT 'major',
  training_mode VARCHAR(50) NULL COMMENT 'training mode',
  industry VARCHAR(100) NULL COMMENT 'industry',
  work_unit VARCHAR(255) NULL COMMENT 'work unit',
  position VARCHAR(100) NULL COMMENT 'position',
  mailing_address VARCHAR(255) NULL COMMENT 'mailing address',
  gender VARCHAR(20) NULL COMMENT 'gender',
  mobile VARCHAR(30) NULL COMMENT 'mobile',
  remark TEXT NULL COMMENT 'admin remark',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/deleted',
  created_by BIGINT UNSIGNED NULL COMMENT 'creator user id',
  updated_by BIGINT UNSIGNED NULL COMMENT 'updater user id',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  INDEX idx_alumni_name (name),
  INDEX idx_alumni_grade (grade),
  INDEX idx_alumni_class_name (class_name),
  INDEX idx_alumni_cohort (cohort),
  INDEX idx_alumni_major (major),
  INDEX idx_alumni_training_mode (training_mode),
  INDEX idx_alumni_industry (industry),
  INDEX idx_alumni_mobile (mobile),
  INDEX idx_alumni_status (status),
  FULLTEXT KEY ft_alumni_search (name, work_unit, position, mentor, counselor)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='alumni profiles';

CREATE TABLE IF NOT EXISTS operation_logs (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  operator_id BIGINT UNSIGNED NOT NULL COMMENT 'operator user id',
  operator_role VARCHAR(32) NOT NULL COMMENT 'operator role',
  action VARCHAR(100) NOT NULL COMMENT 'action',
  target_type VARCHAR(100) NOT NULL COMMENT 'target type',
  target_id BIGINT UNSIGNED NULL COMMENT 'target id',
  detail JSON NULL COMMENT 'detail',
  ip_address VARCHAR(64) NULL COMMENT 'ip address',
  user_agent VARCHAR(512) NULL COMMENT 'user agent',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_logs_operator_id (operator_id),
  INDEX idx_logs_action (action),
  INDEX idx_logs_target (target_type, target_id),
  INDEX idx_logs_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='operation logs';
