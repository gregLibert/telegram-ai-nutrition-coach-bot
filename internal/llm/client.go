package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	ModelVision   = "qwen/qwen-2.5-vl-72b-instruct"
	ModelReason   = "deepseek/deepseek-chat"
	openRouterURL = "https://openrouter.ai/api/v1/chat/completions"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
	auditFn    AuditFunc
}

type AuditFunc func(ctx context.Context, entry AuditEntry) error

type AuditEntry struct {
	UserID       *int64
	Operation    string
	Model        string
	Prompt       string
	RawResponse  string
	TokensPrompt int
	TokensOutput int
	LatencyMs    int64
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Temperature    float64         `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string  `json:"type"`
	Text     string  `json:"text,omitempty"`
	ImageURL *imgRef `json:"image_url,omitempty"`
}

type imgRef struct {
	URL string `json:"url"`
}

type responseFormat struct {
	Type       string         `json:"type"`
	JSONSchema *jsonSchemaDef `json:"json_schema,omitempty"`
}

type jsonSchemaDef struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func NewClient(apiKey string, auditFn AuditFunc) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		auditFn:    auditFn,
	}
}

func (c *Client) CompleteJSON(ctx context.Context, userID *int64, operation, model, systemPrompt, userPrompt string, schema map[string]any) (string, error) {
	return c.complete(ctx, userID, operation, model, systemPrompt, userPrompt, nil, schema)
}

func (c *Client) AnalyzeMealPhoto(ctx context.Context, userID *int64, imagePath, systemPrompt, userPrompt string, schema map[string]any) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	mime := detectMIME(imagePath)
	b64 := base64.StdEncoding.EncodeToString(data)
	url := fmt.Sprintf("data:%s;base64,%s", mime, b64)
	return c.complete(ctx, userID, "meal_vision", ModelVision, systemPrompt, userPrompt, &url, schema)
}

func (c *Client) CompleteText(ctx context.Context, userID *int64, operation, model, systemPrompt, userPrompt string) (string, error) {
	return c.complete(ctx, userID, operation, model, systemPrompt, userPrompt, nil, nil)
}

func (c *Client) complete(ctx context.Context, userID *int64, operation, model, systemPrompt, userPrompt string, imageURL *string, schema map[string]any) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY not set")
	}

	content := []contentPart{{Type: "text", Text: userPrompt}}
	if imageURL != nil {
		content = append(content, contentPart{Type: "image_url", ImageURL: &imgRef{URL: *imageURL}})
	}

	reqBody := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: []contentPart{{Type: "text", Text: systemPrompt}}},
			{Role: "user", Content: content},
		},
		Temperature: 0.2,
	}
	if schema != nil {
		reqBody.ResponseFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchemaDef{
				Name:   operation,
				Strict: true,
				Schema: schema,
			},
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	start := time.Now()
	respBody, err := c.doRequest(ctx, body)
	latency := time.Since(start).Milliseconds()

	promptText := systemPrompt + "\n---\n" + userPrompt
	if err != nil {
		c.audit(ctx, userID, operation, model, promptText, err.Error(), 0, 0, latency)
		return "", err
	}

	var resp chatResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty LLM response")
	}

	contentStr := resp.Choices[0].Message.Content
	c.audit(ctx, userID, operation, model, promptText, contentStr,
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, latency)
	return contentStr, nil
}

func (c *Client) doRequest(ctx context.Context, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/greg/telegram-ai-nutrition-coach-bot")
	req.Header.Set("X-Title", "Nutrition Coach Bot")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter status %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func (c *Client) audit(ctx context.Context, userID *int64, operation, model, prompt, rawResp string, tokIn, tokOut int, latency int64) {
	if c.auditFn == nil {
		return
	}
	_ = c.auditFn(ctx, AuditEntry{
		UserID: userID, Operation: operation, Model: model,
		Prompt: prompt, RawResponse: rawResp,
		TokensPrompt: tokIn, TokensOutput: tokOut, LatencyMs: latency,
	})
}

func detectMIME(path string) string {
	switch {
	case len(path) > 4 && path[len(path)-4:] == ".png":
		return "image/png"
	case len(path) > 4 && path[len(path)-5:] == ".jpeg":
		return "image/jpeg"
	default:
		return "image/jpeg"
	}
}

var MealEstimateSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"description": map[string]any{"type": "string"},
		"calories":    map[string]any{"type": "number"},
		"protein_g":   map[string]any{"type": "number"},
		"fat_g":       map[string]any{"type": "number"},
		"carbs_g":     map[string]any{"type": "number"},
		"confidence":  map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
		"notes":       map[string]any{"type": "string"},
	},
	"required":             []string{"description", "calories", "protein_g", "fat_g", "carbs_g", "confidence", "notes"},
	"additionalProperties": false,
}

var PortionSolverSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"ingredients": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":      map[string]any{"type": "string"},
					"raw_grams": map[string]any{"type": "number"},
					"protein_g": map[string]any{"type": "number"},
					"fat_g":     map[string]any{"type": "number"},
					"carbs_g":   map[string]any{"type": "number"},
					"calories":  map[string]any{"type": "number"},
				},
				"required":             []string{"name", "raw_grams", "protein_g", "fat_g", "carbs_g", "calories"},
				"additionalProperties": false,
			},
		},
		"total_calories":  map[string]any{"type": "number"},
		"total_protein_g": map[string]any{"type": "number"},
		"total_fat_g":     map[string]any{"type": "number"},
		"total_carbs_g":   map[string]any{"type": "number"},
		"explanation":     map[string]any{"type": "string"},
	},
	"required":             []string{"ingredients", "total_calories", "total_protein_g", "total_fat_g", "total_carbs_g", "explanation"},
	"additionalProperties": false,
}

var RecipeOptionsSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"options": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":             map[string]any{"type": "integer"},
					"title":          map[string]any{"type": "string"},
					"macros_summary": map[string]any{"type": "string"},
					"key_ingredients": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
				"required":             []string{"id", "title", "macros_summary", "key_ingredients"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"options"},
	"additionalProperties": false,
}
