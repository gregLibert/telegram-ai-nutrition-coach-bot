package nutrition

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/domain"
)

const (
	openFoodFactsSearchURL = "https://world.openfoodfacts.org/cgi/search.pl"
	defaultPageSize        = 5
	httpTimeout            = 12 * time.Second
	userAgent              = "telegram-ai-nutrition-coach-bot/1.0 (https://github.com/gregLibert/telegram-ai-nutrition-coach-bot)"
	defaultServingGrams    = 100.0
)

// TraceFunc emits structured observability events for Open Food Facts calls.
type TraceFunc func(ctx context.Context, event string, fields map[string]any)

// Client queries Open Food Facts for packaged-food macros.
type Client struct {
	httpClient *http.Client
	searchURL  string
	trace      TraceFunc
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: httpTimeout},
		searchURL:  openFoodFactsSearchURL,
	}
}

// SetTracer registers structured logging for every Open Food Facts HTTP call.
func (c *Client) SetTracer(fn TraceFunc) {
	if c == nil {
		return
	}
	c.trace = fn
}

// EstimateMeal looks up the query in Open Food Facts and maps per-100g nutrients
// to a MealEstimate for an assumed 100 g serving when no quantity is parsed.
func (c *Client) EstimateMeal(ctx context.Context, query string) (domain.MealEstimate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return domain.MealEstimate{}, fmt.Errorf("empty food query")
	}

	products, err := c.search(ctx, query)
	if err != nil {
		return domain.MealEstimate{}, err
	}
	if len(products) == 0 {
		return domain.MealEstimate{}, fmt.Errorf("no products found")
	}

	product := products[0]
	n := product.Nutriments
	calories := firstPositive(n.EnergyKcal100g, n.EnergyKcal)
	protein := firstPositive(n.Proteins100g, n.Proteins)
	fat := firstPositive(n.Fat100g, n.Fat)
	carbs := firstPositive(n.Carbohydrates100g, n.Carbohydrates)
	if calories <= 0 && protein <= 0 && fat <= 0 && carbs <= 0 {
		return domain.MealEstimate{}, fmt.Errorf("product missing nutrients")
	}

	name := product.ProductName
	if name == "" {
		name = query
	}
	return domain.MealEstimate{
		Description: fmt.Sprintf("%s (%.0fg, Open Food Facts)", name, defaultServingGrams),
		Calories:    calories,
		ProteinG:    protein,
		FatG:        fat,
		CarbsG:      carbs,
		Confidence:  "medium",
		Notes:       "Estimated from Open Food Facts per-100g values",
	}, nil
}

type offSearchResponse struct {
	Products []offProduct `json:"products"`
}

type offProduct struct {
	ProductName string        `json:"product_name"`
	Nutriments  offNutriments `json:"nutriments"`
}

type offNutriments struct {
	EnergyKcal100g    float64 `json:"energy-kcal_100g"`
	EnergyKcal        float64 `json:"energy-kcal"`
	Proteins100g      float64 `json:"proteins_100g"`
	Proteins          float64 `json:"proteins"`
	Fat100g           float64 `json:"fat_100g"`
	Fat               float64 `json:"fat"`
	Carbohydrates100g float64 `json:"carbohydrates_100g"`
	Carbohydrates     float64 `json:"carbohydrates"`
}

func (c *Client) search(ctx context.Context, query string) ([]offProduct, error) {
	endpoint, err := url.Parse(c.searchURL)
	if err != nil {
		return nil, err
	}
	q := endpoint.Query()
	q.Set("search_terms", query)
	q.Set("search_simple", "1")
	q.Set("action", "process")
	q.Set("json", "1")
	q.Set("page_size", strconv.Itoa(defaultPageSize))
	q.Set("fields", "product_name,nutriments")
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	started := time.Now()
	resp, err := c.httpClient.Do(req)
	latencyMs := time.Since(started).Milliseconds()
	if err != nil {
		c.emitTrace(ctx, query, 0, latencyMs, err)
		return nil, err
	}
	defer closeBody(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.emitTrace(ctx, query, resp.StatusCode, latencyMs, err)
		return nil, err
	}
	c.emitTrace(ctx, query, resp.StatusCode, latencyMs, nil)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open food facts status %d: %s", resp.StatusCode, string(body))
	}

	var parsed offSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode open food facts: %w", err)
	}
	return parsed.Products, nil
}

func (c *Client) emitTrace(ctx context.Context, query string, status int, latencyMs int64, callErr error) {
	if c == nil || c.trace == nil {
		return
	}
	fields := map[string]any{
		"query":       query,
		"http_status": status,
		"latency_ms":  latencyMs,
	}
	if barcode := lookLikeBarcode(query); barcode != "" {
		fields["barcode"] = barcode
	}
	if callErr != nil {
		fields["error"] = callErr.Error()
	}
	c.trace(ctx, "open_food_facts_call", fields)
}

func lookLikeBarcode(query string) string {
	trimmed := strings.TrimSpace(query)
	if len(trimmed) < 8 || len(trimmed) > 14 {
		return ""
	}
	for _, r := range trimmed {
		if !unicode.IsDigit(r) {
			return ""
		}
	}
	return trimmed
}

func firstPositive(values ...float64) float64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}
