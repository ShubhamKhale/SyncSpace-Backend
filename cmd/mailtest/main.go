// mailtest is a diagnostic CLI for the invite-email pipeline.
//
// The real invite flow (OrgService.sendInviteEmail) runs in a goroutine and
// swallows every error, so a broken setup fails silently. This tool runs the
// same steps one at a time and reports exactly which one fails.
//
// Usage (reads .env / environment exactly like the server does):
//
//	go run ./cmd/mailtest -to you@gmail.com          # dry run: checks only, sends nothing
//	go run ./cmd/mailtest -to you@gmail.com -send    # also sends a real invite email
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"syncspace-backend/config"
	"syncspace-backend/pkg/db"
	pkgemail "syncspace-backend/pkg/email"
	"syncspace-backend/service/database"
)

func main() {
	to := flag.String("to", "", "recipient email address (required)")
	send := flag.Bool("send", false, "actually send the email (default: dry run)")
	flag.Parse()
	if *to == "" {
		fmt.Println("usage: go run ./cmd/mailtest -to you@gmail.com [-send]")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := config.Load()

	// ── 1. Config ────────────────────────────────────────────────────────────
	step("1. Config")
	if cfg.BrevoAPIKey == "" {
		fail("BREVO_API_KEY is empty — server skips invite emails entirely (mailer is nil)")
	}
	ok("BREVO_API_KEY    = " + mask(cfg.BrevoAPIKey))
	ok("BREVO_FROM_EMAIL = " + cfg.BrevoFromEmail)
	ok("BREVO_FROM_NAME  = " + cfg.BrevoFromName)
	ok("FRONTEND_URL     = " + cfg.FrontendURL)
	if strings.HasSuffix(cfg.FrontendURL, "/") {
		warn("FRONTEND_URL has a trailing slash — invite links will contain '//invite'")
	}
	if strings.Contains(cfg.FrontendURL, "localhost") {
		warn("FRONTEND_URL points at localhost — invite links won't work for other people")
	}

	// ── 2. Brevo API key ─────────────────────────────────────────────────────
	step("2. Brevo API key (GET /v3/account)")
	var account struct {
		Email       string `json:"email"`
		CompanyName string `json:"companyName"`
		Plan        []struct {
			Type    string  `json:"type"`
			Credits float64 `json:"credits"`
		} `json:"plan"`
	}
	if err := brevoGet(ctx, cfg.BrevoAPIKey, "/v3/account", &account); err != nil {
		fail("API key rejected: " + err.Error())
	}
	ok("key valid — account " + account.Email)
	for _, p := range account.Plan {
		ok(fmt.Sprintf("plan %s, credits left: %.0f", p.Type, p.Credits))
		if p.Credits <= 0 {
			warn("no email credits left on this plan")
		}
	}

	// ── 3. Sender verification ───────────────────────────────────────────────
	step("3. Sender verification (GET /v3/senders)")
	var senders struct {
		Senders []struct {
			Email  string `json:"email"`
			Active bool   `json:"active"`
		} `json:"senders"`
	}
	if err := brevoGet(ctx, cfg.BrevoAPIKey, "/v3/senders", &senders); err != nil {
		warn("could not list senders: " + err.Error())
	} else {
		found := false
		for _, s := range senders.Senders {
			mark := "inactive"
			if s.Active {
				mark = "active"
			}
			fmt.Printf("     - %s (%s)\n", s.Email, mark)
			if strings.EqualFold(s.Email, cfg.BrevoFromEmail) {
				found = true
				if !s.Active {
					fail("BREVO_FROM_EMAIL is registered but NOT verified/active in Brevo")
				}
			}
		}
		if !found {
			fail("BREVO_FROM_EMAIL (" + cfg.BrevoFromEmail + ") is not a sender in Brevo — " +
				"set it to one of the active senders above, or add+verify it in Brevo → Senders")
		}
		ok("sender is verified")
	}

	// ── 4. Database template ─────────────────────────────────────────────────
	step("4. Database: email_templates row 'org_invite'")
	if err := db.Connect(ctx, cfg.DBURL); err != nil {
		fail("DB connect: " + err.Error())
	}
	defer db.Close()
	tpl, err := database.NewEmailTemplateRepo(db.Pool).Get(ctx, "org_invite")
	if err != nil {
		fail("template load: " + err.Error() + " — run the org_invite INSERT from schema.sql")
	}
	ok("template found (id=" + tpl.ID + ")")

	// ── 5. Render ────────────────────────────────────────────────────────────
	step("5. Render template")
	data := struct {
		OrgName, InviterName, Role, InviteLink, RecipientEmail string
	}{
		OrgName:        "Mailtest Org",
		InviterName:    "Mailtest",
		Role:           "member",
		InviteLink:     cfg.FrontendURL + "/invite?token=mailtest-dry-run",
		RecipientEmail: *to,
	}
	subject, err := render(tpl.Subject, data)
	if err != nil {
		fail("render subject: " + err.Error())
	}
	html, err := render(tpl.HTMLBody, data)
	if err != nil {
		fail("render html: " + err.Error())
	}
	ok("subject: " + subject)
	ok(fmt.Sprintf("html body: %d bytes, link: %s", len(html), data.InviteLink))

	// ── 6. Send ──────────────────────────────────────────────────────────────
	step("6. Send")
	if !*send {
		ok("dry run — skipped. Re-run with -send to deliver a real email to " + *to)
		fmt.Println("\nALL CHECKS PASSED")
		return
	}
	mailer := pkgemail.NewMailer(cfg.BrevoAPIKey, cfg.BrevoFromEmail, cfg.BrevoFromName)
	if err := mailer.Send(ctx, *to, "[mailtest] "+subject, html); err != nil {
		fail("send: " + err.Error())
	}
	ok("Brevo accepted the email. If it doesn't arrive, check Spam, then " +
		"Brevo dashboard → Transactional → Logs for bounces/blocks")
	fmt.Println("\nALL CHECKS PASSED")
}

func brevoGet(ctx context.Context, apiKey, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.brevo.com"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, raw)
	}
	return json.Unmarshal(raw, out)
}

func render(src string, data any) (string, error) {
	t, err := template.New("t").Parse(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func mask(s string) string {
	if len(s) <= 12 {
		return "****"
	}
	return s[:8] + "…" + s[len(s)-4:]
}

func step(name string) { fmt.Println("\n== " + name) }
func ok(msg string)    { fmt.Println("  ✓ " + msg) }
func warn(msg string)  { fmt.Println("  ! " + msg) }
func fail(msg string) {
	fmt.Println("  ✗ " + msg)
	fmt.Println("\nFAILED")
	os.Exit(1)
}
