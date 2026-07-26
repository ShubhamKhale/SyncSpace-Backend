package service

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"syncspace-backend/errs"
	"syncspace-backend/service/database"
)

// Participant is the full presence shape returned by REST and emitted by WebSocket events.
type Participant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Initials  string `json:"initials"`
	Color     string `json:"color"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	IsOnline  bool   `json:"isOnline"`
}

var presenceColors = []string{
	"#3b82f6", "#ef4444", "#10b981", "#f59e0b",
	"#8b5cf6", "#ec4899", "#06b6d4", "#84cc16",
}

func colorForUser(userID string) string {
	h := fnv.New32a()
	h.Write([]byte(userID))
	return presenceColors[h.Sum32()%uint32(len(presenceColors))]
}

func initialsFor(name string) string {
	words := strings.Fields(name)
	var b strings.Builder
	for _, w := range words {
		if len(w) > 0 {
			b.WriteRune([]rune(strings.ToUpper(w))[0])
		}
	}
	return b.String()
}

func presenceKey(flowID string) string { return "flow:" + flowID + ":participants" }

// PresenceService manages Redis-backed participant state for flow diagram rooms.
type PresenceService struct {
	rdb      *redis.Client
	userRepo *database.UserRepo
}

// NewPresenceService wires the service to Redis and the user repository.
func NewPresenceService(rdb *redis.Client, userRepo *database.UserRepo) *PresenceService {
	return &PresenceService{rdb: rdb, userRepo: userRepo}
}

// Join fetches the user's profile, builds the Participant record, stores it in Redis,
// and returns it for the HTTP response. The caller should also emit a WS presence.join event.
func (s *PresenceService) Join(ctx context.Context, flowID, userID string) (*Participant, error) {
	u, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	p := &Participant{
		ID:        u.ID,
		Name:      u.Name,
		Initials:  initialsFor(u.Name),
		Color:     colorForUser(u.ID),
		AvatarURL: u.AvatarURL,
		IsOnline:  true,
	}

	raw, err := json.Marshal(p)
	if err != nil {
		return nil, errs.Internal("failed to marshal participant")
	}

	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, presenceKey(flowID), userID, raw)
	pipe.Expire(ctx, presenceKey(flowID), 24*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, errs.Internal("failed to store participant")
	}

	return p, nil
}

// ListParticipants returns all participants currently in a flow room.
func (s *PresenceService) ListParticipants(ctx context.Context, flowID string) ([]Participant, error) {
	all, err := s.rdb.HGetAll(ctx, presenceKey(flowID)).Result()
	if err != nil {
		return nil, errs.Internal("failed to get participants")
	}

	participants := make([]Participant, 0, len(all))
	for _, raw := range all {
		var p Participant
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			continue
		}
		participants = append(participants, p)
	}
	return participants, nil
}

// Leave removes a participant from a flow room.
func (s *PresenceService) Leave(ctx context.Context, flowID, userID string) error {
	if err := s.rdb.HDel(ctx, presenceKey(flowID), userID).Err(); err != nil {
		return errs.Internal("failed to remove participant")
	}
	return nil
}
