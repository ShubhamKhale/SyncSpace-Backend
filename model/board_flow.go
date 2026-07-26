package model

import (
	"encoding/json"
	"time"
)

// BoardFlow is the metadata view of a board-scoped flow diagram.
// Diagram data (nodes/edges) is stored separately in the flow_diagrams table.
type BoardFlow struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"boardId"`
	Name      string    `json:"name"`
	NodeCount int       `json:"nodeCount"`
	EdgeCount int       `json:"edgeCount"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// DiagramData is the full canvas state stored in the flow_diagrams table.
type DiagramData struct {
	Title string          `json:"title"`
	Nodes json.RawMessage `json:"nodes"`
	Edges json.RawMessage `json:"edges"`
}
