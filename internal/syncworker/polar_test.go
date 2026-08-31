package syncworker

import "testing"

func TestBuildActivityURL(t *testing.T) {
	tests := []struct {
		name string
		date string
		want string
	}{
		{
			name: "standard date",
			date: "2026-08-29",
			want: "https://www.polaraccesslink.com/v3/users/activities?date=2026-08-29",
		},
		{
			name: "year boundary",
			date: "2025-01-01",
			want: "https://www.polaraccesslink.com/v3/users/activities?date=2025-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildActivityURL(tt.date)
			if got != tt.want {
				t.Errorf("BuildActivityURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseActivityResponse(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    float64
		wantErr bool
	}{
		{
			name: "array of summaries",
			body: `[{"date":"2026-08-29","active-calories":450},{"date":"2026-08-29","active-calories":120}]`,
			want: 570,
		},
		{
			name: "single summary object",
			body: `{"date":"2026-08-29","active-calories":380}`,
			want: 380,
		},
		{
			name: "legacy snake_case field",
			body: `{"active_calories":275}`,
			want: 275,
		},
		{
			name:    "empty body",
			body:    ``,
			wantErr: true,
		},
		{
			name:    "invalid json",
			body:    `{not json}`,
			wantErr: true,
		},
		{
			name: "zero calories array",
			body: `[{"date":"2026-08-29","active-calories":0}]`,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseActivityResponse([]byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseActivityResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}
