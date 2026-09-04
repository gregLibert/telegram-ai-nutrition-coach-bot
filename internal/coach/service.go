package coach

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/db"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/llm"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/state"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/trace"
)

const (
	cmdStart         = "/start"
	cmdProfile       = "/profile"
	cmdUpdateProfile = "/update_profile"
	cmdWeight        = "/weight"
	cmdWeightHistory = "/weight_history"
	cmdForfait       = "/forfait"
	cmdPortion       = "/portion"
	cmdConnectPolar  = "/connect_polar"
	cmdUndo          = "/undo"
	cmdRecipe        = "/recipe"
	cmdSport         = "/sport"
	cmdWhatIf        = "/whatif"
	cmdHelp          = "/help"
)

// Legacy French command kept as alias so existing Telegram habits still work.
const cmdUpdateProfileFR = "/update_profil"

type Service struct {
	store        *db.Store
	llm          *llm.Client
	logger       *trace.Logger
	polarAuthURL func(userID int64) (string, error)
}

type Input struct {
	UserID    int64
	Username  string
	Text      string
	ImagePath string
	VoiceText string
}

type Response struct {
	Text    string
	Replies []string
}

func New(store *db.Store, llmClient *llm.Client, logger *trace.Logger) *Service {
	return &Service{store: store, llm: llmClient, logger: logger}
}

func (s *Service) SetPolarAuthURL(fn func(userID int64) (string, error)) {
	s.polarAuthURL = fn
}

func (s *Service) Handle(ctx context.Context, in Input) (Response, error) {
	user, err := s.store.GetOrCreateLocalUser(ctx, in.Username)
	if err != nil {
		return Response{}, err
	}
	return s.dispatch(ctx, user, in)
}

func (s *Service) HandleTelegram(ctx context.Context, telegramID, chatID int64, username, text, imagePath, voiceText string) (Response, error) {
	user, err := s.store.GetOrCreateUser(ctx, telegramID, chatID, username)
	if err != nil {
		return Response{}, err
	}
	return s.dispatch(ctx, user, Input{
		UserID: user.ID, Username: username, Text: text,
		ImagePath: imagePath, VoiceText: voiceText,
	})
}

func (s *Service) dispatch(ctx context.Context, user *db.User, in Input) (Response, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" && in.VoiceText != "" {
		text = strings.TrimSpace(in.VoiceText)
	}
	return s.routeCommand(ctx, user, text, in)
}

func (s *Service) routeCommand(ctx context.Context, user *db.User, text string, in Input) (Response, error) {
	if resp, ok, err := s.routeNamedCommand(ctx, user, text); ok {
		return resp, err
	}
	if in.ImagePath != "" {
		return s.handleMealPhoto(ctx, user, in.ImagePath)
	}
	// Slash commands that did not match must not be treated as meal descriptions.
	if strings.HasPrefix(text, "/") {
		return Response{Text: "Unknown command. Send /help to see available commands."}, nil
	}
	if text != "" {
		return s.handleText(ctx, user, text, in.VoiceText != "")
	}
	return Response{Text: "Send a command or describe your meal."}, nil
}

func (s *Service) routeNamedCommand(ctx context.Context, user *db.User, text string) (Response, bool, error) {
	cmd, args, isCmd := splitCommand(text)
	if !isCmd {
		return Response{}, false, nil
	}
	if resp, ok, err := s.routeProfileCommands(ctx, user, cmd); ok {
		return resp, true, err
	}
	if resp, ok, err := s.routeMealCommands(ctx, user, cmd, args); ok {
		return resp, true, err
	}
	return Response{}, false, nil
}

func (s *Service) routeProfileCommands(ctx context.Context, user *db.User, cmd string) (Response, bool, error) {
	switch cmd {
	case cmdStart:
		resp, err := s.handleStart(ctx, user)
		return resp, true, err
	case cmdProfile:
		resp, err := s.handleProfile(ctx, user)
		return resp, true, err
	case cmdUpdateProfile, cmdUpdateProfileFR:
		resp, err := s.handleUpdateProfile(ctx, user)
		return resp, true, err
	case cmdWeightHistory:
		resp, err := s.handleWeightHistory(ctx, user)
		return resp, true, err
	case cmdWeight:
		resp, err := s.handleWeightPrompt(ctx, user)
		return resp, true, err
	case cmdHelp:
		resp, err := s.handleHelp(ctx, user)
		return resp, true, err
	default:
		return Response{}, false, nil
	}
}

func (s *Service) routeMealCommands(ctx context.Context, user *db.User, cmd, args string) (Response, bool, error) {
	switch cmd {
	case cmdForfait:
		resp, err := s.handleForfaitMenu(ctx, user)
		return resp, true, err
	case cmdPortion:
		resp, err := s.handlePortion(ctx, user, args)
		return resp, true, err
	case cmdWhatIf:
		resp, err := s.handleWhatIf(ctx, user, args)
		return resp, true, err
	case cmdConnectPolar:
		resp, err := s.handleConnectPolar(ctx, user)
		return resp, true, err
	case cmdUndo:
		resp, err := s.handleUndo(ctx, user)
		return resp, true, err
	case cmdRecipe:
		resp, err := s.handleRecette(ctx, user, args)
		return resp, true, err
	case cmdSport:
		resp, err := s.handleSportPrompt(ctx, user)
		return resp, true, err
	default:
		return Response{}, false, nil
	}
}

// splitCommand extracts "/cmd" and args from Telegram text, stripping "@BotName" suffixes.
func splitCommand(text string) (cmd, args string, ok bool) {
	text = strings.TrimSpace(text)
	if text == "" || text[0] != '/' {
		return "", "", false
	}
	parts := strings.SplitN(text, " ", 2)
	cmd = strings.ToLower(parts[0])
	if at := strings.IndexByte(cmd, '@'); at >= 0 {
		cmd = cmd[:at]
	}
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return cmd, args, true
}

func (s *Service) handleStart(ctx context.Context, user *db.User) (Response, error) {
	_, err := s.store.GetProfile(ctx, user.ID)
	if err == nil {
		return Response{Text: "Welcome back! Use /profile to view your targets or log a meal by describing it.\nSend /update_profile to re-run onboarding."}, nil
	}
	if err := s.store.UpdateUserState(ctx, user.ID, state.OnboardingLanguage, state.Data{}); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, user.State, state.OnboardingLanguage, cmdStart)
	return Response{Text: "Welcome! Let's set up your profile.\n\nPreferred language? (en / fr)"}, nil
}

func (s *Service) handleUpdateProfile(ctx context.Context, user *db.User) (Response, error) {
	if err := s.store.UpdateUserState(ctx, user.ID, state.OnboardingLanguage, state.Data{}); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, user.State, state.OnboardingLanguage, cmdUpdateProfile)
	return Response{Text: "Let's update your profile.\n\nPreferred language? (en / fr)"}, nil
}

func (s *Service) handleSportPrompt(ctx context.Context, user *db.User) (Response, error) {
	if _, err := s.store.GetProfile(ctx, user.ID); err != nil {
		return Response{Text: "Complete /start onboarding first."}, nil
	}
	if err := s.store.UpdateUserState(ctx, user.ID, state.AwaitingSport, user.StateData); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, user.State, state.AwaitingSport, cmdSport)
	return Response{Text: "How many active calories did you burn? (e.g. 350)"}, nil
}

func (s *Service) handleProfile(ctx context.Context, user *db.User) (Response, error) {
	p, err := s.store.GetProfile(ctx, user.ID)
	if err != nil {
		return Response{Text: "No profile yet. Send /start to begin onboarding."}, nil
	}
	targets, err := s.store.EffectiveTargets(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}
	msg := fmt.Sprintf(
		"📊 Your Profile\n\nAge: %d | Height: %.0f cm | Weight: %.1f kg\nTarget weight: %.1f kg\nActivity: %s\nLanguage: %s\nRegion: %s\nExclusions: %s\n\n"+
			"BMR: %.0f kcal | TDEE: %.0f kcal\n\n🎯 Daily Targets:\nCalories: %.0f kcal\nProtein: %.0f g | Fat: %.0f g | Carbs: %.0f g",
		p.Age, p.HeightCm, p.WeightKg, p.TargetWeightKg, p.ActivityLevel,
		normalizeLanguage(user.Language), displayOrNone(p.Region), displayOrNone(p.ExcludedIngredients),
		targets.BMR, targets.TDEE,
		targets.TargetCalories, targets.TargetProteinG, targets.TargetFatG, targets.TargetCarbsG,
	)
	return Response{Text: msg}, nil
}

func (s *Service) handleWeightPrompt(ctx context.Context, user *db.User) (Response, error) {
	if err := s.store.UpdateUserState(ctx, user.ID, state.AwaitingWeight, user.StateData); err != nil {
		return Response{}, err
	}
	return Response{Text: "Enter your current weight in kg (e.g. 78.5):"}, nil
}

func (s *Service) handleWeightHistory(ctx context.Context, user *db.User) (Response, error) {
	entries, err := s.store.ListWeightEntries(ctx, user.ID, 100)
	if err != nil {
		return Response{}, err
	}
	if len(entries) == 0 {
		return Response{Text: "No weight entries yet. Use /weight to log your weight."}, nil
	}

	weights := make([]float64, len(entries))
	for i, e := range entries {
		weights[i] = e.WeightKg
	}
	stats := domain.WeightStatsFromEntries(weights)

	var sb strings.Builder
	fmt.Fprintf(&sb, "⚖️ Weight History (%d entries)\n\n", len(entries))
	fmt.Fprintf(&sb, "Current: %.1f kg\n", stats.CurrentKg)
	fmt.Fprintf(&sb, "7-day avg: %.2f kg\n", stats.MovingAvg7DayKg)
	fmt.Fprintf(&sb, "Delta from start: %+.1f kg\n\n", stats.DeltaFromStart)

	show := entries
	if len(show) > 10 {
		show = show[len(show)-10:]
	}
	for _, e := range show {
		fmt.Fprintf(&sb, "  • %.1f kg — %s\n", e.WeightKg, e.RecordedAt.Format("2006-01-02"))
	}
	return Response{Text: sb.String()}, nil
}

func (s *Service) handleForfaitMenu(ctx context.Context, user *db.User) (Response, error) {
	if err := s.store.UpdateUserState(ctx, user.ID, state.AwaitingForfait, user.StateData); err != nil {
		return Response{}, err
	}
	var sb strings.Builder
	sb.WriteString("🍕 Social Meal Fallback (/forfait)\nPick a preset by sending its key:\n\n")
	for _, p := range domain.ForfaitPresets {
		fmt.Fprintf(&sb, "  %s — %s (~%.0f kcal)\n", p.Key, p.Label, p.Calories)
	}
	sb.WriteString("\nExcess will be smoothed over 3 days (-100 kcal/day).")
	return Response{Text: sb.String()}, nil
}

func (s *Service) handleConnectPolar(ctx context.Context, user *db.User) (Response, error) {
	if s.polarAuthURL == nil {
		return Response{Text: "Polar integration is not configured on this server."}, nil
	}
	authURL, err := s.polarAuthURL(user.ID)
	if err != nil {
		return Response{}, err
	}
	return Response{Text: fmt.Sprintf(
		"⌚ Connect your Polar account:\n%s\n\nAfter authorizing, daily active calories sync at 23:00 and adjust your TDEE.",
		authURL,
	)}, nil
}

func (s *Service) handleText(ctx context.Context, user *db.User, text string, isVoice bool) (Response, error) {
	if resp, ok, err := s.handleOnboardingText(ctx, user, text); ok {
		return resp, err
	}
	switch user.State {
	case state.AwaitingWeight:
		return s.logWeight(ctx, user, text)
	case state.AwaitingForfait:
		return s.applyForfait(ctx, user, text)
	case state.AwaitingRecipeChoice:
		return s.handleRecipeChoice(ctx, user, text)
	case state.AwaitingSport:
		return s.handleSportCalories(ctx, user, text)
	default:
		source := "text"
		if isVoice {
			source = "voice"
		}
		return s.logMealFromText(ctx, user, text, source)
	}
}

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
	if lang != "en" && lang != "fr" {
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
	if err != nil || age < 10 || age > 120 {
		return Response{Text: "Please enter a valid age (10-120)."}, nil
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
	if err != nil || h < 100 || h > 250 {
		return Response{Text: "Please enter a valid height (100-250 cm)."}, nil
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
	if err != nil || w < 30 || w > 300 {
		return Response{Text: "Please enter a valid weight (30-300 kg)."}, nil
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
	if g != "male" && g != "female" {
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

func (s *Service) handleHelp(_ context.Context, _ *db.User) (Response, error) {
	msg := "📖 Nutrition Coach — Commands\n\n" +
		"👤 Profile\n" +
		"  /start — Begin onboarding\n" +
		"  /update_profile — Re-run onboarding\n" +
		"  /profile — View daily targets\n\n" +
		"⚖️ Weight\n" +
		"  /weight — Log current weight\n" +
		"  /weight_history — Weight history & trends\n\n" +
		"🍽 Meals\n" +
		"  Send text, photo, or voice — Log a meal\n" +
		"  /undo — Delete last logged meal\n" +
		"  /whatif food — Simulate meal impact (not saved)\n" +
		"  /forfait — Social meal fallback presets\n\n" +
		"👨‍🍳 Coaching\n" +
		"  /portion — Smart portion solver\n" +
		"  /recipe — AI recipe generator (2-step)\n" +
		"  /sport — Log manual activity calories\n\n" +
		"⌚ Integrations\n" +
		"  /connect_polar — Link Polar for calorie sync\n\n" +
		"  /help — Show this list"
	return Response{Text: msg}, nil
}

func (s *Service) onboardingTarget(ctx context.Context, user *db.User, text string) (Response, error) {
	tw, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || tw < 30 || tw > 300 {
		return Response{Text: "Please enter a valid target weight (30-300 kg)."}, nil
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

	tw, _ := strconv.ParseFloat(user.StateData.Get("target_weight_kg"), 64)
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
	height, _ := strconv.ParseFloat(data.Get("height_cm"), 64)
	weight, _ := strconv.ParseFloat(data.Get("weight_kg"), 64)
	return domain.ProfileInput{
		Age: age, HeightCm: height, WeightKg: weight, TargetWeightKg: targetWeight,
		Gender:              domain.Gender(data.Get("gender")),
		ActivityLevel:       domain.ActivityLevel(data.Get("activity_level")),
		WeightGoal:          domain.DeduceWeightGoal(weight, targetWeight),
		ExcludedIngredients: data.Get("excluded_ingredients"),
		Region:              region,
	}, nil
}

func displayOrNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func (s *Service) logWeight(ctx context.Context, user *db.User, text string) (Response, error) {
	w, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || w < 30 || w > 300 {
		return Response{Text: "Please enter a valid weight (30-300 kg)."}, nil
	}

	p, err := s.store.GetProfile(ctx, user.ID)
	if err != nil {
		return Response{Text: "Complete /start onboarding first."}, nil
	}

	if err := s.store.AddWeightEntry(ctx, user.ID, w); err != nil {
		return Response{}, err
	}

	recalcMsg := ""
	if domain.ShouldRecalculateTargets(w, p.WeightBaselineKg) {
		if err := s.store.RecalculateProfileTargets(ctx, user.ID, w); err != nil {
			return Response{}, err
		}
		recalcMsg = "\n\n🔄 Targets recalculated (weight changed ≥1 kg from baseline)."
	}

	if err := s.store.UpdateUserState(ctx, user.ID, state.Idle, state.Data{}); err != nil {
		return Response{}, err
	}

	entries, _ := s.store.ListWeightEntries(ctx, user.ID, 100)
	weights := make([]float64, len(entries))
	for i, e := range entries {
		weights[i] = e.WeightKg
	}
	stats := domain.WeightStatsFromEntries(weights)

	plateauMsg := s.checkPlateau(ctx, user.ID, weights)

	msg := fmt.Sprintf(
		"⚖️ Weight logged: %.1f kg\n7-day avg: %.2f kg | Delta: %+.1f kg%s%s",
		w, stats.MovingAvg7DayKg, stats.DeltaFromStart, recalcMsg, plateauMsg,
	)
	return Response{Text: msg}, nil
}

func (s *Service) checkPlateau(ctx context.Context, userID int64, weights []float64) string {
	if len(weights) < domain.PlateauWindowDays {
		return ""
	}

	movingAvgs := buildMovingAvgSeries(weights)
	progress, _ := s.store.DailyProgressNow(ctx, userID)
	targets, _ := s.store.EffectiveTargets(ctx, userID)
	loggedDeficit := progress.Calories < targets.TargetCalories

	status := domain.DetectPlateau(movingAvgs, len(weights), loggedDeficit)
	if !status.Detected {
		return ""
	}

	until := time.Now().AddDate(0, 0, status.DietBreakDays)
	_ = s.store.SetDietBreak(ctx, userID, until)

	return fmt.Sprintf(
		"\n\n⚠️ Metabolic Adaptation Alert\nYour 7-day average has been stable (<0.2 kg variance) for 14 days.\n"+
			"Suggested: 7-day diet break at maintenance (TDEE) until %s.",
		until.Format("2006-01-02"),
	)
}

func buildMovingAvgSeries(weights []float64) []float64 {
	avgs := make([]float64, 0, len(weights))
	for i := range weights {
		avgs = append(avgs, domain.WeightMovingAverage(weights[:i+1]))
	}
	return avgs
}

func (s *Service) applyForfait(ctx context.Context, user *db.User, text string) (Response, error) {
	key := strings.ToLower(strings.TrimSpace(text))
	preset, ok := domain.FindForfaitPreset(key)
	if !ok {
		return Response{Text: "Unknown preset. Send /forfait to see options."}, nil
	}

	if err := s.store.AddForfait(ctx, user.ID, preset); err != nil {
		return Response{}, err
	}
	if err := s.store.UpdateUserState(ctx, user.ID, state.Idle, state.Data{}); err != nil {
		return Response{}, err
	}

	return Response{Text: fmt.Sprintf(
		"🍕 Logged: %s (~%.0f kcal)\nTargets reduced by %d kcal/day for the next %d days.",
		preset.Label, preset.Calories, domain.ForfaitDailyOffset, domain.ForfaitSmoothDays,
	)}, nil
}

func (s *Service) logMealFromText(ctx context.Context, user *db.User, text, source string) (Response, error) {
	if _, err := s.store.GetProfile(ctx, user.ID); err != nil {
		return Response{Text: "Complete /start onboarding first."}, nil
	}
	return s.analyzeAndLogMeal(ctx, user.ID, text, source, user.Language)
}

func (s *Service) handleMealPhoto(ctx context.Context, user *db.User, imagePath string) (Response, error) {
	if _, err := s.store.GetProfile(ctx, user.ID); err != nil {
		return Response{Text: "Complete /start onboarding first."}, nil
	}

	systemPrompt := withLanguage(mealVisionSystemPrompt(), user.Language)
	userPrompt := "Analyze this meal photo. Estimate calories and macros."
	uid := user.ID
	raw, err := s.llm.AnalyzeMealPhoto(ctx, &uid, imagePath, systemPrompt, userPrompt, llm.MealEstimateSchema)
	if err != nil {
		return responseFromLLMError(err, "vision analysis")
	}

	var estimate domain.MealEstimate
	if err := json.Unmarshal([]byte(raw), &estimate); err != nil {
		return Response{}, fmt.Errorf("parse meal estimate: %w", err)
	}
	if err := s.store.AddMeal(ctx, user.ID, estimate, "photo"); err != nil {
		return Response{}, err
	}
	return s.formatMealResponse(ctx, user.ID, estimate)
}

func (s *Service) analyzeAndLogMeal(ctx context.Context, userID int64, description, source, lang string) (Response, error) {
	systemPrompt := withLanguage(mealTextSystemPrompt(), lang)
	userPrompt := fmt.Sprintf("Estimate this meal: %s", description)

	raw, err := s.llm.CompleteJSON(ctx, &userID, "meal_text", llm.ModelReason, systemPrompt, userPrompt, llm.MealEstimateSchema)
	if err != nil {
		return responseFromLLMError(err, "meal analysis")
	}

	var estimate domain.MealEstimate
	if err := json.Unmarshal([]byte(raw), &estimate); err != nil {
		return Response{}, fmt.Errorf("parse meal estimate: %w", err)
	}
	if estimate.Description == "" {
		estimate.Description = description
	}
	if err := s.store.AddMeal(ctx, userID, estimate, source); err != nil {
		return Response{}, err
	}
	return s.formatMealResponse(ctx, userID, estimate)
}

func (s *Service) formatMealResponse(ctx context.Context, userID int64, meal domain.MealEstimate) (Response, error) {
	targets, err := s.store.EffectiveTargets(ctx, userID)
	if err != nil {
		return Response{}, err
	}
	progress, err := s.store.DailyProgressNow(ctx, userID)
	if err != nil {
		return Response{}, err
	}
	remaining := domain.RemainingMacros(targets, progress)

	msg := fmt.Sprintf(
		"🍽 %s\n\nEstimated:\n  %.0f kcal | P: %.0f g | F: %.0f g | C: %.0f g\n  Confidence: %s\n",
		meal.Description, meal.Calories, meal.ProteinG, meal.FatG, meal.CarbsG, meal.Confidence,
	)
	if meal.Notes != "" {
		msg += fmt.Sprintf("  Notes: %s\n", meal.Notes)
	}
	msg += fmt.Sprintf(
		"\n📊 Today's Progress:\n  %.0f / %.0f kcal (%.0f%%)\n  P: %.0f/%.0f g | F: %.0f/%.0f g | C: %.0f/%.0f g",
		progress.Calories, targets.TargetCalories, pct(progress.Calories, targets.TargetCalories),
		progress.ProteinG, targets.TargetProteinG,
		progress.FatG, targets.TargetFatG,
		progress.CarbsG, targets.TargetCarbsG,
	)
	msg += fmt.Sprintf(
		"\n\n🎯 Remaining for next meal:\n  %.0f kcal | P: %.0f g | F: %.0f g | C: %.0f g",
		remaining.TargetCalories, remaining.TargetProteinG, remaining.TargetFatG, remaining.TargetCarbsG,
	)
	return Response{Text: msg}, nil
}

func (s *Service) handlePortion(ctx context.Context, user *db.User, query string) (Response, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Response{Text: "Usage: /portion I have chicken and rice, give me exact raw weights for my remaining macros."}, nil
	}

	targets, err := s.store.EffectiveTargets(ctx, user.ID)
	if err != nil {
		return Response{Text: "Complete /start onboarding first."}, nil
	}
	progress, err := s.store.DailyProgressNow(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}
	remaining := domain.RemainingMacros(targets, progress)

	loc := domain.LoadLocationOrUTC(user.Timezone)
	localNow := time.Now().In(loc)

	systemPrompt := withLanguage(portionSolverSystemPrompt(), user.Language)
	userPrompt := fmt.Sprintf(
		"Current local time: %s (hour=%d).\nRemaining daily macros: %.0f kcal, %.0f g protein, %.0f g fat, %.0f g carbs.\nUser query: %s",
		localNow.Format("15:04"), localNow.Hour(),
		remaining.TargetCalories, remaining.TargetProteinG, remaining.TargetFatG, remaining.TargetCarbsG, query,
	)

	uid := user.ID
	raw, err := s.llm.CompleteJSON(ctx, &uid, "portion_solver", llm.ModelReason, systemPrompt, userPrompt, llm.PortionSolverSchema)
	if err != nil {
		return responseFromLLMError(err, "portion solver")
	}

	var solution domain.PortionSolution
	if err := json.Unmarshal([]byte(raw), &solution); err != nil {
		return Response{}, fmt.Errorf("parse portion solution: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("👨‍🍳 Smart Portion Solver\n\n")
	for _, ing := range solution.Ingredients {
		fmt.Fprintf(&sb, "  • %s: %.0f g raw (%.0f kcal, P:%.0f F:%.0f C:%.0f)\n",
			ing.Name, ing.RawGrams, ing.Calories, ing.ProteinG, ing.FatG, ing.CarbsG)
	}
	fmt.Fprintf(&sb,
		"\nTotal: %.0f kcal | P: %.0f g | F: %.0f g | C: %.0f g\n\n%s",
		solution.TotalCalories, solution.TotalProteinG, solution.TotalFatG, solution.TotalCarbsG,
		solution.Explanation,
	)
	return Response{Text: sb.String()}, nil
}

func (s *Service) handleWhatIf(ctx context.Context, user *db.User, description string) (Response, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return Response{Text: "Usage: /whatif burger and fries"}, nil
	}
	if _, err := s.store.GetProfile(ctx, user.ID); err != nil {
		return Response{Text: "Complete /start onboarding first."}, nil
	}

	targets, err := s.store.EffectiveTargets(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}
	progress, err := s.store.DailyProgressNow(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}
	remaining := domain.RemainingMacros(targets, progress)

	systemPrompt := withLanguage(mealTextSystemPrompt(), user.Language)
	userPrompt := fmt.Sprintf("Estimate this meal (simulation only, do not assume it was eaten): %s", description)
	uid := user.ID
	raw, err := s.llm.CompleteJSON(ctx, &uid, "whatif_meal", llm.ModelReason, systemPrompt, userPrompt, llm.MealEstimateSchema)
	if err != nil {
		return responseFromLLMError(err, "whatif analysis")
	}

	var estimate domain.MealEstimate
	if err := json.Unmarshal([]byte(raw), &estimate); err != nil {
		return Response{}, fmt.Errorf("parse whatif estimate: %w", err)
	}
	if estimate.Description == "" {
		estimate.Description = description
	}

	projected := remaining.TargetCalories - estimate.Calories
	msg := fmt.Sprintf(
		"🔮 Simulation: %s (~%.0f kcal, P: %.0fg, F: %.0fg, C: %.0fg)\n\n"+
			"Impact: If you eat this, your remaining budget will drop from %.0f to %.0f kcal.",
		estimate.Description, estimate.Calories, estimate.ProteinG, estimate.FatG, estimate.CarbsG,
		remaining.TargetCalories, projected,
	)
	return Response{Text: msg}, nil
}

func (s *Service) handleSportCalories(ctx context.Context, user *db.User, text string) (Response, error) {
	cal, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || cal <= 0 || cal > 5000 {
		return Response{Text: "Please enter a valid calorie amount (1-5000)."}, nil
	}

	log, err := s.store.AddManualActivityCalories(ctx, user.ID, cal)
	if err != nil {
		return Response{}, err
	}
	if err := s.store.UpdateUserState(ctx, user.ID, state.Idle, state.Data{}); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.AwaitingSport, state.Idle, "sport_input")

	targets, err := s.store.EffectiveTargets(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}
	return Response{Text: fmt.Sprintf(
		"🏃 Logged +%.0f active kcal today (total activity: %.0f).\nAdjusted TDEE: %.0f kcal\nEffective daily target: %.0f kcal",
		cal, log.ActiveCalories, log.AdjustedTDEE, targets.TargetCalories,
	)}, nil
}

func (s *Service) DailyRecap(ctx context.Context, userID int64) (string, error) {
	targets, err := s.store.EffectiveTargets(ctx, userID)
	if err != nil {
		return "", err
	}
	progress, err := s.store.DailyProgressNow(ctx, userID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"📋 Daily Recap\n\nCalories: %.0f / %.0f kcal\nProtein: %.0f / %.0f g\nFat: %.0f / %.0f g\nCarbs: %.0f / %.0f g",
		progress.Calories, targets.TargetCalories,
		progress.ProteinG, targets.TargetProteinG,
		progress.FatG, targets.TargetFatG,
		progress.CarbsG, targets.TargetCarbsG,
	), nil
}

func (s *Service) WeeklyReport(ctx context.Context, userID int64) (string, error) {
	totalCal, avgCal, days, err := s.store.WeeklyMealSummary(ctx, userID)
	if err != nil {
		return "", err
	}

	entries, _ := s.store.ListWeightEntries(ctx, userID, 100)
	weights := make([]float64, len(entries))
	for i, e := range entries {
		weights[i] = e.WeightKg
	}
	stats := domain.WeightStatsFromEntries(weights)

	lang := "en"
	if u, err := s.store.GetUserByID(ctx, userID); err == nil {
		lang = u.Language
	}

	systemPrompt := withLanguage(
		"You are an expert nutrition coach. Provide a concise, actionable weekly summary.",
		lang,
	)
	userPrompt := fmt.Sprintf(
		"Weekly data: %d logging days, %.0f total kcal (avg %.0f/day). Weight delta: %+.1f kg, 7-day avg: %.2f kg.",
		days, totalCal, avgCal, stats.DeltaFromStart, stats.MovingAvg7DayKg,
	)

	raw, err := s.llm.CompleteText(ctx, &userID, "weekly_report", llm.ModelReason, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Sprintf(
			"📈 Weekly Report\n\nDays logged: %d\nAvg calories: %.0f kcal/day\nWeight delta: %+.1f kg",
			days, avgCal, stats.DeltaFromStart,
		), nil
	}
	return "📈 Weekly Report\n\n" + raw, nil
}

func (s *Service) InactivityReminder(mealType string) string {
	switch mealType {
	case "lunch":
		return "🍽 Lunch reminder: haven't logged a meal yet today. Send a description or photo!"
	case "dinner":
		return "🍽 Dinner reminder: don't forget to log your evening meal!"
	default:
		return "🍽 Reminder: log your meals to stay on track!"
	}
}

func (s *Service) logTransition(ctx context.Context, userID int64, from, to state.Name, trigger string) {
	if s.logger != nil {
		s.logger.StateTransition(ctx, userID, string(from), string(to), trigger)
	}
}

func pct(current, target float64) float64 {
	if target <= 0 {
		return 0
	}
	return current / target * 100
}

func mealVisionSystemPrompt() string {
	return `You are a nutrition analysis expert. Analyze meal photos and estimate calories and macros.
Use any visible scale markers (cutlery, plates, hand/thumb) to improve volume and gram estimation when present.
These markers are optional — do your best with whatever visual cues are available.
Return precise numeric estimates. Be conservative when uncertain.`
}

func mealTextSystemPrompt() string {
	return `You are a nutrition analysis expert. Estimate calories and macros from meal descriptions.
Use standard portion sizes when amounts aren't specified. Return precise numeric estimates.`
}

func portionSolverSystemPrompt() string {
	return `You are a precision nutrition sous-chef. Given remaining daily macro targets, current local time, and available ingredients,
calculate exact raw gram weights to hit an appropriate share of those targets.
Evaluate the current time:
- If it is before 15:00 (lunchtime), you MUST NOT use 100% of the remaining macros. Calculate portions to hit approximately 40% to 50% of the remaining daily macros, and explicitly state that you are leaving the rest for dinner.
- If it is 15:00 or later (evening), you may use up to 100% of the remaining macros.
Solve the macro equation systematically. Output specific gram measurements for each ingredient.
All weights are raw/uncooked unless stated otherwise.`
}
