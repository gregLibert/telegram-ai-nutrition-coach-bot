package domain

// MealImprovement suggests a concrete fix for a misaligned meal.
type MealImprovement struct {
	MealName    string `json:"meal_name"`
	Issue       string `json:"issue"`
	Alternative string `json:"alternative"`
}

// NutritionAnalysis is the structured LLM output for daily/weekly coaching.
type NutritionAnalysis struct {
	Congratulations  string            `json:"congratulations"`
	StreakMaintained bool              `json:"streak_maintained"`
	TopAlignedMeals  []string          `json:"top_aligned_meals"`
	Improvements     []MealImprovement `json:"improvements"`
	GroceryHints     []string          `json:"grocery_hints"`
}
