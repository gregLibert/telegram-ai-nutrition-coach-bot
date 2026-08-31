package state

import "encoding/json"

type Name string

const (
	Idle                 Name = "idle"
	OnboardingAge        Name = "onboarding_age"
	OnboardingHeight     Name = "onboarding_height"
	OnboardingWeight     Name = "onboarding_weight"
	OnboardingGender     Name = "onboarding_gender"
	OnboardingActivity   Name = "onboarding_activity"
	OnboardingGoal       Name = "onboarding_goal"
	OnboardingTarget     Name = "onboarding_target_weight"
	OnboardingExclusions Name = "onboarding_exclusions"
	OnboardingRegion     Name = "onboarding_region"
	AwaitingWeight       Name = "awaiting_weight"
	AwaitingMeal         Name = "awaiting_meal"
	AwaitingForfait      Name = "awaiting_forfait"
	AwaitingRecipeChoice Name = "awaiting_recipe_choice"
)

type Data map[string]string

func ParseData(raw string) Data {
	if raw == "" || raw == "{}" {
		return Data{}
	}
	var d Data
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return Data{}
	}
	return d
}

func (d Data) JSON() string {
	if len(d) == 0 {
		return "{}"
	}
	b, err := json.Marshal(d)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (d Data) Set(key, value string) Data {
	if d == nil {
		d = Data{}
	}
	d[key] = value
	return d
}

func (d Data) Get(key string) string {
	if d == nil {
		return ""
	}
	return d[key]
}
