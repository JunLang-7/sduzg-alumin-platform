SET NAMES utf8mb4;

USE sdu_alumni_db;

INSERT INTO alumni_profiles (name, grade, class_name, cohort, counselor, mentor, major, training_mode, industry, work_unit, position, mobile, gender)
VALUES ('测试校友', '2020', 'MPA1班', '2020', '张导员', '李导师', '公共管理', '非全日制', '政府机关', '某某市人社局', '科员', '13800001111', '男')
ON DUPLICATE KEY UPDATE name = VALUES(name);

INSERT INTO users (account, password_hash, role, alumni_id, real_name, mobile, status)
SELECT 'testuser', '$2a$10$pQ.VLGGG9eSRSkvBcT22se8oqjURnsfeniK3tBG48hPdWssHDJSDC', 'alumni', id, '测试校友', '13800001111', 'active'
FROM alumni_profiles
WHERE name = '测试校友' AND grade = '2020'
ON DUPLICATE KEY UPDATE role = VALUES(role), real_name = VALUES(real_name), status = VALUES(status);
