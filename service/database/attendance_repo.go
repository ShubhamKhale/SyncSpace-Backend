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

type AttendanceRepo struct {
	db *pgxpool.Pool
}

func NewAttendanceRepo(db *pgxpool.Pool) *AttendanceRepo {
	return &AttendanceRepo{db: db}
}

func (r *AttendanceRepo) CheckIn(ctx context.Context, id, userID, orgID, note string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.attendance_logs (id, user_id, org_id, note)
		 VALUES ($1, $2, $3, $4)`,
		id, userID, orgID, note,
	)
	if err != nil {
		return errs.Internal("failed to check in")
	}
	return nil
}

func (r *AttendanceRepo) CheckOut(ctx context.Context, userID, orgID string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE public.attendance_logs
		 SET check_out_at = NOW()
		 WHERE user_id = $1 AND org_id = $2
		   AND check_out_at IS NULL
		   AND check_in_at::date = CURRENT_DATE`,
		userID, orgID,
	)
	if err != nil {
		return errs.Internal("failed to check out")
	}
	if tag.RowsAffected() == 0 {
		return errs.BadRequest("no open check-in found for today")
	}
	return nil
}

func (r *AttendanceRepo) GetTodayLog(ctx context.Context, userID, orgID string) (*model.AttendanceLog, error) {
	var l model.AttendanceLog
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, org_id, check_in_at, check_out_at, note, created_at
		 FROM public.attendance_logs
		 WHERE user_id = $1 AND org_id = $2
		   AND check_in_at::date = CURRENT_DATE
		 ORDER BY check_in_at DESC LIMIT 1`,
		userID, orgID,
	).Scan(&l.ID, &l.UserID, &l.OrgID, &l.CheckInAt, &l.CheckOutAt, &l.Note, &l.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, errs.Internal("failed to get today attendance")
	}
	return &l, nil
}

func (r *AttendanceRepo) GetTeamToday(ctx context.Context, orgID string) ([]model.AttendanceLog, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, org_id, check_in_at, check_out_at, note, created_at
		 FROM public.attendance_logs
		 WHERE org_id = $1
		   AND check_in_at::date = CURRENT_DATE
		 ORDER BY check_in_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, errs.Internal("failed to get team attendance")
	}
	defer rows.Close()

	var logs []model.AttendanceLog
	for rows.Next() {
		var l model.AttendanceLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.OrgID, &l.CheckInAt, &l.CheckOutAt, &l.Note, &l.CreatedAt); err != nil {
			return nil, errs.Internal("failed to scan attendance log")
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (r *AttendanceRepo) GetUserLogs(ctx context.Context, userID, orgID string, from, to time.Time) ([]model.AttendanceLog, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, org_id, check_in_at, check_out_at, note, created_at
		 FROM public.attendance_logs
		 WHERE user_id = $1 AND org_id = $2
		   AND check_in_at >= $3 AND check_in_at < $4
		 ORDER BY check_in_at DESC`,
		userID, orgID, from, to,
	)
	if err != nil {
		return nil, errs.Internal("failed to get user attendance logs")
	}
	defer rows.Close()

	var logs []model.AttendanceLog
	for rows.Next() {
		var l model.AttendanceLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.OrgID, &l.CheckInAt, &l.CheckOutAt, &l.Note, &l.CreatedAt); err != nil {
			return nil, errs.Internal("failed to scan attendance log")
		}
		logs = append(logs, l)
	}
	return logs, nil
}
