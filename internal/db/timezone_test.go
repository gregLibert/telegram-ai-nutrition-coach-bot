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

func TestHasMealInLocalWindow(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	res, err := store.db.ExecContext(ctx, `INSERT INTO users (username, timezone, language) VALUES ('mealwin', 'UTC', 'en')`)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := res.LastInsertId()

	ref := time.Date(2026, 9, 4, 14, 0, 0, 0, time.UTC)
	lunchLogged := time.Date(2026, 9, 4, 12, 30, 0, 0, time.UTC)
	_, err = store.db.ExecContext(ctx, `
		INSERT INTO meals (user_id, description, calories, protein_g, fat_g, carbs_g, source, logged_at)
		VALUES (?, 'lunch', 600, 40, 20, 50, 'text', ?)`,
		userID, domain.FormatSQLiteUTC(lunchLogged))
	if err != nil {
		t.Fatal(err)
	}

	has, err := store.HasMealInLocalWindow(ctx, userID, ref, 11, 14)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("expected lunch meal in 11-14 window")
	}

	hasDinner, err := store.HasMealInLocalWindow(ctx, userID, ref, 18, 21)
	if err != nil {
		t.Fatal(err)
	}
	if hasDinner {
		t.Fatal("did not expect lunch meal in dinner window")
	}
}

func TestAddManualActivityCalories(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	user, err := store.GetOrCreateLocalUser(ctx, "sport_user")
	if err != nil {
		t.Fatal(err)
	}
	input := domain.ProfileInput{
		Age: 30, HeightCm: 180, WeightKg: 80, TargetWeightKg: 75,
		Gender: domain.GenderMale, ActivityLevel: domain.ActivityModerate, WeightGoal: domain.GoalLose,
	}
	targets := domain.CalculateMacroTargets(input)
	if err := store.SaveProfile(ctx, user.ID, input, targets); err != nil {
		t.Fatal(err)
	}

	log, err := store.AddManualActivityCalories(ctx, user.ID, 300)
	if err != nil {
		t.Fatal(err)
	}
	if log.ActiveCalories != 300 {
		t.Fatalf("active = %v, want 300", log.ActiveCalories)
	}
	if log.AdjustedTDEE != targets.TDEE+300 {
		t.Fatalf("adjusted = %v, want %v", log.AdjustedTDEE, targets.TDEE+300)
	}

	log2, err := store.AddManualActivityCalories(ctx, user.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if log2.ActiveCalories != 400 {
		t.Fatalf("active after second = %v, want 400", log2.ActiveCalories)
	}
}

func TestUserLanguageDefault(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	user, err := store.GetOrCreateLocalUser(ctx, "lang_user")
	if err != nil {
		t.Fatal(err)
	}
	if user.Language != "en" {
		t.Fatalf("default language = %q, want en", user.Language)
	}
	if err := store.UpdateUserLanguage(ctx, user.ID, "fr"); err != nil {
		t.Fatal(err)
	}
	updated, err := store.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Language != "fr" {
		t.Fatalf("language = %q, want fr", updated.Language)
	}
}
