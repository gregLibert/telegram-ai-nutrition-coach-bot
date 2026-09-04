package config

import (
	"fmt"
	"strconv"
	"strings"
)

// AllowList is a set of authorized Telegram user IDs for O(1) membership checks.
type AllowList map[int64]struct{}

// ParseAllowedUsers parses a comma-separated list of Telegram IDs into an AllowList.
// Empty or whitespace-only input yields an empty list (deny-all when enforced).
func ParseAllowedUsers(raw string) (AllowList, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return AllowList{}, nil
	}

	parts := strings.Split(raw, ",")
	list := make(AllowList, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid telegram id %q: %w", part, err)
		}
		list[id] = struct{}{}
	}
	return list, nil
}

// IsAllowed reports whether telegramID is on the allowlist.
// An empty allowlist denies everyone (fail-closed).
func (a AllowList) IsAllowed(telegramID int64) bool {
	if len(a) == 0 {
		return false
	}
	_, ok := a[telegramID]
	return ok
}
