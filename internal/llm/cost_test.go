package llm

import "testing"

func TestParseGenerationCost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		want    float64
		wantErr bool
	}{
		{
			name: "nested data total_cost",
			body: `{"data":{"total_cost":0.00123}}`,
			want: 0.00123,
		},
		{
			name: "top-level total_cost",
			body: `{"total_cost":0.42}`,
			want: 0.42,
		},
		{
			name:    "invalid json",
			body:    `{`,
			wantErr: true,
		},
		{
			name: "zero cost",
			body: `{"data":{"total_cost":0}}`,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseGenerationCost([]byte(tt.body))
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
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
