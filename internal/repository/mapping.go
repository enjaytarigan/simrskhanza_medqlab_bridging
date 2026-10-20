package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"lis-khanza-mapper/internal/model"
	"lis-khanza-mapper/internal/resolve"
)

type MappingRepo struct {
	db    *sql.DB
	simrs *SimrsRepo
}

func NewMappingRepo(db *sql.DB, simrs *SimrsRepo) *MappingRepo {
	return &MappingRepo{db: db, simrs: simrs}
}

func (r *MappingRepo) List(ctx context.Context, lisTestsPK uint64, idTemplate int, kdJenisPrw, status string) ([]model.Mapping, error) {
	sqlQ := fmt.Sprintf(`
SELECT m.id, m.lis_tests_pk, t.lis_test_id, t.test_name, m.id_template,
       IFNULL(%s,''), m.kd_jenis_prw, IFNULL(j.nm_perawatan,''),
       m.status, m.created_by, m.created_at, m.updated_at
FROM lis_mapping_tests m
INNER JOIN lis_tests t ON t.id = m.lis_tests_pk
LEFT JOIN %s tl ON tl.id_template = m.id_template
LEFT JOIN %s j ON j.kd_jenis_prw = m.kd_jenis_prw
WHERE 1=1`, r.simrs.pemeriksaanCol("tl"), r.simrs.templateTable(), r.simrs.panelTable())
	var args []any
	if lisTestsPK > 0 {
		sqlQ += " AND m.lis_tests_pk = ?"
		args = append(args, lisTestsPK)
	}
	if idTemplate > 0 {
		sqlQ += " AND m.id_template = ?"
		args = append(args, idTemplate)
	}
	if kdJenisPrw != "" {
		sqlQ += " AND m.kd_jenis_prw = ?"
		args = append(args, kdJenisPrw)
	}
	if status != "" {
		sqlQ += " AND m.status = ?"
		args = append(args, status)
	}
	sqlQ += " ORDER BY t.test_name, m.id_template"

	rows, err := r.db.QueryContext(ctx, sqlQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Mapping
	for rows.Next() {
		var m model.Mapping
		if err := rows.Scan(&m.ID, &m.LisTestsPK, &m.LisTestID, &m.TestName, &m.IDTemplate,
			&m.Pemeriksaan, &m.KdJenisPrw, &m.NmPerawatan, &m.Status, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MappingRepo) BulkUpsert(ctx context.Context, lisTestsPK uint64, idTemplates []int, createdBy string) (model.BulkMapResult, error) {
	var result model.BulkMapResult
	if lisTestsPK == 0 {
		return result, fmt.Errorf("lis_tests_pk is required")
	}
	if len(idTemplates) == 0 {
		return result, fmt.Errorf("no templates selected")
	}
	if len(idTemplates) > 200 {
		return result, fmt.Errorf("maximum 200 templates per request")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, idTpl := range idTemplates {
		kdJenisPrw, err := r.simrs.TemplateKdJenisPrw(ctx, idTpl)
		if errors.Is(err, sql.ErrNoRows) {
			result.Skipped++
			continue
		}
		if err != nil {
			return result, err
		}
		if kdJenisPrw == "" {
			result.Skipped++
			continue
		}

		var existingID uint64
		err = tx.QueryRowContext(ctx, `
SELECT id FROM lis_mapping_tests
WHERE lis_tests_pk=? AND id_template=? AND kd_jenis_prw=?`,
			lisTestsPK, idTpl, kdJenisPrw).Scan(&existingID)

		if errors.Is(err, sql.ErrNoRows) {
			_, err = tx.ExecContext(ctx, `
INSERT INTO lis_mapping_tests (lis_tests_pk, id_template, kd_jenis_prw, status, created_by)
VALUES (?,?,?,'aktif',?)`, lisTestsPK, idTpl, kdJenisPrw, createdBy)
			if err != nil {
				return result, err
			}
			result.Created++
		} else if err != nil {
			return result, err
		} else {
			_, err = tx.ExecContext(ctx, `
UPDATE lis_mapping_tests SET status='aktif', created_by=?, updated_at=NOW() WHERE id=?`,
				createdBy, existingID)
			if err != nil {
				return result, err
			}
			result.Updated++
		}
	}

	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func (r *MappingRepo) Deactivate(ctx context.Context, id uint64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE lis_mapping_tests SET status='nonaktif', updated_at=NOW() WHERE id=?`, id)
	return err
}

func (r *MappingRepo) Delete(ctx context.Context, id uint64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM lis_mapping_tests WHERE id=?`, id)
	return err
}

func (r *MappingRepo) CountActive(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM lis_mapping_tests WHERE status='aktif'`).Scan(&n)
	return n, err
}

func (r *MappingRepo) ListByLisTestPK(ctx context.Context, lisTestsPK uint64) ([]model.Mapping, error) {
	return r.List(ctx, lisTestsPK, 0, "", "aktif")
}

func (r *MappingRepo) ActiveCandidates(ctx context.Context, testIDs, localCodes []string) ([]resolve.Candidate, error) {
	testIDs = uniqueNonEmpty(testIDs)
	localCodes = uniqueNonEmpty(localCodes)
	if len(testIDs) == 0 && len(localCodes) == 0 {
		return nil, nil
	}

	sqlQ := fmt.Sprintf(`
SELECT t.lis_test_id, t.local_code, m.id_template, m.kd_jenis_prw, IFNULL(%s,'')
FROM lis_mapping_tests m
INNER JOIN lis_tests t ON t.id = m.lis_tests_pk
LEFT JOIN %s tl ON tl.id_template = m.id_template
WHERE t.status='aktif' AND m.status='aktif'`, r.simrs.pemeriksaanCol("tl"), r.simrs.templateTable())
	var args []any
	var clauses []string
	if len(testIDs) > 0 {
		clauses = append(clauses, "t.lis_test_id IN ("+placeholders(len(testIDs))+")")
		for _, id := range testIDs {
			args = append(args, id)
		}
	}
	if len(localCodes) > 0 {
		clauses = append(clauses, "(t.local_code <> '' AND t.local_code IN ("+placeholders(len(localCodes))+"))")
		for _, code := range localCodes {
			args = append(args, code)
		}
	}
	sqlQ += " AND (" + strings.Join(clauses, " OR ") + ")"

	rows, err := r.db.QueryContext(ctx, sqlQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []resolve.Candidate
	for rows.Next() {
		var c resolve.Candidate
		if err := rows.Scan(&c.LisTestID, &c.LocalCode, &c.IDTemplate, &c.KdJenisPrw, &c.Pemeriksaan); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func uniqueNonEmpty(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// FilterLisTestsPKByLisTestID supports API query by LIS testId string.
func FilterLisTestsPKByLisTestID(ctx context.Context, db *sql.DB, lisTestID string) (uint64, error) {
	lisTestID = strings.TrimSpace(lisTestID)
	var pk uint64
	err := db.QueryRowContext(ctx, `SELECT id FROM lis_tests WHERE lis_test_id=? AND status='aktif'`, lisTestID).Scan(&pk)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return pk, err
}
