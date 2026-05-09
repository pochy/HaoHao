package service

import "context"

const (
	LocalSearchDefaultEmbeddingDimension = 1024
	LocalSearchMaxEmbeddingDimension     = 2000
)

type EmbeddingProvider interface {
	Embed(ctx context.Context, input EmbeddingRequest) (EmbeddingResult, error)
}

type EmbeddingRequest struct {
	TenantID int64
	Model    string
	Texts    []string
}

type EmbeddingResult struct {
	Model      string
	Dimension  int
	Embeddings [][]float32
}

type LocalSearchChunk struct {
	Ordinal     int32
	Text        string
	ContentHash string
}
