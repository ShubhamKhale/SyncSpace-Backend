package model

import "time"

type LeaveType struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	DefaultDays int       `json:"default_days"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type LeaveBalance struct {
	UserID      string `json:"user_id"`
	LeaveTypeID string `json:"leave_type_id"`
	TypeName    string `json:"type_name"`
	Year        int    `json:"year"`
	Allocated   int    `json:"allocated"`
	Used        int    `json:"used"`
	Remaining   int    `json:"remaining"`
}

type LeaveRequest struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	UserName    string     `json:"user_name,omitempty"`
	OrgID       string     `json:"org_id"`
	LeaveTypeID string     `json:"leave_type_id"`
	TypeName    string     `json:"type_name,omitempty"`
	StartDate   string     `json:"start_date"`
	EndDate     string     `json:"end_date"`
	Days        int        `json:"days"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"`
	ReviewedBy  *string    `json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
