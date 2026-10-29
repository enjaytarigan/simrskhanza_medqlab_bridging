CREATE TABLE IF NOT EXISTS lis_order_outbox (
  noorder          VARCHAR(50)     NOT NULL COMMENT 'SIMRS permintaan_lab.noorder (full)',
  no_rawat         VARCHAR(20)     NOT NULL DEFAULT '',
  no_laboratorium  VARCHAR(50)     NOT NULL DEFAULT '' COMMENT '00 while claimed; MedQLab lab no when sent',
  status           ENUM('claimed','sent','failed') NOT NULL DEFAULT 'claimed',
  error_message    TEXT            NULL,
  payload_json     LONGTEXT        NULL,
  claimed_at       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  sent_at          DATETIME        NULL,
  created_at       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (noorder),
  KEY idx_lis_order_outbox_status (status),
  KEY idx_lis_order_outbox_claimed (status, claimed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
