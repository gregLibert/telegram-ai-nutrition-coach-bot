package coach

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/db"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/llm"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/state"
)

const stateKeyRecipeOptions = "recipe_options"

func (s *Service) handleUndo(ctx context.Context, user *db.User) (Response, error) {
	deleted, err := s.store.DeleteLastMeal(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}
	if !deleted {
		return Response{Text: "No meals to undo."}, nil
	}

	targets, err := s.store.EffectiveTargets(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}
	progress, err := s.store.DailyProgressNow(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}

	return Response{Text: fmt.Sprintf(
		"✅ Last meal deleted. Updated today's progress: %.0f / %.0f kcal",
		progress.Calories, targets.TargetCalories,
	)}, nil
}

func (s *Service) handleRecette(ctx context.Context, user *db.User, ingredients string) (Response, error) {
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

	profile, err := s.store.GetProfile(ctx, user.ID)
	if err != nil {
		return Response{}, err
	}

	systemPrompt := recipeIdeasSystemPrompt()
	userPrompt := buildRecipeIdeasPrompt(remaining, profile, ingredients)

	uid := user.ID
	raw, err := s.llm.CompleteJSON(ctx, &uid, "recipe_ideas", llm.ModelReason, systemPrompt, userPrompt, llm.RecipeOptionsSchema)
	if err != nil {
		return Response{}, fmt.Errorf("recipe ideas: %w", err)
	}

	var result domain.RecipeOptionsResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return Response{}, fmt.Errorf("parse recipe options: %w", err)
	}
	if len(result.Options) == 0 {
		return Response{Text: "Could not generate recipe ideas. Try again with /recette."}, nil
	}

	optionsJSON, err := json.Marshal(result.Options)
	if err != nil {
		return Response{}, err
	}

	data := user.StateData.Set(stateKeyRecipeOptions, string(optionsJSON))
	if err := s.store.UpdateUserState(ctx, user.ID, state.AwaitingRecipeChoice, data); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, user.State, state.AwaitingRecipeChoice, cmdRecette)

	return Response{Text: formatRecipeOptions(result.Options)}, nil
}

func (s *Service) handleRecipeChoice(ctx context.Context, user *db.User, text string) (Response, error) {
	choice, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || choice < 1 || choice > 3 {
		return Response{Text: "Please reply with 1, 2, or 3 to select a recipe."}, nil
	}

	options, err := parseStoredRecipeOptions(user.StateData.Get(stateKeyRecipeOptions))
	if err != nil {
		return Response{Text: "Recipe session expired. Send /recette to start again."}, nil
	}

	selected, ok := findRecipeOption(options, choice)
	if !ok {
		return Response{Text: "Invalid choice. Reply with 1, 2, or 3."}, nil
	}

	systemPrompt := recipeDetailSystemPrompt()
	userPrompt := fmt.Sprintf(
		"Write the recipe for: %s\nMacros: %s\nKey ingredients: %s",
		selected.Title, selected.MacrosSummary, strings.Join(selected.KeyIngredients, ", "),
	)

	uid := user.ID
	raw, err := s.llm.CompleteText(ctx, &uid, "recipe_detail", llm.ModelReason, systemPrompt, userPrompt)
	if err != nil {
		return Response{}, fmt.Errorf("recipe detail: %w", err)
	}

	if err := s.store.UpdateUserState(ctx, user.ID, state.Idle, state.Data{}); err != nil {
		return Response{}, err
	}
	s.logTransition(ctx, user.ID, state.AwaitingRecipeChoice, state.Idle, "recipe_selected")

	return Response{Text: fmt.Sprintf("👨‍🍳 %s\n\n%s", selected.Title, raw)}, nil
}

func parseStoredRecipeOptions(raw string) ([]domain.RecipeOption, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty options")
	}
	var options []domain.RecipeOption
	if err := json.Unmarshal([]byte(raw), &options); err != nil {
		return nil, err
	}
	return options, nil
}

func findRecipeOption(options []domain.RecipeOption, id int) (domain.RecipeOption, bool) {
	for _, o := range options {
		if o.ID == id {
			return o, true
		}
	}
	return domain.RecipeOption{}, false
}

func formatRecipeOptions(options []domain.RecipeOption) string {
	var sb strings.Builder
	sb.WriteString("🍳 Recipe Ideas (pick one):\n\n")
	for _, o := range options {
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   Ingredients: %s\n\n",
			o.ID, o.Title, o.MacrosSummary, strings.Join(o.KeyIngredients, ", "))
	}
	sb.WriteString("Reply with 1, 2, or 3 to get the full recipe.")
	return sb.String()
}

func buildRecipeIdeasPrompt(remaining domain.MacroTargets, profile *db.Profile, ingredients string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb,
		"Remaining macros: %.0f kcal, %.0f g protein, %.0f g fat, %.0f g carbs.\n",
		remaining.TargetCalories, remaining.TargetProteinG, remaining.TargetFatG, remaining.TargetCarbsG,
	)
	fmt.Fprintf(&sb, "Region: %s\n", profile.Region)
	if profile.ExcludedIngredients != "" {
		fmt.Fprintf(&sb, "Dietary exclusions: %s\n", profile.ExcludedIngredients)
	}
	if strings.TrimSpace(ingredients) != "" {
		fmt.Fprintf(&sb, "Available ingredients: %s\n", ingredients)
	} else {
		sb.WriteString("No specific fridge ingredients provided.\n")
	}
	sb.WriteString("Propose exactly 3 recipe ideas with ids 1, 2, and 3.")
	return sb.String()
}

func recipeIdeasSystemPrompt() string {
	return `You are a local culinary assistant. Propose 3 brief recipe ideas that strictly fit the remaining macros.
Prioritize ingredients standard to the user's region. Strictly exclude dietary restrictions and disliked foods.
Use the provided available ingredients when given. Each option must have id (1, 2, or 3), title, macros_summary, and key_ingredients.`
}

func recipeDetailSystemPrompt() string {
	return `You are a chef. Write the full, step-by-step recipe for the selected concept.
Format with clear bullet points. Include exact raw gram weights for each ingredient.
Keep instructions practical and concise.`
}
