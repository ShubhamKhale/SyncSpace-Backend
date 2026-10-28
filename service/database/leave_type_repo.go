package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
)

type LeaveTypeRepo struct {
	db *pgxpool.Pool
}

func NewLeaveTypeRepo(db *pgxpool.Pool) *LeaveTypeRepo {
	return &LeaveTypeRepo{db: db}
}

func (r *LeaveTypeRepo) List(ctx context.Context, orgID string) ([]model.LeaveType, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, org_id, name, default_days, is_active, created_at
		 FROM public.leave_types
		 WHERE org_id = $1 AND is_active = TRUE
		 ORDER BY created_at`,
		orgID,
	)
	if err != nil {
		return nil, errs.Internal("failed to list leave types")
	}
	defer rows.Close()

	var types []model.LeaveType
	for rows.Next() {
		var t model.LeaveType
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Name, &t.DefaultDays, &t.IsActive, &t.CreatedAt); err != nil {
			return nil, errs.Internal("failed to scan leave type")
		}
		types = append(types, t)
	}
	return types, nil
}

func (r *LeaveTypeRepo) Create(ctx context.Context, lt *model.LeaveType) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.leave_types (id, org_id, name, default_days)
		 VALUES ($1, $2, $3, $4)`,
		lt.ID, lt.OrgID, lt.Name, lt.DefaultDays,
	)
	if err != nil {
		return errs.Internal("failed to create leave type")
	}
	return nil
}

func (r *LeaveTypeRepo) Update(ctx context.Context, id, name string, defaultDays int) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE public.leave_types SET name = $2, default_days = $3 WHERE id = $1`,
		id, name, defaultDays,
	)
	if err != nil {
		return errs.Internal("failed to update leave type")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("leave type not found")
	}
	return nil
}

func (r *LeaveTypeRepo) SeedDefaults(ctx context.Context, orgID string) error {
	defaults := []struct {
		Name string
		Days int
	}{
		{"Sick Leave", 12},
		{"Casual Leave", 12},
		{"Earned Leave", 15},
	}
	for _, d := range defaults {
		_, err := r.db.Exec(ctx,
			`INSERT INTO public.leave_types (id, org_id, name, default_days)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (org_id, name) DO NOTHING`,
			utils.NewUUID(), orgID, d.Name, d.Days,
		)
		if err != nil {
			return errs.Internal("failed to seed leave types")
		}
	}
	return nil
}
