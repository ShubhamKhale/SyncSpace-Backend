package database

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// BoardFlowRepo handles database operations for board_flows and flow_diagrams tables.
type BoardFlowRepo struct {
	db *pgxpool.Pool
}

// NewBoardFlowRepo creates a BoardFlowRepo with the provided connection pool.
func NewBoardFlowRepo(db *pgxpool.Pool) *BoardFlowRepo {
	return &BoardFlowRepo{db: db}
}

func scanFlow(scan func(dest ...any) error) (*model.BoardFlow, error) {
	f := &model.BoardFlow{}
	err := scan(&f.ID, &f.BoardID, &f.Name, &f.NodeCount, &f.EdgeCount, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("flow not found")
		}
		return nil, errs.Internal("failed to scan flow")
	}
	return f, nil
}

func jsonArrayLen(raw json.RawMessage) int {
	if raw == nil {
		return 0
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return 0
	}
	return len(arr)
}

const flowCols = `id, board_id, name, node_count, edge_count, created_at, updated_at`

// ListByBoard returns all flow metadata for a board, newest-updated first.
func (r *BoardFlowRepo) ListByBoard(ctx context.Context, boardID string) ([]model.BoardFlow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+flowCols+`
		 FROM   public.board_flows
		 WHERE  board_id = $1
		 ORDER  BY updated_at DESC`,
		boardID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query flows")
	}
	defer rows.Close()

	flows := []model.BoardFlow{}
	for rows.Next() {
		f := model.BoardFlow{}
		if err := rows.Scan(&f.ID, &f.BoardID, &f.Name, &f.NodeCount, &f.EdgeCount, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, errs.Internal("failed to scan flow row")
		}
		flows = append(flows, f)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("flow list query error")
	}
	return flows, nil
}

// Create inserts a new board_flows row and its default flow_diagrams row in one transaction.
func (r *BoardFlowRepo) Create(ctx context.Context, f *model.BoardFlow) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Internal("failed to begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx,
		`INSERT INTO public.board_flows (id, board_id, name, node_count, edge_count, created_at, updated_at)
		 VALUES ($1, $2, $3, 0, 0, $4, $5)`,
		f.ID, f.BoardID, f.Name, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return errs.Internal("failed to create flow")
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at)
		 VALUES ($1, '{"title":"Untitled Diagram","nodes":[],"edges":[]}', NOW())`,
		f.ID,
	)
	if err != nil {
		return errs.Internal("failed to create diagram record")
	}

	if err := tx.Commit(ctx); err != nil {
		return errs.Internal("failed to commit flow creation")
	}
	return nil
}

// GetByID returns the flow metadata, or errs.NotFound if id/boardID don't match.
func (r *BoardFlowRepo) GetByID(ctx context.Context, boardID, flowID string) (*model.BoardFlow, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+flowCols+` FROM public.board_flows WHERE id = $1 AND board_id = $2`,
		flowID, boardID,
	)
	return scanFlow(row.Scan)
}

// Rename updates the flow name and returns the updated row.
func (r *BoardFlowRepo) Rename(ctx context.Context, boardID, flowID, name string) (*model.BoardFlow, error) {
	row := r.db.QueryRow(ctx,
		`UPDATE public.board_flows
		 SET    name = $3, updated_at = NOW()
		 WHERE  id = $1 AND board_id = $2
		 RETURNING `+flowCols,
		flowID, boardID, name,
	)
	return scanFlow(row.Scan)
}

// Delete permanently removes a flow (cascade deletes its flow_diagrams row).
func (r *BoardFlowRepo) Delete(ctx context.Context, boardID, flowID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM public.board_flows WHERE id = $1 AND board_id = $2`,
		flowID, boardID,
	)
	if err != nil {
		return errs.Internal("failed to delete flow")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("flow not found")
	}
	return nil
}

// Duplicate copies a flow's metadata and diagram data into a new row.
// Returns errs.NotFound if the source flow doesn't exist in boardID.
func (r *BoardFlowRepo) Duplicate(ctx context.Context, boardID, sourceFlowID, newID, newName string) (*model.BoardFlow, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errs.Internal("failed to begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	row := tx.QueryRow(ctx,
		`INSERT INTO public.board_flows (id, board_id, name, node_count, edge_count, created_at, updated_at)
		 SELECT $1, board_id, $2, node_count, edge_count, NOW(), NOW()
		 FROM   public.board_flows
		 WHERE  id = $3 AND board_id = $4
		 RETURNING `+flowCols,
		newID, newName, sourceFlowID, boardID,
	)
	f, err := scanFlow(row.Scan)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at)
		 SELECT $1, diagram, NOW()
		 FROM   public.flow_diagrams
		 WHERE  flow_id = $2
		 ON CONFLICT (flow_id) DO NOTHING`,
		newID, sourceFlowID,
	)
	if err != nil {
		return nil, errs.Internal("failed to copy diagram data")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errs.Internal("failed to commit duplicate")
	}
	return f, nil
}

// GetDiagram returns the diagram data for a flow.
// Returns a default empty diagram when no diagram has been saved yet.
func (r *BoardFlowRepo) GetDiagram(ctx context.Context, flowID string) (*model.DiagramData, error) {
	var raw json.RawMessage
	err := r.db.QueryRow(ctx,
		`SELECT diagram FROM public.flow_diagrams WHERE flow_id = $1`,
		flowID,
	).Scan(&raw)

	if errors.Is(err, pgx.ErrNoRows) {
		return &model.DiagramData{
			Title: "Untitled Diagram",
			Nodes: json.RawMessage("[]"),
			Edges: json.RawMessage("[]"),
		}, nil
	}
	if err != nil {
		return nil, errs.Internal("failed to get diagram")
	}

	d := &model.DiagramData{}
	if err := json.Unmarshal(raw, d); err != nil {
		return nil, errs.Internal("failed to parse diagram data")
	}
	if d.Nodes == nil {
		d.Nodes = json.RawMessage("[]")
	}
	if d.Edges == nil {
		d.Edges = json.RawMessage("[]")
	}
	return d, nil
}

// UpsertDiagram saves diagram data and updates node_count/edge_count on the flow row.
// Returns (updatedAt, nodeCount, edgeCount, error).
func (r *BoardFlowRepo) UpsertDiagram(ctx context.Context, flowID string, d *model.DiagramData) (time.Time, int, int, error) {
	nodeCount := jsonArrayLen(d.Nodes)
	edgeCount := jsonArrayLen(d.Edges)

	diagramJSON, err := json.Marshal(d)
	if err != nil {
		return time.Time{}, 0, 0, errs.Internal("failed to marshal diagram")
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO public.flow_diagrams (flow_id, diagram, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (flow_id) DO UPDATE SET diagram = $2, updated_at = NOW()`,
		flowID, diagramJSON,
	)
	if err != nil {
		return time.Time{}, 0, 0, errs.Internal("failed to upsert diagram")
	}

	var updatedAt time.Time
	err = r.db.QueryRow(ctx,
		`UPDATE public.board_flows
		 SET node_count = $2, edge_count = $3, updated_at = NOW()
		 WHERE id = $1
		 RETURNING updated_at`,
		flowID, nodeCount, edgeCount,
	).Scan(&updatedAt)
	if err != nil {
		return time.Time{}, 0, 0, errs.Internal("failed to update flow counts")
	}

	return updatedAt, nodeCount, edgeCount, nil
}
