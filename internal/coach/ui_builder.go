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

// AnalysisView carries rendered daily analysis inputs.
type AnalysisView struct {
	Data           domain.NutritionAnalysis
	Lang           string
	Period         analysisPeriod
	CurrentKcal    int
	TargetKcal     int
	CurrentProtein float64
	TargetProtein  float64
	CurrentCarbs   float64
	TargetCarbs    float64
	CurrentFat     float64
	TargetFat      float64
	Streak         int
}

// WeeklyAnalysisView carries rendered weekly bilan inputs.
type WeeklyAnalysisView struct {
	Data          domain.WeeklyAnalysis
	Lang          string
	AvgKcal       int
	TargetKcal    int
	AvgProtein    float64
	TargetProtein float64
	AvgCarbs      float64
	TargetCarbs   float64
	AvgFat        float64
	TargetFat     float64
}

type analysisLabels struct {
	TitleDaily   string
	TitleWeekly  string
	Streak       string
	StatusOK     string
	StatusBad    string
	Calories     string
	Protein      string
	Carbs        string
	Fat          string
	TopMeals     string
	Improvements string
	EveningSnack string
	DaySingular  string
	DayPlural    string
	AvgOver7Days string
	Attention    string
	TipsSwaps    string
	LegumeFocus  string
	GroceryList  string
}

func analysisLabelsFor(lang string) analysisLabels {
	if normalizeLanguage(lang) == "fr" {
		return analysisLabels{
			TitleDaily:   "📋 Analyse Quotidienne",
			TitleWeekly:  "📋 Bilan Hebdomadaire",
			Streak:       "🔥 Série",
			StatusOK:     "✅ Statut : maintenu",
			StatusBad:    "⚠️ Statut : en danger / rompu",
			Calories:     "📊 Calories",
			Protein:      "🥩 Protéines",
			Carbs:        "🍚 Glucides",
			Fat:          "🥑 Lipides",
			TopMeals:     "✅ Meilleurs repas",
			Improvements: "🔧 Pistes d'amélioration",
			EveningSnack: "🎯 Pour atteindre 100% ce soir",
			DaySingular:  "jour",
			DayPlural:    "jours",
			AvgOver7Days: "📊 Moyenne sur 7 jours",
			Attention:    "⚠️ Point d'attention",
			TipsSwaps:    "💡 Astuces & Remplacements",
			LegumeFocus:  "🍲 Focus Légumineuses",
			GroceryList:  "🛒 Idées Courses pour la semaine pro",
		}
	}
	return analysisLabels{
		TitleDaily:   "📋 Daily Analysis",
		TitleWeekly:  "📋 Weekly Review",
		Streak:       "🔥 Streak",
		StatusOK:     "✅ Status: maintained",
		StatusBad:    "⚠️ Status: at risk / broken",
		Calories:     "📊 Calories",
		Protein:      "🥩 Protein",
		Carbs:        "🍚 Carbs",
		Fat:          "🥑 Fat",
		TopMeals:     "✅ Top aligned meals",
		Improvements: "🔧 Improvement ideas",
		EveningSnack: "🎯 To hit 100% tonight",
		DaySingular:  "day",
		DayPlural:    "days",
		AvgOver7Days: "📊 7-day average",
		Attention:    "⚠️ Watch-out",
		TipsSwaps:    "💡 Tips & Swaps",
		LegumeFocus:  "🍲 Legume Focus",
		GroceryList:  "🛒 Grocery ideas for next week",
	}
}

// BuildAnalysisMessage renders a Telegram-ready daily analysis message.
func BuildAnalysisMessage(v AnalysisView) string {
	labels := analysisLabelsFor(v.Lang)

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n\n", labels.TitleDaily)
	fmt.Fprintf(&sb, "%s\n\n", v.Data.Congratulations)

	dayUnit := labels.DayPlural
	if v.Streak == 1 {
		dayUnit = labels.DaySingular
	}
	fmt.Fprintf(&sb, "%s : %d %s\n", labels.Streak, v.Streak, dayUnit)
	if v.Data.StreakMaintained {
		sb.WriteString(labels.StatusOK + "\n")
	} else {
		sb.WriteString(labels.StatusBad + "\n")
	}

	fmt.Fprintf(&sb, "\n%s : %d / %d kcal\n", labels.Calories, v.CurrentKcal, v.TargetKcal)
	fmt.Fprintf(&sb, "%s\n", BuildCalorieProgressBar(v.CurrentKcal, v.TargetKcal))
	fmt.Fprintf(&sb, "%s : %.0f / %.0f g | %s : %.0f / %.0f g | %s : %.0f / %.0f g\n",
		labels.Protein, v.CurrentProtein, v.TargetProtein,
		labels.Carbs, v.CurrentCarbs, v.TargetCarbs,
		labels.Fat, v.CurrentFat, v.TargetFat,
	)

	if len(v.Data.TopAlignedMeals) > 0 {
		fmt.Fprintf(&sb, "\n%s :\n", labels.TopMeals)
		for _, meal := range v.Data.TopAlignedMeals {
			fmt.Fprintf(&sb, "  • %s\n", meal)
		}
	}

	if len(v.Data.Improvements) > 0 {
		fmt.Fprintf(&sb, "\n%s :\n", labels.Improvements)
		for _, item := range v.Data.Improvements {
			fmt.Fprintf(&sb, "  • %s — %s\n    → %s\n", item.MealName, item.Issue, item.Alternative)
		}
	}

	if shouldShowEveningSnack(v) {
		fmt.Fprintf(&sb, "\n%s :\n%s\n", labels.EveningSnack, strings.TrimSpace(v.Data.EveningSnack))
	}

	return strings.TrimRight(sb.String(), "\n")
}

// BuildWeeklyAnalysisMessage renders the weekly bilan layout.
func BuildWeeklyAnalysisMessage(v WeeklyAnalysisView) string {
	labels := analysisLabelsFor(v.Lang)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n\n", labels.TitleWeekly)
	if overview := strings.TrimSpace(v.Data.Overview); overview != "" {
		fmt.Fprintf(&sb, "%s\n\n", overview)
	}

	fmt.Fprintf(&sb, "%s : %d / %d kcal\n", labels.AvgOver7Days, v.AvgKcal, v.TargetKcal)
	fmt.Fprintf(&sb, "%s : %.0f / %.0f g | %s : %.0f / %.0f g | %s : %.0f / %.0f g\n",
		labels.Protein, v.AvgProtein, v.TargetProtein,
		labels.Carbs, v.AvgCarbs, v.TargetCarbs,
		labels.Fat, v.AvgFat, v.TargetFat,
	)

	if bottleneck := strings.TrimSpace(v.Data.MacroBottleneck); bottleneck != "" {
		fmt.Fprintf(&sb, "\n%s : %s\n", labels.Attention, bottleneck)
	}

	if len(v.Data.MealTips) > 0 || strings.TrimSpace(v.Data.LegumeFocus) != "" {
		fmt.Fprintf(&sb, "\n%s :\n", labels.TipsSwaps)
		for _, tip := range v.Data.MealTips {
			fmt.Fprintf(&sb, "  • %s : %s\n", tip.MealName, tip.Tip)
		}
		if legume := strings.TrimSpace(v.Data.LegumeFocus); legume != "" {
			fmt.Fprintf(&sb, "  • %s : %s\n", labels.LegumeFocus, legume)
		}
	}

	if len(v.Data.GroceryList) > 0 {
		fmt.Fprintf(&sb, "\n%s :\n", labels.GroceryList)
		for _, item := range v.Data.GroceryList {
			fmt.Fprintf(&sb, "  • %s\n", item)
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

func shouldShowEveningSnack(v AnalysisView) bool {
	if v.Period != analysisPeriodDaily {
		return false
	}
	snack := strings.TrimSpace(v.Data.EveningSnack)
	if snack == "" {
		return false
	}
	underCalories := v.TargetKcal > 0 && v.CurrentKcal < v.TargetKcal
	underProtein := v.TargetProtein > 0 && v.CurrentProtein < v.TargetProtein
	underCarbs := v.TargetCarbs > 0 && v.CurrentCarbs < v.TargetCarbs
	underFat := v.TargetFat > 0 && v.CurrentFat < v.TargetFat
	return underCalories || underProtein || underCarbs || underFat
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
