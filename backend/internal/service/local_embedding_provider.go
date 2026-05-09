package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type LocalEmbeddingProvider struct {
	runtime string
	baseURL string
	client  *http.Client
}

func NewLocalEmbeddingProvider(runtime, baseURL string) *LocalEmbeddingProvider {
	return &LocalEmbeddingProvider{
		runtime: strings.ToLower(strings.TrimSpace(runtime)),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *LocalEmbeddingProvider) Embed(ctx context.Context, input EmbeddingRequest) (EmbeddingResult, error) {
	if p == nil || p.client == nil {
		return EmbeddingResult{}, fmt.Errorf("embedding provider is not configured")
	}
	if strings.TrimSpace(input.Model) == "" || len(input.Texts) == 0 {
		return EmbeddingResult{}, fmt.Errorf("%w: embedding model and texts are required", ErrDriveInvalidInput)
	}
	switch p.runtime {
	case "ollama":
		return p.embedOllama(ctx, input)
	case "lmstudio":
		return p.embedOpenAICompatible(ctx, input)
	case "infinity":
		return p.embedInfinity(ctx, input)
	default:
		return EmbeddingResult{}, fmt.Errorf("%w: unsupported embedding runtime", ErrInvalidTenantSettings)
	}
}

func (p *LocalEmbeddingProvider) embedOllama(ctx context.Context, input EmbeddingRequest) (EmbeddingResult, error) {
	var response struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := p.postJSON(ctx, "/api/embed", map[string]any{
		"model": input.Model,
		"input": input.Texts,
	}, &response); err != nil {
		return EmbeddingResult{}, err
	}
	return embeddingResult(input.Model, response.Embeddings)
}

func (p *LocalEmbeddingProvider) embedOpenAICompatible(ctx context.Context, input EmbeddingRequest) (EmbeddingResult, error) {
	return p.embedOpenAICompatiblePath(ctx, input, "/v1/embeddings")
}

func (p *LocalEmbeddingProvider) embedInfinity(ctx context.Context, input EmbeddingRequest) (EmbeddingResult, error) {
	return p.embedOpenAICompatiblePath(ctx, input, "/embeddings")
}

func (p *LocalEmbeddingProvider) embedOpenAICompatiblePath(ctx context.Context, input EmbeddingRequest, path string) (EmbeddingResult, error) {
	var response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := p.postJSON(ctx, path, map[string]any{
		"model": input.Model,
		"input": input.Texts,
	}, &response); err != nil {
		return EmbeddingResult{}, err
	}
	embeddings := make([][]float32, 0, len(response.Data))
	for _, item := range response.Data {
		embeddings = append(embeddings, item.Embedding)
	}
	return embeddingResult(input.Model, embeddings)
}

func (p *LocalEmbeddingProvider) postJSON(ctx context.Context, path string, payload any, target any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode embedding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("call embedding runtime: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("embedding runtime returned status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode embedding response: %w", err)
	}
	return nil
}

func embeddingResult(model string, embeddings [][]float32) (EmbeddingResult, error) {
	if len(embeddings) == 0 {
		return EmbeddingResult{}, fmt.Errorf("embedding runtime returned no embeddings")
	}
	dimension := len(embeddings[0])
	for _, embedding := range embeddings {
		if len(embedding) != dimension {
			return EmbeddingResult{}, fmt.Errorf("embedding runtime returned mixed dimensions")
		}
	}
	return EmbeddingResult{Model: model, Dimension: dimension, Embeddings: embeddings}, nil
}
