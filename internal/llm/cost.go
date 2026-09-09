package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	openRouterGenerationURL = "https://openrouter.ai/api/v1/generation"
	generationHTTPTimeout   = 15 * time.Second
)

// CostLogger logs structured LLM cost events.
type CostLogger func(ctx context.Context, generationID string, totalCost float64)

// FetchAndLogCost retrieves total_cost for a generation and logs it asynchronously-safe.
func (c *Client) FetchAndLogCost(ctx context.Context, generationID string) {
	if c == nil || generationID == "" || c.costLog == nil {
		return
	}
	cost, err := c.fetchGenerationCost(ctx, generationID)
	if err != nil {
		return
	}
	c.costLog(ctx, generationID, cost)
}

func (c *Client) fetchGenerationCost(ctx context.Context, generationID string) (float64, error) {
	endpoint, err := url.Parse(openRouterGenerationURL)
	if err != nil {
		return 0, err
	}
	q := endpoint.Query()
	q.Set("id", generationID)
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: generationHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer closeHTTPBody(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, &StatusError{StatusCode: resp.StatusCode, Body: string(body)}
	}
	return parseGenerationCost(body)
}

func parseGenerationCost(body []byte) (float64, error) {
	var payload struct {
		Data struct {
			TotalCost float64 `json:"total_cost"`
		} `json:"data"`
		TotalCost float64 `json:"total_cost"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, fmt.Errorf("parse generation cost: %w", err)
	}
	if payload.Data.TotalCost > 0 {
		return payload.Data.TotalCost, nil
	}
	return payload.TotalCost, nil
}
