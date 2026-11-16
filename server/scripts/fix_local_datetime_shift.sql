-- 一次性修复：DB_PARAMS=loc=Local 期间的“未来时间”偏移
--
-- 背景：
--   本地 server/.env 曾把 DB_PARAMS 改成 loc=Local（正确值为
--   charset=utf8mb4&parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27）。
--   数据库会话时区固定为 +00:00，所以 CURRENT_TIMESTAMP 写入的是 UTC；
--   但 Go 代码显式写入的时间（updated_at、deleted_at 等）是本地时间（UTC+8）。
--   修复配置后统一按 UTC 读取，这些“本地时间”就会显示成提前 8 小时。
--
-- 判定规则（无需人工指定时间窗口）：
--   真实业务时间不可能落在“现在（UTC）”之后。凡是 DATETIME 值 > UTC_TIMESTAMP()
--   的，都是按 CEST/CST 本地时间写入的记录，统一减去 8 小时即可回到真实 UTC。
--   正常按 UTC 写入的行不会被命中，因此脚本可重复执行：第二次运行不会改动任何数据。
--
-- 用法（只读预检，默认行为）：
--   本地开发库容器名为 platform-mysql-1，生产为 sduzg-alumin-platform-mysql-1。
--   docker exec -i platform-mysql-1 sh -c \
--     'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" sdu_alumni_db' \
--     < server/scripts/fix_local_datetime_shift.sql
--   默认只输出报告，不修改数据。
--
-- 用法（确认报告后执行修复）：
--   docker exec -i platform-mysql-1 sh -c \
--     'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" --init-command="SET @apply_fix=1" \
--        sdu_alumni_db' < server/scripts/fix_local_datetime_shift.sql
--   --init-command 在建立连接后、执行脚本前生效；不加时默认只输出预检报告。
--
-- 注意：
--   * 中国标准时间全年固定 UTC+8，不存在夏令时，因此可以安全地按固定 8 小时平移。
--   * 只处理 DATETIME 列；TIMESTAMP 列由 MySQL 自身按会话时区换算，不在此列。
--   * 带 ON UPDATE CURRENT_TIMESTAMP 的表会额外写 updated_at = updated_at，
--     避免修复其他列时把 updated_at 改成当前时间。
--   * 本脚本只修复“落在未来”的记录。如果某个环境长期使用 loc=Local，
--     更早的历史行（写入时间已超过 8 小时）不会被自动命中；这类数据请按
--     docs/deployment.md 的说明评估后重置或单独处理。

SET NAMES utf8mb4;
SET @apply_fix = COALESCE(@apply_fix, 0);

DROP PROCEDURE IF EXISTS fix_local_datetime_shift;
DROP TEMPORARY TABLE IF EXISTS tmp_local_datetime_shift_report;

DELIMITER $$
CREATE PROCEDURE fix_local_datetime_shift(IN apply_fix TINYINT)
BEGIN
  DECLARE done INT DEFAULT 0;
  DECLARE tname VARCHAR(64);
  DECLARE cname VARCHAR(64);
  DECLARE has_updated_at INT DEFAULT 0;
  DECLARE affected BIGINT DEFAULT 0;
  DECLARE total_fixed BIGINT DEFAULT 0;

  DECLARE col_cursor CURSOR FOR
    SELECT cols.table_name, cols.column_name
    FROM information_schema.columns AS cols
    JOIN information_schema.tables AS tbls
      ON tbls.table_schema = cols.table_schema
     AND tbls.table_name = cols.table_name
     AND tbls.table_type = 'BASE TABLE'
    WHERE cols.table_schema = DATABASE()
      AND cols.data_type = 'datetime'
    ORDER BY cols.table_name, cols.ordinal_position;
  DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;

  CREATE TEMPORARY TABLE tmp_local_datetime_shift_report (
    table_name VARCHAR(64) NOT NULL,
    column_name VARCHAR(64) NOT NULL,
    future_rows BIGINT NOT NULL,
    action VARCHAR(32) NOT NULL
  );

  OPEN col_cursor;
  read_loop: LOOP
    FETCH col_cursor INTO tname, cname;
    IF done = 1 THEN
      LEAVE read_loop;
    END IF;

    -- PREPARE 只接受字符串字面量或用户变量，不能使用局部变量。
    SET @dyn_sql = CONCAT('SELECT COUNT(*) INTO @future_rows FROM `', tname,
                          '` WHERE `', cname, '` > UTC_TIMESTAMP()');
    PREPARE probe FROM @dyn_sql;
    EXECUTE probe;
    DEALLOCATE PREPARE probe;
    SET affected = COALESCE(@future_rows, 0);

    IF affected = 0 THEN
      ITERATE read_loop;
    END IF;

    IF apply_fix = 1 THEN
      -- ON UPDATE CURRENT_TIMESTAMP 会顺带改 updated_at：显式自赋值抵消该行为。
      SET has_updated_at = (
        SELECT COUNT(*) FROM information_schema.columns
        WHERE table_schema = DATABASE() AND table_name = tname AND column_name = 'updated_at'
      );
      SET @dyn_sql = CONCAT('UPDATE `', tname, '` SET `', cname, '` = `', cname,
                            '` - INTERVAL 8 HOUR');
      IF has_updated_at > 0 AND cname <> 'updated_at' THEN
        SET @dyn_sql = CONCAT(@dyn_sql, ', `updated_at` = `updated_at`');
      END IF;
      SET @dyn_sql = CONCAT(@dyn_sql, ' WHERE `', cname, '` > UTC_TIMESTAMP()');
      PREPARE fixup FROM @dyn_sql;
      EXECUTE fixup;
      -- DEALLOCATE 会重置 ROW_COUNT()，必须先取值。
      SET affected = ROW_COUNT();
      DEALLOCATE PREPARE fixup;
      SET total_fixed = total_fixed + affected;
      INSERT INTO tmp_local_datetime_shift_report
      VALUES (tname, cname, affected, 'shifted -8h');
    ELSE
      INSERT INTO tmp_local_datetime_shift_report
      VALUES (tname, cname, affected, 'would shift -8h');
    END IF;
  END LOOP;
  CLOSE col_cursor;

  SELECT table_name, column_name, future_rows, action
  FROM tmp_local_datetime_shift_report
  ORDER BY table_name, column_name;

  IF apply_fix = 1 THEN
    SELECT total_fixed AS rows_fixed;
  ELSE
    SELECT COALESCE(SUM(future_rows), 0) AS rows_pending,
           'dry-run：把 @apply_fix 设为 1 后重新执行即可修复' AS hint
    FROM tmp_local_datetime_shift_report;
  END IF;
END$$
DELIMITER ;

CALL fix_local_datetime_shift(@apply_fix);

DROP PROCEDURE IF EXISTS fix_local_datetime_shift;
DROP TEMPORARY TABLE IF EXISTS tmp_local_datetime_shift_report;
