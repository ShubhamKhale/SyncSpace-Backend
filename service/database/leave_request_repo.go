package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

type LeaveRequestRepo struct {
	db *pgxpool.Pool
}

func NewLeaveRequestRepo(db *pgxpool.Pool) *LeaveRequestRepo {
	return &LeaveRequestRepo{db: db}
}

func (r *LeaveRequestRepo) Create(ctx context.Context, req *model.LeaveRequest) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.leave_requests (id, user_id, org_id, leave_type_id, start_date, end_date, reason)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		req.ID, req.UserID, req.OrgID, req.LeaveTypeID, req.StartDate, req.EndDate, req.Reason,
	)
	if err != nil {
		return errs.Internal("failed to create leave request")
	}
	return nil
}

func (r *LeaveRequestRepo) GetByID(ctx context.Context, id string) (*model.LeaveRequest, error) {
	var req model.LeaveRequest
	err := r.db.QueryRow(ctx,
		`SELECT lr.id, lr.user_id, u.name, lr.org_id, lr.leave_type_id, lt.name,
		        lr.start_date::text, lr.end_date::text,
		        (lr.end_date - lr.start_date + 1),
		        lr.reason, lr.status, lr.reviewed_by, lr.reviewed_at, lr.created_at
		 FROM public.leave_requests lr
		 JOIN public.users u ON u.id = lr.user_id
		 JOIN public.leave_types lt ON lt.id = lr.leave_type_id
		 WHERE lr.id = $1`,
		id,
	).Scan(
		&req.ID, &req.UserID, &req.UserName, &req.OrgID, &req.LeaveTypeID, &req.TypeName,
		&req.StartDate, &req.EndDate, &req.Days,
		&req.Reason, &req.Status, &req.ReviewedBy, &req.ReviewedAt, &req.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("leave request not found")
		}
		return nil, errs.Internal("failed to get leave request")
	}
	return &req, nil
}

func (r *LeaveRequestRepo) ListByUser(ctx context.Context, userID, orgID string) ([]model.LeaveRequest, error) {
	rows, err := r.db.Query(ctx,
		`SELECT lr.id, lr.user_id, '' AS user_name, lr.org_id, lr.leave_type_id, lt.name,
		        lr.start_date::text, lr.end_date::text,
		        (lr.end_date - lr.start_date + 1),
		        lr.reason, lr.status, lr.reviewed_by, lr.reviewed_at, lr.created_at
		 FROM public.leave_requests lr
		 JOIN public.leave_types lt ON lt.id = lr.leave_type_id
		 WHERE lr.user_id = $1 AND lr.org_id = $2
		 ORDER BY lr.created_at DESC`,
		userID, orgID,
	)
	if err != nil {
		return nil, errs.Internal("failed to list leave requests")
	}
	defer rows.Close()
	return scanLeaveRequests(rows)
}

func (r *LeaveRequestRepo) ListByOrg(ctx context.Context, orgID, status string) ([]model.LeaveRequest, error) {
	var rows pgx.Rows
	var err error
	if status != "" {
		rows, err = r.db.Query(ctx,
			`SELECT lr.id, lr.user_id, u.name, lr.org_id, lr.leave_type_id, lt.name,
			        lr.start_date::text, lr.end_date::text,
			        (lr.end_date - lr.start_date + 1),
			        lr.reason, lr.status, lr.reviewed_by, lr.reviewed_at, lr.created_at
			 FROM public.leave_requests lr
			 JOIN public.users u ON u.id = lr.user_id
			 JOIN public.leave_types lt ON lt.id = lr.leave_type_id
			 WHERE lr.org_id = $1 AND lr.status = $2
			 ORDER BY lr.created_at DESC`,
			orgID, status,
		)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT lr.id, lr.user_id, u.name, lr.org_id, lr.leave_type_id, lt.name,
			        lr.start_date::text, lr.end_date::text,
			        (lr.end_date - lr.start_date + 1),
			        lr.reason, lr.status, lr.reviewed_by, lr.reviewed_at, lr.created_at
			 FROM public.leave_requests lr
			 JOIN public.users u ON u.id = lr.user_id
			 JOIN public.leave_types lt ON lt.id = lr.leave_type_id
			 WHERE lr.org_id = $1
			 ORDER BY lr.created_at DESC`,
			orgID,
		)
	}
	if err != nil {
		return nil, errs.Internal("failed to list org leave requests")
	}
	defer rows.Close()
	return scanLeaveRequests(rows)
}

func (r *LeaveRequestRepo) UpdateStatus(ctx context.Context, id, status, reviewedBy string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE public.leave_requests
		 SET status = $2, reviewed_by = $3, reviewed_at = NOW()
		 WHERE id = $1`,
		id, status, reviewedBy,
	)
	if err != nil {
		return errs.Internal("failed to update leave request status")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("leave request not found")
	}
	return nil
}

func (r *LeaveRequestRepo) GetApprovedForDate(ctx context.Context, orgID string, date time.Time) ([]model.LeaveRequest, error) {
	rows, err := r.db.Query(ctx,
		`SELECT lr.id, lr.user_id, u.name, lr.org_id, lr.leave_type_id, lt.name,
		        lr.start_date::text, lr.end_date::text,
		        (lr.end_date - lr.start_date + 1),
		        lr.reason, lr.status, lr.reviewed_by, lr.reviewed_at, lr.created_at
		 FROM public.leave_requests lr
		 JOIN public.users u ON u.id = lr.user_id
		 JOIN public.leave_types lt ON lt.id = lr.leave_type_id
		 WHERE lr.org_id = $1 AND lr.status = 'approved'
		   AND $2::date BETWEEN lr.start_date AND lr.end_date
		 ORDER BY lr.created_at DESC`,
		orgID, date,
	)
	if err != nil {
		return nil, errs.Internal("failed to get approved leaves for date")
	}
	defer rows.Close()
	return scanLeaveRequests(rows)
}

func scanLeaveRequests(rows pgx.Rows) ([]model.LeaveRequest, error) {
	var reqs []model.LeaveRequest
	for rows.Next() {
		var req model.LeaveRequest
		if err := rows.Scan(
			&req.ID, &req.UserID, &req.UserName, &req.OrgID, &req.LeaveTypeID, &req.TypeName,
			&req.StartDate, &req.EndDate, &req.Days,
			&req.Reason, &req.Status, &req.ReviewedBy, &req.ReviewedAt, &req.CreatedAt,
		); err != nil {
			return nil, errs.Internal("failed to scan leave request")
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}
