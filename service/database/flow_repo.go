package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// FlowRepo handles all database operations for the Flow entity.
type FlowRepo struct {
	db *pgxpool.Pool
}

// NewFlowRepo creates a FlowRepo with the provided connection pool.
func NewFlowRepo(db *pgxpool.Pool) *FlowRepo {
	return &FlowRepo{db: db}
}

// GetFlowByID returns the flow with the given ID, or errs.NotFound.
func (r *FlowRepo) GetFlowByID(ctx context.Context, id string) (*model.Flow, error) {
	f := &model.Flow{}
	err := r.db.QueryRow(ctx,
		`SELECT id, board_id, title, data, version,
		        last_modified_by, created_by, created_at, updated_at
		 FROM flows WHERE id = $1`,
		id,
	).Scan(
		&f.ID, &f.BoardID, &f.Title, &f.Data,
		&f.Version, &f.LastModifiedBy, &f.CreatedBy,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("flow not found")
		}
		return nil, errs.Internal("failed to fetch flow")
	}
	return f, nil
}

// UpdateFlow replaces the flow's data payload and increments its version.
// The WHERE clause includes the expected version for optimistic locking:
//   - 0 rows affected + row exists → errs.Conflict (concurrent edit)
//   - 0 rows affected + row missing → errs.NotFound
func (r *FlowRepo) UpdateFlow(ctx context.Context, f *model.Flow, expectedVersion int) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE flows
		 SET    data=$2, version=version+1, last_modified_by=$3, updated_at=$4
		 WHERE  id=$1 AND version=$5`,
		f.ID, f.Data, f.LastModifiedBy, f.UpdatedAt, expectedVersion,
	)
	if err != nil {
		return errs.Internal("failed to update flow")
	}
	if tag.RowsAffected() == 0 {
		// Determine whether the miss was a version conflict or a missing row.
		var exists bool
		_ = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM flows WHERE id=$1)`, f.ID).Scan(&exists)
		if !exists {
			return errs.NotFound("flow not found")
		}
		return errs.Conflict("flow was modified by another client; re-fetch the latest version and retry")
	}
	return nil
}
