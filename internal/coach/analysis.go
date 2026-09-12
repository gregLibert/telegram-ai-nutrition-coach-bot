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
	return s.runDailyAnalysis(ctx, userID, time.Now())
}

// WeeklyReport builds the proactive weekly bilan ending on the current local day.
func (s *Service) WeeklyReport(ctx context.Context, userID int64) (string, error) {
	return s.runWeeklyAnalysis(ctx, userID, time.Now())
}

func (s *Service) handleDailyReport(ctx context.Context, user *db.User) (Response, error) {
	msg, err := s.DailyRecap(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}
	return Response{Text: msg}, nil
}

func (s *Service) handleWeeklyReport(ctx context.Context, user *db.User) (Response, error) {
	msg, err := s.WeeklyReport(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}
	return Response{Text: msg}, nil
}

func (s *Service) runDailyAnalysis(ctx context.Context, userID int64, now time.Time) (string, error) {
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

	start, end := domain.DayBounds(loc, localNow)
	meals, err := s.store.ListMealsBetween(ctx, userID, start, end)
	if err != nil {
		return "", err
	}
	progress := sumMealProgress(meals)
	streak, err := s.store.LoggingStreak(ctx, userID, localNow)
	if err != nil {
		return "", err
	}

	view := AnalysisView{
		Lang:           user.Language,
		Period:         analysisPeriodDaily,
		CurrentKcal:    int(progress.Calories),
		TargetKcal:     int(targets.TargetCalories),
		CurrentProtein: progress.ProteinG,
		TargetProtein:  targets.TargetProteinG,
		CurrentCarbs:   progress.CarbsG,
		TargetCarbs:    targets.TargetCarbsG,
		CurrentFat:     progress.FatG,
		TargetFat:      targets.TargetFatG,
		Streak:         streak,
	}

	analysis, err := s.requestDailyNutritionAnalysis(ctx, userID, user.Language, targets, progress, meals, streak)
	if err != nil {
		view.Data = domain.NutritionAnalysis{
			Congratulations:  fallbackCongrats(analysisPeriodDaily, user.Language),
			StreakMaintained: streak > 0,
		}
		return BuildAnalysisMessage(view), nil
	}
	view.Data = analysis
	return BuildAnalysisMessage(view), nil
}

func (s *Service) runWeeklyAnalysis(ctx context.Context, userID int64, now time.Time) (string, error) {
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

	start, end := domain.WeekBounds(loc, localNow)
	meals, err := s.store.ListMealsBetween(ctx, userID, start, end)
	if err != nil {
		return "", err
	}
	progress := sumMealProgress(meals)

	const weekDays = 7.0
	view := WeeklyAnalysisView{
		Lang:          user.Language,
		AvgKcal:       int(progress.Calories / weekDays),
		TargetKcal:    int(targets.TargetCalories),
		AvgProtein:    progress.ProteinG / weekDays,
		TargetProtein: targets.TargetProteinG,
		AvgCarbs:      progress.CarbsG / weekDays,
		TargetCarbs:   targets.TargetCarbsG,
		AvgFat:        progress.FatG / weekDays,
		TargetFat:     targets.TargetFatG,
	}

	weightGoal, weightNote := s.weeklyWeightContext(ctx, userID)
	analysis, err := s.requestWeeklyNutritionAnalysis(ctx, userID, user.Language, targets, progress, meals, weightGoal, weightNote)
	if err != nil {
		view.Data = domain.WeeklyAnalysis{
			Overview: fallbackCongrats(analysisPeriodWeekly, user.Language),
		}
		return BuildWeeklyAnalysisMessage(view), nil
	}
	view.Data = analysis
	return BuildWeeklyAnalysisMessage(view), nil
}

func (s *Service) weeklyWeightContext(ctx context.Context, userID int64) (goal domain.WeightGoal, note string) {
	profile, err := s.store.GetProfile(ctx, userID)
	if err == nil && profile != nil {
		goal = domain.DeduceWeightGoal(profile.WeightKg, profile.TargetWeightKg)
		note = fmt.Sprintf("Current weight %.1f kg, target %.1f kg, goal=%s.",
			profile.WeightKg, profile.TargetWeightKg, goal)
	}
	entries, err := s.store.ListWeightEntries(ctx, userID, 100)
	if err != nil || len(entries) == 0 {
		return goal, note
	}
	weights := make([]float64, 0, len(entries))
	for _, e := range entries {
		weights = append(weights, e.WeightKg)
	}
	stats := domain.WeightStatsFromEntries(weights)
	note = strings.TrimSpace(note + fmt.Sprintf(
		" Recent weights: latest %.1f kg, 7-day moving avg %.1f kg, delta vs start %.1f kg.",
		stats.CurrentKg, stats.MovingAvg7DayKg, stats.DeltaFromStart,
	))
	return goal, note
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

func (s *Service) requestDailyNutritionAnalysis(
	ctx context.Context,
	userID int64,
	lang string,
	targets domain.MacroTargets,
	progress domain.DailyProgress,
	meals []db.MealRow,
	streak int,
) (domain.NutritionAnalysis, error) {
	systemPrompt := withLanguage(dailyNutritionAnalysisSystemPrompt(), lang)
	userPrompt := buildDailyNutritionAnalysisUserPrompt(targets, progress, meals, streak)
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

func (s *Service) requestWeeklyNutritionAnalysis(
	ctx context.Context,
	userID int64,
	lang string,
	targets domain.MacroTargets,
	progress domain.DailyProgress,
	meals []db.MealRow,
	weightGoal domain.WeightGoal,
	weightNote string,
) (domain.WeeklyAnalysis, error) {
	systemPrompt := withLanguage(weeklyNutritionAnalysisSystemPrompt(), lang)
	userPrompt := buildWeeklyNutritionAnalysisUserPrompt(targets, progress, meals, weightGoal, weightNote)
	uid := userID
	raw, err := s.llm.CompleteJSON(ctx, &uid, "weekly_nutrition_analysis", llm.ModelReason, systemPrompt, userPrompt, llm.WeeklyAnalysisSchema)
	if err != nil {
		return domain.WeeklyAnalysis{}, err
	}
	var analysis domain.WeeklyAnalysis
	if err := json.Unmarshal([]byte(raw), &analysis); err != nil {
		return domain.WeeklyAnalysis{}, fmt.Errorf("parse weekly nutrition analysis: %w", err)
	}
	return analysis, nil
}

func dailyNutritionAnalysisSystemPrompt() string {
	return `You are an expert nutrition coach. Analyze today's meals against the user's macro targets.
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
- Do NOT generate grocery hints or shopping lists.
- evening_snack: REQUIRED when the user is below calorie and/or macro targets. Propose ONE realistic late-night snack ("collation de fin de journée") to reach ~100% of targets.
  Constraint 1: NO cooking — only ready-to-eat items (whey, petits suisses, skyr, nuts, fresh fruit, canned tuna, yogurt, etc.).
  Constraint 2: Standard human portions (e.g. "150g de skyr et 15g d'amandes"), never unrealistic amounts like 500g of one item.
  Constraint 3: Briefly explain how the snack fills the specific missing macros (e.g. remaining protein without exploding carbs).
  If the user is already at or above all targets, set evening_snack to an empty string.
Keep strings concise and actionable.`
}

func weeklyNutritionAnalysisSystemPrompt() string {
	return `You are an expert nutrition coach writing a TRUE WEEKLY bilan (not a daily recap).
Analyze the FULL 7-day period. Return ONLY JSON matching the required schema.

STRICT LOCALIZATION (critical):
- Write EVERY string field in the exact same language as the user's meal descriptions / preferred language.
- Never mix languages. No English headers inside content fields.
- The client UI localizes section titles; put only content in JSON fields.

Core goals:
1) Weekly aggregation & weight-loss focus:
   - Reason from week totals and daily averages vs daily targets.
   - Explicitly judge whether the overall caloric deficit (or surplus) supports weight loss.
   - Be motivating about weight loss, but stay honest with the numbers.

2) Macro bottleneck:
   - Identify which macronutrient (proteins, carbs, or fats) was hardest to reach or maintain this week.
   - Put that diagnosis in macro_bottleneck (one clear sentence).

3) Actionable meal swaps & legume focus:
   - In meal_tips: zoom in on 2 or 3 problematic meals MAX from the past week.
   - Each tip must be concrete and mechanical (e.g. reduce crepe count and increase ham; choose thinly sliced chicken breast; pick a lower-fat cheese).
   - In legume_focus: propose substituting a standard starch from the week's meals with legumes (green lentils, chickpeas, split peas, etc.), tied to the macro bottleneck.
     Example style: "Remplacez la purée de pommes de terre par des lentilles vertes ou une purée de pois cassés avec du fromage blanc".

4) Grocery list:
   - grocery_list: targeted shopping ideas for next week, coherent with the macro bottleneck and meal tips.
   - Items can be full-meal ingredients or simple snacks if that best fills the gap.
   - Prefer specific products (e.g. green lentils, thinly sliced chicken breast), not vague categories.

Field guide:
- overview: short paragraph on the week's trend and weight-loss impact.
- macro_bottleneck: the single hardest macro pattern this week.
- meal_tips: 2–3 items max [{meal_name, tip}].
- legume_focus: one legumes-vs-starch swap recommendation.
- grocery_list: 3–6 practical items.
Keep strings concise and actionable.`
}

func buildDailyNutritionAnalysisUserPrompt(
	targets domain.MacroTargets,
	progress domain.DailyProgress,
	meals []db.MealRow,
	streak int,
) string {
	var sb strings.Builder
	sb.WriteString("Period: daily.\n")
	fmt.Fprintf(&sb, "Current streak: %d days.\n", streak)
	fmt.Fprintf(&sb,
		"Targets (daily): %.0f kcal | P %.0f g | F %.0f g | C %.0f g\n",
		targets.TargetCalories, targets.TargetProteinG, targets.TargetFatG, targets.TargetCarbsG,
	)
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
	sb.WriteString("Meals:\n")
	appendMealLines(&sb, meals)
	return sb.String()
}

func buildWeeklyNutritionAnalysisUserPrompt(
	targets domain.MacroTargets,
	progress domain.DailyProgress,
	meals []db.MealRow,
	weightGoal domain.WeightGoal,
	weightNote string,
) string {
	const weekDays = 7.0
	var sb strings.Builder
	sb.WriteString("Period: weekly (full 7-day window).\n")
	fmt.Fprintf(&sb, "Weight goal: %s.\n", weightGoal)
	if weightNote != "" {
		fmt.Fprintf(&sb, "Weight context: %s\n", weightNote)
	}
	fmt.Fprintf(&sb,
		"Targets (daily): %.0f kcal | P %.0f g | F %.0f g | C %.0f g\n",
		targets.TargetCalories, targets.TargetProteinG, targets.TargetFatG, targets.TargetCarbsG,
	)
	fmt.Fprintf(&sb,
		"Consumed (week total): %.0f kcal | P %.0f g | F %.0f g | C %.0f g\n",
		progress.Calories, progress.ProteinG, progress.FatG, progress.CarbsG,
	)
	fmt.Fprintf(&sb,
		"Consumed (daily avg): %.0f kcal | P %.0f g | F %.0f g | C %.0f g\n",
		progress.Calories/weekDays, progress.ProteinG/weekDays, progress.FatG/weekDays, progress.CarbsG/weekDays,
	)
	fmt.Fprintf(&sb,
		"Avg vs target delta: %.0f kcal | P %.0f g | F %.0f g | C %.0f g\n",
		progress.Calories/weekDays-targets.TargetCalories,
		progress.ProteinG/weekDays-targets.TargetProteinG,
		progress.FatG/weekDays-targets.TargetFatG,
		progress.CarbsG/weekDays-targets.TargetCarbsG,
	)
	sb.WriteString("Meals (entire week):\n")
	appendMealLines(&sb, meals)
	return sb.String()
}

func appendMealLines(sb *strings.Builder, meals []db.MealRow) {
	if len(meals) == 0 {
		sb.WriteString("  (none logged)\n")
		return
	}
	for _, m := range meals {
		fmt.Fprintf(sb, "  - %s | %.0f kcal | P %.0f F %.0f C %.0f\n",
			m.Description, m.Calories, m.ProteinG, m.FatG, m.CarbsG)
	}
}
