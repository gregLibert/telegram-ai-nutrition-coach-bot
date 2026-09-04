package db

import (
	"context"
	"time"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
)

func (s *Store) userLocation(ctx context.Context, userID int64) *time.Location {
	u, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return time.UTC
	}
	return domain.LoadLocationOrUTC(u.Timezone)
}

func (s *Store) migrateColumns() error {
	alters := []string{
		`ALTER TABLE profiles ADD COLUMN excluded_ingredients TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE profiles ADD COLUMN region TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN language TEXT NOT NULL DEFAULT 'en'`,
	}
	for _, stmt := range alters {
		_, _ = s.db.Exec(stmt)
	}
	return nil
}
