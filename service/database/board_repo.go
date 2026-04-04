// Package database contains repository types that interact directly with
// the PostgreSQL database. Each repository receives a *pgxpool.Pool via
// constructor injection and exposes methods that accept a context.Context.
package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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

// InsertBoard persists a new board record to the database.
// TODO: replace the placeholder implementation with a real INSERT once the
// schema migration is in place.
func (r *BoardRepo) InsertBoard(ctx context.Context, board *model.Board) error {
	// Placeholder — real query:
	// _, err := r.db.Exec(ctx,
	//   `INSERT INTO boards (id, title, description, created_at)
	//    VALUES ($1, $2, $3, $4)`,
	//   board.ID, board.Title, board.Description, board.CreatedAt,
	// )
	// return err
	_ = ctx
	board.CreatedAt = time.Now()
	return nil
}

// GetBoards returns all board records from the database.
// TODO: replace the placeholder implementation with a real SELECT once the
// schema migration is in place.
func (r *BoardRepo) GetBoards(ctx context.Context) ([]model.Board, error) {
	// Placeholder — real query:
	// rows, err := r.db.Query(ctx, `SELECT id, title, description, created_at FROM boards`)
	// ...scan rows...
	_ = ctx
	return []model.Board{
		{ID: "mock-id-1", Title: "Mock Board", Description: "Placeholder from DB layer", CreatedAt: time.Now()},
	}, nil
}
