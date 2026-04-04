// Package operation contains pure business-logic helpers for the board domain.
// These functions do not touch the database; they validate and transform data
// before it reaches the repository layer.
package operation

import (
	"syncspace-backend/errs"
)

// ValidateCreateBoard checks that the fields required to create a board are
// present and within acceptable bounds.
func ValidateCreateBoard(title, description string) error {
	if title == "" {
		return errs.BadRequest("board title cannot be empty")
	}
	if len(title) > 200 {
		return errs.BadRequest("board title must be 200 characters or fewer")
	}
	_ = description // description is optional
	return nil
}
