package telegram

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/retry"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/trace"
)

const (
	telegramConnectMaxAttempts = 8
	telegramConnectInitial     = 1 * time.Second
	telegramConnectMaxBackoff  = 30 * time.Second
	getUpdatesTimeoutSec       = 60
	pollErrorBackoff           = 3 * time.Second
)

func connectBotAPI(ctx context.Context, token string, logger *trace.Logger) (*tgbotapi.BotAPI, error) {
	var api *tgbotapi.BotAPI
	err := retry.Do(ctx, retry.Config{
		Initial:     telegramConnectInitial,
		MaxBackoff:  telegramConnectMaxBackoff,
		MaxAttempts: telegramConnectMaxAttempts,
	}, func(attempt int) error {
		created, createErr := tgbotapi.NewBotAPI(token)
		if createErr != nil {
			if logger != nil {
				logger.Warn("telegram getMe failed, retrying",
					"attempt", attempt,
					"error", createErr.Error(),
				)
			}
			return createErr
		}
		api = created
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create bot after retries: %w", err)
	}
	return api, nil
}

func (b *Bot) pollUpdates(ctx context.Context) error {
	offset := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		updates, err := b.api.GetUpdates(tgbotapi.UpdateConfig{
			Offset:  offset,
			Timeout: getUpdatesTimeoutSec,
		})
		if err != nil {
			b.logPollError(ctx, err)
			if waitErr := retry.Wait(ctx, pollErrorBackoff); waitErr != nil {
				return nil
			}
			continue
		}

		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			b.handleUpdate(ctx, update)
		}
	}
}

func (b *Bot) logPollError(ctx context.Context, err error) {
	if b.logger == nil {
		return
	}
	b.logger.WarnContext(ctx, "telegram poll error",
		"error", err.Error(),
	)
}
