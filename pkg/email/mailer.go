// Package email provides a thin Brevo (api.brevo.com) transactional email client.
// It uses net/http directly — no external SDK dependency.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const brevoSendURL = "https://api.brevo.com/v3/smtp/email"

// Mailer sends transactional emails via the Brevo SMTP API.
type Mailer struct {
	apiKey    string
	fromEmail string
	fromName  string
	client    *http.Client
}

// NewMailer creates a Mailer. fromEmail must be a verified sender in Brevo.
func NewMailer(apiKey, fromEmail, fromName string) *Mailer {
	return &Mailer{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
		client:    &http.Client{},
	}
}

// Send delivers a single HTML email via Brevo.
// Returns an error on non-2xx response or network failure.
func (m *Mailer) Send(ctx context.Context, to, subject, htmlContent string) error {
	payload := map[string]any{
		"sender":      map[string]string{"email": m.fromEmail, "name": m.fromName},
		"to":          []map[string]string{{"email": to}},
		"subject":     subject,
		"htmlContent": htmlContent,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, brevoSendURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("email: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-key", m.apiKey)

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("email: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("email: brevo returned %d: %s", resp.StatusCode, raw)
	}
	return nil
}
