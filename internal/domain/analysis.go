package domain

// MealImprovement suggests a concrete fix for a misaligned meal (daily report).
type MealImprovement struct {
	MealName    string `json:"meal_name"`
	Issue       string `json:"issue"`
	Alternative string `json:"alternative"`
}

// NutritionAnalysis is the structured LLM output for the daily coaching report.
type NutritionAnalysis struct {
	Congratulations  string            `json:"congratulations"`
	StreakMaintained bool              `json:"streak_maintained"`
	TopAlignedMeals  []string          `json:"top_aligned_meals"`
	Improvements     []MealImprovement `json:"improvements"`
	EveningSnack     string            `json:"evening_snack"`
}

// WeeklyMealTip is a concrete, mechanical tip for one problematic meal.
type WeeklyMealTip struct {
	MealName string `json:"meal_name"`
	Tip      string `json:"tip"`
}

// WeeklyAnalysis is the structured LLM output for the weekly bilan.
type WeeklyAnalysis struct {
	Overview        string          `json:"overview"`
	MacroBottleneck string          `json:"macro_bottleneck"`
	MealTips        []WeeklyMealTip `json:"meal_tips"`
	LegumeFocus     string          `json:"legume_focus"`
	GroceryList     []string        `json:"grocery_list"`
}
