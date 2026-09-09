package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPromptCacheRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	hash := HashPromptKey("meal_text", "en", "chicken rice")
	if _, ok, err := store.GetPromptCache(ctx, hash); err != nil || ok {
		t.Fatalf("expected miss, ok=%v err=%v", ok, err)
	}

	if err := store.UpsertPromptCache(ctx, hash, `{"description":"chicken"}`); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.GetPromptCache(ctx, hash)
	if err != nil || !ok {
		t.Fatalf("expected hit, ok=%v err=%v", ok, err)
	}
	if got != `{"description":"chicken"}` {
		t.Fatalf("got %q", got)
	}

	if err := store.UpsertPromptCache(ctx, hash, `{"description":"updated"}`); err != nil {
		t.Fatal(err)
	}
	got, ok, err = store.GetPromptCache(ctx, hash)
	if err != nil || !ok || got != `{"description":"updated"}` {
		t.Fatalf("upsert failed: %q ok=%v err=%v", got, ok, err)
	}
}

func TestHashPromptKeyStable(t *testing.T) {
	t.Parallel()
	a := HashPromptKey("meal_text", "en", "oats")
	b := HashPromptKey("meal_text", "en", "oats")
	c := HashPromptKey("meal_text", "fr", "oats")
	if a != b {
		t.Fatal("hash not stable")
	}
	if a == c {
		t.Fatal("language should change hash")
	}
}
