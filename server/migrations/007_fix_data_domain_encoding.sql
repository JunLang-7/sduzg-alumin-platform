-- 修复 006 迁移在非 UTF-8 客户端执行时写入的数据域中文乱码。

SET NAMES utf8mb4;

UPDATE data_domains
SET name = CASE code
  WHEN 'undergraduate' THEN '本科生'
  WHEN 'academic_graduate' THEN '学术学位研究生'
  WHEN 'mpa' THEN 'MPA专业学位研究生'
  ELSE name
END
WHERE code IN ('undergraduate', 'academic_graduate', 'mpa');
