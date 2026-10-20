package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"lis-khanza-mapper/internal/model"
)

type SimrsRepo struct {
	db    *sql.DB
	local bool
}

func NewSimrsRepo(db *sql.DB, local bool) *SimrsRepo {
	return &SimrsRepo{db: db, local: local}
}

func (r *SimrsRepo) Local() bool {
	return r.local
}

func (r *SimrsRepo) panelTable() string {
	if r.local {
		return "simrs_panels"
	}
	return "jns_perawatan_lab"
}

func (r *SimrsRepo) templateTable() string {
	if r.local {
		return "simrs_templates"
	}
	return "template_laboratorium"
}

func (r *SimrsRepo) pemeriksaanCol(alias string) string {
	if r.local {
		return alias + ".pemeriksaan"
	}
	return alias + ".Pemeriksaan"
}

// ListPanels returns panel rows with tariff summary.
// q matches kd_jenis_prw / nm_perawatan; empty kategori means no kategori filter.
func (r *SimrsRepo) ListPanels(ctx context.Context, q, kategori string) ([]model.Panel, error) {
	sqlQ := fmt.Sprintf(`
SELECT j.kd_jenis_prw, j.nm_perawatan,
       IFNULL(j.total_byr,0), IFNULL(j.kategori,''), IFNULL(j.status,''),
       COUNT(t.id_template) AS jml_template,
       IFNULL(SUM(t.biaya_item),0) AS sum_biaya_item,
       CASE
         WHEN IFNULL(j.total_byr,0) > 0 THEN 'tindakan'
         WHEN IFNULL(SUM(t.biaya_item),0) > 0 THEN 'template'
         ELSE 'none'
       END AS tarif_mode
FROM %s j
LEFT JOIN %s t ON t.kd_jenis_prw = j.kd_jenis_prw
WHERE 1=1`, r.panelTable(), r.templateTable())
	args := []any{}
	kategori = strings.TrimSpace(kategori)
	if kategori != "" {
		sqlQ += " AND j.kategori = ?"
		args = append(args, kategori)
	}
	q = strings.TrimSpace(q)
	if q != "" {
		sqlQ += " AND (j.kd_jenis_prw LIKE ? OR j.nm_perawatan LIKE ?)"
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	sqlQ += `
GROUP BY j.kd_jenis_prw, j.nm_perawatan, j.total_byr, j.kategori, j.status
ORDER BY j.nm_perawatan, j.kd_jenis_prw`

	rows, err := r.db.QueryContext(ctx, sqlQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Panel
	for rows.Next() {
		var p model.Panel
		if err := rows.Scan(
			&p.KdJenisPrw, &p.NmPerawatan,
			&p.TotalByr, &p.Kategori, &p.Status,
			&p.TemplateCount, &p.SumBiayaItem, &p.TarifMode,
		); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *SimrsRepo) ListTemplates(ctx context.Context, kdJenisPrw, q string, lisTestsPK uint64, limit int) ([]model.Template, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	sqlQ := fmt.Sprintf(`
SELECT t.id_template, %s, t.satuan, IFNULL(t.urut,0), t.kd_jenis_prw, j.nm_perawatan,
       IFNULL(t.biaya_item,0),
       EXISTS(
         SELECT 1 FROM lis_mapping_tests m
         WHERE m.id_template = t.id_template AND m.status = 'aktif'
           AND m.lis_tests_pk = ?
       ) AS mapped
FROM %s t
INNER JOIN %s j ON t.kd_jenis_prw = j.kd_jenis_prw
WHERE 1=1`, r.pemeriksaanCol("t"), r.templateTable(), r.panelTable())
	args := []any{lisTestsPK}
	if kdJenisPrw != "" {
		sqlQ += " AND t.kd_jenis_prw = ?"
		args = append(args, kdJenisPrw)
	}
	if q != "" {
		sqlQ += " AND " + r.pemeriksaanCol("t") + " LIKE ?"
		args = append(args, "%"+q+"%")
	}
	sqlQ += " ORDER BY j.nm_perawatan, t.urut LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, sqlQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Template
	for rows.Next() {
		var t model.Template
		var mapped int
		if err := rows.Scan(
			&t.IDTemplate, &t.Pemeriksaan, &t.Satuan, &t.Urut, &t.KdJenisPrw, &t.NmPerawatan,
			&t.BiayaItem, &mapped,
		); err != nil {
			return nil, err
		}
		t.Mapped = mapped == 1
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *SimrsRepo) TemplateExists(ctx context.Context, idTemplate int) (bool, error) {
	var n int
	q := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE id_template=?`, r.templateTable())
	err := r.db.QueryRowContext(ctx, q, idTemplate).Scan(&n)
	return n > 0, err
}

func (r *SimrsRepo) TemplateKdJenisPrw(ctx context.Context, idTemplate int) (string, error) {
	var kd string
	q := fmt.Sprintf(`SELECT kd_jenis_prw FROM %s WHERE id_template=?`, r.templateTable())
	err := r.db.QueryRowContext(ctx, q, idTemplate).Scan(&kd)
	return strings.TrimSpace(kd), err
}

func (r *SimrsRepo) CountTemplates(ctx context.Context) (int, error) {
	var n int
	q := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, r.templateTable())
	err := r.db.QueryRowContext(ctx, q).Scan(&n)
	return n, err
}

func (r *SimrsRepo) UpsertCatalog(ctx context.Context, rows []model.CatalogRow) (model.CatalogImportResult, error) {
	var result model.CatalogImportResult
	if !r.local {
		return result, fmt.Errorf("catalog import is only available in standalone mode")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()

	for i, row := range rows {
		row.KdJenisPrw = strings.TrimSpace(row.KdJenisPrw)
		row.Pemeriksaan = strings.TrimSpace(row.Pemeriksaan)
		row.NmPerawatan = strings.TrimSpace(row.NmPerawatan)
		row.Satuan = strings.TrimSpace(row.Satuan)
		row.Kategori = strings.TrimSpace(row.Kategori)
		row.Status = strings.TrimSpace(row.Status)
		if row.Status == "" {
			row.Status = "1"
		}
		if row.IDTemplate == 0 || row.KdJenisPrw == "" {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: id_template and kd_jenis_prw are required", i+1))
			continue
		}
		if row.NmPerawatan == "" {
			row.NmPerawatan = row.KdJenisPrw
		}
		if row.Pemeriksaan == "" {
			row.Pemeriksaan = fmt.Sprintf("template %d", row.IDTemplate)
		}

		panelRes, err := tx.ExecContext(ctx, `
INSERT INTO simrs_panels (kd_jenis_prw, nm_perawatan, kategori, status, total_byr)
VALUES (?,?,?,?,?)
ON DUPLICATE KEY UPDATE
  nm_perawatan=VALUES(nm_perawatan),
  kategori=VALUES(kategori),
  status=VALUES(status),
  total_byr=VALUES(total_byr),
  updated_at=NOW()`,
			row.KdJenisPrw, row.NmPerawatan, row.Kategori, row.Status, row.TotalByr)
		if err != nil {
			return result, err
		}
		if n, _ := panelRes.RowsAffected(); n == 1 {
			result.PanelsCreated++
		} else {
			result.PanelsUpdated++
		}

		tplRes, err := tx.ExecContext(ctx, `
INSERT INTO simrs_templates (id_template, pemeriksaan, satuan, urut, kd_jenis_prw, biaya_item)
VALUES (?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE
  pemeriksaan=VALUES(pemeriksaan),
  satuan=VALUES(satuan),
  urut=VALUES(urut),
  kd_jenis_prw=VALUES(kd_jenis_prw),
  biaya_item=VALUES(biaya_item),
  updated_at=NOW()`,
			row.IDTemplate, row.Pemeriksaan, row.Satuan, row.Urut, row.KdJenisPrw, row.BiayaItem)
		if err != nil {
			return result, err
		}
		if n, _ := tplRes.RowsAffected(); n == 1 {
			result.TemplatesCreated++
		} else {
			result.TemplatesUpdated++
		}
	}

	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
