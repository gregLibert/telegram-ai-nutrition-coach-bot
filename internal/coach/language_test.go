package coach

import "testing"

func TestNormalizeLanguage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"en", "en"},
		{"fr", "fr"},
		{"", "en"},
		{"de", "en"},
	}
	for _, tt := range tests {
		if got := normalizeLanguage(tt.in); got != tt.want {
			t.Errorf("normalizeLanguage(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWithLanguage(t *testing.T) {
	t.Parallel()
	got := withLanguage("Base prompt.", "fr")
	if got != "Base prompt.\nRespond entirely in French." {
		t.Fatalf("unexpected: %q", got)
	}
	got = withLanguage("Base prompt.", "en")
	if got != "Base prompt.\nRespond entirely in English." {
		t.Fatalf("unexpected: %q", got)
	}
}
