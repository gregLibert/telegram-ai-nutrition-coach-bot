package coach

import (
	"fmt"
	"math"
	"strings"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
)

const (
	progressBarSegments     = 10
	progressNearTargetRatio = 0.95
	progressOverTargetRatio = 1.05
)

// BuildAnalysisMessage renders a Telegram-ready daily/weekly analysis message.
func BuildAnalysisMessage(data domain.NutritionAnalysis, currentKcal, targetKcal, streak int, title string) string {
	var sb strings.Builder
	if title == "" {
		title = "📋 Nutrition Analysis"
	}
	fmt.Fprintf(&sb, "%s\n\n", title)
	fmt.Fprintf(&sb, "%s\n\n", data.Congratulations)
	fmt.Fprintf(&sb, "🔥 Streak: %d day%s\n", streak, pluralSuffix(streak))
	if data.StreakMaintained {
		sb.WriteString("✅ Streak status: maintained\n")
	} else {
		sb.WriteString("⚠️ Streak status: at risk / broken\n")
	}

	fmt.Fprintf(&sb, "\nCalories: %d / %d kcal\n", currentKcal, targetKcal)
	fmt.Fprintf(&sb, "%s\n", BuildCalorieProgressBar(currentKcal, targetKcal))

	if len(data.TopAlignedMeals) > 0 {
		sb.WriteString("\n✅ Top aligned meals:\n")
		for _, meal := range data.TopAlignedMeals {
			fmt.Fprintf(&sb, "  • %s\n", meal)
		}
	}

	if len(data.Improvements) > 0 {
		sb.WriteString("\n🔧 Improvements:\n")
		for _, item := range data.Improvements {
			fmt.Fprintf(&sb, "  • %s — %s\n    → %s\n", item.MealName, item.Issue, item.Alternative)
		}
	}

	if len(data.GroceryHints) > 0 {
		sb.WriteString("\n🛒 Grocery hints:\n")
		for _, hint := range data.GroceryHints {
			fmt.Fprintf(&sb, "  • %s\n", hint)
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// BuildCalorieProgressBar returns a Unicode bar comparing current vs target kcal.
func BuildCalorieProgressBar(currentKcal, targetKcal int) string {
	ratio := 0.0
	if targetKcal > 0 {
		ratio = float64(currentKcal) / float64(targetKcal)
	}
	filled := int(math.Round(ratio * float64(progressBarSegments)))
	switch {
	case filled < 0:
		filled = 0
	case filled > progressBarSegments:
		filled = progressBarSegments
	}

	fillEmoji := progressFillEmoji(ratio)
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < progressBarSegments; i++ {
		if i < filled {
			b.WriteString(fillEmoji)
		} else {
			b.WriteString("⬜️")
		}
	}
	b.WriteString("] ")
	fmt.Fprintf(&b, "%.0f%%", ratio*100)
	return b.String()
}

func progressFillEmoji(ratio float64) string {
	switch {
	case ratio > progressOverTargetRatio:
		return "🟥"
	case ratio >= progressNearTargetRatio:
		return "🟨"
	default:
		return "🟩"
	}
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
