package coach

import "testing"

func TestStripForceFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		wantText  string
		wantForce bool
	}{
		{name: "plain meal", in: "200g chicken", wantText: "200g chicken"},
		{name: "force suffix", in: "200g chicken /force", wantText: "200g chicken", wantForce: true},
		{name: "force case", in: "rice /FORCE", wantText: "rice", wantForce: true},
		{name: "force only", in: "/force", wantText: "", wantForce: true},
		{name: "force embedded not suffix", in: "chicken /force bowl", wantText: "chicken /force bowl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, force := stripForceFlag(tt.in)
			if got != tt.wantText || force != tt.wantForce {
				t.Fatalf("stripForceFlag(%q) = (%q, %v), want (%q, %v)",
					tt.in, got, force, tt.wantText, tt.wantForce)
			}
		})
	}
}
