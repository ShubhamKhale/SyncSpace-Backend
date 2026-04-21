package service

import (
	"context"
	"time"

	"syncspace-backend/constants"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service/database"
)

// ── Action constants ──────────────────────────────────────────────────────────
// Use these when calling ActivityLogger.Log to keep action strings consistent
// across all services.

const (
	ActionCreated  = "created"
	ActionUpdated  = "updated"
	ActionDeleted  = "deleted"
	ActionMoved    = "moved"    // task stage change
	ActionInvited  = "invited"  // org member invite
	ActionAssigned = "assigned" // task assignee change
	ActionRemoved  = "removed"  // member removed
)

// ── Entity type constants ─────────────────────────────────────────────────────

const (
	EntityBoard = "board"
	EntityTask  = "task"
	EntityOrg   = "org"
)

// ── ActivityLogger ────────────────────────────────────────────────────────────

// ActivityLogger is a lightweight helper that other services use to record
// mutations without importing ActivityService (which would create a cycle).
//
// Inject it into any service that performs writes:
//
//	type BoardService struct {
//	    repo   *database.BoardRepo
//	    logger *ActivityLogger
//	}
//
// Then call after a successful mutation:
//
//	s.logger.Log(ctx, callerID, EntityBoard, board.ID, ActionCreated, nil)
//
// Logging failure never fails the main operation — the error is written to
// the application log and the call returns silently.
type ActivityLogger struct {
	repo *database.ActivityRepo
}

// NewActivityLogger creates an ActivityLogger backed by the given repository.
func NewActivityLogger(repo *database.ActivityRepo) *ActivityLogger {
	return &ActivityLogger{repo: repo}
}

// Log records one activity event. meta may be nil for events with no extra context.
// The call is synchronous but non-fatal: if the insert fails the error is
// logged internally and the caller receives no error.
func (l *ActivityLogger) Log(
	ctx context.Context,
	actorID, entityType, entityID, action string,
	meta map[string]any,
) {
	if meta == nil {
		meta = map[string]any{}
	}
	al := &model.ActivityLog{
		ID:         utils.NewUUID(),
		EntityType: entityType,
		EntityID:   entityID,
		ActorID:    actorID,
		Action:     action,
		Metadata:   meta,
		CreatedAt:  time.Now(),
	}
	if err := l.repo.InsertActivityLog(ctx, al); err != nil {
		utils.Error(constants.LogTagActivity,
			"activity log insert failed ("+entityType+"/"+entityID+"/"+action+")", err)
	}
}

// LogBoard is a convenience wrapper for board-entity events.
func (l *ActivityLogger) LogBoard(ctx context.Context, actorID, boardID, action string, meta map[string]any) {
	l.Log(ctx, actorID, EntityBoard, boardID, action, meta)
}

// LogTask is a convenience wrapper for task-entity events.
func (l *ActivityLogger) LogTask(ctx context.Context, actorID, taskID, action string, meta map[string]any) {
	l.Log(ctx, actorID, EntityTask, taskID, action, meta)
}

// LogOrg is a convenience wrapper for organisation-entity events.
func (l *ActivityLogger) LogOrg(ctx context.Context, actorID, orgID, action string, meta map[string]any) {
	l.Log(ctx, actorID, EntityOrg, orgID, action, meta)
}
