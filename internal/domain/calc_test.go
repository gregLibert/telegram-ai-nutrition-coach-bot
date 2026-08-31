package domain

import (
	"math"
	"testing"
)

func TestCalculateBMR(t *testing.T) {
	tests := []struct {
		name      string
		weight    float64
		height    float64
		age       int
		gender    Gender
		want      float64
		tolerance float64
	}{
		{
			name:   "male standard",
			weight: 80, height: 180, age: 30, gender: GenderMale,
			want: 1780, tolerance: 1,
		},
		{
			name:   "female standard",
			weight: 65, height: 165, age: 28, gender: GenderFemale,
			want: 1380.25, tolerance: 1,
		},
		{
			name:   "male elderly",
			weight: 90, height: 175, age: 55, gender: GenderMale,
			want: 1723.75, tolerance: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateBMR(tt.weight, tt.height, tt.age, tt.gender)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("CalculateBMR() = %v, want %v (±%v)", got, tt.want, tt.tolerance)
			}
		})
	}
}

func TestCalculateTDEE(t *testing.T) {
	tests := []struct {
		name  string
		bmr   float64
		level ActivityLevel
		want  float64
	}{
		{"sedentary", 1500, ActivitySedentary, 1800},
		{"moderate", 1500, ActivityModerate, 2325},
		{"very active", 2000, ActivityVeryActive, 3800},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateTDEE(tt.bmr, tt.level)
			if math.Abs(got-tt.want) > 1 {
				t.Errorf("CalculateTDEE() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateMacroTargets(t *testing.T) {
	tests := []struct {
		name  string
		input ProfileInput
		check func(t *testing.T, m MacroTargets)
	}{
		{
			name: "lose weight male",
			input: ProfileInput{
				Age: 30, HeightCm: 180, WeightKg: 85, TargetWeightKg: 75,
				Gender: GenderMale, ActivityLevel: ActivityModerate, WeightGoal: GoalLose,
			},
			check: func(t *testing.T, m MacroTargets) {
				t.Helper()
				if m.TDEE-m.TargetCalories != DeficitKcal {
					t.Errorf("deficit = %v, want %v", m.TDEE-m.TargetCalories, float64(DeficitKcal))
				}
				wantProtein := ProteinPerKg * 75
				if math.Abs(m.TargetProteinG-wantProtein) > 0.2 {
					t.Errorf("protein = %v, want %v", m.TargetProteinG, wantProtein)
				}
				fatCalPct := (m.TargetFatG * 9) / m.TargetCalories
				if math.Abs(fatCalPct-FatCalorieFraction) > 0.02 {
					t.Errorf("fat pct = %v, want ~%v", fatCalPct, FatCalorieFraction)
				}
			},
		},
		{
			name: "maintain weight female",
			input: ProfileInput{
				Age: 25, HeightCm: 165, WeightKg: 60, TargetWeightKg: 60,
				Gender: GenderFemale, ActivityLevel: ActivityLight, WeightGoal: GoalKeep,
			},
			check: func(t *testing.T, m MacroTargets) {
				t.Helper()
				if m.TargetCalories != m.TDEE {
					t.Errorf("maintain calories = %v, tdee = %v", m.TargetCalories, m.TDEE)
				}
			},
		},
		{
			name: "gain weight",
			input: ProfileInput{
				Age: 22, HeightCm: 190, WeightKg: 70, TargetWeightKg: 80,
				Gender: GenderMale, ActivityLevel: ActivityActive, WeightGoal: GoalGain,
			},
			check: func(t *testing.T, m MacroTargets) {
				t.Helper()
				if m.TargetCalories-m.TDEE != DeficitKcal {
					t.Errorf("surplus = %v, want %v", m.TargetCalories-m.TDEE, float64(DeficitKcal))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateMacroTargets(tt.input)
			if got.BMR <= 0 || got.TDEE <= 0 || got.TargetCalories <= 0 {
				t.Fatalf("invalid targets: %+v", got)
			}
			tt.check(t, got)
		})
	}
}

func TestWeightMovingAverage(t *testing.T) {
	tests := []struct {
		name    string
		weights []float64
		want    float64
	}{
		{"empty", nil, 0},
		{"single", []float64{80}, 80},
		{"seven days", []float64{80, 79.5, 79.8, 79.2, 78.9, 78.5, 78.0}, 79.13},
		{"more than seven", []float64{85, 84, 83, 82, 81, 80, 79.5, 79, 78.5, 78}, 79.71},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WeightMovingAverage(tt.weights)
			if math.Abs(got-tt.want) > 0.1 {
				t.Errorf("WeightMovingAverage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldRecalculateTargets(t *testing.T) {
	tests := []struct {
		current  float64
		baseline float64
		want     bool
	}{
		{80, 80, false},
		{80.9, 80, false},
		{81, 80, true},
		{79, 80, true},
		{79.1, 80, false},
	}

	for _, tt := range tests {
		got := ShouldRecalculateTargets(tt.current, tt.baseline)
		if got != tt.want {
			t.Errorf("ShouldRecalculateTargets(%v, %v) = %v, want %v",
				tt.current, tt.baseline, got, tt.want)
		}
	}
}

func TestApplyForfaitAdjustment(t *testing.T) {
	base := MacroTargets{
		TargetCalories: 2000,
		TargetProteinG: 120,
		TargetFatG:     69,
		TargetCarbsG:   200,
	}
	adjusted := ApplyForfaitAdjustment(base, ForfaitDailyOffset)
	if adjusted.TargetCalories != 1900 {
		t.Errorf("adjusted calories = %v, want 1900", adjusted.TargetCalories)
	}
}

func TestRemainingMacros(t *testing.T) {
	targets := MacroTargets{
		TargetCalories: 2000,
		TargetProteinG: 120,
		TargetFatG:     69,
		TargetCarbsG:   200,
	}
	consumed := DailyProgress{Calories: 800, ProteinG: 40, FatG: 25, CarbsG: 80}
	remaining := RemainingMacros(targets, consumed)
	if remaining.TargetCalories != 1200 {
		t.Errorf("remaining calories = %v, want 1200", remaining.TargetCalories)
	}
}
