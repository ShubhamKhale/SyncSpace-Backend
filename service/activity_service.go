package service

import (
	"context"

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
	OrgID   string // scope to boards in this org
	BoardID string // further filter to a specific board (and its tasks)
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

	f := database.ActivityFilter{
		OrgID:   q.OrgID,
		BoardID: q.BoardID,
		UserID:  q.UserID,
	}
	return s.repo.GetFiltered(ctx, f, q.Limit, q.Offset)
}
