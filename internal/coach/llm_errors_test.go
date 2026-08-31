package coach

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestIsLLMTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "wrapped deadline", err: fmt.Errorf("http request: %w", context.DeadlineExceeded), want: true},
		{name: "net timeout", err: &net.OpError{Op: "read", Err: timeoutError{}}, want: true},
		{name: "other error", err: errors.New("openrouter status 500"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isLLMTimeout(tt.err); got != tt.want {
				t.Fatalf("isLLMTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResponseFromLLMError(t *testing.T) {
	t.Parallel()

	resp, err := responseFromLLMError(context.DeadlineExceeded, "meal analysis")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Text != llmTimeoutUserMessage {
		t.Fatalf("unexpected message: %q", resp.Text)
	}

	resp, err = responseFromLLMError(errors.New("boom"), "meal analysis")
	if err == nil {
		t.Fatal("expected error")
	}
	if resp.Text != "" {
		t.Fatalf("expected empty response text, got %q", resp.Text)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

var _ net.Error = timeoutError{}

func TestTimeoutErrorInterface(t *testing.T) {
	t.Parallel()
	if !isLLMTimeout(&net.OpError{Op: "read", Err: timeoutError{}}) {
		t.Fatal("expected net timeout to be detected")
	}
}
