SET NAMES utf8mb4;

-- migrate: proves schema_migrations

USE sdu_alumni_db;

-- MySQL 仅在数据卷首次初始化时执行 docker-entrypoint-initdb.d。该表由
-- server/scripts/migrate.sh 使用，以便已有数据卷在后续启动时也能应用新增迁移。
CREATE TABLE IF NOT EXISTS schema_migrations (
  name VARCHAR(255) NOT NULL PRIMARY KEY,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='已执行的数据库迁移';

INSERT IGNORE INTO schema_migrations (name)
VALUES
  ('001_init_schema.sql'),
  ('002_seed_test_user.sql'),
  ('003_add_alumni_files.sql'),
  ('004_add_user_email.sql'),
  ('005_add_indexes.sql'),
  ('006_add_admin_access_control.sql'),
  ('007_fix_data_domain_encoding.sql'),
  ('008_add_history_wiki.sql'),
  ('009_add_migration_tracking.sql');
