package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LMStudioDriveProductExtractor struct {
	client *http.Client
}

const lmStudioProductExtractionTextLimit = 2000

func NewLMStudioDriveProductExtractor(client *http.Client) LMStudioDriveProductExtractor {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	return LMStudioDriveProductExtractor{client: client}
}

func (LMStudioDriveProductExtractor) Name() string {
	return "lmstudio"
}

func (e LMStudioDriveProductExtractor) ExtractProducts(ctx context.Context, input DriveProductExtractionInput) (DriveProductExtractionResult, error) {
	model := strings.TrimSpace(input.Policy.LMStudioModel)
	if model == "" {
		return DriveProductExtractionResult{}, fmt.Errorf("%w: LM Studio model is required", ErrDriveOCRStructuredUnsupported)
	}
	baseURL, err := normalizedLMStudioBaseURL(input.Policy.LMStudioBaseURL)
	if err != nil {
		return DriveProductExtractionResult{}, err
	}
	requestBody, err := json.Marshal(lmStudioChatCompletionRequest{
		Model: model,
		Messages: []lmStudioChatMessage{
			{Role: "system", Content: "You extract structured product records from OCR text. Return compact JSON only."},
			{Role: "user", Content: buildLMStudioProductPrompt(input)},
		},
		Temperature:    0,
		MaxTokens:      900,
		Stream:         false,
		ResponseFormat: lmStudioProductExtractionResponseFormat(),
	})
	if err != nil {
		return DriveProductExtractionResult{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, ollamaProductExtractionTimeout(input.Policy))
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return DriveProductExtractionResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return DriveProductExtractionResult{}, fmt.Errorf("call LM Studio: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return DriveProductExtractionResult{}, fmt.Errorf("LM Studio chat completion failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var completed lmStudioChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completed); err != nil {
		return DriveProductExtractionResult{}, fmt.Errorf("decode LM Studio chat completion response: %w", err)
	}
	payload := completed.firstContent()
	items, err := parseOllamaProductItems(payload)
	if err != nil {
		return DriveProductExtractionResult{}, fmt.Errorf("decode LM Studio product extraction response: %w", err)
	}
	result := DriveProductExtractionResult{Items: make([]DriveProductExtractionItem, 0, len(items))}
	for _, item := range items {
		converted := item.toDriveProductExtractionItem(input)
		if strings.TrimSpace(converted.Name) == "" {
			continue
		}
		result.Items = append(result.Items, converted)
		if len(result.Items) >= ollamaProductExtractionItemLimit {
			break
		}
	}
	return result, nil
}

func CheckDriveOCRLMStudio(ctx context.Context, policy DriveOCRPolicy) DriveOCRLMStudioStatus {
	status := DriveOCRLMStudioStatus{Configured: strings.TrimSpace(policy.LMStudioModel) != ""}
	baseURL, err := normalizedLMStudioBaseURL(policy.LMStudioBaseURL)
	if err != nil {
		return status
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return status
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return status
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return status
	}
	status.Reachable = true
	var models lmStudioModelsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&models); err != nil {
		return status
	}
	model := strings.TrimSpace(policy.LMStudioModel)
	for _, item := range models.Data {
		if item.ID == model {
			status.ModelAvailable = true
			break
		}
	}
	return status
}

type lmStudioChatCompletionRequest struct {
	Model          string                `json:"model"`
	Messages       []lmStudioChatMessage `json:"messages"`
	Temperature    float64               `json:"temperature"`
	MaxTokens      int                   `json:"max_tokens"`
	Stream         bool                  `json:"stream"`
	ResponseFormat any                   `json:"response_format,omitempty"`
}

type lmStudioChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type lmStudioChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
}

func (r lmStudioChatCompletionResponse) firstContent() string {
	if len(r.Choices) == 0 {
		return ""
	}
	content := strings.TrimSpace(r.Choices[0].Message.Content)
	if content != "" {
		return content
	}
	return strings.TrimSpace(r.Choices[0].Message.ReasoningContent)
}

type lmStudioModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func normalizedLMStudioBaseURL(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		value = "http://127.0.0.1:1234"
	}
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(value), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: invalid LM Studio base URL", ErrDriveOCRStructuredUnsupported)
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/v1")
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return parsed.String(), nil
}

func buildLMStudioProductPrompt(input DriveProductExtractionInput) string {
	text := ollamaProductPromptText(input.FullText)
	text = truncateRunes(text, lmStudioProductExtractionTextLimit)
	return `/no_think
Extract catalog product records from the OCR text below.
Return JSON matching the provided schema.
Rules:
- Include concrete products and model numbers.
- Do not invent prices, JAN codes, or availability.
- Use empty strings or empty objects when unknown.
- Keep sourceText short.
- Limit the result to the most important 4 items.

OCR text:
` + text
}

func lmStudioProductExtractionResponseFormat() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	objectSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": true,
	}
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name": "drive_product_extraction",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"itemType":     stringSchema,
								"name":         stringSchema,
								"brand":        stringSchema,
								"manufacturer": stringSchema,
								"model":        stringSchema,
								"sku":          stringSchema,
								"janCode":      stringSchema,
								"category":     stringSchema,
								"description":  stringSchema,
								"price":        objectSchema,
								"promotion":    objectSchema,
								"availability": objectSchema,
								"sourceText":   stringSchema,
								"evidence": map[string]any{
									"type": "array",
									"items": map[string]any{
										"type":                 "object",
										"additionalProperties": true,
									},
								},
								"attributes": objectSchema,
								"confidence": map[string]any{
									"type": "number",
								},
							},
							"required": []string{"name"},
						},
					},
				},
				"required": []string{"items"},
			},
		},
	}
}
