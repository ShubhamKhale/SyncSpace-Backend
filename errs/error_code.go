// Package errs defines application-level error types and sentinel values.
// All service and repository layers return *AppError so the controller can
// map them to the correct HTTP status code.
package errs

import "fmt"

// AppError is a structured application error with a machine-readable Code
// and a human-readable Message.
type AppError struct {
	Code    string
	Message string
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Sentinel error constructors — use these instead of raw &AppError{} so that
// callers can check errors.Is / errors.As if needed in the future.

func NotFound(msg string) *AppError {
	return &AppError{Code: "NOT_FOUND", Message: msg}
}

func Internal(msg string) *AppError {
	return &AppError{Code: "INTERNAL_ERROR", Message: msg}
}

func BadRequest(msg string) *AppError {
	return &AppError{Code: "BAD_REQUEST", Message: msg}
}

func Unauthorized(msg string) *AppError {
	return &AppError{Code: "UNAUTHORIZED", Message: msg}
}

func Conflict(msg string) *AppError {
	return &AppError{Code: "CONFLICT", Message: msg}
}

func Forbidden(msg string) *AppError {
	return &AppError{Code: "FORBIDDEN", Message: msg}
}
