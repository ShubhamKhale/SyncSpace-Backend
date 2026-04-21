package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// LinkedResourceRepo handles all database operations for the linked_resources table.
type LinkedResourceRepo struct {
	db *pgxpool.Pool
}

// NewLinkedResourceRepo creates a LinkedResourceRepo with the provided pool.
func NewLinkedResourceRepo(db *pgxpool.Pool) *LinkedResourceRepo {
	return &LinkedResourceRepo{db: db}
}

// GetByBoard returns all linked resources for a board, ordered by creation time.
func (r *LinkedResourceRepo) GetByBoard(ctx context.Context, boardID string) ([]model.LinkedResource, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, board_id, label, url, created_by, created_at, updated_at
		 FROM   linked_resources
		 WHERE  board_id = $1
		 ORDER  BY created_at ASC`,
		boardID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query linked resources")
	}
	defer rows.Close()

	var resources []model.LinkedResource
	for rows.Next() {
		var lr model.LinkedResource
		if err := rows.Scan(
			&lr.ID, &lr.BoardID, &lr.Label, &lr.URL,
			&lr.CreatedBy, &lr.CreatedAt, &lr.UpdatedAt,
		); err != nil {
			return nil, errs.Internal("failed to scan linked resource row")
		}
		resources = append(resources, lr)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("linked resource query error")
	}
	if resources == nil {
		resources = []model.LinkedResource{}
	}
	return resources, nil
}

// ReplaceAll atomically replaces the full set of linked resources for a board.
// It runs a DELETE then a batch INSERT inside a single transaction, so the
// board is never left in a partial state.
//
// Passing an empty slice clears all resources for the board.
func (r *LinkedResourceRepo) ReplaceAll(ctx context.Context, boardID string, resources []model.LinkedResource) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Internal("failed to begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck — rollback on all non-commit paths

	// ── 1. Remove the existing set ────────────────────────────────────────────
	if _, err = tx.Exec(ctx,
		`DELETE FROM linked_resources WHERE board_id = $1`,
		boardID,
	); err != nil {
		return errs.Internal("failed to delete linked resources")
	}

	// ── 2. Insert the new set (no-op when slice is empty) ─────────────────────
	for _, lr := range resources {
		if _, err = tx.Exec(ctx,
			`INSERT INTO linked_resources
			     (id, board_id, label, url, created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			lr.ID, lr.BoardID, lr.Label, lr.URL,
			lr.CreatedBy, lr.CreatedAt, lr.UpdatedAt,
		); err != nil {
			return errs.Internal("failed to insert linked resource")
		}
	}

	return tx.Commit(ctx)
}
