package coach

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/db"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/state"
)

const (
	minOnboardingAge    = 10
	maxOnboardingAge    = 120
	minOnboardingHeight = 100
	maxOnboardingHeight = 250
	minOnboardingWeight = 30
	maxOnboardingWeight = 300
)

func (s *Service) handleOnboardingText(ctx context.Context, user *db.User, text string) (Response, bool, error) {
	switch user.State {
	case state.OnboardingLanguage:
		resp, err := s.onboardingLanguage(ctx, user, text)
		return resp, true, err
	case state.OnboardingAge:
		resp, err := s.onboardingAge(ctx, user, text)
		return resp, true, err
	case state.OnboardingHeight:
		resp, err := s.onboardingHeight(ctx, user, text)
		return resp, true, err
	case state.OnboardingWeight:
		resp, err := s.onboardingWeight(ctx, user, text)
		return resp, true, err
	case state.OnboardingGender:
		resp, err := s.onboardingGender(ctx, user, text)
		return resp, true, err
	case state.OnboardingActivity:
		resp, err := s.onboardingActivity(ctx, user, text)
		return resp, true, err
	case state.OnboardingTarget, state.Name("onboarding_goal"):
		resp, err := s.onboardingTarget(ctx, user, text)
		return resp, true, err
	case state.OnboardingExclusions:
		resp, err := s.onboardingExclusions(ctx, user, text)
		return resp, true, err
	case state.OnboardingRegion:
		resp, err := s.onboardingRegion(ctx, user, text)
		return resp, true, err
	default:
		return Response{}, false, nil
	}
}

func (s *Service) onboardingLanguage(ctx context.Context, user *db.User, text string) (Response, error) {
	lang := strings.ToLower(strings.TrimSpace(text))
	switch lang {
	case "en", "fr":
	default:
		return Response{Text: "Please enter 'en' or 'fr'."}, nil
	}
	if err := s.store.UpdateUserLanguage(ctx, user.ID, lang); err != nil {
		return Response{}, err
	}
	user.Language = lang
	if err := s.store.UpdateUserState(ctx, user.ID, state.OnboardingAge, user.StateData); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.OnboardingLanguage, state.OnboardingAge, "language_input")
	return Response{Text: "How old are you? (years)"}, nil
}

func (s *Service) onboardingAge(ctx context.Context, user *db.User, text string) (Response, error) {
	age, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || age < minOnboardingAge || age > maxOnboardingAge {
		return Response{Text: fmt.Sprintf("Please enter a valid age (%d-%d).", minOnboardingAge, maxOnboardingAge)}, nil
	}
	data := user.StateData.Set("age", strconv.Itoa(age))
	if err := s.store.UpdateUserState(ctx, user.ID, state.OnboardingHeight, data); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.OnboardingAge, state.OnboardingHeight, "age_input")
	return Response{Text: "What's your height in cm? (e.g. 175)"}, nil
}

func (s *Service) onboardingHeight(ctx context.Context, user *db.User, text string) (Response, error) {
	h, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || h < minOnboardingHeight || h > maxOnboardingHeight {
		return Response{Text: fmt.Sprintf("Please enter a valid height (%d-%d cm).", minOnboardingHeight, maxOnboardingHeight)}, nil
	}
	data := user.StateData.Set("height_cm", fmt.Sprintf("%.0f", h))
	if err := s.store.UpdateUserState(ctx, user.ID, state.OnboardingWeight, data); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.OnboardingHeight, state.OnboardingWeight, "height_input")
	return Response{Text: "What's your current weight in kg? (e.g. 80)"}, nil
}

func (s *Service) onboardingWeight(ctx context.Context, user *db.User, text string) (Response, error) {
	w, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || w < minOnboardingWeight || w > maxOnboardingWeight {
		return Response{Text: fmt.Sprintf("Please enter a valid weight (%d-%d kg).", minOnboardingWeight, maxOnboardingWeight)}, nil
	}
	data := user.StateData.Set("weight_kg", fmt.Sprintf("%.1f", w))
	if err := s.store.UpdateUserState(ctx, user.ID, state.OnboardingGender, data); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.OnboardingWeight, state.OnboardingGender, "weight_input")
	return Response{Text: "What's your gender? (male / female)"}, nil
}

func (s *Service) onboardingGender(ctx context.Context, user *db.User, text string) (Response, error) {
	g := strings.ToLower(strings.TrimSpace(text))
	switch g {
	case "male", "female":
	default:
		return Response{Text: "Please enter 'male' or 'female'."}, nil
	}
	data := user.StateData.Set("gender", g)
	if err := s.store.UpdateUserState(ctx, user.ID, state.OnboardingActivity, data); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.OnboardingGender, state.OnboardingActivity, "gender_input")
	return Response{Text: s.activityPrompt()}, nil
}

func (s *Service) activityPrompt() string {
	return "What's your baseline activity level?\n" +
		"  sedentary — desk job, little exercise\n" +
		"  light — 1-3 workouts/week\n" +
		"  moderate — 3-5 workouts/week\n" +
		"  active — 6-7 workouts/week\n" +
		"  very_active — athlete / physical job"
}

func (s *Service) onboardingActivity(ctx context.Context, user *db.User, text string) (Response, error) {
	level := domain.ActivityLevel(strings.ToLower(strings.TrimSpace(text)))
	switch level {
	case domain.ActivitySedentary, domain.ActivityLight, domain.ActivityModerate,
		domain.ActivityActive, domain.ActivityVeryActive:
	default:
		return Response{Text: "Invalid activity level.\n\n" + s.activityPrompt()}, nil
	}
	data := user.StateData.Set("activity_level", string(level))
	if err := s.store.UpdateUserState(ctx, user.ID, state.OnboardingTarget, data); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.OnboardingActivity, state.OnboardingTarget, "activity_input")
	return Response{Text: "What's your target weight in kg?"}, nil
}

func (s *Service) onboardingTarget(ctx context.Context, user *db.User, text string) (Response, error) {
	tw, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || tw < minOnboardingWeight || tw > maxOnboardingWeight {
		return Response{Text: fmt.Sprintf("Please enter a valid target weight (%d-%d kg).", minOnboardingWeight, maxOnboardingWeight)}, nil
	}

	data := user.StateData.Set("target_weight_kg", fmt.Sprintf("%.1f", tw))
	if err := s.store.UpdateUserState(ctx, user.ID, state.OnboardingExclusions, data); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.OnboardingTarget, state.OnboardingExclusions, "target_input")
	return Response{Text: "Any allergies or foods you dislike? (e.g. 'peanuts, vegan' or 'none')"}, nil
}

func (s *Service) onboardingExclusions(ctx context.Context, user *db.User, text string) (Response, error) {
	exclusions := strings.TrimSpace(text)
	if strings.EqualFold(exclusions, "none") || exclusions == "" {
		exclusions = ""
	}
	data := user.StateData.Set("excluded_ingredients", exclusions)
	if err := s.store.UpdateUserState(ctx, user.ID, state.OnboardingRegion, data); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.OnboardingExclusions, state.OnboardingRegion, "exclusions_input")
	return Response{Text: "In which country/region do you live? (Helps tailor recipe ingredients)"}, nil
}

func (s *Service) onboardingRegion(ctx context.Context, user *db.User, text string) (Response, error) {
	region := strings.TrimSpace(text)
	if region == "" {
		return Response{Text: "Please enter your country or region."}, nil
	}

	tw, err := strconv.ParseFloat(user.StateData.Get("target_weight_kg"), 64)
	if err != nil {
		return Response{Text: "Missing target weight. Restart with /start."}, nil
	}
	input, err := s.buildProfileInput(user.StateData, tw, region)
	if err != nil {
		return Response{Text: err.Error()}, nil
	}

	targets := domain.CalculateMacroTargets(input)
	if err := s.store.SaveProfile(ctx, user.ID, input, targets); err != nil {
		return Response{}, err
	}
	if err := s.store.AddWeightEntry(ctx, user.ID, input.WeightKg); err != nil {
		return Response{}, err
	}
	if err := s.store.UpdateUserState(ctx, user.ID, state.Idle, state.Data{}); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.OnboardingRegion, state.Idle, "profile_complete")

	msg := fmt.Sprintf(
		"✅ Profile created!\n\nBMR: %.0f kcal | TDEE: %.0f kcal\nRegion: %s\n\n🎯 Daily Targets:\n"+
			"Calories: %.0f kcal (deficit: %.0f)\nProtein: %.0f g | Fat: %.0f g | Carbs: %.0f g\n\n"+
			"Describe meals in text, send photos, or use /weight to track weight.",
		targets.BMR, targets.TDEE, region,
		targets.TargetCalories, targets.TDEE-targets.TargetCalories,
		targets.TargetProteinG, targets.TargetFatG, targets.TargetCarbsG,
	)
	return Response{Text: msg}, nil
}

func (s *Service) buildProfileInput(data state.Data, targetWeight float64, region string) (domain.ProfileInput, error) {
	age, err := strconv.Atoi(data.Get("age"))
	if err != nil {
		return domain.ProfileInput{}, fmt.Errorf("missing profile data, restart with /start")
	}
	height, err := strconv.ParseFloat(data.Get("height_cm"), 64)
	if err != nil {
		return domain.ProfileInput{}, fmt.Errorf("missing height, restart with /start")
	}
	weight, err := strconv.ParseFloat(data.Get("weight_kg"), 64)
	if err != nil {
		return domain.ProfileInput{}, fmt.Errorf("missing weight, restart with /start")
	}
	return domain.ProfileInput{
		Age: age, HeightCm: height, WeightKg: weight, TargetWeightKg: targetWeight,
		Gender:              domain.Gender(data.Get("gender")),
		ActivityLevel:       domain.ActivityLevel(data.Get("activity_level")),
		WeightGoal:          domain.DeduceWeightGoal(weight, targetWeight),
		ExcludedIngredients: data.Get("excluded_ingredients"),
		Region:              region,
	}, nil
}
