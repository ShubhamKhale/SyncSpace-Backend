# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o syncspace ./cmd/main.go

# ── Run stage ────────────────────────────────────────────────────────────────
FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates
COPY --from=build /app/syncspace .

EXPOSE 8080
CMD ["./syncspace"]
