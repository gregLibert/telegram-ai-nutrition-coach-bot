package domain

import (
	"testing"
	"time"
)

func TestDayBounds(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		loc       *time.Location
		ref       time.Time
		wantStart string
		wantEnd   string
	}{
		{
			name:      "paris midday",
			loc:       paris,
			ref:       time.Date(2026, 8, 29, 12, 0, 0, 0, paris),
			wantStart: "2026-08-28T22:00:00Z", // CEST UTC+2 → midnight Paris = 22:00 UTC prev day
			wantEnd:   "2026-08-29T22:00:00Z",
		},
		{
			name:      "utc midnight",
			loc:       time.UTC,
			ref:       time.Date(2026, 1, 15, 0, 30, 0, 0, time.UTC),
			wantStart: "2026-01-15T00:00:00Z",
			wantEnd:   "2026-01-16T00:00:00Z",
		},
		{
			name:      "near midnight paris",
			loc:       paris,
			ref:       time.Date(2026, 8, 29, 23, 30, 0, 0, paris),
			wantStart: "2026-08-28T22:00:00Z",
			wantEnd:   "2026-08-29T22:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := DayBounds(tt.loc, tt.ref)
			if start.Format(time.RFC3339) != tt.wantStart {
				t.Errorf("start = %s, want %s", start.Format(time.RFC3339), tt.wantStart)
			}
			if end.Format(time.RFC3339) != tt.wantEnd {
				t.Errorf("end = %s, want %s", end.Format(time.RFC3339), tt.wantEnd)
			}
		})
	}
}

func TestWeekBounds(t *testing.T) {
	loc := time.UTC
	ref := time.Date(2026, 8, 29, 15, 0, 0, 0, loc)
	start, end := WeekBounds(loc, ref)

	if start.Format("2006-01-02") != "2026-08-23" {
		t.Errorf("week start = %s, want 2026-08-23", start.Format("2006-01-02"))
	}
	if end.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("week end = %s, want 2026-08-30", end.Format("2006-01-02"))
	}
}

func TestFormatSQLiteUTC(t *testing.T) {
	tm := time.Date(2026, 8, 29, 22, 15, 30, 0, time.UTC)
	got := FormatSQLiteUTC(tm)
	want := "2026-08-29 22:15:30"
	if got != want {
		t.Errorf("FormatSQLiteUTC() = %q, want %q", got, want)
	}
}

func TestLoadLocationOrUTC(t *testing.T) {
	if LoadLocationOrUTC("Invalid/Zone") != time.UTC {
		t.Error("expected UTC fallback")
	}
	if LoadLocationOrUTC("") != time.UTC {
		t.Error("expected UTC for empty")
	}
}
