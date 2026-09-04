package domain

import "math"

func activityMultiplier(level ActivityLevel) float64 {
	switch level {
	case ActivitySedentary:
		return 1.2
	case ActivityLight:
		return 1.375
	case ActivityModerate:
		return 1.55
	case ActivityActive:
		return 1.725
	case ActivityVeryActive:
		return 1.9
	default:
		return 1.2
	}
}

// CalculateBMR uses Schofield for adolescents (age < 18) and Mifflin-St Jeor for adults.
func CalculateBMR(weightKg, heightCm float64, age int, gender Gender) float64 {
	if age < 18 {
		return calculateSchofieldBMR(weightKg, gender)
	}
	base := 10*weightKg + 6.25*heightCm - 5*float64(age)
	switch gender {
	case GenderMale:
		return base + 5
	case GenderFemale:
		return base - 161
	default:
		return base + 5
	}
}

// calculateSchofieldBMR applies the Schofield equation for ages 10–17.
func calculateSchofieldBMR(weightKg float64, gender Gender) float64 {
	switch gender {
	case GenderFemale:
		return 12.2*weightKg + 746
	default: // male / boys
		return 17.5*weightKg + 651
	}
}

// CalculateTDEE returns total daily energy expenditure.
func CalculateTDEE(bmr float64, level ActivityLevel) float64 {
	return bmr * activityMultiplier(level)
}

// CalculateMacroTargets computes caloric and macro targets from profile input.
func CalculateMacroTargets(input ProfileInput) MacroTargets {
	bmr := CalculateBMR(input.WeightKg, input.HeightCm, input.Age, input.Gender)
	tdee := CalculateTDEE(bmr, input.ActivityLevel)

	targetCalories := tdee
	switch input.WeightGoal {
	case GoalLose:
		targetCalories = tdee - DeficitKcal
	case GoalGain:
		targetCalories = tdee + DeficitKcal
	case GoalKeep:
		targetCalories = tdee
	}

	targetWeight := input.TargetWeightKg
	if targetWeight <= 0 {
		targetWeight = input.WeightKg
	}

	proteinG := ProteinPerKg * targetWeight
	proteinCal := proteinG * 4
	fatCal := targetCalories * FatCalorieFraction
	fatG := fatCal / 9
	carbCal := targetCalories - proteinCal - fatCal
	if carbCal < 0 {
		carbCal = 0
	}
	carbG := carbCal / 4

	return MacroTargets{
		BMR:            round1(bmr),
		TDEE:           round1(tdee),
		TargetCalories: round1(targetCalories),
		TargetProteinG: round1(proteinG),
		TargetFatG:     round1(fatG),
		TargetCarbsG:   round1(carbG),
	}
}

// ApplyForfaitAdjustment reduces daily target calories during smoothing period.
func ApplyForfaitAdjustment(targets MacroTargets, dailyOffset float64) MacroTargets {
	if dailyOffset <= 0 {
		return targets
	}
	adjusted := targets
	adjusted.TargetCalories = round1(targets.TargetCalories - dailyOffset)

	fatCal := adjusted.TargetCalories * FatCalorieFraction
	proteinCal := targets.TargetProteinG * 4
	carbCal := adjusted.TargetCalories - proteinCal - fatCal
	if carbCal < 0 {
		carbCal = 0
	}
	adjusted.TargetFatG = round1(fatCal / 9)
	adjusted.TargetCarbsG = round1(carbCal / 4)
	return adjusted
}

// RemainingMacros returns what's left for the day after logged meals.
func RemainingMacros(targets MacroTargets, consumed DailyProgress) MacroTargets {
	return MacroTargets{
		TargetCalories: round1(targets.TargetCalories - consumed.Calories),
		TargetProteinG: round1(targets.TargetProteinG - consumed.ProteinG),
		TargetFatG:     round1(targets.TargetFatG - consumed.FatG),
		TargetCarbsG:   round1(targets.TargetCarbsG - consumed.CarbsG),
	}
}

// WeightMovingAverage computes a simple 7-day moving average from weight entries.
func WeightMovingAverage(weights []float64) float64 {
	if len(weights) == 0 {
		return 0
	}
	window := weights
	if len(weights) > 7 {
		window = weights[len(weights)-7:]
	}
	sum := 0.0
	for _, w := range window {
		sum += w
	}
	return round2(sum / float64(len(window)))
}

// WeightStatsFromEntries builds weight statistics from chronological entries.
func WeightStatsFromEntries(weights []float64) WeightStats {
	if len(weights) == 0 {
		return WeightStats{}
	}
	current := weights[len(weights)-1]
	start := weights[0]
	return WeightStats{
		CurrentKg:       round2(current),
		MovingAvg7DayKg: WeightMovingAverage(weights),
		DeltaFromStart:  round2(current - start),
		EntryCount:      len(weights),
	}
}

// ShouldRecalculateTargets returns true when weight changed enough from baseline.
func ShouldRecalculateTargets(currentKg, baselineKg float64) bool {
	return math.Abs(currentKg-baselineKg) >= WeightRecalcDeltaKg
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
