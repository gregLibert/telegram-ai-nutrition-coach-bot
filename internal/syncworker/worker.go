package syncworker

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/db"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/trace"
)

type Worker struct {
	store      *db.Store
	logger     *trace.Logger
	client     *http.Client
	oauthCfg   OAuthConfig
	exchanger  *TokenExchanger
	location   *time.Location
	syncHour   int
	syncMinute int
}

func New(store *db.Store, logger *trace.Logger, oauthCfg OAuthConfig, tz string) *Worker {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return &Worker{
		store:      store,
		logger:     logger,
		client:     &http.Client{Timeout: 30 * time.Second},
		oauthCfg:   oauthCfg,
		exchanger:  NewTokenExchanger(oauthCfg),
		location:   loc,
		syncHour:   23,
		syncMinute: 0,
	}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			w.tick(ctx, now.In(w.location))
		}
	}
}

func (w *Worker) tick(ctx context.Context, now time.Time) {
	if now.Hour() == w.syncHour && now.Minute() == w.syncMinute {
		w.syncAll(ctx, now)
	}
}

func (w *Worker) syncAll(ctx context.Context, now time.Time) {
	integrations, err := w.store.ListPolarIntegrations(ctx)
	if err != nil {
		w.logEvent(ctx, "polar_sync_list_error", 0, 0, err)
		return
	}

	date := now.Format("2006-01-02")
	for _, integration := range integrations {
		w.syncUser(ctx, integration, date)
	}
}

func (w *Worker) syncUser(ctx context.Context, integration db.UserIntegration, date string) {
	accessToken, err := EnsureTokenValid(ctx, w.store, w.exchanger, &integration)
	if err != nil {
		w.logEvent(ctx, "polar_token_refresh", integration.UserID, 0, err)
		return
	}
	if accessToken == "" {
		return
	}

	if accessToken != integration.AccessToken {
		w.logEvent(ctx, "polar_token_refreshed", integration.UserID, 0, nil)
	}

	calories, err := FetchPolarCalories(ctx, accessToken, date, w.client)
	if err != nil {
		w.logEvent(ctx, "polar_fetch_error", integration.UserID, 0, err)
		return
	}

	profile, err := w.store.GetProfile(ctx, integration.UserID)
	if err != nil {
		w.logEvent(ctx, "polar_profile_error", integration.UserID, calories, err)
		return
	}

	adjustedTDEE := profile.TDEE + calories
	if err := w.store.UpsertDailyLog(ctx, db.DailyLog{
		UserID:         integration.UserID,
		LogDate:        date,
		BaseTDEE:       profile.TDEE,
		ActiveCalories: calories,
		AdjustedTDEE:   adjustedTDEE,
	}); err != nil {
		w.logEvent(ctx, "polar_daily_log_error", integration.UserID, calories, err)
		return
	}

	if err := w.store.UpsertActivitySync(ctx, integration.UserID, "polar", calories); err != nil {
		w.logEvent(ctx, "polar_activity_sync_error", integration.UserID, calories, err)
	}

	w.logEvent(ctx, "polar_sync_success", integration.UserID, calories, nil)
}

func (w *Worker) logEvent(ctx context.Context, event string, userID int64, calories float64, err error) {
	if w.logger == nil {
		return
	}
	fields := map[string]any{"user_id": userID, "calories": calories}
	if err != nil {
		fields["error"] = err.Error()
	}
	w.logger.DomainEvent(ctx, event, fields)
}

// SyncNow runs an immediate sync for testing or manual triggers.
func (w *Worker) SyncNow(ctx context.Context) {
	w.syncAll(ctx, time.Now().In(w.location))
}

// AuthHandler returns a configured OAuth callback handler.
func (w *Worker) AuthHandler() *AuthHandler {
	return NewAuthHandler(w.store, w.oauthCfg, w.logger)
}

// AuthURLForUser builds the Polar connect URL for a Telegram user.
func AuthURLForUser(cfg OAuthConfig, userID int64) (string, error) {
	return AuthURL(cfg, userID)
}

// FormatConnectMessage returns a user-facing Polar connect instruction.
func FormatConnectMessage(authURL string) string {
	return fmt.Sprintf("⌚ Connect your Polar account:\n%s\n\nAfter authorizing, your daily active calories will sync at 23:00.", authURL)
}
