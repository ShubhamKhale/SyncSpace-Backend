// Package constants holds application-wide constant values.
package constants

// contextKey is an unexported type for context keys to avoid collisions
// with keys defined in other packages.
type contextKey string

const (
	// ContextKeyUserID is the key used to store the authenticated user's ID
	// in a request context.
	ContextKeyUserID contextKey = "user_id"

	// ContextKeyRequestID is the key used to store the request trace ID.
	ContextKeyRequestID contextKey = "request_id"
)
