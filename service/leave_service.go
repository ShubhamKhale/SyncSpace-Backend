package service

import (
	"context"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service/database"
)

type LeaveService struct {
	typeRepo    *database.LeaveTypeRepo
	balanceRepo *database.LeaveBalanceRepo
	requestRepo *database.LeaveRequestRepo
}

func NewLeaveService(typeRepo *database.LeaveTypeRepo, balanceRepo *database.LeaveBalanceRepo, requestRepo *database.LeaveRequestRepo) *LeaveService {
	return &LeaveService{typeRepo: typeRepo, balanceRepo: balanceRepo, requestRepo: requestRepo}
}

func (s *LeaveService) GetLeaveTypes(ctx context.Context, orgID string) ([]model.LeaveType, error) {
	types, err := s.typeRepo.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if len(types) == 0 {
		if err := s.typeRepo.SeedDefaults(ctx, orgID); err != nil {
			return nil, err
		}
		return s.typeRepo.List(ctx, orgID)
	}
	return types, nil
}

func (s *LeaveService) CreateLeaveType(ctx context.Context, orgID, name string, defaultDays int) (*model.LeaveType, error) {
	lt := &model.LeaveType{
		ID:          utils.NewUUID(),
		OrgID:       orgID,
		Name:        name,
		DefaultDays: defaultDays,
	}
	if err := s.typeRepo.Create(ctx, lt); err != nil {
		return nil, err
	}
	return lt, nil
}

func (s *LeaveService) UpdateLeaveType(ctx context.Context, id, name string, defaultDays int) error {
	return s.typeRepo.Update(ctx, id, name, defaultDays)
}

func (s *LeaveService) GetMyBalances(ctx context.Context, userID, orgID string) ([]model.LeaveBalance, error) {
	year := time.Now().Year()
	balances, err := s.balanceRepo.GetByUser(ctx, userID, orgID, year)
	if err != nil {
		return nil, err
	}
	if len(balances) == 0 {
		if _, err := s.GetLeaveTypes(ctx, orgID); err != nil {
			return nil, err
		}
		if err := s.balanceRepo.InitForUser(ctx, userID, orgID, year); err != nil {
			return nil, err
		}
		return s.balanceRepo.GetByUser(ctx, userID, orgID, year)
	}
	return balances, nil
}

func (s *LeaveService) AdjustBalance(ctx context.Context, userID, leaveTypeID string, allocated int) error {
	year := time.Now().Year()
	return s.balanceRepo.Adjust(ctx, userID, leaveTypeID, year, allocated)
}

func (s *LeaveService) RequestLeave(ctx context.Context, userID, orgID, leaveTypeID, startDate, endDate, reason string) (*model.LeaveRequest, error) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, errs.BadRequest("start_date must be YYYY-MM-DD format")
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, errs.BadRequest("end_date must be YYYY-MM-DD format")
	}
	if end.Before(start) {
		return nil, errs.BadRequest("end_date must be on or after start_date")
	}

	days := int(end.Sub(start).Hours()/24) + 1

	year := start.Year()
	balance, err := s.balanceRepo.GetBalance(ctx, userID, leaveTypeID, year)
	if err != nil {
		return nil, err
	}
	if balance.Remaining < days {
		return nil, errs.BadRequest("insufficient leave balance")
	}

	req := &model.LeaveRequest{
		ID:          utils.NewUUID(),
		UserID:      userID,
		OrgID:       orgID,
		LeaveTypeID: leaveTypeID,
		StartDate:   startDate,
		EndDate:     endDate,
		Days:        days,
		Reason:      reason,
		Status:      "pending",
	}
	if err := s.requestRepo.Create(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *LeaveService) GetMyRequests(ctx context.Context, userID, orgID string) ([]model.LeaveRequest, error) {
	return s.requestRepo.ListByUser(ctx, userID, orgID)
}

func (s *LeaveService) GetTeamRequests(ctx context.Context, orgID, status string) ([]model.LeaveRequest, error) {
	return s.requestRepo.ListByOrg(ctx, orgID, status)
}

func (s *LeaveService) ApproveLeave(ctx context.Context, requestID, reviewerID string) error {
	req, err := s.requestRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Status != "pending" {
		return errs.BadRequest("leave request is already " + req.Status)
	}

	if err := s.requestRepo.UpdateStatus(ctx, requestID, "approved", reviewerID); err != nil {
		return err
	}

	start, _ := time.Parse("2006-01-02", req.StartDate)
	year := start.Year()
	return s.balanceRepo.Increment(ctx, req.UserID, req.LeaveTypeID, year, req.Days)
}

func (s *LeaveService) DenyLeave(ctx context.Context, requestID, reviewerID string) error {
	req, err := s.requestRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Status != "pending" {
		return errs.BadRequest("leave request is already " + req.Status)
	}
	return s.requestRepo.UpdateStatus(ctx, requestID, "denied", reviewerID)
}

