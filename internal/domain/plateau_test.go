package domain

import "testing"

func TestDetectPlateau(t *testing.T) {
	tests := []struct {
		name          string
		movingAvgs    []float64
		trackingDays  int
		loggedDeficit bool
		wantDetected  bool
	}{
		{
			name:         "insufficient data",
			movingAvgs:   []float64{80, 79.9},
			trackingDays: 5,
			wantDetected: false,
		},
		{
			name: "stable weight with deficit",
			movingAvgs: []float64{
				80.0, 79.95, 80.05, 79.98, 80.02, 79.97, 80.01,
				80.0, 79.99, 80.03, 79.96, 80.04, 79.98, 80.01,
			},
			trackingDays:  14,
			loggedDeficit: true,
			wantDetected:  true,
		},
		{
			name: "stable weight no deficit logged",
			movingAvgs: []float64{
				80.0, 79.95, 80.05, 79.98, 80.02, 79.97, 80.01,
				80.0, 79.99, 80.03, 79.96, 80.04, 79.98, 80.01,
			},
			trackingDays:  14,
			loggedDeficit: false,
			wantDetected:  false,
		},
		{
			name: "declining weight",
			movingAvgs: []float64{
				82, 81.5, 81, 80.5, 80, 79.5, 79,
				78.5, 78, 77.5, 77, 76.5, 76, 75.5,
			},
			trackingDays:  14,
			loggedDeficit: true,
			wantDetected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectPlateau(tt.movingAvgs, tt.trackingDays, tt.loggedDeficit)
			if got.Detected != tt.wantDetected {
				t.Errorf("Detected = %v, want %v (variance=%v)", got.Detected, tt.wantDetected, got.VarianceKg)
			}
			if got.Detected && got.DietBreakDays != 7 {
				t.Errorf("DietBreakDays = %v, want 7", got.DietBreakDays)
			}
		})
	}
}

func TestDietBreakTargets(t *testing.T) {
	targets := DietBreakTargets(2500, 128)
	if targets.TargetCalories != 2500 {
		t.Errorf("TargetCalories = %v, want 2500", targets.TargetCalories)
	}
	if targets.TargetProteinG != 128 {
		t.Errorf("TargetProteinG = %v, want 128", targets.TargetProteinG)
	}
}

func TestMovingAvgVariance(t *testing.T) {
	v := MovingAvgVariance([]float64{80.0, 80.1, 79.95, 80.05})
	if v > PlateauVarianceKg {
		t.Errorf("variance %v should be < %v", v, PlateauVarianceKg)
	}
}
