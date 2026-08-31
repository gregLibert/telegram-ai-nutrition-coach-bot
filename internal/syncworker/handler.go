package syncworker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/db"
	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/trace"
)

type AuthHandler struct {
	store      *db.Store
	exchanger  *TokenExchanger
	logger     *trace.Logger
	httpClient *http.Client
}

func NewAuthHandler(store *db.Store, cfg OAuthConfig, logger *trace.Logger) *AuthHandler {
	return &AuthHandler{
		store:      store,
		exchanger:  NewTokenExchanger(cfg),
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/auth/polar/callback", h.handlePolarCallback)
}

func (h *AuthHandler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *AuthHandler) handlePolarCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		http.Error(w, "Polar authorization denied: "+errMsg, http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseInt(state, 10, 64)
	if err != nil || userID <= 0 {
		http.Error(w, "invalid state parameter", http.StatusBadRequest)
		return
	}

	token, err := h.exchanger.Exchange(ctx, code)
	if err != nil {
		h.logSync(ctx, "polar_oauth_exchange_failed", userID, 0, err)
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	memberID := fmt.Sprintf("coach_user_%d", userID)
	if err := RegisterPolarUser(ctx, token.AccessToken, memberID, h.httpClient); err != nil {
		h.logSync(ctx, "polar_register_failed", userID, 0, err)
	}

	if err := SaveToken(ctx, h.store, userID, token); err != nil {
		h.logSync(ctx, "polar_token_save_failed", userID, 0, err)
		http.Error(w, "failed to save token", http.StatusInternalServerError)
		return
	}

	h.logSync(ctx, "polar_oauth_success", userID, 0, nil)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, "<html><body><h2>Polar connected!</h2><p>You can close this window and return to Telegram.</p></body></html>")
}

func (h *AuthHandler) logSync(ctx context.Context, event string, userID int64, calories float64, err error) {
	if h.logger == nil {
		return
	}
	fields := map[string]any{"user_id": userID, "calories": calories}
	if err != nil {
		fields["error"] = err.Error()
	}
	h.logger.DomainEvent(ctx, event, fields)
}

// EnsureTokenValid refreshes the Polar token when expired.
func EnsureTokenValid(ctx context.Context, store *db.Store, exchanger *TokenExchanger, integration *db.UserIntegration) (string, error) {
	if integration.TokenExpiry.Valid {
		expiry, err := time.Parse(time.RFC3339, integration.TokenExpiry.String)
		if err == nil && time.Now().Before(expiry.Add(-time.Minute)) {
			return integration.AccessToken, nil
		}
	}

	if integration.RefreshToken == "" {
		return integration.AccessToken, nil
	}

	token, err := exchanger.Refresh(ctx, integration.RefreshToken)
	if err != nil {
		return "", err
	}

	if err := SaveToken(ctx, store, integration.UserID, token); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

// FetchPolarCalories retrieves active calories for a date using a valid access token.
func FetchPolarCalories(ctx context.Context, accessToken, date string, client *http.Client) (float64, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BuildActivityURL(date), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return 0, nil
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("polar API status %d: %s", resp.StatusCode, string(body))
	}

	return ParseActivityResponse(body)
}
