package coach

import (
	"strings"
	"testing"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
)

func TestBuildAnalysisMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		data        domain.NutritionAnalysis
		currentKcal int
		targetKcal  int
		streak      int
		title       string
		wantContain []string
		wantAbsent  []string
	}{
		{
			name: "perfect streak under target",
			data: domain.NutritionAnalysis{
				Congratulations:  "Great job logging every meal!",
				StreakMaintained: true,
				TopAlignedMeals:  []string{"Chicken bowl", "Greek yogurt"},
				Improvements:     nil,
				GroceryHints:     []string{"Keep skyr stocked"},
			},
			currentKcal: 1800,
			targetKcal:  2000,
			streak:      5,
			title:       "📋 Daily Analysis",
			wantContain: []string{
				"📋 Daily Analysis",
				"Great job logging every meal!",
				"🔥 Streak: 5 days",
				"✅ Streak status: maintained",
				"Calories: 1800 / 2000 kcal",
				"🟩",
				"✅ Top aligned meals:",
				"Chicken bowl",
				"🛒 Grocery hints:",
				"Keep skyr stocked",
			},
			wantAbsent: []string{"🔧 Improvements:"},
		},
		{
			name: "missing meals broken streak",
			data: domain.NutritionAnalysis{
				Congratulations:  "You missed lunch — restart strong tomorrow.",
				StreakMaintained: false,
				TopAlignedMeals:  nil,
				Improvements: []domain.MealImprovement{
					{
						MealName:    "Dinner pizza",
						Issue:       "Too high in fat/calories",
						Alternative: "Thin-crust + side salad, add whey shake",
					},
				},
				GroceryHints: []string{"Buy whey and legumes"},
			},
			currentKcal: 900,
			targetKcal:  2000,
			streak:      0,
			title:       "📋 Daily Analysis",
			wantContain: []string{
				"You missed lunch — restart strong tomorrow.",
				"🔥 Streak: 0 days",
				"⚠️ Streak status: at risk / broken",
				"🔧 Improvements:",
				"Dinner pizza",
				"Thin-crust + side salad",
				"Buy whey and legumes",
			},
		},
		{
			name: "exceeding calorie target uses red bar",
			data: domain.NutritionAnalysis{
				Congratulations:  "Calories ran high — trim evening snacks.",
				StreakMaintained: true,
			},
			currentKcal: 2600,
			targetKcal:  2000,
			streak:      1,
			title:       "📈 Weekly Analysis",
			wantContain: []string{
				"📈 Weekly Analysis",
				"🔥 Streak: 1 day",
				"Calories: 2600 / 2000 kcal",
				"🟥",
				"130%",
			},
		},
		{
			name: "near target uses yellow bar",
			data: domain.NutritionAnalysis{
				Congratulations:  "Almost perfect day.",
				StreakMaintained: true,
			},
			currentKcal: 1980,
			targetKcal:  2000,
			streak:      3,
			wantContain: []string{"🟨", "99%"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := BuildAnalysisMessage(tt.data, tt.currentKcal, tt.targetKcal, tt.streak, tt.title)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q in:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Fatalf("unexpected %q in:\n%s", absent, got)
				}
			}
		})
	}
}

func TestBuildCalorieProgressBar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current int
		target  int
		want    string
	}{
		{name: "zero target", current: 0, target: 0, want: "0%"},
		{name: "half", current: 1000, target: 2000, want: "50%"},
		{name: "over", current: 3000, target: 2000, want: "150%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := BuildCalorieProgressBar(tt.current, tt.target)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("got %q, want substring %q", got, tt.want)
			}
			if !strings.HasPrefix(got, "[") || !strings.Contains(got, "]") {
				t.Fatalf("expected bracketed bar, got %q", got)
			}
		})
	}
}
