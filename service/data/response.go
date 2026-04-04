// Package data defines shared data-transfer types used across the service
// and controller layers — most importantly the standard API response envelope.
package data

// APIResponse is the standard JSON envelope returned by every endpoint.
//
//	{ "success": true,  "data": {...} }
//	{ "success": false, "error": "..." }
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// OK builds a successful response wrapping the given payload.
func OK(data any) APIResponse {
	return APIResponse{Success: true, Data: data}
}

// Fail builds an error response with the provided message.
func Fail(errMsg string) APIResponse {
	return APIResponse{Success: false, Error: errMsg}
}
