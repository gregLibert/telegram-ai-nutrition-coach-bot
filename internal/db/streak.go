package db

import (
	"context"
	"fmt"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
)

const (
	// streakLookbackDays limits how far we scan meal history for streak calculation.
	streakLookbackDays = 90
	// minMealsForStreakDay is the minimum logged meals for a calendar day to count.
	minMealsForStreakDay = 1
)

// MealRow is a persisted meal used for analysis prompts.
type MealRow struct {
	Description string
	Calories    float64
	ProteinG    float64
	FatG        float64
	CarbsG      float64
	LoggedAt    time.Time
}

// LoggingStreak returns consecutive local days (ending on ref's day) with enough meals logged.
func (s *Store) LoggingStreak(ctx context.Context, userID int64, ref time.Time) (int, error) {
	loc := s.userLocation(ctx, userID)
	localRef := ref.In(loc)
	lookbackStart := localRef.AddDate(0, 0, -(streakLookbackDays - 1))
	start, _ := domain.DayBounds(loc, lookbackStart)
	_, end := domain.DayBounds(loc, localRef)

	rows, err := s.db.QueryContext(ctx, `
		SELECT logged_at FROM meals
		WHERE user_id = ? AND logged_at >= ? AND logged_at < ?`,
		userID, domain.FormatSQLiteUTC(start), domain.FormatSQLiteUTC(end))
	if err != nil {
		return 0, fmt.Errorf("logging streak query: %w", err)
	}
	defer closeRows(rows)

	dayCounts := make(map[string]int)
	for rows.Next() {
		var ts string
		if err := rows.Scan(&ts); err != nil {
			return 0, err
		}
		logged, parseErr := time.Parse("2006-01-02 15:04:05", ts)
		if parseErr != nil {
			continue
		}
		dayKey := logged.In(loc).Format("2006-01-02")
		dayCounts[dayKey]++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	return countConsecutiveStreak(dayCounts, localRef, minMealsForStreakDay), nil
}

// countConsecutiveStreak walks backward from ref's local day while each day meets minMeals.
func countConsecutiveStreak(dayCounts map[string]int, localRef time.Time, minMeals int) int {
	streak := 0
	day := time.Date(localRef.Year(), localRef.Month(), localRef.Day(), 0, 0, 0, 0, localRef.Location())
	for {
		key := day.Format("2006-01-02")
		if dayCounts[key] < minMeals {
			break
		}
		streak++
		day = day.AddDate(0, 0, -1)
	}
	return streak
}

// ListMealsBetween returns meals logged in [start, end) UTC for a user.
func (s *Store) ListMealsBetween(ctx context.Context, userID int64, start, end time.Time) ([]MealRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT description, calories, protein_g, fat_g, carbs_g, logged_at
		FROM meals
		WHERE user_id = ? AND logged_at >= ? AND logged_at < ?
		ORDER BY logged_at ASC`,
		userID, domain.FormatSQLiteUTC(start), domain.FormatSQLiteUTC(end))
	if err != nil {
		return nil, fmt.Errorf("list meals: %w", err)
	}
	defer closeRows(rows)

	var meals []MealRow
	for rows.Next() {
		var m MealRow
		var ts string
		if err := rows.Scan(&m.Description, &m.Calories, &m.ProteinG, &m.FatG, &m.CarbsG, &ts); err != nil {
			return nil, err
		}
		m.LoggedAt, _ = time.Parse("2006-01-02 15:04:05", ts)
		meals = append(meals, m)
	}
	return meals, rows.Err()
}
