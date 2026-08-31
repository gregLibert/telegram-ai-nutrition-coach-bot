package syncworker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/db"
)

const (
	polarAuthURL       = "https://flow.polar.com/oauth2/authorization"
	polarOAuthEndpoint = "https://polarremote.com/v2/oauth2/token" //nolint:gosec // public OAuth endpoint URL
	polarScope         = "accesslink.read_all"
)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

func PolarOAuthConfig(cfg OAuthConfig) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Scopes:       []string{polarScope},
		Endpoint: oauth2.Endpoint{
			AuthURL:  polarAuthURL,
			TokenURL: polarOAuthEndpoint,
		},
	}
}

func OAuthConfigFromEnv() OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("POLAR_CLIENT_ID"),
		ClientSecret: os.Getenv("POLAR_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("POLAR_REDIRECT_URI"),
	}
}

// AuthURL returns the Polar authorization URL for a given user.
func AuthURL(cfg OAuthConfig, userID int64) (string, error) {
	if cfg.ClientID == "" || cfg.RedirectURI == "" {
		return "", fmt.Errorf("POLAR_CLIENT_ID and POLAR_REDIRECT_URI must be set")
	}
	oauthCfg := PolarOAuthConfig(cfg)
	state := fmt.Sprintf("%d", userID)
	return oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

type TokenExchanger struct {
	cfg    OAuthConfig
	client *http.Client
}

func NewTokenExchanger(cfg OAuthConfig) *TokenExchanger {
	return &TokenExchanger{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *TokenExchanger) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	oauthCfg := PolarOAuthConfig(t.cfg)
	return oauthCfg.Exchange(ctx, code)
}

func (t *TokenExchanger) Refresh(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, polarOAuthEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(t.cfg.ClientID, t.cfg.ClientSecret)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("polar token refresh status %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	token := &oauth2.Token{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		TokenType:    parsed.TokenType,
	}
	if parsed.ExpiresIn > 0 {
		token.Expiry = time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second)
	}
	return token, nil
}

func RegisterPolarUser(ctx context.Context, accessToken, memberID string, client *http.Client) error {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	payload := fmt.Sprintf(`{"member-id":%q}`, memberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, polarBaseURL+"/users", strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req) //nolint:gosec // request targets fixed Polar Accesslink API
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("polar register status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func SaveToken(ctx context.Context, store *db.Store, userID int64, token *oauth2.Token) error {
	memberID := fmt.Sprintf("coach_user_%d", userID)
	var expiry sql.NullString
	if !token.Expiry.IsZero() {
		expiry = sql.NullString{String: token.Expiry.UTC().Format(time.RFC3339), Valid: true}
	}
	return store.SaveIntegration(ctx, db.UserIntegration{
		UserID:         userID,
		Provider:       "polar",
		AccessToken:    token.AccessToken,
		RefreshToken:   token.RefreshToken,
		TokenExpiry:    expiry,
		ExternalUserID: memberID,
	})
}
