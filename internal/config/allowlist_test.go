package config

import "testing"

func TestParseAllowedUsers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    []int64
		wantErr bool
	}{
		{
			name: "empty",
			raw:  "",
			want: nil,
		},
		{
			name: "whitespace only",
			raw:  "  , , ",
			want: nil,
		},
		{
			name: "single id",
			raw:  "12345678",
			want: []int64{12345678},
		},
		{
			name: "multiple ids with spaces",
			raw:  "12345678, 87654321, 111",
			want: []int64{12345678, 87654321, 111},
		},
		{
			name:    "invalid token",
			raw:     "123,abc",
			wantErr: true,
		},
		{
			name:    "float not allowed",
			raw:     "12.5",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseAllowedUsers(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for _, id := range tt.want {
				if !got.IsAllowed(id) {
					t.Errorf("expected id %d to be allowed", id)
				}
			}
		})
	}
}

func TestAllowListIsAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		allowed    string
		telegramID int64
		want       bool
	}{
		{
			name:       "empty list denies all",
			allowed:    "",
			telegramID: 12345678,
			want:       false,
		},
		{
			name:       "authorized user passes",
			allowed:    "12345678,87654321",
			telegramID: 12345678,
			want:       true,
		},
		{
			name:       "second authorized user passes",
			allowed:    "12345678,87654321",
			telegramID: 87654321,
			want:       true,
		},
		{
			name:       "unauthorized user blocked",
			allowed:    "12345678,87654321",
			telegramID: 99999999,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			list, err := ParseAllowedUsers(tt.allowed)
			if err != nil {
				t.Fatal(err)
			}
			if got := list.IsAllowed(tt.telegramID); got != tt.want {
				t.Fatalf("IsAllowed(%d) = %v, want %v", tt.telegramID, got, tt.want)
			}
		})
	}
}
