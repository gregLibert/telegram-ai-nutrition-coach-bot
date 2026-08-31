package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
)

func TestDailyProgressRespectsTimezone(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	res, err := store.db.ExecContext(ctx, `INSERT INTO users (username, timezone) VALUES ('tzuser', 'Europe/Paris')`)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := res.LastInsertId()

	paris, _ := time.LoadLocation("Europe/Paris")
	// 23:30 Paris on Aug 29 = still Aug 29 locally, but UTC date may differ
	localEvening := time.Date(2026, 8, 29, 23, 30, 0, 0, paris)
	loggedAt := domain.FormatSQLiteUTC(localEvening.UTC())

	_, err = store.db.ExecContext(ctx, `
		INSERT INTO meals (user_id, description, calories, protein_g, fat_g, carbs_g, source, logged_at)
		VALUES (?, 'late dinner', 500, 30, 15, 40, 'text', ?)`, userID, loggedAt)
	if err != nil {
		t.Fatal(err)
	}

	progress, err := store.DailyProgress(ctx, userID, localEvening)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Calories != 500 {
		t.Errorf("Calories = %v, want 500 (meal should count for Paris local day)", progress.Calories)
	}

	nextDay := time.Date(2026, 8, 30, 1, 0, 0, 0, paris)
	progressNext, err := store.DailyProgress(ctx, userID, nextDay)
	if err != nil {
		t.Fatal(err)
	}
	if progressNext.Calories != 0 {
		t.Errorf("next day Calories = %v, want 0", progressNext.Calories)
	}
}

func TestDeleteLastMeal(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	user, err := store.GetOrCreateLocalUser(ctx, "undo_user")
	if err != nil {
		t.Fatal(err)
	}

	meal := domain.MealEstimate{Description: "test", Calories: 100, ProteinG: 10, FatG: 5, CarbsG: 10, Confidence: "high"}
	if err := store.AddMeal(ctx, user.ID, meal, "text"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddMeal(ctx, user.ID, meal, "text"); err != nil {
		t.Fatal(err)
	}

	deleted, err := store.DeleteLastMeal(ctx, user.ID)
	if err != nil || !deleted {
		t.Fatalf("DeleteLastMeal() = %v, %v", deleted, err)
	}

	progress, err := store.DailyProgressNow(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Calories != 100 {
		t.Errorf("remaining calories = %v, want 100", progress.Calories)
	}
}
