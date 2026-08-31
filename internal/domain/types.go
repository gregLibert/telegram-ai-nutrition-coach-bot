package domain

import "time"

const (
	DeficitKcal         = 500
	ProteinPerKg        = 1.6
	FatCalorieFraction  = 0.31
	WeightRecalcDeltaKg = 1.0
	PlateauVarianceKg   = 0.2
	PlateauWindowDays   = 14
	ForfaitSmoothDays   = 3
	ForfaitDailyOffset  = 100
)

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
)

type ActivityLevel string

const (
	ActivitySedentary  ActivityLevel = "sedentary"
	ActivityLight      ActivityLevel = "light"
	ActivityModerate   ActivityLevel = "moderate"
	ActivityActive     ActivityLevel = "active"
	ActivityVeryActive ActivityLevel = "very_active"
)

type WeightGoal string

const (
	GoalLose WeightGoal = "lose"
	GoalKeep WeightGoal = "keep"
	GoalGain WeightGoal = "gain"
)

type ProfileInput struct {
	Age                 int
	HeightCm            float64
	WeightKg            float64
	TargetWeightKg      float64
	Gender              Gender
	ActivityLevel       ActivityLevel
	WeightGoal          WeightGoal
	ExcludedIngredients string
	Region              string
}

type MacroTargets struct {
	BMR            float64
	TDEE           float64
	TargetCalories float64
	TargetProteinG float64
	TargetFatG     float64
	TargetCarbsG   float64
}

type WeightStats struct {
	CurrentKg       float64
	MovingAvg7DayKg float64
	DeltaFromStart  float64
	EntryCount      int
}

type PlateauStatus struct {
	Detected      bool
	VarianceKg    float64
	TrackingDays  int
	DietBreakDays int
	SuggestedTDEE float64
}

type DailyProgress struct {
	Date           time.Time
	Calories       float64
	ProteinG       float64
	FatG           float64
	CarbsG         float64
	TargetCalories float64
	TargetProteinG float64
	TargetFatG     float64
	TargetCarbsG   float64
}

type MealEstimate struct {
	Description string  `json:"description"`
	Calories    float64 `json:"calories"`
	ProteinG    float64 `json:"protein_g"`
	FatG        float64 `json:"fat_g"`
	CarbsG      float64 `json:"carbs_g"`
	Confidence  string  `json:"confidence"`
	Notes       string  `json:"notes"`
}

type PortionSolution struct {
	Ingredients   []PortionIngredient `json:"ingredients"`
	TotalCalories float64             `json:"total_calories"`
	TotalProteinG float64             `json:"total_protein_g"`
	TotalFatG     float64             `json:"total_fat_g"`
	TotalCarbsG   float64             `json:"total_carbs_g"`
	Explanation   string              `json:"explanation"`
}

type PortionIngredient struct {
	Name     string  `json:"name"`
	RawGrams float64 `json:"raw_grams"`
	ProteinG float64 `json:"protein_g"`
	FatG     float64 `json:"fat_g"`
	CarbsG   float64 `json:"carbs_g"`
	Calories float64 `json:"calories"`
}

type RecipeOption struct {
	ID             int      `json:"id"`
	Title          string   `json:"title"`
	MacrosSummary  string   `json:"macros_summary"`
	KeyIngredients []string `json:"key_ingredients"`
}

type RecipeOptionsResult struct {
	Options []RecipeOption `json:"options"`
}

type ForfaitPreset struct {
	Key      string
	Label    string
	Calories float64
}

var ForfaitPresets = []ForfaitPreset{
	{Key: "pizza_night", Label: "Family Pizza Night", Calories: 1200},
	{Key: "bbq_party", Label: "BBQ Party", Calories: 1500},
	{Key: "restaurant", Label: "Restaurant Meal", Calories: 900},
	{Key: "birthday", Label: "Birthday Celebration", Calories: 1100},
	{Key: "fast_food", Label: "Fast Food Run", Calories: 800},
}

func FindForfaitPreset(key string) (ForfaitPreset, bool) {
	for _, p := range ForfaitPresets {
		if p.Key == key {
			return p, true
		}
	}
	return ForfaitPreset{}, false
}
