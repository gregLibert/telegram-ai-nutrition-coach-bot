package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const promptCacheTTL = 7 * 24 * time.Hour

// HashPromptKey returns a stable SHA-256 hex digest for cache lookups.
func HashPromptKey(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			_, _ = h.Write([]byte{0})
		}
		_, _ = h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// GetPromptCache returns a cached response when present and not expired.
func (s *Store) GetPromptCache(ctx context.Context, hashKey string) (string, bool, error) {
	var response, createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT response, created_at FROM prompt_cache WHERE hash_key = ?`, hashKey).
		Scan(&response, &createdAt)
	switch {
	case err == sql.ErrNoRows:
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("get prompt cache: %w", err)
	}

	created, err := time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		created, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return "", false, nil
		}
	}
	if time.Since(created.UTC()) > promptCacheTTL {
		return "", false, nil
	}
	return response, true, nil
}

// UpsertPromptCache stores or replaces a cached LLM/API response.
func (s *Store) UpsertPromptCache(ctx context.Context, hashKey, response string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO prompt_cache (hash_key, response, created_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(hash_key) DO UPDATE SET
			response = excluded.response,
			created_at = datetime('now')`, hashKey, response)
	if err != nil {
		return fmt.Errorf("upsert prompt cache: %w", err)
	}
	return nil
}
