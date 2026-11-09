package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"lis-khanza-mapper/internal/model"
)

type OrderOutboxRepo struct {
	db *sql.DB
}

func NewOrderOutboxRepo(db *sql.DB) *OrderOutboxRepo {
	return &OrderOutboxRepo{db: db}
}

func (r *OrderOutboxRepo) List(ctx context.Context, q, status string, page, limit int) (model.OrderOutboxListResult, error) {
	out := model.OrderOutboxListResult{Page: page, Limit: limit}
	if page < 1 {
		page = 1
		out.Page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 50
		out.Limit = 50
	}
	offset := (page - 1) * limit

	where := "WHERE 1=1"
	var args []any
	q = strings.TrimSpace(q)
	status = strings.TrimSpace(status)
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if q != "" {
		where += " AND (noorder LIKE ? OR no_rawat LIKE ? OR no_laboratorium LIKE ? OR IFNULL(error_message,'') LIKE ?)"
		like := "%" + q + "%"
		args = append(args, like, like, like, like)
	}

	countQ := "SELECT COUNT(*) FROM lis_order_outbox " + where
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&out.Total); err != nil {
		return out, err
	}

	listQ := `
SELECT noorder, no_rawat, no_laboratorium, status, IFNULL(error_message,''),
       claimed_at, sent_at, created_at, updated_at
FROM lis_order_outbox ` + where + `
ORDER BY updated_at DESC, noorder DESC
LIMIT ? OFFSET ?`
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, listQ, listArgs...)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		m, err := scanOrderOutbox(rows, false)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, *m)
	}
	return out, rows.Err()
}

func (r *OrderOutboxRepo) GetByNoorder(ctx context.Context, noorder string) (*model.OrderOutbox, error) {
	noorder = strings.TrimSpace(noorder)
	if noorder == "" {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, `
SELECT noorder, no_rawat, no_laboratorium, status, IFNULL(error_message,''), IFNULL(payload_json,''),
       claimed_at, sent_at, created_at, updated_at
FROM lis_order_outbox WHERE noorder = ?`, noorder)
	return scanOrderOutbox(row, true)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOrderOutbox(row rowScanner, withPayload bool) (*model.OrderOutbox, error) {
	var m model.OrderOutbox
	var sentAt sql.NullTime
	var err error
	if withPayload {
		err = row.Scan(
			&m.NoOrder, &m.NoRawat, &m.NoLaboratorium, &m.Status, &m.ErrorMessage, &m.PayloadJSON,
			&m.ClaimedAt, &sentAt, &m.CreatedAt, &m.UpdatedAt,
		)
	} else {
		err = row.Scan(
			&m.NoOrder, &m.NoRawat, &m.NoLaboratorium, &m.Status, &m.ErrorMessage,
			&m.ClaimedAt, &sentAt, &m.CreatedAt, &m.UpdatedAt,
		)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if sentAt.Valid {
		t := sentAt.Time
		m.SentAt = &t
	}
	return &m, nil
}
