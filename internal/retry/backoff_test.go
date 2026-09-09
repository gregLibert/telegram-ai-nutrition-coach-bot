package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoSuccessFirstAttempt(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Do(context.Background(), Config{MaxAttempts: 3, Initial: time.Millisecond}, func(attempt int) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestDoRetriesThenSucceeds(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Do(context.Background(), Config{MaxAttempts: 5, Initial: time.Millisecond}, func(attempt int) error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestDoExhaustsAttempts(t *testing.T) {
	t.Parallel()
	want := errors.New("always fail")
	err := Do(context.Background(), Config{MaxAttempts: 3, Initial: time.Millisecond}, func(attempt int) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestDoRespectsContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Do(ctx, Config{MaxAttempts: 5, Initial: time.Second}, func(attempt int) error {
		return errors.New("should not run much")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want canceled", err)
	}
}
