package nutrition

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEstimateMeal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		body     string
		query    string
		wantErr  bool
		wantCals float64
		wantName string
	}{
		{
			name:   "success",
			status: http.StatusOK,
			body: `{
				"products":[{
					"product_name":"Greek Yogurt",
					"nutriments":{
						"energy-kcal_100g":97,
						"proteins_100g":9,
						"fat_100g":5,
						"carbohydrates_100g":3.5
					}
				}]
			}`,
			query:    "greek yogurt",
			wantCals: 97,
			wantName: "Greek Yogurt",
		},
		{
			name:    "empty products",
			status:  http.StatusOK,
			body:    `{"products":[]}`,
			query:   "xyzunknown",
			wantErr: true,
		},
		{
			name:    "http error",
			status:  http.StatusBadGateway,
			body:    `bad gateway`,
			query:   "rice",
			wantErr: true,
		},
		{
			name:    "empty query",
			query:   "  ",
			wantErr: true,
		},
		{
			name:   "missing nutrients",
			status: http.StatusOK,
			body: `{
				"products":[{"product_name":"Water","nutriments":{}}]
			}`,
			query:   "water",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var client *Client
			if tt.status != 0 {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.status)
					_, _ = w.Write([]byte(tt.body))
				}))
				t.Cleanup(srv.Close)
				client = &Client{httpClient: srv.Client(), searchURL: srv.URL}
			} else {
				client = NewClient()
			}

			got, err := client.EstimateMeal(context.Background(), tt.query)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Calories != tt.wantCals {
				t.Fatalf("calories = %v, want %v", got.Calories, tt.wantCals)
			}
			if !strings.Contains(got.Description, tt.wantName) {
				t.Fatalf("description %q missing %q", got.Description, tt.wantName)
			}
		})
	}
}

func TestParseNutrimentFallback(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"products":[{"product_name":"Oats","nutriments":{"energy-kcal":389,"proteins":13,"fat":7,"carbohydrates":66}}]}`)
	var parsed offSearchResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	n := parsed.Products[0].Nutriments
	if firstPositive(n.EnergyKcal100g, n.EnergyKcal) != 389 {
		t.Fatal("expected energy-kcal fallback")
	}
}
