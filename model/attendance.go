package model

import "time"

type AttendanceLog struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	OrgID      string     `json:"org_id"`
	CheckInAt  time.Time  `json:"check_in_at"`
	CheckOutAt *time.Time `json:"check_out_at,omitempty"`
	Note       string     `json:"note,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type AttendanceStatus struct {
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	AvatarURL string     `json:"avatar_url"`
	Status    string     `json:"status"`
	CheckInAt *time.Time `json:"check_in_at,omitempty"`
}

type TeamAttendanceSummary struct {
	CheckedIn int                `json:"checked_in"`
	OnLeave   int                `json:"on_leave"`
	Absent    int                `json:"absent"`
	Total     int                `json:"total"`
	Members   []AttendanceStatus `json:"members"`
}
