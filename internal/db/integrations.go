package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type UserIntegration struct {
	UserID         int64
	Provider       string
	AccessToken    string
	RefreshToken   string
	TokenExpiry    sql.NullString
	ExternalUserID string
}

type DailyLog struct {
	UserID         int64
	LogDate        string
	BaseTDEE       float64
	ActiveCalories float64
	AdjustedTDEE   float64
}

func (s *Store) SaveIntegration(ctx context.Context, integration UserIntegration) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_integrations (user_id, provider, access_token, refresh_token, token_expiry, external_user_id)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, provider) DO UPDATE SET
			access_token=excluded.access_token,
			refresh_token=excluded.refresh_token,
			token_expiry=excluded.token_expiry,
			external_user_id=excluded.external_user_id,
			updated_at=datetime('now')`,
		integration.UserID, integration.Provider,
		integration.AccessToken, integration.RefreshToken,
		nullString(integration.TokenExpiry), integration.ExternalUserID)
	if err != nil {
		return fmt.Errorf("save integration: %w", err)
	}
	return nil
}

func (s *Store) GetIntegration(ctx context.Context, userID int64, provider string) (*UserIntegration, error) {
	i := &UserIntegration{UserID: userID, Provider: provider}
	err := s.db.QueryRowContext(ctx, `
		SELECT access_token, refresh_token, token_expiry, external_user_id
		FROM user_integrations WHERE user_id = ? AND provider = ?`,
		userID, provider).
		Scan(&i.AccessToken, &i.RefreshToken, &i.TokenExpiry, &i.ExternalUserID)
	if err != nil {
		return nil, fmt.Errorf("get integration: %w", err)
	}
	return i, nil
}

func (s *Store) ListPolarIntegrations(ctx context.Context) ([]UserIntegration, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, access_token, refresh_token, token_expiry, external_user_id
		FROM user_integrations WHERE provider = 'polar' AND access_token != ''`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var integrations []UserIntegration
	for rows.Next() {
		var i UserIntegration
		i.Provider = "polar"
		if err := rows.Scan(&i.UserID, &i.AccessToken, &i.RefreshToken, &i.TokenExpiry, &i.ExternalUserID); err != nil {
			return nil, err
		}
		integrations = append(integrations, i)
	}
	return integrations, rows.Err()
}

func (s *Store) UpsertDailyLog(ctx context.Context, log DailyLog) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_logs (user_id, log_date, base_tdee, active_calories, adjusted_tdee)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, log_date) DO UPDATE SET
			base_tdee=excluded.base_tdee,
			active_calories=excluded.active_calories,
			adjusted_tdee=excluded.adjusted_tdee,
			updated_at=datetime('now')`,
		log.UserID, log.LogDate, log.BaseTDEE, log.ActiveCalories, log.AdjustedTDEE)
	if err != nil {
		return fmt.Errorf("upsert daily log: %w", err)
	}
	return nil
}

func (s *Store) GetDailyLogForUser(ctx context.Context, userID int64) (*DailyLog, error) {
	loc := s.userLocation(ctx, userID)
	day := time.Now().In(loc)
	return s.GetDailyLog(ctx, userID, day)
}

func (s *Store) GetDailyLog(ctx context.Context, userID int64, day time.Time) (*DailyLog, error) {
	dayStr := day.Format("2006-01-02")
	log := &DailyLog{UserID: userID, LogDate: dayStr}
	err := s.db.QueryRowContext(ctx, `
		SELECT base_tdee, active_calories, adjusted_tdee
		FROM daily_logs WHERE user_id = ? AND log_date = ?`, userID, dayStr).
		Scan(&log.BaseTDEE, &log.ActiveCalories, &log.AdjustedTDEE)
	if err != nil {
		return nil, err
	}
	return log, nil
}

func nullString(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}
