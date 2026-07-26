package model

import "time"

// EmailTemplate holds an HTML email template stored in the database.
// Name is the lookup key (e.g. "org_invite"). Subject and HTMLBody are
// rendered as Go html/template strings before sending.
type EmailTemplate struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Subject   string    `json:"subject"`
	HTMLBody  string    `json:"html_body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
