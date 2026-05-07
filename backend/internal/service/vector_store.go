package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"

	db "example.com/haohao/backend/internal/db"
)

type VectorStore interface {
	UpsertEmbedding(ctx context.Context, input VectorUpsertInput) error
	Search(ctx context.Context, input VectorSearchInput) ([]VectorSearchHit, error)
	DeleteForDocument(ctx context.Context, tenantID, documentID int64) error
}

type VectorUpsertInput struct {
	TenantID         int64
	ResourceKind     string
	ResourceID       int64
	ResourcePublicID string
	DocumentID       int64
	ChunkOrdinal     int32
	SourceText       string
	Model            string
	Dimension        int32
	ContentHash      string
	Embedding        []float32
	Metadata         map[string]any
	Status           string
	ErrorSummary     string
}

type VectorSearchInput struct {
	TenantID     int64
	ResourceKind string
	Model        string
	Embedding    []float32
	Limit        int32
}

type VectorSearchHit struct {
	DocumentID       int64
	ResourceKind     string
	ResourceID       int64
	ResourcePublicID string
	ChunkOrdinal     int32
	SourceText       string
	Model            string
	ContentHash      string
	Score            float64
}

type PgVectorStore struct {
	queries *db.Queries
}

func NewPgVectorStore(queries *db.Queries) *PgVectorStore {
	return &PgVectorStore{queries: queries}
}

func (s *PgVectorStore) UpsertEmbedding(ctx context.Context, input VectorUpsertInput) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("vector store is not configured")
	}
	publicID, err := uuid.Parse(strings.TrimSpace(input.ResourcePublicID))
	if err != nil {
		return fmt.Errorf("%w: invalid vector resource public id", ErrDriveInvalidInput)
	}
	metadata := []byte("{}")
	if input.Metadata != nil {
		encoded, err := json.Marshal(input.Metadata)
		if err != nil {
			return fmt.Errorf("encode vector metadata: %w", err)
		}
		metadata = encoded
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "completed"
	}
	_, err = s.queries.UpsertLocalSearchEmbedding(ctx, db.UpsertLocalSearchEmbeddingParams{
		TenantID:         input.TenantID,
		ResourceKind:     input.ResourceKind,
		ResourceID:       input.ResourceID,
		ResourcePublicID: publicID,
		DocumentID:       input.DocumentID,
		ChunkOrdinal:     input.ChunkOrdinal,
		SourceText:       input.SourceText,
		Model:            input.Model,
		Dimension:        input.Dimension,
		ContentHash:      input.ContentHash,
		Embedding:        pgvector.NewVector(input.Embedding),
		Metadata:         metadata,
		Status:           status,
		ErrorSummary:     pgtype.Text{String: input.ErrorSummary, Valid: strings.TrimSpace(input.ErrorSummary) != ""},
	})
	if err != nil {
		return fmt.Errorf("upsert local search embedding: %w", err)
	}
	return nil
}

func (s *PgVectorStore) Search(ctx context.Context, input VectorSearchInput) ([]VectorSearchHit, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("vector store is not configured")
	}
	limit := input.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.queries.SearchLocalSearchEmbeddingsCosine(ctx, db.SearchLocalSearchEmbeddingsCosineParams{
		QueryEmbedding: pgvector.NewVector(input.Embedding),
		TenantID:       input.TenantID,
		ResourceKind:   input.ResourceKind,
		Model:          input.Model,
		LimitCount:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search local search embeddings: %w", err)
	}
	hits := make([]VectorSearchHit, 0, len(rows))
	for _, row := range rows {
		hits = append(hits, VectorSearchHit{
			DocumentID:       row.DocumentID,
			ResourceKind:     row.ResourceKind,
			ResourceID:       row.ResourceID,
			ResourcePublicID: row.ResourcePublicID,
			ChunkOrdinal:     row.ChunkOrdinal,
			SourceText:       row.SourceText,
			Model:            row.Model,
			ContentHash:      row.ContentHash,
			Score:            row.Score,
		})
	}
	return hits, nil
}

func (s *PgVectorStore) DeleteForDocument(ctx context.Context, tenantID, documentID int64) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("vector store is not configured")
	}
	if err := s.queries.DeleteLocalSearchEmbeddingsForDocument(ctx, db.DeleteLocalSearchEmbeddingsForDocumentParams{
		TenantID:   tenantID,
		DocumentID: documentID,
	}); err != nil {
		return fmt.Errorf("delete local search embeddings: %w", err)
	}
	return nil
}
