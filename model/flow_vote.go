package model

import "time"

// FlowVote records a single user's vote on a flow diagram.
// The (flow_id, user_id) pair is unique — casting a new vote replaces the old one.
//
// DB table (run before starting the server):
//
//	CREATE TABLE public.flow_votes (
//	    flow_id    TEXT        NOT NULL REFERENCES public.flows(id)  ON DELETE CASCADE,
//	    user_id    TEXT        NOT NULL REFERENCES public.users(id)  ON DELETE CASCADE,
//	    vote_type  TEXT        NOT NULL CHECK (vote_type IN ('up','down')),
//	    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
//	    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
//	    PRIMARY KEY (flow_id, user_id)
//	);
type FlowVote struct {
	FlowID    string    `json:"flow_id"`
	UserID    string    `json:"user_id"`
	VoteType  string    `json:"vote_type"` // "up" | "down"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FlowVoteSummary is the read projection returned by GET /api/flows/:id/votes.
// MyVote is the authenticated caller's current vote ("up", "down", or "" if not voted).
type FlowVoteSummary struct {
	Up     int    `json:"up"`
	Down   int    `json:"down"`
	MyVote string `json:"my_vote"` // "up" | "down" | ""
}
