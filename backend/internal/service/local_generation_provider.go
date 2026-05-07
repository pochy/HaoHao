package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type LocalGenerationProvider struct {
	runtime string
	baseURL string
	client  *http.Client
}

type GenerationRequest struct {
	Model       string
	System      string
	User        string
	MaxTokens   int
	JSONSchema  any
	Temperature float64
}

type GenerationResult struct {
	Text string
}

func NewLocalGenerationProvider(runtime, baseURL string) *LocalGenerationProvider {
	return &LocalGenerationProvider{
		runtime: strings.ToLower(strings.TrimSpace(runtime)),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

func (p *LocalGenerationProvider) Generate(ctx context.Context, input GenerationRequest) (GenerationResult, error) {
	if p == nil || p.client == nil {
		return GenerationResult{}, fmt.Errorf("generation provider is not configured")
	}
	if strings.TrimSpace(input.Model) == "" {
		return GenerationResult{}, fmt.Errorf("%w: generation model is required", ErrDriveInvalidInput)
	}
	switch p.runtime {
	case "ollama":
		return p.generateOllama(ctx, input)
	case "lmstudio":
		return p.generateOpenAICompatible(ctx, input)
	default:
		return GenerationResult{}, fmt.Errorf("%w: unsupported generation runtime", ErrInvalidTenantSettings)
	}
}

func (p *LocalGenerationProvider) generateOllama(ctx context.Context, input GenerationRequest) (GenerationResult, error) {
	prompt := strings.TrimSpace(input.System + "\n\n" + input.User)
	payload := ollamaGenerateRequest{
		Model:  input.Model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
		Options: map[string]any{
			"temperature": input.Temperature,
			"num_predict": input.MaxTokens,
		},
	}
	var response ollamaGenerateResponse
	if err := p.postJSON(ctx, "/api/generate", payload, &response); err != nil {
		return GenerationResult{}, err
	}
	text := strings.TrimSpace(response.Response)
	if text == "" {
		text = strings.TrimSpace(response.Thinking)
	}
	return GenerationResult{Text: text}, nil
}

func (p *LocalGenerationProvider) generateOpenAICompatible(ctx context.Context, input GenerationRequest) (GenerationResult, error) {
	payload := lmStudioChatCompletionRequest{
		Model: input.Model,
		Messages: []lmStudioChatMessage{
			{Role: "system", Content: input.System},
			{Role: "user", Content: input.User},
		},
		Temperature: input.Temperature,
		MaxTokens:   input.MaxTokens,
		Stream:      false,
	}
	if input.JSONSchema != nil {
		payload.ResponseFormat = input.JSONSchema
	}
	var response lmStudioChatCompletionResponse
	if err := p.postJSON(ctx, "/v1/chat/completions", payload, &response); err != nil {
		return GenerationResult{}, err
	}
	return GenerationResult{Text: response.firstContent()}, nil
}

func (p *LocalGenerationProvider) postJSON(ctx context.Context, path string, payload any, target any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode generation request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("create generation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("call generation runtime: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("generation runtime returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode generation response: %w", err)
	}
	return nil
}
