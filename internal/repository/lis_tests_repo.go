package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"lis-khanza-mapper/internal/model"
)

type LisTestRepo struct {
	db *sql.DB
}

func NewLisTestRepo(db *sql.DB) *LisTestRepo {
	return &LisTestRepo{db: db}
}

func (r *LisTestRepo) List(ctx context.Context, q, status string) ([]model.LisTest, error) {
	q = strings.TrimSpace(q)
	status = strings.TrimSpace(status)
	sqlQ := `
SELECT t.id, t.lis_test_id, t.local_code, t.test_name, t.status, t.created_at, t.updated_at,
       (SELECT COUNT(*) FROM lis_mapping_tests m WHERE m.lis_tests_pk = t.id AND m.status = 'aktif') AS map_count
FROM lis_tests t
WHERE 1=1`
	var args []any
	if status != "" {
		sqlQ += " AND t.status = ?"
		args = append(args, status)
	}
	if q != "" {
		sqlQ += " AND (t.lis_test_id LIKE ? OR t.local_code LIKE ? OR t.test_name LIKE ?)"
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	sqlQ += " ORDER BY t.test_name, t.lis_test_id"

	rows, err := r.db.QueryContext(ctx, sqlQ, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.LisTest
	for rows.Next() {
		var t model.LisTest
		if err := rows.Scan(&t.ID, &t.LisTestID, &t.LocalCode, &t.TestName, &t.Status, &t.CreatedAt, &t.UpdatedAt, &t.MapCount); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *LisTestRepo) ActiveIDs(ctx context.Context, lisTestIDs []string) (map[string]struct{}, error) {
	ids := uniqueNonEmpty(lisTestIDs)
	out := map[string]struct{}{}
	if len(ids) == 0 {
		return out, nil
	}
	q := `SELECT lis_test_id FROM lis_tests WHERE status='aktif' AND lis_test_id IN (` + placeholders(len(ids)) + `)`
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func (r *LisTestRepo) GetByID(ctx context.Context, id uint64) (*model.LisTest, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, lis_test_id, local_code, test_name, status, created_at, updated_at
FROM lis_tests WHERE id = ?`, id)
	var t model.LisTest
	if err := row.Scan(&t.ID, &t.LisTestID, &t.LocalCode, &t.TestName, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// UpsertSeen inserts unknown LIS tests (or refreshes name/code) from resolve traffic.
// Empty test_id rows are skipped. Empty names on insert fall back to lis_test_id;
// empty names/codes on update leave the stored value.
func (r *LisTestRepo) UpsertSeen(ctx context.Context, tests []model.LisTest) error {
	uniq := map[string]model.LisTest{}
	for _, t := range tests {
		id := strings.TrimSpace(t.LisTestID)
		if id == "" {
			continue
		}
		t.LisTestID = id
		t.LocalCode = strings.TrimSpace(t.LocalCode)
		t.TestName = strings.TrimSpace(t.TestName)
		uniq[id] = t
	}
	for _, t := range uniq {
		insertName := t.TestName
		if insertName == "" {
			insertName = t.LisTestID
		}
		if _, err := r.db.ExecContext(ctx, `
INSERT INTO lis_tests (lis_test_id, local_code, test_name, status)
VALUES (?,?,?,'aktif')
ON DUPLICATE KEY UPDATE
  local_code=IF(VALUES(local_code)='', local_code, VALUES(local_code)),
  test_name=IF(?='', test_name, VALUES(test_name)),
  updated_at=NOW()`,
			t.LisTestID, t.LocalCode, insertName, t.TestName); err != nil {
			return err
		}
	}
	return nil
}

func (r *LisTestRepo) Create(ctx context.Context, t *model.LisTest) error {
	res, err := r.db.ExecContext(ctx, `
INSERT INTO lis_tests (lis_test_id, local_code, test_name, status) VALUES (?,?,?,?)`,
		t.LisTestID, t.LocalCode, t.TestName, t.Status)
	if err != nil {
		if isDuplicate(err) {
			return fmt.Errorf("lis_test_id already exists")
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	t.ID = uint64(id)
	return nil
}

func (r *LisTestRepo) Update(ctx context.Context, t *model.LisTest) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE lis_tests SET local_code=?, test_name=?, status=?, updated_at=NOW()
WHERE id=?`, t.LocalCode, t.TestName, t.Status, t.ID)
	return err
}

func (r *LisTestRepo) MappingCount(ctx context.Context, id uint64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM lis_mapping_tests WHERE lis_tests_pk=?`, id).Scan(&n)
	return n, err
}

func (r *LisTestRepo) Delete(ctx context.Context, id uint64) error {
	n, err := r.MappingCount(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		_, err = r.db.ExecContext(ctx, `UPDATE lis_tests SET status='nonaktif', updated_at=NOW() WHERE id=?`, id)
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM lis_tests WHERE id=?`, id)
	return err
}

func isDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Duplicate")
}
