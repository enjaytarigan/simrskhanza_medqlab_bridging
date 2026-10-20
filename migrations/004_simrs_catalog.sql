CREATE TABLE IF NOT EXISTS simrs_panels (
  kd_jenis_prw  VARCHAR(15)  NOT NULL COMMENT 'Khanza jns_perawatan_lab.kd_jenis_prw',
  nm_perawatan  VARCHAR(200) NOT NULL DEFAULT '',
  kategori      VARCHAR(50)  NOT NULL DEFAULT '',
  status        VARCHAR(20)  NOT NULL DEFAULT '1',
  total_byr     DOUBLE       NOT NULL DEFAULT 0,
  created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (kd_jenis_prw)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS simrs_templates (
  id_template   INT(11)      NOT NULL COMMENT 'Khanza template_laboratorium.id_template',
  pemeriksaan   VARCHAR(200) NOT NULL DEFAULT '',
  satuan        VARCHAR(20)  NOT NULL DEFAULT '',
  urut          INT          NOT NULL DEFAULT 0,
  kd_jenis_prw  VARCHAR(15)  NOT NULL,
  biaya_item    DOUBLE       NOT NULL DEFAULT 0,
  created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id_template),
  KEY idx_simrs_templates_panel (kd_jenis_prw),
  CONSTRAINT fk_simrs_templates_panel
    FOREIGN KEY (kd_jenis_prw) REFERENCES simrs_panels (kd_jenis_prw)
    ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
