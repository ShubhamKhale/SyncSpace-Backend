package service

import (
	"context"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/service/database"
)

// ActivityService handles activity log retrieval.
type ActivityService struct {
	repo *database.ActivityRepo
}

// NewActivityService creates an ActivityService with the provided repository.
func NewActivityService(repo *database.ActivityRepo) *ActivityService {
	return &ActivityService{repo: repo}
}

// ActivityQuery carries the caller-supplied filter and pagination parameters.
// Zero values mean "no constraint" for filters, "use default" for pagination.
type ActivityQuery struct {
	BoardID string // filter to a specific board (and its tasks)
	UserID  string // filter to a specific actor
	Limit   int    // page size; clamped to [1, 100], default 20
	Offset  int    // number of records to skip; default 0
}

// GetLogs returns a paginated, enriched list of activity logs matching the
// query filters together with the total matching count (all pages).
func (s *ActivityService) GetLogs(ctx context.Context, q ActivityQuery) (*model.ActivityLogPage, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}

	// At least one of board or user should be provided so we don't dump the
	// entire activity_logs table. Callers that legitimately want all logs for
	// the authenticated user should always pass UserID.
	if q.BoardID == "" && q.UserID == "" {
		return nil, errs.BadRequest("provide at least one of board_id or user_id")
	}

	f := database.ActivityFilter{
		BoardID: q.BoardID,
		UserID:  q.UserID,
	}
	return s.repo.GetFiltered(ctx, f, q.Limit, q.Offset)
}
