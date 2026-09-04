package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/app"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/coach"
)

func main() {
	username := flag.String("user", "local_dev", "Local username for testing")
	dbPath := flag.String("db", "data/cli_coach.db", "SQLite database path")
	imagePath := flag.String("image", "", "Optional image path for meal photo analysis")
	flag.Parse()

	cfg := app.DefaultConfig()
	cfg.DBPath = *dbPath
	cfg.LogLevel = slog.LevelDebug

	application, err := app.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init error: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = application.Close() }()

	ctx := context.Background()

	args := flag.Args()
	if len(args) > 0 {
		runSingle(ctx, application.Coach(), *username, strings.Join(args, " "), *imagePath)
		return
	}

	runInteractive(ctx, application.Coach(), *username, *imagePath)
}

func runSingle(ctx context.Context, svc *coach.Service, username, text, imagePath string) {
	resp, err := svc.Handle(ctx, coach.Input{
		Username: username, Text: text, ImagePath: imagePath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp.Text)
	for _, r := range resp.Replies {
		fmt.Println(r)
	}
}

func runInteractive(ctx context.Context, svc *coach.Service, username, defaultImage string) {
	fmt.Println("Nutrition Coach CLI — type messages or commands (/start, /profile, /weight, /portion, /recipe, /whatif, /sport)")
	fmt.Println("Use --image <path> flag or prefix with @image:<path> for photo analysis")
	fmt.Println("Type 'exit' to quit.")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}

		imgPath := defaultImage
		text := line
		if strings.HasPrefix(line, "@image:") {
			parts := strings.SplitN(line, " ", 2)
			imgPath = strings.TrimPrefix(parts[0], "@image:")
			if len(parts) > 1 {
				text = parts[1]
			} else {
				text = ""
			}
		}

		resp, err := svc.Handle(ctx, coach.Input{
			Username: username, Text: text, ImagePath: imgPath,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
		fmt.Println(resp.Text)
		for _, r := range resp.Replies {
			fmt.Println(r)
		}
		fmt.Println()
	}
}
