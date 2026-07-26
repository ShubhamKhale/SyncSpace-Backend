package model

// BoardFlowVote is a single user's vote on a board flow diagram.
type BoardFlowVote struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Vote   string `json:"vote"` // "approve" | "review" | "reject"
}
