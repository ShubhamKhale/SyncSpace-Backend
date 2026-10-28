package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

type LeaveBalanceRepo struct {
	db *pgxpool.Pool
}

func NewLeaveBalanceRepo(db *pgxpool.Pool) *LeaveBalanceRepo {
	return &LeaveBalanceRepo{db: db}
}

func (r *LeaveBalanceRepo) GetByUser(ctx context.Context, userID, orgID string, year int) ([]model.LeaveBalance, error) {
	rows, err := r.db.Query(ctx,
		`SELECT lb.user_id, lb.leave_type_id, lt.name, lb.year, lb.allocated, lb.used
		 FROM public.leave_balances lb
		 JOIN public.leave_types lt ON lt.id = lb.leave_type_id
		 WHERE lb.user_id = $1 AND lb.org_id = $2 AND lb.year = $3
		 ORDER BY lt.name`,
		userID, orgID, year,
	)
	if err != nil {
		return nil, errs.Internal("failed to get leave balances")
	}
	defer rows.Close()

	var balances []model.LeaveBalance
	for rows.Next() {
		var b model.LeaveBalance
		if err := rows.Scan(&b.UserID, &b.LeaveTypeID, &b.TypeName, &b.Year, &b.Allocated, &b.Used); err != nil {
			return nil, errs.Internal("failed to scan leave balance")
		}
		b.Remaining = b.Allocated - b.Used
		balances = append(balances, b)
	}
	return balances, nil
}

func (r *LeaveBalanceRepo) InitForUser(ctx context.Context, userID, orgID string, year int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.leave_balances (user_id, org_id, leave_type_id, year, allocated)
		 SELECT $1, $2, id, $3, default_days
		 FROM public.leave_types
		 WHERE org_id = $2 AND is_active = TRUE
		 ON CONFLICT (user_id, leave_type_id, year) DO NOTHING`,
		userID, orgID, year,
	)
	if err != nil {
		return errs.Internal("failed to init leave balances")
	}
	return nil
}

// Increment adds days to the used count (on leave approval).
func (r *LeaveBalanceRepo) Increment(ctx context.Context, userID, leaveTypeID string, year, days int) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE public.leave_balances
		 SET used = used + $4, updated_at = NOW()
		 WHERE user_id = $1 AND leave_type_id = $2 AND year = $3`,
		userID, leaveTypeID, year, days,
	)
	if err != nil {
		return errs.Internal("failed to increment leave balance")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("leave balance not found")
	}
	return nil
}

// Decrement subtracts days from the used count (on leave denial after prior approval).
func (r *LeaveBalanceRepo) Decrement(ctx context.Context, userID, leaveTypeID string, year, days int) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE public.leave_balances
		 SET used = GREATEST(used - $4, 0), updated_at = NOW()
		 WHERE user_id = $1 AND leave_type_id = $2 AND year = $3`,
		userID, leaveTypeID, year, days,
	)
	if err != nil {
		return errs.Internal("failed to decrement leave balance")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("leave balance not found")
	}
	return nil
}

func (r *LeaveBalanceRepo) Adjust(ctx context.Context, userID, leaveTypeID string, year, allocated int) error {
	// Upsert: create balance if missing, or update allocated.
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.leave_balances (user_id, org_id, leave_type_id, year, allocated)
		 SELECT $1, om.organization_id, $2, $3, $4
		 FROM public.organization_members om
		 WHERE om.user_id = $1
		 LIMIT 1
		 ON CONFLICT (user_id, leave_type_id, year)
		 DO UPDATE SET allocated = $4, updated_at = NOW()`,
		userID, leaveTypeID, year, allocated,
	)
	if err != nil {
		return errs.Internal("failed to adjust leave balance")
	}
	return nil
}

// GetBalance returns a single leave balance for computing remaining days.
func (r *LeaveBalanceRepo) GetBalance(ctx context.Context, userID, leaveTypeID string, year int) (*model.LeaveBalance, error) {
	var b model.LeaveBalance
	err := r.db.QueryRow(ctx,
		`SELECT lb.user_id, lb.leave_type_id, lt.name, lb.year, lb.allocated, lb.used
		 FROM public.leave_balances lb
		 JOIN public.leave_types lt ON lt.id = lb.leave_type_id
		 WHERE lb.user_id = $1 AND lb.leave_type_id = $2 AND lb.year = $3`,
		userID, leaveTypeID, year,
	).Scan(&b.UserID, &b.LeaveTypeID, &b.TypeName, &b.Year, &b.Allocated, &b.Used)
	if err != nil {
		return nil, errs.NotFound("leave balance not found")
	}
	b.Remaining = b.Allocated - b.Used
	return &b, nil
}
