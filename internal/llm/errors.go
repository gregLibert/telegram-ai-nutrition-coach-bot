package llm

import (
	"errors"
	"fmt"
	"net/http"
)

// StatusError is returned when OpenRouter responds with a non-2xx status.
type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("openrouter status %d: %s", e.StatusCode, e.Body)
}

// IsRateLimited reports whether err is an OpenRouter HTTP 429.
func IsRateLimited(err error) bool {
	var se *StatusError
	if errors.As(err, &se) {
		return se.StatusCode == http.StatusTooManyRequests
	}
	return false
}

// StatusCodeOf extracts an HTTP status from err when present.
func StatusCodeOf(err error) int {
	var se *StatusError
	if errors.As(err, &se) {
		return se.StatusCode
	}
	return 0
}
