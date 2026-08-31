package domain

import "time"

// DayBounds returns UTC [start, end) instants for the calendar day containing ref in loc.
func DayBounds(loc *time.Location, ref time.Time) (start, end time.Time) {
	local := ref.In(loc)
	y, m, d := local.Date()
	startLocal := time.Date(y, m, d, 0, 0, 0, 0, loc)
	return startLocal.UTC(), startLocal.Add(24 * time.Hour).UTC()
}

// WeekBounds returns UTC [start, end) for a 7-day window ending on ref's local day (inclusive).
func WeekBounds(loc *time.Location, ref time.Time) (start, end time.Time) {
	local := ref.In(loc)
	weekStartDay := local.AddDate(0, 0, -6)
	start, _ = DayBounds(loc, weekStartDay)
	_, end = DayBounds(loc, ref)
	return start, end
}

// FormatSQLiteUTC formats a time for SQLite logged_at comparisons (UTC, no TZ suffix).
func FormatSQLiteUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

// LoadLocationOrUTC loads a timezone name, falling back to UTC on error.
func LoadLocationOrUTC(tz string) *time.Location {
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}
