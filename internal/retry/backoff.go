package retry

import (
	"context"
	"time"
)

const (
	DefaultInitialBackoff = 1 * time.Second
	DefaultMaxBackoff     = 30 * time.Second
	DefaultMaxAttempts    = 5
	backoffMultiplier     = 2
)

// Config controls exponential backoff retries.
type Config struct {
	Initial     time.Duration
	MaxBackoff  time.Duration
	MaxAttempts int
}

func (c Config) withDefaults() Config {
	if c.Initial <= 0 {
		c.Initial = DefaultInitialBackoff
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = DefaultMaxBackoff
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultMaxAttempts
	}
	return c
}

// Do runs fn with exponential backoff until success, ctx cancel, or max attempts.
func Do(ctx context.Context, cfg Config, fn func(attempt int) error) error {
	cfg = cfg.withDefaults()
	backoff := cfg.Initial
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn(attempt)
		if lastErr == nil {
			return nil
		}
		if attempt == cfg.MaxAttempts {
			break
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		backoff *= backoffMultiplier
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}
	return lastErr
}

// Wait sleeps for duration or until ctx is cancelled.
func Wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
