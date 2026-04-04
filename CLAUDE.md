# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run the server
go run ./cmd/main.go

# Build binary
go build -o syncspace ./cmd/main.go

# Run all tests
go test ./...

# Run tests for a specific package
go test ./service/...

# Format code
gofmt -w .

# Tidy dependencies
go mod tidy
```

**Environment setup:** Copy `.env` and set `DB_URL` (required). Other vars default: `PORT=8080`, `JWT_SECRET`.

```
DB_URL=postgres://user:password@localhost:5432/syncspace?sslmode=disable
JWT_SECRET=your_jwt_secret_here
PORT=8080
```

## Architecture

Go 1.22 backend using Gin + PostgreSQL (pgx/v5, no ORM). Follows a strict 4-layer dependency pattern — dependencies flow inward only:

```
HTTP → Controller → Service → Repository (DB) / Operation (validation) → Model
```

**Dependency injection** is manual and wired in [cmd/main.go](cmd/main.go): config → DB pool → repos → services → controllers → routes.

### Layers

| Directory | Role |
|---|---|
| `controller/` | HTTP handlers; bind requests, call service, map errors to HTTP status |
| `service/` | Business logic orchestration; coordinates repos and operations |
| `service/database/` | Repository pattern — raw SQL via pgxpool; accept `context.Context` |
| `service/operation/` | Pure validation/transformation (no DB access) |
| `model/` | Domain entities |
| `routes/` | Centralized route registration; all path strings come from `constants/uri.go` |
| `errs/` | Structured `AppError{Code, Message}` with constructors: `NotFound()`, `BadRequest()`, `Internal()` |
| `config/` | Loads env via godotenv; fatal if `DB_URL` missing |
| `pkg/db/` | pgxpool connection (min 2, max 10 connections) |
| `pkg/utils/` | Tagged logger: `Info(tag, msg)` / `Error(tag, msg)` |
| `constants/` | Route URIs, context keys (`user_id`, `request_id`), log tags |

### Response envelope

All endpoints return:
```json
{ "success": true, "data": {...} }
{ "success": false, "error": "message" }
```

Use `data.Ok(payload)` and `data.Fail(message)` from `service/data/response.go`.

### Error flow

Operation/repo returns `*errs.AppError` → service propagates → controller maps code to HTTP status and calls `data.Fail()`.

### Auth

JWT infrastructure is wired in config (`JWT_SECRET`) and context keys are defined in `constants/context.go`, but no auth middleware exists yet. All current routes are unauthenticated.

### Database

The repository layer (`service/database/`) uses raw SQL via `pgxpool`. Current implementations are placeholders (TODOs) awaiting schema migrations. No ORM — write SQL directly.
