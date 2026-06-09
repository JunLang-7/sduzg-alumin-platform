-- Phase 3: Add email column to users and alumni_profiles tables
-- users 表增加 email 列，添加 UNIQUE 约束（NULL 除外）
ALTER TABLE users ADD COLUMN email VARCHAR(255) NULL COMMENT 'email address' AFTER mobile;
ALTER TABLE users ADD UNIQUE INDEX uk_users_email (email);

-- alumni_profiles 表增加 email 列，无唯一约束
ALTER TABLE alumni_profiles ADD COLUMN email VARCHAR(255) NULL COMMENT 'email address' AFTER mobile;
