package service

import "context"

const LocalSearchEmbeddingDimension = 1024

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
