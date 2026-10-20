-- Backfill kd_jenis_prw from Khanza catalog when that table exists (standalone DBs skip this).
SET @has_tpl := (
  SELECT COUNT(*) FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'template_laboratorium'
);
SET @sql := IF(
  @has_tpl > 0,
  'UPDATE lis_mapping_tests m INNER JOIN template_laboratorium t ON t.id_template = m.id_template SET m.kd_jenis_prw = t.kd_jenis_prw WHERE TRIM(m.kd_jenis_prw) = ''''',
  'DO 0'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

ALTER TABLE lis_mapping_tests
  MODIFY COLUMN kd_jenis_prw VARCHAR(15) NOT NULL COMMENT 'template_laboratorium.kd_jenis_prw';
