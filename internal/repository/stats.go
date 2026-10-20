package repository

import (
	"context"
	"database/sql"

	"lis-khanza-mapper/internal/model"
)

func Dashboard(ctx context.Context, db *sql.DB, simrs *SimrsRepo) (model.DashboardStats, error) {
	var s model.DashboardStats
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM lis_tests WHERE status='aktif'`).Scan(&s.LisTestsActive)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM lis_mapping_tests WHERE status='aktif'`).Scan(&s.MappingsActive)
	if simrs != nil {
		s.TemplateCount, _ = simrs.CountTemplates(ctx)
	}
	return s, nil
}
