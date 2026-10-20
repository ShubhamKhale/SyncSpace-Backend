# SyncSpace — Backend

Go (Gin) API for [SyncSpace](https://github.com/ShubhamKhale/SyncSpace): REST endpoints, a WebSocket hub for real-time collaboration, AI diagram generation, email invites and the PostgreSQL schema.

**Full setup and deployment guide: [SETUP.md in the SyncSpace repo](https://github.com/ShubhamKhale/SyncSpace/blob/main/SETUP.md).**
Endpoint reference: [API_DOCS.md](API_DOCS.md).

## Quick start

```bash
cp .env.example .env                 # set DB_URL, JWT_SECRET, FRONTEND_URL
psql "$DB_URL" -f schema.sql         # create the tables
go run ./cmd                         # → http://localhost:8080 (GET /health)
```

Requires Go 1.25+ and PostgreSQL 14+. Redis, Groq, Brevo and Cloudinary are optional — see comments in `.env.example`.

## What's inside

| Area | Details |
|---|---|
| Auth | Email/password + JWT; NextAuth on the frontend for Google/GitHub |
| Multi-tenancy | Organizations, roles (owner/admin/member/viewer), invites; every query is org-scoped |
| Boards & tasks | CRUD, stage moves with contiguous positions, org-wide task list, health and activity |
| Flows | Multiple diagrams per board, diagram storage, votes, presence |
| Real-time | `GET /ws` WebSocket hub; Redis pub/sub fans out across instances |
| AI | Groq: board chat, summarize, diagram generation with model fallback chain, rate-limit retries and a short result cache |
| Email | Brevo transactional email from a DB-stored template |
| Security | AES-GCM payload encryption, no secrets in logs, CORS locked to `FRONTEND_URL` |

## Layout

```
cmd/            main.go (server); aitest, mailtest (diagnostic CLIs)
routes/         route table
controller/     HTTP handlers (request parsing, status codes)
service/        business logic, authorization
service/database/  repositories (pgx)
pkg/            groq, ws, redis, email, cloudinary, crypto, utils
model/          domain types
schema.sql      full schema for a fresh database
migrations/     incremental SQL for existing databases (idempotent)
```

## Commands

| Command | Purpose |
|---|---|
| `go run ./cmd` | Run the API |
| `go build -o syncspace ./cmd` | Production binary (or `docker build .`) |
| `go test ./...` | Unit tests |
| `go run ./cmd/aitest [-n 8] [-search]` | Check Groq models and run (stress) diagram generation |
| `go run ./cmd/mailtest -to you@x.com [-send]` | Diagnose invite-email delivery |
