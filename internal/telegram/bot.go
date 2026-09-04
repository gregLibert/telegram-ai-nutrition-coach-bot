package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/coach"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/config"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/llm"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/trace"
)

type Bot struct {
	api          *tgbotapi.BotAPI
	coach        *coach.Service
	logger       *trace.Logger
	whisper      *llm.WhisperClient
	httpClient   *http.Client
	allowedUsers config.AllowList
}

func New(token string, coachSvc *coach.Service, logger *trace.Logger, whisper *llm.WhisperClient, allowedUsers config.AllowList) (*Bot, error) {
	if token == "" {
		token = os.Getenv("TELEGRAM_BOT_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}
	if allowedUsers == nil {
		allowedUsers = config.AllowList{}
	}
	return &Bot{
		api: api, coach: coachSvc, logger: logger, whisper: whisper,
		httpClient:   &http.Client{Timeout: 60 * time.Second},
		allowedUsers: allowedUsers,
	}, nil
}

func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			b.handleUpdate(ctx, update)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	chatID := msg.Chat.ID
	user := msg.From
	if user == nil {
		return
	}

	telegramID := int64(user.ID)
	if !b.allowedUsers.IsAllowed(telegramID) {
		if b.logger != nil {
			b.logger.Warn("Unauthorized access attempt", "telegram_id", telegramID)
		}
		b.send(chatID, "⛔ Unauthorized access.")
		return
	}

	text := msg.Text
	var imagePath, voiceText string

	switch {
	case len(msg.Photo) > 0:
		imagePath = b.downloadPhoto(ctx, msg.Photo)
	case msg.Voice != nil:
		voiceText = b.transcribeVoice(ctx, msg.Voice)
		if voiceText == "" && b.whisper != nil {
			b.send(chatID, "Could not transcribe your voice note. Please try again or type your meal.")
			return
		}
	}

	resp, err := b.coach.HandleTelegram(ctx, telegramID, chatID, user.UserName, text, imagePath, voiceText)
	if err != nil {
		b.send(chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	b.send(chatID, resp.Text)
	for _, r := range resp.Replies {
		b.send(chatID, r)
	}

	if imagePath != "" {
		if err := os.Remove(imagePath); err != nil {
			b.logError(ctx, "remove_photo", err)
		}
	}
}

func (b *Bot) send(chatID int64, text string) {
	if text == "" {
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)
	// Plain text: HTML mode broke /help because of "<food>" in the message body.
	if _, err := b.api.Send(msg); err != nil {
		b.logError(context.Background(), "telegram_send", err)
	}
}

func (b *Bot) SendTo(chatID int64, text string) {
	b.send(chatID, text)
}

func (b *Bot) downloadPhoto(ctx context.Context, photos []tgbotapi.PhotoSize) string {
	if len(photos) == 0 {
		return ""
	}
	photo := photos[len(photos)-1]
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: photo.FileID})
	if err != nil {
		b.logError(ctx, "download_photo", err)
		return ""
	}

	url := file.Link(b.api.Token)
	resp, err := b.fetchURL(ctx, url)
	if err != nil {
		b.logError(ctx, "fetch_photo", err)
		return ""
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			b.logError(ctx, "close_photo_body", closeErr)
		}
	}()

	dir := os.TempDir()
	path := filepath.Join(dir, fmt.Sprintf("meal_%s.jpg", photo.FileID))
	f, err := os.Create(path)
	if err != nil {
		b.logError(ctx, "create_temp", err)
		return ""
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		b.logError(ctx, "save_photo", err)
		if closeErr := f.Close(); closeErr != nil {
			b.logError(ctx, "close_photo_file", closeErr)
		}
		return ""
	}
	if err := f.Close(); err != nil {
		b.logError(ctx, "close_photo_file", err)
	}
	return path
}

func (b *Bot) transcribeVoice(ctx context.Context, voice *tgbotapi.Voice) string {
	if b.whisper == nil {
		b.logError(ctx, "voice_transcription", fmt.Errorf("whisper client not configured"))
		return ""
	}

	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: voice.FileID})
	if err != nil {
		b.logError(ctx, "download_voice", err)
		return ""
	}

	url := file.Link(b.api.Token)
	resp, err := b.fetchURL(ctx, url)
	if err != nil {
		b.logError(ctx, "fetch_voice", err)
		return ""
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			b.logError(ctx, "close_voice_body", closeErr)
		}
	}()

	dir := os.TempDir()
	path := filepath.Join(dir, fmt.Sprintf("voice_%s.ogg", voice.FileID))
	f, err := os.Create(path)
	if err != nil {
		b.logError(ctx, "create_voice_temp", err)
		return ""
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		b.logError(ctx, "save_voice", err)
		if closeErr := f.Close(); closeErr != nil {
			b.logError(ctx, "close_voice_file", closeErr)
		}
		if removeErr := os.Remove(path); removeErr != nil {
			b.logError(ctx, "remove_voice_temp", removeErr)
		}
		return ""
	}
	if err := f.Close(); err != nil {
		b.logError(ctx, "close_voice_file", err)
	}

	defer func() {
		if removeErr := os.Remove(path); removeErr != nil {
			b.logError(ctx, "remove_voice_temp", removeErr)
		}
	}()

	text, err := b.whisper.TranscribeFile(ctx, path)
	if err != nil {
		b.logError(ctx, "whisper_transcribe", err)
		return ""
	}
	return text
}

func (b *Bot) fetchURL(ctx context.Context, fileURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	return b.httpClient.Do(req)
}

func (b *Bot) logError(ctx context.Context, event string, err error) {
	if b.logger != nil {
		b.logger.DomainEvent(ctx, event, map[string]any{"error": err.Error()})
	}
}
