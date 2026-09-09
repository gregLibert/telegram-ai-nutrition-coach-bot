package coach

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/db"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/llm"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/retry"
)

const (
	forceCacheSuffix      = "/force"
	llmRateLimitedMessage = "L'API est temporairement saturée. Je retente automatiquement dans quelques instants... ⏳"
	mealRetryMaxAttempts  = 5
	mealRetryInitialWait  = 2 * time.Second
	mealRetryMaxWait      = 60 * time.Second
	cacheOpMealText       = "meal_text"
)

type mealJob struct {
	ChatID      int64
	UserID      int64
	Description string
	Source      string
	Lang        string
	ForceCache  bool
}

func stripForceFlag(text string) (cleaned string, force bool) {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, forceCacheSuffix) {
		cleaned = strings.TrimSpace(trimmed[:len(trimmed)-len(forceCacheSuffix)])
		return cleaned, true
	}
	return trimmed, false
}

func (s *Service) logMealFromText(ctx context.Context, user *db.User, text, source string, chatID int64) (Response, error) {
	if _, err := s.store.GetProfile(ctx, user.ID); err != nil {
		return Response{Text: "Complete /start onboarding first."}, nil
	}
	description, force := stripForceFlag(text)
	if description == "" {
		return Response{Text: "Please describe the meal to log."}, nil
	}
	return s.analyzeAndLogMeal(ctx, mealJob{
		ChatID: chatID, UserID: user.ID, Description: description,
		Source: source, Lang: user.Language, ForceCache: force,
	})
}

func (s *Service) analyzeAndLogMeal(ctx context.Context, job mealJob) (Response, error) {
	estimate, err := s.resolveMealEstimate(ctx, job)
	if err != nil {
		if llm.IsRateLimited(err) {
			s.enqueueMealRetry(job)
			return Response{Text: llmRateLimitedMessage}, nil
		}
		return responseFromLLMError(err, "meal analysis")
	}
	if estimate.Description == "" {
		estimate.Description = job.Description
	}
	if err := s.store.AddMeal(ctx, job.UserID, estimate, job.Source); err != nil {
		return Response{}, err
	}
	return s.formatMealResponse(ctx, job.UserID, estimate)
}

func (s *Service) resolveMealEstimate(ctx context.Context, job mealJob) (domain.MealEstimate, error) {
	hashKey := db.HashPromptKey(cacheOpMealText, job.Lang, job.Description)

	if !job.ForceCache {
		if cached, ok, err := s.store.GetPromptCache(ctx, hashKey); err != nil {
			s.logDomain(ctx, "cache_error", map[string]any{"error": err.Error(), "hash": hashKey})
		} else if ok {
			s.logDomain(ctx, "cache_hit", map[string]any{"hash": hashKey})
			var estimate domain.MealEstimate
			if err := json.Unmarshal([]byte(cached), &estimate); err == nil {
				return estimate, nil
			}
		}
	}

	if s.nutrition != nil {
		if estimate, err := s.nutrition.EstimateMeal(ctx, job.Description); err == nil {
			s.cacheMealEstimate(ctx, hashKey, estimate)
			return estimate, nil
		}
	}

	return s.estimateMealWithLLM(ctx, job, hashKey)
}

func (s *Service) estimateMealWithLLM(ctx context.Context, job mealJob, hashKey string) (domain.MealEstimate, error) {
	systemPrompt := withLanguage(mealTextSystemPrompt(), job.Lang)
	userPrompt := fmt.Sprintf("Estimate this meal: %s", job.Description)
	uid := job.UserID
	raw, err := s.llm.CompleteJSON(ctx, &uid, cacheOpMealText, llm.ModelReason, systemPrompt, userPrompt, llm.MealEstimateSchema)
	if err != nil {
		return domain.MealEstimate{}, err
	}

	var estimate domain.MealEstimate
	if err := json.Unmarshal([]byte(raw), &estimate); err != nil {
		return domain.MealEstimate{}, fmt.Errorf("parse meal estimate: %w", err)
	}
	s.cacheMealEstimate(ctx, hashKey, estimate)
	return estimate, nil
}

func (s *Service) cacheMealEstimate(ctx context.Context, hashKey string, estimate domain.MealEstimate) {
	raw, err := json.Marshal(estimate)
	if err != nil {
		return
	}
	if err := s.store.UpsertPromptCache(ctx, hashKey, string(raw)); err != nil {
		s.logDomain(ctx, "cache_upsert_error", map[string]any{"error": err.Error(), "hash": hashKey})
	}
}

func (s *Service) enqueueMealRetry(job mealJob) {
	go s.retryMealJob(job)
}

func (s *Service) retryMealJob(job mealJob) {
	ctx := context.Background()
	cfg := retry.Config{
		Initial:     mealRetryInitialWait,
		MaxBackoff:  mealRetryMaxWait,
		MaxAttempts: mealRetryMaxAttempts,
	}
	err := retry.Do(ctx, cfg, func(attempt int) error {
		s.logDomain(ctx, "meal_retry_attempt", map[string]any{
			"attempt": attempt, "user_id": job.UserID,
		})
		estimate, err := s.resolveMealEstimate(ctx, mealJob{
			UserID: job.UserID, Description: job.Description,
			Source: job.Source, Lang: job.Lang, ForceCache: true,
		})
		if err != nil {
			return err
		}
		if estimate.Description == "" {
			estimate.Description = job.Description
		}
		if err := s.store.AddMeal(ctx, job.UserID, estimate, job.Source); err != nil {
			return err
		}
		resp, err := s.formatMealResponse(ctx, job.UserID, estimate)
		if err != nil {
			return err
		}
		if s.notify != nil && job.ChatID != 0 {
			s.notify(job.ChatID, resp.Text)
			for _, r := range resp.Replies {
				s.notify(job.ChatID, r)
			}
		}
		return nil
	})
	if err != nil && s.notify != nil && job.ChatID != 0 {
		s.notify(job.ChatID, "Désolé, l'API est toujours saturée. Réessaie un peu plus tard.")
	}
}

func (s *Service) logDomain(ctx context.Context, event string, fields map[string]any) {
	if s.logger == nil {
		return
	}
	switch event {
	case "cache_hit":
		args := []any{"hash", fields["hash"]}
		s.logger.InfoContext(ctx, "cache_hit", args...)
	default:
		s.logger.DomainEvent(ctx, event, fields)
	}
}
