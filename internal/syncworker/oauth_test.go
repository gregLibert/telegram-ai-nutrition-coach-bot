package syncworker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     OAuthConfig
		userID  int64
		wantErr bool
		contain string
	}{
		{
			name:    "missing client id",
			cfg:     OAuthConfig{RedirectURI: "http://localhost/callback"},
			userID:  42,
			wantErr: true,
		},
		{
			name:    "missing redirect uri",
			cfg:     OAuthConfig{ClientID: "cid"},
			userID:  42,
			wantErr: true,
		},
		{
			name: "valid config embeds state",
			cfg: OAuthConfig{
				ClientID:    "cid",
				RedirectURI: "http://localhost/callback",
			},
			userID:  99,
			contain: "state=99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := AuthURL(tt.cfg, tt.userID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(got, tt.contain) {
				t.Fatalf("url %q missing %q", got, tt.contain)
			}
			if !strings.Contains(got, polarAuthURL) {
				t.Fatalf("url %q missing auth host", got)
			}
		})
	}
}

func TestParseOAuthTokenResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		body       string
		wantErr    bool
		wantAccess string
		wantExpiry bool
	}{
		{
			name:       "success with expiry",
			status:     http.StatusOK,
			body:       `{"access_token":"at","refresh_token":"rt","expires_in":3600,"token_type":"bearer"}`,
			wantAccess: "at",
			wantExpiry: true,
		},
		{
			name:       "success without expiry",
			status:     http.StatusOK,
			body:       `{"access_token":"at2","refresh_token":"rt2","token_type":"bearer"}`,
			wantAccess: "at2",
		},
		{
			name:    "http error status",
			status:  http.StatusUnauthorized,
			body:    `{"error":"invalid_grant"}`,
			wantErr: true,
		},
		{
			name:    "invalid json body",
			status:  http.StatusOK,
			body:    `{not-json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token, err := parseOAuthTokenResponse(tt.status, []byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token.AccessToken != tt.wantAccess {
				t.Fatalf("access = %q, want %q", token.AccessToken, tt.wantAccess)
			}
			if tt.wantExpiry && token.Expiry.Before(time.Now()) {
				t.Fatal("expected future expiry")
			}
			if !tt.wantExpiry && !token.Expiry.IsZero() {
				t.Fatalf("expected zero expiry, got %v", token.Expiry)
			}
		})
	}
}

func TestTokenExchangerRefresh(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		body       string
		wantErr    bool
		wantAccess string
	}{
		{
			name:       "success",
			status:     http.StatusOK,
			body:       `{"access_token":"live-at","refresh_token":"live-rt","expires_in":120,"token_type":"bearer"}`,
			wantAccess: "live-at",
		},
		{
			name:    "provider rejects refresh",
			status:  http.StatusBadRequest,
			body:    `{"error":"invalid_request"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s", r.Method)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			ex := &TokenExchanger{
				cfg:      OAuthConfig{ClientID: "cid", ClientSecret: "secret"},
				client:   srv.Client(),
				tokenURL: srv.URL,
			}
			token, err := ex.Refresh(context.Background(), "refresh-token")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token.AccessToken != tt.wantAccess {
				t.Fatalf("access = %q, want %q", token.AccessToken, tt.wantAccess)
			}
		})
	}
}

func TestRegisterPolarUser(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "created", status: http.StatusCreated},
		{name: "ok", status: http.StatusOK},
		{name: "already registered conflict", status: http.StatusConflict},
		{name: "server error", status: http.StatusInternalServerError, body: "boom", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				if tt.body != "" {
					_, _ = w.Write([]byte(tt.body))
				}
			}))
			t.Cleanup(srv.Close)

			prev := polarUsersRegisterURL
			polarUsersRegisterURL = srv.URL
			t.Cleanup(func() { polarUsersRegisterURL = prev })

			err := RegisterPolarUser(context.Background(), "token", "member", srv.Client())
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
