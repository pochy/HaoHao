package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	ollamaProductExtractionTextLimit = 5000
	ollamaProductExtractionItemLimit = 20
)

var ollamaProductModelPattern = regexp.MustCompile(`\b(?:[24]B-C[0-9A-Z]+|AN-[0-9A-Z]+|VR-[0-9A-Z]+|RP-[0-9A-Z]+)\b`)

type OllamaDriveProductExtractor struct {
	client *http.Client
}

func NewOllamaDriveProductExtractor(client *http.Client) OllamaDriveProductExtractor {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	return OllamaDriveProductExtractor{client: client}
}

func (OllamaDriveProductExtractor) Name() string {
	return "ollama"
}

func (e OllamaDriveProductExtractor) ExtractProducts(ctx context.Context, input DriveProductExtractionInput) (DriveProductExtractionResult, error) {
	model := strings.TrimSpace(input.Policy.OllamaModel)
	if model == "" {
		return DriveProductExtractionResult{}, fmt.Errorf("%w: ollama model is required", ErrDriveOCRStructuredUnsupported)
	}
	baseURL, err := normalizedOllamaBaseURL(input.Policy.OllamaBaseURL)
	if err != nil {
		return DriveProductExtractionResult{}, err
	}
	requestBody, err := json.Marshal(ollamaGenerateRequest{
		Model:  model,
		Prompt: buildOllamaProductPrompt(input),
		Stream: false,
		Format: "json",
		Options: map[string]any{
			"temperature": 0,
			"num_predict": 700,
		},
	})
	if err != nil {
		return DriveProductExtractionResult{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, ollamaProductExtractionTimeout(input.Policy))
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, baseURL+"/api/generate", bytes.NewReader(requestBody))
	if err != nil {
		return DriveProductExtractionResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return DriveProductExtractionResult{}, fmt.Errorf("call ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return DriveProductExtractionResult{}, fmt.Errorf("ollama generate failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var generated ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&generated); err != nil {
		return DriveProductExtractionResult{}, fmt.Errorf("decode ollama generate response: %w", err)
	}
	payload := strings.TrimSpace(generated.Response)
	if payload == "" {
		payload = strings.TrimSpace(generated.Thinking)
	}
	items, err := parseOllamaProductItems(payload)
	if err != nil {
		return DriveProductExtractionResult{}, fmt.Errorf("decode ollama product extraction response: %w", err)
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

func CheckDriveOCROllama(ctx context.Context, policy DriveOCRPolicy) DriveOCROllamaStatus {
	status := DriveOCROllamaStatus{Configured: strings.TrimSpace(policy.OllamaModel) != ""}
	baseURL, err := normalizedOllamaBaseURL(policy.OllamaBaseURL)
	if err != nil {
		return status
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, baseURL+"/api/tags", nil)
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
	var tags ollamaTagsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tags); err != nil {
		return status
	}
	model := strings.TrimSpace(policy.OllamaModel)
	for _, item := range tags.Models {
		if item.Name == model || item.Model == model {
			status.ModelAvailable = true
			break
		}
	}
	return status
}

type ollamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Format  string         `json:"format,omitempty"`
	Options map[string]any `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Thinking string `json:"thinking"`
	Done     bool   `json:"done"`
}

type ollamaTagsResponse struct {
	Models []struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	} `json:"models"`
}

type ollamaProductEnvelope struct {
	Items []ollamaProductItem `json:"items"`
}

type ollamaProductItem struct {
	ItemType     string           `json:"itemType"`
	Name         string           `json:"name"`
	Brand        string           `json:"brand"`
	Manufacturer string           `json:"manufacturer"`
	Model        string           `json:"model"`
	SKU          string           `json:"sku"`
	JANCode      string           `json:"janCode"`
	Category     string           `json:"category"`
	Description  string           `json:"description"`
	Price        map[string]any   `json:"price"`
	Promotion    map[string]any   `json:"promotion"`
	Availability map[string]any   `json:"availability"`
	SourceText   string           `json:"sourceText"`
	Evidence     []map[string]any `json:"evidence"`
	Attributes   map[string]any   `json:"attributes"`
	Confidence   *float64         `json:"confidence"`
}

func (item ollamaProductItem) toDriveProductExtractionItem(input DriveProductExtractionInput) DriveProductExtractionItem {
	name := cleanOllamaString(item.Name)
	model := cleanOllamaString(item.Model)
	brand := cleanOllamaString(item.Brand)
	if name == "" {
		name = strings.TrimSpace(strings.Join(nonEmptyStrings(brand, model), " "))
	}
	sourceText := cleanOllamaString(item.SourceText)
	evidence := item.Evidence
	if len(evidence) == 0 && sourceText != "" {
		evidence = []map[string]any{{"text": sourceText}}
	}
	attributes := item.Attributes
	if attributes == nil {
		attributes = map[string]any{}
	}
	attributes["schemaVersion"] = 1
	extractor := defaultString(input.Policy.StructuredExtractor, "ollama")
	attributes["extractor"] = extractor
	switch extractor {
	case "lmstudio":
		attributes["lmStudioModel"] = input.Policy.LMStudioModel
	case "ollama":
		attributes["ollamaModel"] = input.Policy.OllamaModel
	case "gemini", "codex", "claude":
		attributes["localCommand"] = extractor
	case "python", "ginza", "sudachipy":
		attributes["pythonHelper"] = "drive_product_extraction_nlp.py"
		if _, ok := attributes["nlpEngine"]; !ok {
			attributes["nlpEngine"] = extractor
		}
	}
	confidence := item.Confidence
	if confidence != nil {
		value := clampConfidence(*confidence)
		confidence = &value
	}
	return DriveProductExtractionItem{
		TenantID:     input.TenantID,
		FilePublicID: input.File.PublicID,
		ItemType:     defaultString(item.ItemType, "product"),
		Name:         name,
		Brand:        brand,
		Manufacturer: cleanOllamaString(item.Manufacturer),
		Model:        model,
		SKU:          cleanOllamaString(item.SKU),
		JANCode:      cleanOllamaString(item.JANCode),
		Category:     cleanOllamaString(item.Category),
		Description:  cleanOllamaString(item.Description),
		Price:        nonNilObject(item.Price),
		Promotion:    nonNilObject(item.Promotion),
		Availability: nonNilObject(item.Availability),
		SourceText:   sourceText,
		Evidence:     evidence,
		Attributes:   attributes,
		Confidence:   confidence,
		CreatedAt:    time.Now(),
	}
}

func buildOllamaProductPrompt(input DriveProductExtractionInput) string {
	text := ollamaProductPromptText(input.FullText)
	return `/no_think
Extract structured product records from the OCR text below.
Return only a JSON object with this exact shape:
{"items":[{"itemType":"product","name":"","brand":"","manufacturer":"","model":"","sku":"","janCode":"","category":"","description":"","price":{},"promotion":{},"availability":{},"sourceText":"","evidence":[{"pageNumber":1,"text":""}],"attributes":{},"confidence":0.0}]}

Rules:
- Include concrete catalog products, variants, and model numbers.
- For recorder catalogs, model numbers such as 4B-C40GT3 or 2B-C20GT1 are separate products even when there is no price.
- Do not invent prices, JAN codes, or availability. Use empty strings or empty objects when the text does not contain the value.
- Keep sourceText short and copy the OCR text that supports the item.
- Limit the result to the most important 12 items.

OCR text:
` + text
}

func ollamaProductPromptText(fullText string) string {
	lines := ocrCandidateLines(fullText)
	if len(lines) == 0 {
		return truncateRunes(fullText, ollamaProductExtractionTextLimit)
	}
	selected := make(map[int]struct{})
	for i, line := range lines {
		if !ollamaProductModelPattern.MatchString(line) && !strings.Contains(line, "形名") {
			continue
		}
		for j := max(0, i-2); j < min(len(lines), i+3); j++ {
			selected[j] = struct{}{}
		}
	}
	if len(selected) == 0 {
		return truncateRunes(fullText, ollamaProductExtractionTextLimit)
	}
	out := make([]string, 0, len(selected))
	for i, line := range lines {
		if _, ok := selected[i]; ok {
			out = append(out, line)
		}
	}
	return truncateRunes(strings.Join(out, "\n"), ollamaProductExtractionTextLimit)
}

func parseOllamaProductItems(raw string) ([]ollamaProductItem, error) {
	payload := extractOllamaJSONPayload(raw)
	var envelope ollamaProductEnvelope
	if err := json.Unmarshal([]byte(payload), &envelope); err == nil && envelope.Items != nil {
		return envelope.Items, nil
	}
	var items []ollamaProductItem
	if err := json.Unmarshal([]byte(payload), &items); err == nil {
		return items, nil
	}
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		return nil, err
	}
	return envelope.Items, nil
}

func extractOllamaJSONPayload(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		return value
	}
	if start, end := strings.Index(value, "{"), strings.LastIndex(value, "}"); start >= 0 && end > start {
		return value[start : end+1]
	}
	if start, end := strings.Index(value, "["), strings.LastIndex(value, "]"); start >= 0 && end > start {
		return value[start : end+1]
	}
	return value
}

func normalizedOllamaBaseURL(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		value = "http://127.0.0.1:11434"
	}
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(value), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: invalid ollama base URL", ErrDriveOCRStructuredUnsupported)
	}
	return parsed.String(), nil
}

func ollamaProductExtractionTimeout(policy DriveOCRPolicy) time.Duration {
	seconds := policy.TimeoutSecondsPerPage
	if seconds <= 0 {
		seconds = 60
	}
	if seconds < 15 {
		seconds = 15
	}
	if seconds > 300 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func cleanOllamaString(value string) string {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "null") || strings.EqualFold(value, "unknown") {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}

func nonNilObject(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func clampConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
