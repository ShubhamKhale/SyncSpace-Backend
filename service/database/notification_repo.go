package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// NotificationRepo handles all database operations for the notifications table.
type NotificationRepo struct {
	db *pgxpool.Pool
}

// NewNotificationRepo creates a NotificationRepo with the provided connection pool.
func NewNotificationRepo(db *pgxpool.Pool) *NotificationRepo {
	return &NotificationRepo{db: db}
}

// GetByUser returns a paginated slice of notifications for the user together
// with the global unread count and total count.
//
// Optimisation: a single query computes both aggregate values using PostgreSQL
// window functions that run before LIMIT/OFFSET, so the database scans the
// user's notification rows only once:
//
//	COUNT(*) FILTER (WHERE NOT is_read) OVER ()  → total unread (all pages)
//	COUNT(*)                            OVER ()  → total rows   (all pages)
func (r *NotificationRepo) GetByUser(
	ctx context.Context,
	userID string,
	limit, offset int,
) (*model.NotificationPage, error) {
	rows, err := r.db.Query(ctx,
		`SELECT
		     id, user_id, title, body, is_read,
		     COALESCE(entity_type, ''), COALESCE(entity_id, ''),
		     created_at,
		     COUNT(*) FILTER (WHERE NOT is_read) OVER () AS unread_count,
		     COUNT(*)                            OVER () AS total_count
		 FROM   public.notifications
		 WHERE  user_id = $1
		 ORDER  BY created_at DESC
		 LIMIT  $2
		 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, errs.Internal("failed to query notifications")
	}
	defer rows.Close()

	page := &model.NotificationPage{
		Notifications: []model.Notification{},
	}

	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.Title, &n.Body, &n.IsRead,
			&n.EntityType, &n.EntityID, &n.CreatedAt,
			&page.UnreadCount, &page.Total,
		); err != nil {
			return nil, errs.Internal("failed to scan notification row")
		}
		page.Notifications = append(page.Notifications, n)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("notification query error")
	}
	return page, nil
}

// MarkRead marks a single notification as read.
// The WHERE clause includes user_id so users can only mark their own notifications.
// Returns errs.NotFound when no matching row exists (wrong id or wrong user).
func (r *NotificationRepo) MarkRead(ctx context.Context, notifID, userID string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE public.notifications
		 SET    is_read = TRUE
		 WHERE  id = $1 AND user_id = $2`,
		notifID, userID,
	)
	if err != nil {
		return errs.Internal("failed to mark notification as read")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("notification not found")
	}
	return nil
}

// MarkAllRead marks every unread notification for the user as read in one
// statement. Returns the number of rows updated.
func (r *NotificationRepo) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE public.notifications
		 SET    is_read = TRUE
		 WHERE  user_id = $1 AND is_read = FALSE`,
		userID,
	)
	if err != nil {
		return 0, errs.Internal("failed to mark all notifications as read")
	}
	return tag.RowsAffected(), nil
}

// InsertNotification persists a new notification row.
// Intended to be called by other services (e.g. OrgService after an invite).
func (r *NotificationRepo) InsertNotification(ctx context.Context, n *model.Notification) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.notifications
		     (id, user_id, title, body, is_read, entity_type, entity_id, created_at)
		 VALUES ($1, $2, $3, $4, FALSE, $5, $6, $7)`,
		n.ID, n.UserID, n.Title, n.Body, n.EntityType, n.EntityID, n.CreatedAt,
	)
	if err != nil {
		return errs.Internal("failed to insert notification")
	}
	return nil
}

// GetUnreadCount returns only the unread notification count for the user.
// Use this when the full notification list is not needed (e.g. badge refresh).
// The partial index idx_notifications_unread makes this O(unread) not O(total).
func (r *NotificationRepo) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)::int
		 FROM   public.notifications
		 WHERE  user_id = $1 AND is_read = FALSE`,
		userID,
	).Scan(&count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, errs.Internal("failed to count unread notifications")
	}
	return count, nil
}
