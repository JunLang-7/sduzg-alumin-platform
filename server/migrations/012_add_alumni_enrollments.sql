SET NAMES utf8mb4;

-- 一人多段学籍：把原 alumni_profiles 上按学籍归属的字段拆到 alumni_enrollments。
--
-- 升级策略：
--   1. 每份旧档案先迁成「一份个人档案 + 一段学籍」，培养层次可判定的按数据域回填，
--      学术学位研究生无法区分硕士/博士，统一标记为 unverified（待核验）。
--   2. 原有文件、管理员备注、操作历史保留；历史日志补齐 data_domain_ids，
--      使升级后仍按原归属数据域保守过滤。
--   3. 旧列在回填完成后删除；回退方式为使用升级前的数据库备份恢复
--      （见 docs/deployment.md「多段学籍升级与回退」）。
--
-- 迁移可重复执行：所有 DDL 先查 information_schema，回填使用 NOT EXISTS 去重，
-- 且只在旧列仍存在时执行对应回填。
--
-- migrate: requires alumni_profiles,alumni_files,data_domains,operation_logs
-- migrate: proves alumni_enrollments

USE sdu_alumni_db;

CREATE TABLE IF NOT EXISTS alumni_enrollments (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  alumni_id BIGINT UNSIGNED NOT NULL COMMENT '个人主档案 ID',
  data_domain_id BIGINT UNSIGNED NOT NULL COMMENT '管理数据域 ID',
  training_level VARCHAR(32) NOT NULL DEFAULT 'unverified' COMMENT 'undergraduate / master / doctor / unverified',
  grade VARCHAR(50) NOT NULL COMMENT '年级',
  class_name VARCHAR(100) NULL COMMENT '班级',
  cohort VARCHAR(50) NULL COMMENT '届数',
  counselor VARCHAR(100) NULL COMMENT '辅导员',
  mentor VARCHAR(100) NULL COMMENT '导师',
  major VARCHAR(100) NULL COMMENT '专业',
  training_mode VARCHAR(50) NULL COMMENT '培养方式',
  remark TEXT NULL COMMENT '学籍内部备注',
  status VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active / deleted',
  version INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '乐观锁版本',
  created_by BIGINT UNSIGNED NULL COMMENT '创建者用户 ID',
  updated_by BIGINT UNSIGNED NULL COMMENT '更新者用户 ID',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  INDEX idx_ae_alumni (alumni_id, status),
  INDEX idx_ae_data_domain (data_domain_id),
  INDEX idx_ae_grade (grade),
  INDEX idx_ae_class_name (class_name),
  INDEX idx_ae_cohort (cohort),
  INDEX idx_ae_major (major),
  INDEX idx_ae_training_mode (training_mode),
  INDEX idx_ae_status (status),
  CONSTRAINT fk_ae_alumni FOREIGN KEY (alumni_id) REFERENCES alumni_profiles(id),
  CONSTRAINT fk_ae_data_domain FOREIGN KEY (data_domain_id) REFERENCES data_domains(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='校友学籍段';

DROP PROCEDURE IF EXISTS migrate_alumni_enrollments;

DELIMITER $$
CREATE PROCEDURE migrate_alumni_enrollments()
BEGIN
  -- 旧个人档案 → 一份个人档案 + 一段学籍。本科数据域可确定层次；MPA 为公共管理
  -- 硕士专业学位；学术学位研究生无法区分硕/博，按 Issue #240 要求保留待核验，
  -- 不能从数据域名称猜测。
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'data_domain_id'
  ) THEN
    INSERT INTO alumni_enrollments (
      alumni_id, data_domain_id, training_level, grade, class_name, cohort, counselor, mentor,
      major, training_mode, remark, status, version, created_by, updated_by, created_at, updated_at, deleted_at
    )
    SELECT
      p.id, p.data_domain_id,
      CASE d.code
        WHEN 'undergraduate' THEN 'undergraduate'
        WHEN 'mpa' THEN 'master'
        ELSE 'unverified'
      END,
      p.grade, p.class_name, p.cohort, p.counselor, p.mentor,
      p.major, p.training_mode, p.remark, p.status, 1, p.created_by, p.updated_by,
      p.created_at, p.updated_at, p.deleted_at
    FROM alumni_profiles AS p
    JOIN data_domains AS d ON d.id = p.data_domain_id
    WHERE NOT EXISTS (
      SELECT 1 FROM alumni_enrollments AS e WHERE e.alumni_id = p.id
    );
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'version'
  ) THEN
    ALTER TABLE alumni_profiles
      ADD COLUMN version INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '个人信息乐观锁版本' AFTER status;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_files' AND column_name = 'enrollment_id'
  ) THEN
    ALTER TABLE alumni_files
      ADD COLUMN enrollment_id BIGINT UNSIGNED NULL COMMENT '学籍段 ID' AFTER alumni_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_files' AND index_name = 'idx_af_enrollment_type'
  ) THEN
    ALTER TABLE alumni_files ADD INDEX idx_af_enrollment_type (enrollment_id, file_type);
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_files' AND column_name = 'alumni_id'
  ) THEN
    UPDATE alumni_files AS f
    JOIN (
      SELECT alumni_id, MIN(id) AS enrollment_id
      FROM alumni_enrollments
      WHERE status = 'active' AND deleted_at IS NULL
      GROUP BY alumni_id
    ) AS e ON e.alumni_id = f.alumni_id
    SET f.enrollment_id = e.enrollment_id
    WHERE f.enrollment_id IS NULL;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = DATABASE() AND table_name = 'alumni_files' AND constraint_name = 'fk_af_enrollment'
  ) THEN
    ALTER TABLE alumni_files
      ADD CONSTRAINT fk_af_enrollment FOREIGN KEY (enrollment_id) REFERENCES alumni_enrollments(id);
  END IF;

  -- 旧日志按原校友档案的数据域补归属，升级后普通管理员仍按原范围可见。
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'data_domain_id'
  ) THEN
    UPDATE operation_logs AS l
    JOIN alumni_profiles AS p ON p.id = l.target_id
    SET l.detail = JSON_SET(
      COALESCE(l.detail, JSON_OBJECT()),
      '$.data_domain_ids',
      JSON_ARRAY(p.data_domain_id)
    )
    WHERE l.target_type = 'alumni_profile'
      AND l.detail IS NOT NULL
      AND JSON_EXTRACT(l.detail, '$.data_domain_ids') IS NULL;

    -- 旧日志的 management_scope 多为“全部校友”；升级后统一回填为原数据域名称，
    -- 保证按管理范围筛选时仍能找到历史记录。
    UPDATE operation_logs AS l
    JOIN alumni_profiles AS p ON p.id = l.target_id
    JOIN data_domains AS d ON d.id = p.data_domain_id
    SET l.detail = JSON_SET(
      COALESCE(l.detail, JSON_OBJECT()),
      '$.management_scope',
      d.name
    )
    WHERE l.target_type = 'alumni_profile'
      AND (
        JSON_EXTRACT(l.detail, '$.management_scope') IS NULL
        OR JSON_UNQUOTE(JSON_EXTRACT(l.detail, '$.management_scope')) = '全部校友'
      );
  END IF;

  -- 学籍字段与文件外键回填完成后清理旧列；先删索引/外键，避免 DROP COLUMN 报错。
  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND index_name = 'ft_alumni_search'
  ) THEN
    ALTER TABLE alumni_profiles DROP INDEX ft_alumni_search;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND constraint_name = 'fk_alumni_profiles_data_domain'
  ) THEN
    ALTER TABLE alumni_profiles DROP FOREIGN KEY fk_alumni_profiles_data_domain;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND index_name = 'idx_alumni_data_domain_id'
  ) THEN
    ALTER TABLE alumni_profiles DROP INDEX idx_alumni_data_domain_id;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND index_name = 'idx_alumni_grade'
  ) THEN
    ALTER TABLE alumni_profiles DROP INDEX idx_alumni_grade;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND index_name = 'idx_alumni_class_name'
  ) THEN
    ALTER TABLE alumni_profiles DROP INDEX idx_alumni_class_name;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND index_name = 'idx_alumni_cohort'
  ) THEN
    ALTER TABLE alumni_profiles DROP INDEX idx_alumni_cohort;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND index_name = 'idx_alumni_major'
  ) THEN
    ALTER TABLE alumni_profiles DROP INDEX idx_alumni_major;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND index_name = 'idx_alumni_training_mode'
  ) THEN
    ALTER TABLE alumni_profiles DROP INDEX idx_alumni_training_mode;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'data_domain_id'
  ) THEN
    ALTER TABLE alumni_profiles DROP COLUMN data_domain_id;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'grade'
  ) THEN
    ALTER TABLE alumni_profiles DROP COLUMN grade;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'class_name'
  ) THEN
    ALTER TABLE alumni_profiles DROP COLUMN class_name;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'cohort'
  ) THEN
    ALTER TABLE alumni_profiles DROP COLUMN cohort;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'counselor'
  ) THEN
    ALTER TABLE alumni_profiles DROP COLUMN counselor;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'mentor'
  ) THEN
    ALTER TABLE alumni_profiles DROP COLUMN mentor;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'major'
  ) THEN
    ALTER TABLE alumni_profiles DROP COLUMN major;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'training_mode'
  ) THEN
    ALTER TABLE alumni_profiles DROP COLUMN training_mode;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND column_name = 'remark'
  ) THEN
    ALTER TABLE alumni_profiles DROP COLUMN remark;
  END IF;

  -- 关键字搜索覆盖个人信息字段；学籍导师/辅导员由 alumni_enrollments 的子查询匹配。
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles' AND index_name = 'ft_alumni_search'
  ) THEN
    ALTER TABLE alumni_profiles
      ADD FULLTEXT KEY ft_alumni_search (name, work_unit, position);
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_files' AND index_name = 'idx_af_alumni_id'
  ) THEN
    ALTER TABLE alumni_files DROP INDEX idx_af_alumni_id;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'alumni_files' AND index_name = 'idx_af_alumni_type'
  ) THEN
    ALTER TABLE alumni_files DROP INDEX idx_af_alumni_type;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'alumni_files' AND column_name = 'alumni_id'
  ) THEN
    ALTER TABLE alumni_files DROP COLUMN alumni_id;
  END IF;
END$$
DELIMITER ;

CALL migrate_alumni_enrollments();
DROP PROCEDURE migrate_alumni_enrollments;
