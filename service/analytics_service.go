package service

import (
	"context"
	"fmt"
	"time"

	"syncspace-backend/model"
	"syncspace-backend/service/database"
)

// AnalyticsService provides aggregated metrics for the analytics endpoints.
// All public methods convert the human-readable period string into a concrete
// start time before delegating to the repository layer.
type AnalyticsService struct {
	repo *database.AnalyticsRepo
}

// NewAnalyticsService creates an AnalyticsService with the provided repository.
func NewAnalyticsService(repo *database.AnalyticsRepo) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

// ── Period helper ─────────────────────────────────────────────────────────────

// periodStart converts a period name into a start time (midnight UTC).
//
//	"week"    → 7 days ago  (default)
//	"month"   → 30 days ago
//	"quarter" → 90 days ago
func periodStart(period string) time.Time {
	days := 7
	switch period {
	case "month":
		days = 30
	case "quarter":
		days = 90
	}
	return time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -days)
}

// boardFilter converts an empty string into a nil pointer (= no filter).
func boardFilter(boardID string) *string {
	if boardID == "" {
		return nil
	}
	return &boardID
}

// ── Service methods ───────────────────────────────────────────────────────────

// GetTaskCompletionTrend returns daily created/completed task counts for the
// requested period. The series always covers every calendar day (zeros on
// quiet days) so the client can render a continuous chart without gap-filling.
func (s *AnalyticsService) GetTaskCompletionTrend(
	ctx context.Context,
	orgID, period, boardID string,
) ([]model.TaskTrendPoint, error) {
	return s.repo.GetTaskCompletionTrend(ctx, orgID, periodStart(period), boardFilter(boardID))
}

// GetTaskDistribution returns current task counts bucketed by stage and by
// priority. Results reflect the live state of the database, not a time window.
func (s *AnalyticsService) GetTaskDistribution(
	ctx context.Context,
	orgID, boardID string,
) (*model.TaskDistribution, error) {
	return s.repo.GetTaskDistribution(ctx, orgID, boardFilter(boardID))
}

// GetBoardActivity returns daily task-creation and task-update counts for the
// requested period. tasks_updated only counts genuine edits (days where
// updated_at differs from created_at).
func (s *AnalyticsService) GetBoardActivity(
	ctx context.Context,
	orgID, period, boardID string,
) ([]model.BoardActivityPoint, error) {
	return s.repo.GetBoardActivity(ctx, orgID, periodStart(period), boardFilter(boardID))
}

// GetTeamContribution returns per-member task statistics: total created,
// completed (stage='done'), and overdue (past due_date and not done).
// Results are ordered by tasks_created descending.
func (s *AnalyticsService) GetTeamContribution(
	ctx context.Context,
	orgID, boardID string,
) ([]model.TeamMemberContribution, error) {
	return s.repo.GetTeamContribution(ctx, orgID, boardFilter(boardID))
}

// ── New dashboard analytics methods ──────────────────────────────────────────

func (s *AnalyticsService) GetMonthlyTaskTrend(ctx context.Context, orgID string) ([]model.MonthlyTrendPoint, error) {
	return s.repo.GetMonthlyTaskTrend(ctx, orgID)
}

func (s *AnalyticsService) GetTaskDistributionByStage(ctx context.Context, orgID string) ([]model.DistributionItem, error) {
	return s.repo.GetTaskDistributionByStage(ctx, orgID)
}

func (s *AnalyticsService) GetBoardActivityStats(ctx context.Context, orgID string) ([]model.BoardActivityItem, error) {
	return s.repo.GetBoardActivityStats(ctx, orgID)
}

func (s *AnalyticsService) GetTeamContributionByPhase(ctx context.Context, orgID string) (*model.TeamContributionResult, error) {
	return s.repo.GetTeamContributionByPhase(ctx, orgID)
}

func (s *AnalyticsService) GetDashboardAnalytics(ctx context.Context, orgID string) (*model.DashboardAnalytics, error) {
	trend, err := s.repo.GetMonthlyTaskTrend(ctx, orgID)
	if err != nil {
		fmt.Println("get monthly task trend error:- ", err.Error())
		return nil, err
	}
	dist, err := s.repo.GetTaskDistributionByStage(ctx, orgID)
	if err != nil {
		fmt.Println("get task distribution error:- ", err.Error())
		return nil, err
	}
	activity, err := s.repo.GetBoardActivityStats(ctx, orgID)
	if err != nil {
		fmt.Println("get board activity error:- ", err.Error())
		return nil, err
	}
	team, err := s.repo.GetTeamContributionByPhase(ctx, orgID)
	if err != nil {
		fmt.Println("get team contribution error:- ", err.Error())
		return nil, err
	}
	return &model.DashboardAnalytics{
		TaskCompletionTrend: trend,
		TaskDistribution:    dist,
		BoardActivity:       activity,
		TeamContribution:    *team,
	}, nil
}
