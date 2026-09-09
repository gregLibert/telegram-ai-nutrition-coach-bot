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
		title      string
		barCurrent int
		barTarget  int
	)
	switch period {
	case analysisPeriodWeekly:
		start, end = domain.WeekBounds(loc, localNow)
		title = "📈 Weekly Analysis"
	default:
		start, end = domain.DayBounds(loc, localNow)
		title = "📋 Daily Analysis"
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
		// Progress bar uses average daily calories vs daily target.
		const weekDays = 7
		barCurrent = int(progress.Calories / weekDays)
		barTarget = int(targets.TargetCalories)
	default:
		barCurrent = int(progress.Calories)
		barTarget = int(targets.TargetCalories)
	}

	analysis, err := s.requestNutritionAnalysis(ctx, userID, user.Language, period, targets, progress, meals, streak)
	if err != nil {
		return BuildAnalysisMessage(domain.NutritionAnalysis{
			Congratulations:  fallbackCongrats(period),
			StreakMaintained: streak > 0,
		}, barCurrent, barTarget, streak, title), nil
	}
	return BuildAnalysisMessage(analysis, barCurrent, barTarget, streak, title), nil
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

func fallbackCongrats(period analysisPeriod) string {
	switch period {
	case analysisPeriodWeekly:
		return "Weekly summary unavailable from AI. Keep logging meals to unlock coaching insights."
	default:
		return "Keep logging — detailed AI analysis was unavailable this time."
	}
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
	return fmt.Sprintf(`You are an expert nutrition coach. Analyze %s against the user's macro targets.
Return ONLY JSON matching the required schema. Do not write free-form user text outside the JSON fields.
Rules:
- congratulations: encouraging if logging is complete and macros are on track; motivating if meals were missed or macros drifted.
- streak_maintained: true when the user kept consecutive logging days; false if the streak is broken or at risk.
- top_aligned_meals: meals that fit the plan well (protein-forward, balanced calories).
- improvements: concrete meal-level issues with actionable alternatives that reduce bad macros and add missing ones.
- grocery_hints: practical shopping tips (e.g. skyr, whey, legumes) when protein or fiber is lacking.
Keep strings concise and actionable.`, scope)
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
