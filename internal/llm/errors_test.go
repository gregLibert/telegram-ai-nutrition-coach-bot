package llm

import (
	"errors"
	"net/http"
	"testing"
)

func TestIsRateLimited(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "plain error", err: errors.New("boom"), want: false},
		{name: "429", err: &StatusError{StatusCode: http.StatusTooManyRequests, Body: "slow down"}, want: true},
		{name: "500", err: &StatusError{StatusCode: http.StatusInternalServerError, Body: "err"}, want: false},
		{name: "wrapped 429", err: fmtWrap(&StatusError{StatusCode: http.StatusTooManyRequests}), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsRateLimited(tt.err); got != tt.want {
				t.Fatalf("IsRateLimited() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusCodeOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err  error
		want int
	}{
		{nil, 0},
		{errors.New("x"), 0},
		{&StatusError{StatusCode: 429}, 429},
	}
	for _, tt := range tests {
		if got := StatusCodeOf(tt.err); got != tt.want {
			t.Fatalf("StatusCodeOf(%v) = %d, want %d", tt.err, got, tt.want)
		}
	}
}

func fmtWrap(err error) error {
	return errors.Join(errors.New("outer"), err)
}
