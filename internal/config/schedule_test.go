package config

import (
	"testing"
	"time"
)

func TestParseReportSchedule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		dailyTime  string
		weeklyDay  string
		weeklyTime string
		want       ReportSchedule
		wantErr    bool
	}{
		{
			name: "defaults when empty",
			want: DefaultReportSchedule(),
		},
		{
			name:       "custom french-friendly values",
			dailyTime:  "20:00",
			weeklyDay:  "Sunday",
			weeklyTime: "09:00",
			want: ReportSchedule{
				DailyHour: 20, DailyMinute: 0,
				WeeklyDay: time.Sunday, WeeklyHour: 9, WeeklyMinute: 0,
			},
		},
		{
			name:       "weekday as int",
			dailyTime:  "21:30",
			weeklyDay:  "0",
			weeklyTime: "20:00",
			want: ReportSchedule{
				DailyHour: 21, DailyMinute: 30,
				WeeklyDay: time.Sunday, WeeklyHour: 20, WeeklyMinute: 0,
			},
		},
		{
			name:       "french weekday name",
			dailyTime:  "20:15",
			weeklyDay:  "dimanche",
			weeklyTime: "09:30",
			want: ReportSchedule{
				DailyHour: 20, DailyMinute: 15,
				WeeklyDay: time.Sunday, WeeklyHour: 9, WeeklyMinute: 30,
			},
		},
		{
			name:      "invalid daily time",
			dailyTime: "25:00",
			wantErr:   true,
		},
		{
			name:      "invalid weekday",
			weeklyDay: "funday",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseReportSchedule(tt.dailyTime, tt.weeklyDay, tt.weeklyTime)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
