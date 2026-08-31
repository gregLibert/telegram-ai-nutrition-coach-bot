package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/app"
)

func main() {
	cfg := app.DefaultConfig()
	cfg.LogLevel = slog.LevelInfo

	application, err := app.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init error: %v\n", err)
		os.Exit(1)
	}

	err = application.RunWithGracefulShutdown(application.RunBot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bot error: %v\n", err)
		os.Exit(1)
	}
}
