# Telegram AI Nutrition & Weight Coach Bot

[![CI](https://github.com/gregLibert/telegram-ai-nutrition-coach-bot/actions/workflows/ci.yml/badge.svg)](https://github.com/gregLibert/telegram-ai-nutrition-coach-bot/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/gregLibert/telegram-ai-nutrition-coach-bot/graph/badge.svg)](https://codecov.io/gh/gregLibert/telegram-ai-nutrition-coach-bot)

Production-ready AI nutrition coach with a decoupled core engine, Telegram interface, and local CLI for testing.

## Architecture

```
cmd/bot/          → Telegram bot entrypoint (production)
cmd/cli/          → Local CLI for TDD / agent testing
internal/
  domain/         → BMR/TDEE/macro calculations, plateau detection
  db/             → SQLite repository (modernc.org/sqlite, CGO-free)
  llm/            → OpenRouter client with structured JSON outputs
  coach/          → Core orchestration (decoupled from Telegram)
  state/          → Onboarding state machine
  telegram/       → Telegram Bot API adapter
  scheduler/      → Proactive reminders & reports
  syncworker/     → Sports watch API sync (Polar / Google Fit)
  trace/          → Structured JSON logging (log/slog)
  app/            → Wiring & graceful shutdown
migrations/       → Reference schema SQL
```

## Quick Start

### Prerequisites

- Go 1.26+
- [golangci-lint](https://golangci-lint.run/welcome/install/) (for local linting)
- OpenRouter API key
- Telegram Bot Token (for production bot only)

### Code Quality

```bash
# Lint (must pass before pushing)
golangci-lint run

# Tests with coverage
go test -v -coverprofile=coverage.out ./...
```

### Environment Variables

| Variable | Required | Description |
|---|---|---|
| `ALLOWED_USERS` | Bot mode (production) | Comma-separated Telegram user IDs allowed to use the bot. Empty = deny all. |
| `OPENROUTER_API_KEY` | Yes (for LLM) | OpenRouter API key |
| `OPENAI_API_KEY` | Bot mode (voice) | OpenAI API key for Whisper STT |
| `TELEGRAM_BOT_TOKEN` | Bot mode only | Telegram bot token |
| `DB_PATH` | No | SQLite path (default: `data/coach.db`) |
| `TIMEZONE` | No | Scheduler timezone (default: `Europe/Paris`) |
| `HTTP_ADDR` | No | OAuth callback server (default: `:8080`) |
| `POLAR_CLIENT_ID` | Polar sync | Polar Accesslink client ID |
| `POLAR_CLIENT_SECRET` | Polar sync | Polar Accesslink client secret |
| `POLAR_REDIRECT_URI` | Polar sync | OAuth callback URL (e.g. `http://host:8080/auth/polar/callback`) |

### Local CLI Testing (no Telegram token needed)

```bash
# Run tests
go test ./...

# Interactive CLI
go run ./cmd/cli

# Single command
go run ./cmd/cli -- /start
go run ./cmd/cli -- /profile
go run ./cmd/cli -- "200g chicken breast with rice"

# Photo analysis
go run ./cmd/cli --image meal.jpg -- "analyze this meal"
```

### Run Telegram Bot

```bash
export TELEGRAM_BOT_TOKEN=your_token
export OPENROUTER_API_KEY=your_key
export OPENAI_API_KEY=your_key
go run ./cmd/bot
```

## Deployment

CI pushes a pre-built **linux/arm64** image to GitHub Container Registry on every push to `main`.

### Orange Pi / ARM64 (pull from GHCR)

On the target host:

```bash
# Authenticate once (read packages permission on GHCR)
echo "$GITHUB_TOKEN" | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin

mkdir -p ~/nutrition-coach/data
cd ~/nutrition-coach

# Copy env template and fill secrets
curl -O https://raw.githubusercontent.com/gregLibert/telegram-ai-nutrition-coach-bot/main/.env.example
cp .env.example .env
# edit .env with your API keys

# Pull compose file and start
curl -O https://raw.githubusercontent.com/gregLibert/telegram-ai-nutrition-coach-bot/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

Image reference: `ghcr.io/greglibert/telegram-ai-nutrition-coach-bot:latest`

The SQLite database is persisted at `./data/coach.db` via a bind mount.

### Local Docker Build (Linux / ARM64)

```bash
cp .env.example .env   # fill in secrets
docker compose up -d --build
```

## Commands

| Command | Description |
|---|---|
| `/start` | Onboarding flow (includes language preference) |
| `/profile` | View current targets |
| `/weight` | Log weight |
| `/weight_history` | Weight history with 7-day avg |
| `/portion <query>` | Smart portion solver (time-aware macro split) |
| `/recipe [ingredients]` | Two-phase recipe generator |
| `/whatif <food>` | Simulate meal macros without logging |
| `/sport` | Log manual activity calories |
| `/undo` | Delete last logged meal |
| `/forfait` | Social meal fallback presets |
| `/connect_polar` | Link Polar account for auto calorie sync |
| `/help` | List all commands |

## LLM Routing

| Use Case | Model |
|---|---|
| Meal photo analysis | `qwen/qwen-2.5-vl-72b-instruct` |
| Text meals, portions, recipes, what-if | `deepseek/deepseek-chat` |
| Weekly summary & coaching | `deepseek/deepseek-chat` |

All LLM calls use structured JSON schema outputs. Full audit trail (prompts, responses, tokens, latency) is logged via `slog` JSON and persisted to `llm_audit_log`. Replies follow the user's `language` preference (`en` / `fr`).

## Features

- **Teen-aware BMR**: Schofield equation for age &lt; 18; Mifflin-St Jeor for adults
- **Macro targets**: 1.6 g/kg protein, 31% fat, remainder carbs
- **Weight tracking** with 7-day moving average and auto-recalculation at ≥1 kg change
- **Multimodal meal logging** (text, photo, voice via OpenAI Whisper)
- **Smart reminders**: lunch (14:00) and dinner (21:00) only if no meal was logged in the window
- **Forfait fallback**: predefined heavy meals with 3-day caloric smoothing
- **Smart portion solver**: at lunch uses ~40–50% of remaining macros; evening may use 100%
- **What-if simulation**: estimate meal impact without saving
- **Plateau detection**: 14-day variance check with diet break suggestion
- **Polar sync**: OAuth2 connect flow, daily active calorie fetch at 23:00, dynamic TDEE via `daily_logs`

## Build

```bash
go build -o bin/coach-bot ./cmd/bot
go build -o bin/coach-cli ./cmd/cli
```

## CI/CD

On every push/PR to `main`:

1. **Test & Lint** — `golangci-lint run`, unit tests with coverage
2. **Docker** (push to `main` only) — builds and pushes `ghcr.io/greglibert/telegram-ai-nutrition-coach-bot:latest` for `linux/arm64`

Repository: [github.com/gregLibert/telegram-ai-nutrition-coach-bot](https://github.com/gregLibert/telegram-ai-nutrition-coach-bot)
