-- operation_logs: 复合索引支撑操作历史列表的有序扫描。
--
-- 列表查询按 created_at DESC, id DESC 排序。若 MySQL 走 filesort，会把包含
-- detail 整列的排序行物化进 sort_buffer；批量导入审计的 detail 可达数百 KB，
-- 排序行超限即报 1038 (Out of sort memory)。该复合索引让查询反向扫描索引完成
-- 排序，filesort 不再发生，排序结果与 detail 体积彻底解耦。
--
-- MySQL 不支持 ADD INDEX IF NOT EXISTS，而本迁移会执行两次——数据卷首次初始化时
-- docker-entrypoint-initdb.d 按序应用全部迁移，之后 server/scripts/migrate.sh 再次
-- 应用未登记的迁移——因此这里用 information_schema 判断后再建索引，保证可重复执行。
--
-- 这里声明 requires 而不是 proves：proves 的语义是"该表存在即视为迁移已执行"，
-- 而 operation_logs 在本迁移之前就已存在，用 proves 会让旧卷跳过本迁移、
-- 导致索引缺失。requires 只校验依赖表存在，不影响迁移执行。
-- migrate: requires operation_logs
SET NAMES utf8mb4;

USE sdu_alumni_db;

SET @audit_sort_index_exists := (
  SELECT COUNT(*)
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'operation_logs'
    AND index_name = 'idx_logs_created_id'
);
SET @audit_sort_index_sql := IF(
  @audit_sort_index_exists > 0,
  'SELECT 1',
  'ALTER TABLE operation_logs ADD INDEX idx_logs_created_id (created_at, id)'
);
PREPARE audit_sort_index_stmt FROM @audit_sort_index_sql;
EXECUTE audit_sort_index_stmt;
DEALLOCATE PREPARE audit_sort_index_stmt;
