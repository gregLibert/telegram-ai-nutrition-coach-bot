package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/coach"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/config"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/db"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/llm"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/scheduler"
	syncworker "github.com/greg/telegram-ai-nutrition-coach-bot/internal/syncworker"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/telegram"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/trace"
)

type Config struct {
	DBPath           string
	LogLevel         slog.Level
	Timezone         string
	HTTPAddr         string
	TelegramToken    string
	OpenRouterAPIKey string
	OpenAIAPIKey     string
	PolarClientID    string
	PolarSecret      string
	PolarRedirectURI string
	AllowedUsers     config.AllowList
}

func DefaultConfig() Config {
	allowed, err := config.ParseAllowedUsers(os.Getenv("ALLOWED_USERS"))
	if err != nil {
		slog.Error("invalid ALLOWED_USERS; denying all Telegram users", "error", err)
		allowed = config.AllowList{}
	}
	return Config{
		DBPath:           envOr("DB_PATH", "data/coach.db"),
		Timezone:         envOr("TIMEZONE", "Europe/Paris"),
		HTTPAddr:         envOr("HTTP_ADDR", ":8080"),
		TelegramToken:    os.Getenv("TELEGRAM_BOT_TOKEN"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		PolarClientID:    os.Getenv("POLAR_CLIENT_ID"),
		PolarSecret:      os.Getenv("POLAR_CLIENT_SECRET"),
		PolarRedirectURI: os.Getenv("POLAR_REDIRECT_URI"),
		AllowedUsers:     allowed,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type App struct {
	store  *db.Store
	coach  *coach.Service
	logger *trace.Logger
	config Config
}

func New(cfg Config) (*App, error) {
	if err := os.MkdirAll(dirOf(cfg.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	logger := trace.New(os.Stdout, cfg.LogLevel)
	if len(cfg.AllowedUsers) == 0 {
		logger.Warn("ALLOWED_USERS is empty; all Telegram access will be denied")
	} else {
		logger.Info("telegram allowlist loaded", "count", len(cfg.AllowedUsers))
	}

	auditFn := func(ctx context.Context, entry llm.AuditEntry) error {
		var uid *int64
		if entry.UserID != nil {
			uid = entry.UserID
		}
		if err := store.SaveLLMAudit(ctx, uid, entry.Operation, entry.Model,
			entry.Prompt, entry.RawResponse, entry.TokensPrompt, entry.TokensOutput, entry.LatencyMs); err != nil {
			logger.DomainEvent(ctx, "audit_save_error", map[string]any{"error": err.Error()})
		}
		logger.LLMCall(ctx, map[string]any{
			"operation": entry.Operation, "model": entry.Model,
			"tokens_prompt": entry.TokensPrompt, "tokens_output": entry.TokensOutput,
			"latency_ms": entry.LatencyMs,
		})
		return nil
	}

	llmClient := llm.NewClient(cfg.OpenRouterAPIKey, auditFn)
	coachSvc := coach.New(store, llmClient, logger)

	oauthCfg := syncworker.OAuthConfig{
		ClientID:     cfg.PolarClientID,
		ClientSecret: cfg.PolarSecret,
		RedirectURI:  cfg.PolarRedirectURI,
	}
	coachSvc.SetPolarAuthURL(func(userID int64) (string, error) {
		return syncworker.AuthURL(oauthCfg, userID)
	})

	return &App{store: store, coach: coachSvc, logger: logger, config: cfg}, nil
}

func (a *App) Coach() *coach.Service { return a.coach }
func (a *App) Store() *db.Store      { return a.store }
func (a *App) Logger() *trace.Logger { return a.logger }

func (a *App) Close() error {
	return a.store.Close()
}

func (a *App) RunBot(ctx context.Context) error {
	whisper := llm.NewWhisperClient(a.config.OpenAIAPIKey)

	bot, err := telegram.New(a.config.TelegramToken, a.coach, a.logger, whisper, a.config.AllowedUsers)
	if err != nil {
		return err
	}

	oauthCfg := syncworker.OAuthConfig{
		ClientID:     a.config.PolarClientID,
		ClientSecret: a.config.PolarSecret,
		RedirectURI:  a.config.PolarRedirectURI,
	}
	worker := syncworker.New(a.store, a.logger, oauthCfg, a.config.Timezone)
	sch := scheduler.New(a.store, a.coach, a.logger, bot.SendTo, a.config.Timezone)

	go sch.Run(ctx)
	go worker.Run(ctx)
	go a.runHTTPServer(ctx, worker.AuthHandler())

	return bot.Run(ctx)
}

func (a *App) runHTTPServer(ctx context.Context, authHandler *syncworker.AuthHandler) {
	mux := http.NewServeMux()
	authHandler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:              a.config.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		a.logger.DomainEvent(ctx, "http_server_start", map[string]any{"addr": a.config.HTTPAddr})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.DomainEvent(ctx, "http_server_error", map[string]any{"error": err.Error()})
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			a.logger.DomainEvent(shutdownCtx, "http_server_shutdown_error", map[string]any{"error": err.Error()})
		}
	}()
}

func (a *App) RunWithGracefulShutdown(runFn func(ctx context.Context) error) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	err := runFn(ctx)
	closeErr := a.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
