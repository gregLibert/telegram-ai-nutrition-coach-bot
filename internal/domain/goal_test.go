package domain

import "testing"

func TestDeduceWeightGoal(t *testing.T) {
	tests := []struct {
		name    string
		current float64
		target  float64
		want    WeightGoal
	}{
		{"lose weight", 85, 75, GoalLose},
		{"gain weight", 70, 80, GoalGain},
		{"maintain exact", 80, 80, GoalKeep},
		{"maintain float exact", 80.0, 80.0, GoalKeep},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduceWeightGoal(tt.current, tt.target)
			if got != tt.want {
				t.Errorf("DeduceWeightGoal(%v, %v) = %v, want %v", tt.current, tt.target, got, tt.want)
			}
		})
	}
}
