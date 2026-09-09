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
		{
			name:   "teen boy schofield",
			weight: 60, height: 170, age: 16, gender: GenderMale,
			want: 17.5*60 + 651, tolerance: 0.1,
		},
		{
			name:   "teen girl schofield",
			weight: 55, height: 165, age: 15, gender: GenderFemale,
			want: 12.2*55 + 746, tolerance: 0.1,
		},
		{
			name:   "age 17 uses schofield",
			weight: 70, height: 175, age: 17, gender: GenderMale,
			want: 17.5*70 + 651, tolerance: 0.1,
		},
		{
			name:   "age 18 uses mifflin",
			weight: 70, height: 175, age: 18, gender: GenderMale,
			want: 10*70 + 6.25*175 - 5*18 + 5, tolerance: 0.1,
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
	tests := []struct {
		name   string
		base   MacroTargets
		offset float64
		want   float64
	}{
		{
			name: "applies daily offset",
			base: MacroTargets{
				TargetCalories: 2000,
				TargetProteinG: 120,
				TargetFatG:     69,
				TargetCarbsG:   200,
			},
			offset: ForfaitDailyOffset,
			want:   1900,
		},
		{
			name: "zero offset is no-op",
			base: MacroTargets{
				TargetCalories: 2000,
				TargetProteinG: 120,
				TargetFatG:     69,
				TargetCarbsG:   200,
			},
			offset: 0,
			want:   2000,
		},
		{
			name: "negative offset is no-op",
			base: MacroTargets{
				TargetCalories: 2000,
				TargetProteinG: 120,
				TargetFatG:     69,
				TargetCarbsG:   200,
			},
			offset: -50,
			want:   2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adjusted := ApplyForfaitAdjustment(tt.base, tt.offset)
			if adjusted.TargetCalories != tt.want {
				t.Errorf("adjusted calories = %v, want %v", adjusted.TargetCalories, tt.want)
			}
		})
	}
}

func TestRemainingMacros(t *testing.T) {
	tests := []struct {
		name     string
		targets  MacroTargets
		consumed DailyProgress
		wantCals float64
		wantProt float64
	}{
		{
			name: "partial day",
			targets: MacroTargets{
				TargetCalories: 2000,
				TargetProteinG: 120,
				TargetFatG:     69,
				TargetCarbsG:   200,
			},
			consumed: DailyProgress{Calories: 800, ProteinG: 40, FatG: 25, CarbsG: 80},
			wantCals: 1200,
			wantProt: 80,
		},
		{
			name: "over target goes negative",
			targets: MacroTargets{
				TargetCalories: 2000,
				TargetProteinG: 120,
				TargetFatG:     69,
				TargetCarbsG:   200,
			},
			consumed: DailyProgress{Calories: 2500, ProteinG: 150, FatG: 80, CarbsG: 250},
			wantCals: -500,
			wantProt: -30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remaining := RemainingMacros(tt.targets, tt.consumed)
			if remaining.TargetCalories != tt.wantCals {
				t.Errorf("remaining calories = %v, want %v", remaining.TargetCalories, tt.wantCals)
			}
			if remaining.TargetProteinG != tt.wantProt {
				t.Errorf("remaining protein = %v, want %v", remaining.TargetProteinG, tt.wantProt)
			}
		})
	}
}

func TestWeightStatsFromEntries(t *testing.T) {
	tests := []struct {
		name        string
		weights     []float64
		wantCount   int
		wantDelta   float64
		wantCurrent float64
	}{
		{name: "empty", weights: nil, wantCount: 0},
		{name: "single", weights: []float64{80}, wantCount: 1, wantCurrent: 80, wantDelta: 0},
		{name: "progress", weights: []float64{85, 84, 83}, wantCount: 3, wantCurrent: 83, wantDelta: -2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WeightStatsFromEntries(tt.weights)
			if got.EntryCount != tt.wantCount {
				t.Errorf("count = %d, want %d", got.EntryCount, tt.wantCount)
			}
			if tt.wantCount == 0 {
				return
			}
			if got.CurrentKg != tt.wantCurrent {
				t.Errorf("current = %v, want %v", got.CurrentKg, tt.wantCurrent)
			}
			if got.DeltaFromStart != tt.wantDelta {
				t.Errorf("delta = %v, want %v", got.DeltaFromStart, tt.wantDelta)
			}
		})
	}
}

func TestCalculateMacroTargetsFallbackTargetWeight(t *testing.T) {
	input := ProfileInput{
		Age: 30, HeightCm: 180, WeightKg: 80, TargetWeightKg: 0,
		Gender: GenderMale, ActivityLevel: ActivityModerate, WeightGoal: GoalKeep,
	}
	got := CalculateMacroTargets(input)
	wantProtein := ProteinPerKg * input.WeightKg
	if math.Abs(got.TargetProteinG-wantProtein) > 0.2 {
		t.Fatalf("protein = %v, want %v (fallback to current weight)", got.TargetProteinG, wantProtein)
	}
}

func TestCalculateMacroTargetsUnknownActivityUsesSedentary(t *testing.T) {
	input := ProfileInput{
		Age: 30, HeightCm: 180, WeightKg: 80, TargetWeightKg: 80,
		Gender: GenderMale, ActivityLevel: ActivityLevel("unknown"), WeightGoal: GoalKeep,
	}
	got := CalculateMacroTargets(input)
	bmr := CalculateBMR(input.WeightKg, input.HeightCm, input.Age, input.Gender)
	wantTDEE := round1(bmr * activityMultSedentary)
	if got.TDEE != wantTDEE {
		t.Fatalf("tdee = %v, want sedentary fallback %v", got.TDEE, wantTDEE)
	}
}
