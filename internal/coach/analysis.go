package coach

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/db"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/llm"
)

type analysisPeriod string

const (
	analysisPeriodDaily  analysisPeriod = "daily"
	analysisPeriodWeekly analysisPeriod = "weekly"
)

// DailyRecap builds the proactive evening analysis using the current local day.
func (s *Service) DailyRecap(ctx context.Context, userID int64) (string, error) {
	return s.runNutritionAnalysis(ctx, userID, analysisPeriodDaily, time.Now())
}

// WeeklyReport builds the proactive weekly analysis ending on the current local day.
func (s *Service) WeeklyReport(ctx context.Context, userID int64) (string, error) {
	return s.runNutritionAnalysis(ctx, userID, analysisPeriodWeekly, time.Now())
}

func (s *Service) runNutritionAnalysis(ctx context.Context, userID int64, period analysisPeriod, now time.Time) (string, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	loc := domain.LoadLocationOrUTC(user.Timezone)
	localNow := now.In(loc)

	targets, err := s.store.EffectiveTargets(ctx, userID)
	if err != nil {
		return "", err
	}

	var (
		start, end time.Time
		progress   domain.DailyProgress
		barCurrent int
		barTarget  int
		macroScale float64
	)
	switch period {
	case analysisPeriodWeekly:
		start, end = domain.WeekBounds(loc, localNow)
		macroScale = 7
	default:
		start, end = domain.DayBounds(loc, localNow)
		macroScale = 1
	}

	meals, err := s.store.ListMealsBetween(ctx, userID, start, end)
	if err != nil {
		return "", err
	}
	progress = sumMealProgress(meals)
	streak, err := s.store.LoggingStreak(ctx, userID, localNow)
	if err != nil {
		return "", err
	}

	switch period {
	case analysisPeriodWeekly:
		barCurrent = int(progress.Calories / macroScale)
		barTarget = int(targets.TargetCalories)
	default:
		barCurrent = int(progress.Calories)
		barTarget = int(targets.TargetCalories)
	}

	view := AnalysisView{
		Lang:           user.Language,
		Period:         period,
		CurrentKcal:    barCurrent,
		TargetKcal:     barTarget,
		CurrentProtein: progress.ProteinG / macroScale,
		TargetProtein:  targets.TargetProteinG,
		CurrentCarbs:   progress.CarbsG / macroScale,
		TargetCarbs:    targets.TargetCarbsG,
		CurrentFat:     progress.FatG / macroScale,
		TargetFat:      targets.TargetFatG,
		Streak:         streak,
	}

	analysis, err := s.requestNutritionAnalysis(ctx, userID, user.Language, period, targets, progress, meals, streak)
	if err != nil {
		view.Data = domain.NutritionAnalysis{
			Congratulations:  fallbackCongrats(period, user.Language),
			StreakMaintained: streak > 0,
		}
		return BuildAnalysisMessage(view), nil
	}
	view.Data = analysis
	return BuildAnalysisMessage(view), nil
}

func sumMealProgress(meals []db.MealRow) domain.DailyProgress {
	var p domain.DailyProgress
	for _, m := range meals {
		p.Calories += m.Calories
		p.ProteinG += m.ProteinG
		p.FatG += m.FatG
		p.CarbsG += m.CarbsG
	}
	return p
}

func fallbackCongrats(period analysisPeriod, lang string) string {
	if normalizeLanguage(lang) == "fr" {
		if period == analysisPeriodWeekly {
			return "Résumé hebdomadaire indisponible côté IA. Continue à logger pour débloquer le coaching."
		}
		return "Continue à logger — l'analyse IA détaillée était indisponible cette fois."
	}
	if period == analysisPeriodWeekly {
		return "Weekly summary unavailable from AI. Keep logging meals to unlock coaching insights."
	}
	return "Keep logging — detailed AI analysis was unavailable this time."
}

func (s *Service) requestNutritionAnalysis(
	ctx context.Context,
	userID int64,
	lang string,
	period analysisPeriod,
	targets domain.MacroTargets,
	progress domain.DailyProgress,
	meals []db.MealRow,
	streak int,
) (domain.NutritionAnalysis, error) {
	systemPrompt := withLanguage(nutritionAnalysisSystemPrompt(period), lang)
	userPrompt := buildNutritionAnalysisUserPrompt(period, targets, progress, meals, streak)
	uid := userID
	raw, err := s.llm.CompleteJSON(ctx, &uid, "nutrition_analysis", llm.ModelReason, systemPrompt, userPrompt, llm.NutritionAnalysisSchema)
	if err != nil {
		return domain.NutritionAnalysis{}, err
	}
	var analysis domain.NutritionAnalysis
	if err := json.Unmarshal([]byte(raw), &analysis); err != nil {
		return domain.NutritionAnalysis{}, fmt.Errorf("parse nutrition analysis: %w", err)
	}
	return analysis, nil
}

func nutritionAnalysisSystemPrompt(period analysisPeriod) string {
	scope := "today's meals"
	if period == analysisPeriodWeekly {
		scope = "this week's meals"
	}
	eveningRules := ""
	if period == analysisPeriodDaily {
		eveningRules = `
- evening_snack: REQUIRED when the user is below calorie and/or macro targets. Propose ONE realistic late-night snack ("collation de fin de journée") to reach ~100% of targets.
  Constraint 1: NO cooking — only ready-to-eat items (whey, petits suisses, skyr, nuts, fresh fruit, canned tuna, yogurt, etc.).
  Constraint 2: Standard human portions (e.g. "150g de skyr et 15g d'amandes"), never unrealistic amounts like 500g of one item.
  Constraint 3: Briefly explain how the snack fills the specific missing macros (e.g. remaining protein without exploding carbs).
  If the user is already at or above all targets, set evening_snack to an empty string.`
	} else {
		eveningRules = `
- evening_snack: always set to an empty string for weekly reports.`
	}

	return fmt.Sprintf(`You are an expert nutrition coach. Analyze %s against the user's macro targets.
Return ONLY JSON matching the required schema. Do not write free-form user text outside the JSON fields.

STRICT LOCALIZATION (critical):
- Write EVERY string field in the exact same language as the user's meal descriptions / preferred language.
- Never mix languages. Do not leave English headers, category names, or filler words inside the JSON string values.
- The client UI already localizes section titles; focus on natural localized content in congratulations, meal names commentary, issues, alternatives, and evening_snack.

Rules:
- congratulations: personalized encouragement; motivating if meals were missed or macros drifted.
- streak_maintained: true when the user kept consecutive logging days; false if the streak is broken or at risk.
- top_aligned_meals: short descriptions of meals that fit the plan well.
- improvements: concrete meal-level issues with actionable alternatives (no grocery shopping list).
- Do NOT generate grocery hints or shopping lists.%s
Keep strings concise and actionable.`, scope, eveningRules)
}

func buildNutritionAnalysisUserPrompt(
	period analysisPeriod,
	targets domain.MacroTargets,
	progress domain.DailyProgress,
	meals []db.MealRow,
	streak int,
) string {
	var sb strings.Builder
	switch period {
	case analysisPeriodWeekly:
		sb.WriteString("Period: weekly.\n")
	default:
		sb.WriteString("Period: daily.\n")
	}
	fmt.Fprintf(&sb, "Current streak: %d days.\n", streak)
	fmt.Fprintf(&sb,
		"Targets (daily): %.0f kcal | P %.0f g | F %.0f g | C %.0f g\n",
		targets.TargetCalories, targets.TargetProteinG, targets.TargetFatG, targets.TargetCarbsG,
	)
	switch period {
	case analysisPeriodWeekly:
		fmt.Fprintf(&sb,
			"Consumed (week total): %.0f kcal | P %.0f g | F %.0f g | C %.0f g\n",
			progress.Calories, progress.ProteinG, progress.FatG, progress.CarbsG,
		)
		fmt.Fprintf(&sb, "Consumed (daily avg): %.0f kcal\n", progress.Calories/7)
	default:
		fmt.Fprintf(&sb,
			"Consumed (today): %.0f kcal | P %.0f g | F %.0f g | C %.0f g\n",
			progress.Calories, progress.ProteinG, progress.FatG, progress.CarbsG,
		)
		remainingKcal := targets.TargetCalories - progress.Calories
		remainingP := targets.TargetProteinG - progress.ProteinG
		remainingF := targets.TargetFatG - progress.FatG
		remainingC := targets.TargetCarbsG - progress.CarbsG
		fmt.Fprintf(&sb,
			"Remaining to 100%%: %.0f kcal | P %.0f g | F %.0f g | C %.0f g\n",
			remainingKcal, remainingP, remainingF, remainingC,
		)
	}
	sb.WriteString("Meals:\n")
	if len(meals) == 0 {
		sb.WriteString("  (none logged)\n")
		return sb.String()
	}
	for _, m := range meals {
		fmt.Fprintf(&sb, "  - %s | %.0f kcal | P %.0f F %.0f C %.0f\n",
			m.Description, m.Calories, m.ProteinG, m.FatG, m.CarbsG)
	}
	return sb.String()
}
