package domain

// DeduceWeightGoal infers lose/keep/gain from current and target weight (exact equality => keep).
func DeduceWeightGoal(currentKg, targetKg float64) WeightGoal {
	switch {
	case targetKg < currentKg:
		return GoalLose
	case targetKg > currentKg:
		return GoalGain
	default:
		return GoalKeep
	}
}
