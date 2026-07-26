package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/model"
)

// UserPrefsRepo handles database operations for the user_notification_prefs table.
type UserPrefsRepo struct {
	db *pgxpool.Pool
}

// NewUserPrefsRepo creates a UserPrefsRepo with the provided connection pool.
func NewUserPrefsRepo(db *pgxpool.Pool) *UserPrefsRepo {
	return &UserPrefsRepo{db: db}
}

// GetByUserID returns the notification preferences for userID.
// Returns model.DefaultNotificationPrefs() if no row exists yet.
func (r *UserPrefsRepo) GetByUserID(ctx context.Context, userID string) (*model.NotificationPrefs, error) {
	p := &model.NotificationPrefs{}
	err := r.db.QueryRow(ctx,
		`SELECT comments, invites, product_updates, updated_at
		 FROM public.user_notification_prefs
		 WHERE user_id = $1`,
		userID,
	).Scan(&p.Comments, &p.Invites, &p.ProductUpdates, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.DefaultNotificationPrefs(), nil
		}
		return nil, err
	}
	return p, nil
}

// Upsert inserts or updates the notification preferences for userID (idempotent).
func (r *UserPrefsRepo) Upsert(ctx context.Context, userID string, prefs *model.NotificationPrefs) (*model.NotificationPrefs, error) {
	prefs.UpdatedAt = time.Now()
	err := r.db.QueryRow(ctx,
		`INSERT INTO public.user_notification_prefs
		     (user_id, comments, invites, product_updates, updated_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id) DO UPDATE
		     SET comments        = EXCLUDED.comments,
		         invites         = EXCLUDED.invites,
		         product_updates = EXCLUDED.product_updates,
		         updated_at      = EXCLUDED.updated_at
		 RETURNING comments, invites, product_updates, updated_at`,
		userID, prefs.Comments, prefs.Invites, prefs.ProductUpdates, prefs.UpdatedAt,
	).Scan(&prefs.Comments, &prefs.Invites, &prefs.ProductUpdates, &prefs.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return prefs, nil
}
