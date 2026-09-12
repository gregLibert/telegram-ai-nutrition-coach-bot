package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ReportSchedule holds configurable daily/weekly report trigger times.
type ReportSchedule struct {
	DailyHour    int
	DailyMinute  int
	WeeklyDay    time.Weekday
	WeeklyHour   int
	WeeklyMinute int
}

// DefaultReportSchedule mirrors the historic hardcoded schedule.
func DefaultReportSchedule() ReportSchedule {
	return ReportSchedule{
		DailyHour:    21,
		DailyMinute:  30,
		WeeklyDay:    time.Sunday,
		WeeklyHour:   20,
		WeeklyMinute: 0,
	}
}

// ParseReportSchedule reads HH:MM times and a weekday name/number from env-style strings.
func ParseReportSchedule(dailyTime, weeklyDay, weeklyTime string) (ReportSchedule, error) {
	out := DefaultReportSchedule()

	if strings.TrimSpace(dailyTime) != "" {
		h, m, err := parseHHMM(dailyTime)
		if err != nil {
			return ReportSchedule{}, fmt.Errorf("DAILY_REPORT_TIME: %w", err)
		}
		out.DailyHour, out.DailyMinute = h, m
	}

	if strings.TrimSpace(weeklyTime) != "" {
		h, m, err := parseHHMM(weeklyTime)
		if err != nil {
			return ReportSchedule{}, fmt.Errorf("WEEKLY_REPORT_TIME: %w", err)
		}
		out.WeeklyHour, out.WeeklyMinute = h, m
	}

	if strings.TrimSpace(weeklyDay) != "" {
		day, err := parseWeekday(weeklyDay)
		if err != nil {
			return ReportSchedule{}, fmt.Errorf("WEEKLY_REPORT_DAY: %w", err)
		}
		out.WeeklyDay = day
	}

	return out, nil
}

func parseHHMM(raw string) (hour, minute int, err error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time %q (want HH:MM)", raw)
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour in %q", raw)
	}
	minute, err = strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid minute in %q", raw)
	}
	return hour, minute, nil
}

func parseWeekday(raw string) (time.Weekday, error) {
	raw = strings.TrimSpace(raw)
	if n, err := strconv.Atoi(raw); err == nil {
		if n < 0 || n > 6 {
			return 0, fmt.Errorf("weekday int must be 0-6 (Sunday=0), got %d", n)
		}
		return time.Weekday(n), nil
	}

	switch strings.ToLower(raw) {
	case "sunday", "sun", "dimanche", "dim":
		return time.Sunday, nil
	case "monday", "mon", "lundi", "lun":
		return time.Monday, nil
	case "tuesday", "tue", "mardi", "mar":
		return time.Tuesday, nil
	case "wednesday", "wed", "mercredi", "mer":
		return time.Wednesday, nil
	case "thursday", "thu", "jeudi", "jeu":
		return time.Thursday, nil
	case "friday", "fri", "vendredi", "ven":
		return time.Friday, nil
	case "saturday", "sat", "samedi", "sam":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("unknown weekday %q", raw)
	}
}
