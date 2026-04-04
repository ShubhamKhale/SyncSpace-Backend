// Package utils provides shared utility helpers used across all layers.
package utils

import (
	"fmt"
	"log"
)

// Info logs an informational message with an optional formatted suffix.
func Info(tag, msg string, args ...any) {
	if len(args) > 0 {
		log.Printf("%s INFO  %s — %s", tag, msg, fmt.Sprintf("%v", args))
	} else {
		log.Printf("%s INFO  %s", tag, msg)
	}
}

// Error logs an error message with an optional formatted suffix.
func Error(tag, msg string, args ...any) {
	if len(args) > 0 {
		log.Printf("%s ERROR %s — %s", tag, msg, fmt.Sprintf("%v", args))
	} else {
		log.Printf("%s ERROR %s", tag, msg)
	}
}
