package domain

import "math"

// DetectPlateau checks if weight loss has stalled despite a caloric deficit.
func DetectPlateau(movingAvgs []float64, trackingDays int, loggedDeficit bool) PlateauStatus {
	status := PlateauStatus{
		TrackingDays: trackingDays,
	}

	if trackingDays < PlateauWindowDays || len(movingAvgs) < PlateauWindowDays {
		return status
	}

	window := movingAvgs[len(movingAvgs)-PlateauWindowDays:]
	minVal, maxVal := window[0], window[0]
	for _, v := range window[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	variance := maxVal - minVal
	status.VarianceKg = round2(variance)

	if variance < PlateauVarianceKg && loggedDeficit {
		status.Detected = true
		status.DietBreakDays = 7
	}

	return status
}

// DietBreakTargets resets daily targets to maintenance (TDEE).
func DietBreakTargets(tdee float64, proteinG float64) MacroTargets {
	fatCal := tdee * FatCalorieFraction
	proteinCal := proteinG * 4
	carbCal := tdee - proteinCal - fatCal
	if carbCal < 0 {
		carbCal = 0
	}

	return MacroTargets{
		TDEE:           round1(tdee),
		TargetCalories: round1(tdee),
		TargetProteinG: round1(proteinG),
		TargetFatG:     round1(fatCal / 9),
		TargetCarbsG:   round1(carbCal / 4),
	}
}

// MovingAvgVariance returns max-min spread of values.
func MovingAvgVariance(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	minVal, maxVal := values[0], values[0]
	for _, v := range values[1:] {
		minVal = math.Min(minVal, v)
		maxVal = math.Max(maxVal, v)
	}
	return round2(maxVal - minVal)
}
