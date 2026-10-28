package service

import (
	"context"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service/database"
)

type AttendanceService struct {
	repo         *database.AttendanceRepo
	leaveReqRepo *database.LeaveRequestRepo
	orgRepo      *database.OrgRepo
}

func NewAttendanceService(repo *database.AttendanceRepo, leaveReqRepo *database.LeaveRequestRepo, orgRepo *database.OrgRepo) *AttendanceService {
	return &AttendanceService{repo: repo, leaveReqRepo: leaveReqRepo, orgRepo: orgRepo}
}

func (s *AttendanceService) CheckIn(ctx context.Context, userID, orgID, note string) (*model.AttendanceLog, error) {
	existing, err := s.repo.GetTodayLog(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.CheckOutAt == nil {
		return nil, errs.BadRequest("already checked in today")
	}

	id := utils.NewUUID()
	if err := s.repo.CheckIn(ctx, id, userID, orgID, note); err != nil {
		return nil, err
	}
	return s.repo.GetTodayLog(ctx, userID, orgID)
}

func (s *AttendanceService) CheckOut(ctx context.Context, userID, orgID string) (*model.AttendanceLog, error) {
	if err := s.repo.CheckOut(ctx, userID, orgID); err != nil {
		return nil, err
	}
	return s.repo.GetTodayLog(ctx, userID, orgID)
}

func (s *AttendanceService) GetMyStatus(ctx context.Context, userID, orgID string) (*model.AttendanceLog, error) {
	return s.repo.GetTodayLog(ctx, userID, orgID)
}

func (s *AttendanceService) GetTeamStatus(ctx context.Context, orgID string) (*model.TeamAttendanceSummary, error) {
	members, err := s.orgRepo.GetOrgMembers(ctx, orgID)
	if err != nil {
		return nil, err
	}

	logs, err := s.repo.GetTeamToday(ctx, orgID)
	if err != nil {
		return nil, err
	}

	onLeaveReqs, err := s.leaveReqRepo.GetApprovedForDate(ctx, orgID, time.Now())
	if err != nil {
		return nil, err
	}

	checkedInMap := make(map[string]*model.AttendanceLog)
	for i := range logs {
		checkedInMap[logs[i].UserID] = &logs[i]
	}
	onLeaveSet := make(map[string]bool)
	for _, lr := range onLeaveReqs {
		onLeaveSet[lr.UserID] = true
	}

	summary := &model.TeamAttendanceSummary{Total: len(members)}
	for _, m := range members {
		status := model.AttendanceStatus{
			UserID:    m.UserID,
			Name:      m.Name,
			Email:     m.Email,
			AvatarURL: m.AvatarURL,
		}
		if log, ok := checkedInMap[m.UserID]; ok && log.CheckOutAt == nil {
			status.Status = "checked_in"
			status.CheckInAt = &log.CheckInAt
			summary.CheckedIn++
		} else if onLeaveSet[m.UserID] {
			status.Status = "on_leave"
			summary.OnLeave++
		} else {
			status.Status = "absent"
			summary.Absent++
		}
		summary.Members = append(summary.Members, status)
	}
	return summary, nil
}

func (s *AttendanceService) GetMemberAvailability(ctx context.Context, orgID string) ([]model.AttendanceStatus, error) {
	team, err := s.GetTeamStatus(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return team.Members, nil
}
