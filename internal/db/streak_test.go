package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
)

func TestCountConsecutiveStreak(t *testing.T) {
	t.Parallel()
	loc := time.UTC
	ref := time.Date(2026, 9, 9, 21, 30, 0, 0, loc)

	tests := []struct {
		name     string
		counts   map[string]int
		minMeals int
		want     int
	}{
		{
			name:     "empty",
			counts:   map[string]int{},
			minMeals: 1,
			want:     0,
		},
		{
			name: "three day streak",
			counts: map[string]int{
				"2026-09-09": 2,
				"2026-09-08": 1,
				"2026-09-07": 3,
				"2026-09-05": 2, // gap on 09-06 breaks earlier days
			},
			minMeals: 1,
			want:     3,
		},
		{
			name: "broken today",
			counts: map[string]int{
				"2026-09-08": 2,
				"2026-09-07": 2,
			},
			minMeals: 1,
			want:     0,
		},
		{
			name: "requires min meals",
			counts: map[string]int{
				"2026-09-09": 1,
				"2026-09-08": 2,
			},
			minMeals: 2,
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := countConsecutiveStreak(tt.counts, ref, tt.minMeals)
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLoggingStreakIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "streak.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	res, err := store.db.ExecContext(ctx, `INSERT INTO users (username, timezone) VALUES ('streaker', 'UTC')`)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := res.LastInsertId()

	ref := time.Date(2026, 9, 9, 21, 0, 0, 0, time.UTC)
	insertMeal := func(day time.Time) {
		t.Helper()
		_, err := store.db.ExecContext(ctx, `
			INSERT INTO meals (user_id, description, calories, protein_g, fat_g, carbs_g, source, logged_at)
			VALUES (?, 'meal', 500, 30, 10, 40, 'text', ?)`,
			userID, domain.FormatSQLiteUTC(day))
		if err != nil {
			t.Fatal(err)
		}
	}
	insertMeal(time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC))
	insertMeal(time.Date(2026, 9, 8, 19, 0, 0, 0, time.UTC))
	insertMeal(time.Date(2026, 9, 7, 13, 0, 0, 0, time.UTC))
	// gap on 6th
	insertMeal(time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC))

	got, err := store.LoggingStreak(ctx, userID, ref)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("streak = %d, want 3", got)
	}
}
