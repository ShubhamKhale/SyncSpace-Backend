package model

import (
	"encoding/json"
	"time"
)

// Flow represents a visual flow diagram (canvas of nodes and edges) attached to a board.
// Data is stored as raw JSONB so the shape can evolve without schema migrations.
//
// DB table (run before starting the server):
//
//	CREATE TABLE public.flows (
//	    id               TEXT        PRIMARY KEY,
//	    board_id         TEXT        NOT NULL REFERENCES public.boards(id) ON DELETE CASCADE,
//	    title            TEXT        NOT NULL DEFAULT '',
//	    data             JSONB       NOT NULL DEFAULT '{"nodes":[],"edges":[]}',
//	    version          INTEGER     NOT NULL DEFAULT 0,
//	    last_modified_by TEXT        NOT NULL REFERENCES public.users(id),
//	    created_by       TEXT        NOT NULL REFERENCES public.users(id),
//	    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
//	    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
//	);
//	CREATE INDEX idx_flows_board ON public.flows(board_id);
type Flow struct {
	ID             string          `json:"id"`
	BoardID        string          `json:"board_id"`
	Title          string          `json:"title"`
	Data           json.RawMessage `json:"data"`            // {"nodes":[...],"edges":[...]}
	Version        int             `json:"version"`
	LastModifiedBy string          `json:"last_modified_by"`
	CreatedBy      string          `json:"created_by"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
