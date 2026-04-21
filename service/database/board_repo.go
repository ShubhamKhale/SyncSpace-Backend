// Package database contains repository types that interact directly with
// the PostgreSQL database. Each repository receives a *pgxpool.Pool via
// constructor injection and exposes methods that accept a context.Context.
package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// BoardRepo handles all database operations for the Board entity.
type BoardRepo struct {
	db *pgxpool.Pool
}

// NewBoardRepo creates a new BoardRepo with the provided connection pool.
func NewBoardRepo(db *pgxpool.Pool) *BoardRepo {
	return &BoardRepo{db: db}
}

// scanBoard reads one board row in the standard column order.
func scanBoard(row pgx.Row) (*model.Board, error) {
	b := &model.Board{}
	err := row.Scan(&b.ID, &b.Title, &b.Description, &b.OwnerID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("board not found")
		}
		return nil, errs.Internal("failed to scan board")
	}
	return b, nil
}

// InsertBoard persists a new board record.
func (r *BoardRepo) InsertBoard(ctx context.Context, board *model.Board) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO boards (id, title, description, owner_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		board.ID, board.Title, board.Description, board.OwnerID, board.CreatedAt, board.UpdatedAt,
	)
	if err != nil {
		return errs.Internal("failed to insert board")
	}
	return nil
}

// GetBoardByID returns a single board, or errs.NotFound.
func (r *BoardRepo) GetBoardByID(ctx context.Context, id string) (*model.Board, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, title, description, owner_id, created_at, updated_at
		 FROM boards WHERE id = $1`,
		id,
	)
	return scanBoard(row)
}

// GetBoardsByOwner returns all boards owned by the given user, newest first.
func (r *BoardRepo) GetBoardsByOwner(ctx context.Context, ownerID string) ([]model.Board, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, title, description, owner_id, created_at, updated_at
		 FROM   boards
		 WHERE  owner_id = $1
		 ORDER  BY created_at DESC`,
		ownerID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query boards")
	}
	defer rows.Close()

	return collectBoards(rows)
}

// GetRecentBoardsByOwner returns the most recently updated boards for the user.
func (r *BoardRepo) GetRecentBoardsByOwner(ctx context.Context, ownerID string, limit int) ([]model.Board, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, title, description, owner_id, created_at, updated_at
		 FROM   boards
		 WHERE  owner_id = $1
		 ORDER  BY updated_at DESC
		 LIMIT  $2`,
		ownerID, limit,
	)
	if err != nil {
		return nil, errs.Internal("failed to query recent boards")
	}
	defer rows.Close()

	return collectBoards(rows)
}

// collectBoards scans all rows from a board query into a slice.
func collectBoards(rows pgx.Rows) ([]model.Board, error) {
	var boards []model.Board
	for rows.Next() {
		var b model.Board
		if err := rows.Scan(&b.ID, &b.Title, &b.Description, &b.OwnerID, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, errs.Internal("failed to scan board row")
		}
		boards = append(boards, b)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("board query error")
	}
	return boards, nil
}
