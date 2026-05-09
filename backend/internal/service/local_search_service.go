package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

const (
	LocalSearchResourceDriveFile         = "drive_file"
	LocalSearchResourceOCRRun            = "ocr_run"
	LocalSearchResourceProductExtraction = "product_extraction"
	LocalSearchResourceGoldTable         = "gold_table"
	LocalSearchResourceSchemaColumn      = "schema_column"
	LocalSearchResourceMappingExample    = "mapping_example"

	DriveSearchModeKeyword  = "keyword"
	DriveSearchModeSemantic = "semantic"
	DriveSearchModeHybrid   = "hybrid"

	localSearchRebuildLimit     = 1000
	localSearchMaxChunkRunes    = 1600
	localSearchChunkOverlap     = 200
	localSearchMaxChunks        = 32
	localSearchMinSemanticScore = 0.05

	localSearchDriveMinSemanticScore        = 0.84
	localSearchSchemaColumnMinSemanticScore = 0.78
)

type LocalSearchMatch struct {
	ResourceKind           string
	ResourcePublicID       string
	MedallionAssetPublicID string
	Layer                  string
	Snippet                string
	Score                  float64
	IndexedAt              *time.Time
}

type LocalSearchIndexJob struct {
	ID               int64
	PublicID         string
	TenantID         int64
	ResourceKind     string
	ResourceID       *int64
	ResourcePublicID string
	Reason           string
	Status           string
	Attempts         int32
	IndexedCount     int32
	SkippedCount     int32
	FailedCount      int32
	LastError        string
	StartedAt        *time.Time
	CompletedAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type LocalSearchDocumentInput struct {
	TenantID         int64
	ResourceKind     string
	ResourceID       int64
	ResourcePublicID string
	Title            string
	BodyText         string
	Snippet          string
	ContentHash      string
	SourceUpdatedAt  *time.Time
}

type LocalSearchSemanticHit struct {
	ResourceKind     string
	ResourceID       int64
	ResourcePublicID string
	FileObjectID     int64
	SourceText       string
	Score            float64
}

type localSearchIndexSummary struct {
	Indexed int32
	Skipped int32
	Failed  int32
}

type LocalSearchService struct {
	pool           *pgxpool.Pool
	queries        *db.Queries
	drive          *DriveService
	datasets       *DatasetService
	medallion      *MedallionCatalogService
	outbox         *OutboxService
	tenantSettings *TenantSettingsService
}

func NewLocalSearchService(pool *pgxpool.Pool, queries *db.Queries, drive *DriveService, datasets *DatasetService, medallion *MedallionCatalogService, outbox *OutboxService, tenantSettings *TenantSettingsService) *LocalSearchService {
	return &LocalSearchService{
		pool:           pool,
		queries:        queries,
		drive:          drive,
		datasets:       datasets,
		medallion:      medallion,
		outbox:         outbox,
		tenantSettings: tenantSettings,
	}
}

func (s *LocalSearchService) RequestIndex(ctx context.Context, tenantID int64, resourceKind string, resourceID int64, resourcePublicID, reason string) (LocalSearchIndexJob, error) {
	if err := s.ensureConfigured(); err != nil {
		return LocalSearchIndexJob{}, err
	}
	resourceKind = strings.TrimSpace(resourceKind)
	if !localSearchResourceKindValid(resourceKind) || resourceID <= 0 {
		return LocalSearchIndexJob{}, fmt.Errorf("%w: invalid local search resource", ErrDriveInvalidInput)
	}
	parsedPublicID, err := uuid.Parse(strings.TrimSpace(resourcePublicID))
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("%w: invalid local search resource public id", ErrDriveInvalidInput)
	}
	if strings.TrimSpace(reason) == "" {
		reason = "index_requested"
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("begin local search index request: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := s.queries.WithTx(tx)
	job, err := qtx.CreateLocalSearchIndexJob(ctx, db.CreateLocalSearchIndexJobParams{
		TenantID:         tenantID,
		ResourceKind:     pgtype.Text{String: resourceKind, Valid: true},
		ResourceID:       pgtype.Int8{Int64: resourceID, Valid: true},
		ResourcePublicID: pgtype.UUID{Bytes: parsedPublicID, Valid: true},
		Reason:           reason,
	})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("create local search index job: %w", err)
	}
	event, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "local_search_index_job",
		AggregateID:   job.PublicID.String(),
		EventType:     "local_search.index_requested",
		Payload: map[string]any{
			"tenantId":         tenantID,
			"jobId":            job.ID,
			"resourceKind":     resourceKind,
			"resourceId":       resourceID,
			"resourcePublicId": parsedPublicID.String(),
		},
	})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("enqueue local search index job: %w", err)
	}
	job, err = qtx.LinkLocalSearchIndexJobOutboxEvent(ctx, db.LinkLocalSearchIndexJobOutboxEventParams{
		OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
		ID:            job.ID,
		TenantID:      tenantID,
	})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("link local search index outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("commit local search index request: %w", err)
	}
	return localSearchIndexJobFromDB(job), nil
}

func (s *LocalSearchService) RequestRebuild(ctx context.Context, tenantID int64, reason string) (LocalSearchIndexJob, error) {
	if err := s.ensureConfigured(); err != nil {
		return LocalSearchIndexJob{}, err
	}
	if strings.TrimSpace(reason) == "" {
		reason = "rebuild"
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("begin local search rebuild request: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := s.queries.WithTx(tx)
	job, err := qtx.CreateLocalSearchIndexJob(ctx, db.CreateLocalSearchIndexJobParams{
		TenantID: tenantID,
		Reason:   reason,
	})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("create local search rebuild job: %w", err)
	}
	event, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "local_search_index_job",
		AggregateID:   job.PublicID.String(),
		EventType:     "local_search.rebuild_requested",
		Payload: map[string]any{
			"tenantId": tenantID,
			"jobId":    job.ID,
		},
	})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("enqueue local search rebuild job: %w", err)
	}
	job, err = qtx.LinkLocalSearchIndexJobOutboxEvent(ctx, db.LinkLocalSearchIndexJobOutboxEventParams{
		OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
		ID:            job.ID,
		TenantID:      tenantID,
	})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("link local search rebuild outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("commit local search rebuild request: %w", err)
	}
	return localSearchIndexJobFromDB(job), nil
}

func (s *LocalSearchService) RequestIndexBestEffort(ctx context.Context, tenantID int64, resourceKind string, resourceID int64, resourcePublicID, reason string) {
	if s == nil || s.outbox == nil {
		return
	}
	_, _ = s.RequestIndex(ctx, tenantID, resourceKind, resourceID, resourcePublicID, reason)
}

func (s *LocalSearchService) UpsertDocument(ctx context.Context, input LocalSearchDocumentInput) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("local search service is not configured")
	}
	resourceKind := strings.TrimSpace(input.ResourceKind)
	if !localSearchResourceKindValid(resourceKind) || input.TenantID <= 0 || input.ResourceID <= 0 {
		return fmt.Errorf("%w: invalid local search document resource", ErrDriveInvalidInput)
	}
	publicID, err := uuid.Parse(strings.TrimSpace(input.ResourcePublicID))
	if err != nil {
		return fmt.Errorf("%w: invalid local search document public id", ErrDriveInvalidInput)
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return fmt.Errorf("%w: local search document title is required", ErrDriveInvalidInput)
	}
	bodyText := sanitizeSearchText(input.BodyText)
	snippet := strings.TrimSpace(input.Snippet)
	if snippet == "" {
		snippet = searchSnippet(bodyText, title)
	}
	contentHash := strings.TrimSpace(input.ContentHash)
	if contentHash == "" {
		contentHash = localSearchHash(publicID.String(), title+"\n"+bodyText)
	}
	document, err := s.queries.UpsertLocalSearchDocument(ctx, db.UpsertLocalSearchDocumentParams{
		TenantID:         input.TenantID,
		ResourceKind:     resourceKind,
		ResourceID:       input.ResourceID,
		ResourcePublicID: publicID,
		Title:            title,
		BodyText:         bodyText,
		Snippet:          snippet,
		ContentHash:      contentHash,
		SourceUpdatedAt:  pgTimestampPtr(input.SourceUpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("upsert local search document: %w", err)
	}
	s.requestEmbeddingBestEffort(ctx, document)
	return nil
}

func (s *LocalSearchService) requestEmbeddingBestEffort(ctx context.Context, document db.LocalSearchDocument) {
	if s == nil || s.outbox == nil || s.tenantSettings == nil {
		return
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, document.TenantID)
	if err != nil || !policy.LocalSearch.VectorEnabled || policy.LocalSearch.EmbeddingRuntime == "none" {
		return
	}
	_, _ = s.requestEmbedding(ctx, document.TenantID, document.ResourceKind, document.ResourceID, document.ResourcePublicID.String())
}

func (s *LocalSearchService) requestEmbedding(ctx context.Context, tenantID int64, resourceKind string, resourceID int64, resourcePublicID string) (LocalSearchIndexJob, error) {
	if err := s.ensureConfigured(); err != nil {
		return LocalSearchIndexJob{}, err
	}
	if !localSearchResourceKindValid(resourceKind) || resourceID <= 0 {
		return LocalSearchIndexJob{}, fmt.Errorf("%w: invalid local search embedding resource", ErrDriveInvalidInput)
	}
	parsedPublicID, err := uuid.Parse(strings.TrimSpace(resourcePublicID))
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("%w: invalid local search embedding resource public id", ErrDriveInvalidInput)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("begin local search embedding request: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := s.queries.WithTx(tx)
	job, err := qtx.CreateLocalSearchIndexJob(ctx, db.CreateLocalSearchIndexJobParams{
		TenantID:         tenantID,
		ResourceKind:     pgtype.Text{String: resourceKind, Valid: true},
		ResourceID:       pgtype.Int8{Int64: resourceID, Valid: true},
		ResourcePublicID: pgtype.UUID{Bytes: parsedPublicID, Valid: true},
		Reason:           "embedding_requested",
	})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("create local search embedding job: %w", err)
	}
	event, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "local_search_index_job",
		AggregateID:   job.PublicID.String(),
		EventType:     "local_search.embedding_requested",
		Payload: map[string]any{
			"tenantId":         tenantID,
			"jobId":            job.ID,
			"resourceKind":     resourceKind,
			"resourceId":       resourceID,
			"resourcePublicId": parsedPublicID.String(),
		},
	})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("enqueue local search embedding job: %w", err)
	}
	job, err = qtx.LinkLocalSearchIndexJobOutboxEvent(ctx, db.LinkLocalSearchIndexJobOutboxEventParams{
		OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
		ID:            job.ID,
		TenantID:      tenantID,
	})
	if err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("link local search embedding outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return LocalSearchIndexJob{}, fmt.Errorf("commit local search embedding request: %w", err)
	}
	return localSearchIndexJobFromDB(job), nil
}

func (s *LocalSearchService) ListIndexJobs(ctx context.Context, tenantID int64, status string, limit int32) ([]LocalSearchIndexJob, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("local search service is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.queries.ListLocalSearchIndexJobs(ctx, db.ListLocalSearchIndexJobsParams{
		TenantID:   tenantID,
		Status:     pgText(strings.TrimSpace(status)),
		LimitCount: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list local search index jobs: %w", err)
	}
	out := make([]LocalSearchIndexJob, 0, len(rows))
	for _, row := range rows {
		out = append(out, localSearchIndexJobFromDB(row))
	}
	return out, nil
}

func (s *LocalSearchService) SearchDriveFiles(ctx context.Context, input DriveSearchInput) ([]DriveSearchResult, error) {
	if s == nil || s.queries == nil || s.drive == nil {
		return nil, fmt.Errorf("local search service is not configured")
	}
	actor, err := s.drive.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return nil, err
	}
	policy, err := s.drive.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return nil, err
	}
	if !policy.SearchEnabled {
		return nil, ErrDrivePolicyDenied
	}
	mode := normalizeDriveSearchMode(input.Mode)
	limit := input.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := pgText(strings.TrimSpace(input.Query))
	contentType := pgText(normalizeContentType(input.ContentType))
	if strings.TrimSpace(input.ContentType) == "" {
		contentType = pgtype.Text{}
	}
	candidates := make([]DriveFile, 0, limit)
	semanticMatches := map[int64]LocalSearchMatch{}
	seen := map[int64]struct{}{}
	if mode == DriveSearchModeKeyword || mode == DriveSearchModeHybrid {
		rows, err := s.queries.SearchLocalSearchDriveFileCandidates(ctx, db.SearchLocalSearchDriveFileCandidatesParams{
			TenantID:      input.TenantID,
			Query:         query,
			ContentType:   contentType,
			UpdatedAfter:  pgTimestampPtr(input.UpdatedAfter),
			UpdatedBefore: pgTimestampPtr(input.UpdatedBefore),
			LimitCount:    limit,
		})
		if err != nil {
			return nil, fmt.Errorf("search local drive files: %w", err)
		}
		for _, row := range rows {
			file := driveFileFromDB(row)
			candidates = append(candidates, file)
			seen[file.ID] = struct{}{}
		}
	}
	if mode == DriveSearchModeSemantic || mode == DriveSearchModeHybrid {
		hits, err := s.SearchDriveFileSemantic(ctx, input.TenantID, input.Query, limit)
		if err != nil {
			if mode == DriveSearchModeSemantic {
				return nil, fmt.Errorf("semantic drive search: %w", err)
			}
		}
		for _, hit := range hits {
			row, err := s.drive.getDriveFileRow(ctx, input.TenantID, DriveResourceRef{Type: DriveResourceTypeFile, ID: hit.FileObjectID})
			if errors.Is(err, ErrDriveNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			file := driveFileFromDB(row)
			if !driveFileMatchesSearchInput(file, input) {
				continue
			}
			if _, ok := seen[file.ID]; !ok {
				candidates = append(candidates, file)
				seen[file.ID] = struct{}{}
			}
			semanticMatches[file.ID] = LocalSearchMatch{
				ResourceKind:     hit.ResourceKind,
				ResourcePublicID: hit.ResourcePublicID,
				Snippet:          cleanDriveRAGContextText(searchSnippet(hit.SourceText, file.OriginalFilename)),
				Score:            hit.Score,
			}
		}
	}
	viewable, err := s.drive.authz.FilterViewableFiles(ctx, actor, candidates)
	if err != nil {
		return nil, err
	}
	items := make([]DriveItem, 0, len(viewable))
	for i := range viewable {
		file := viewable[i]
		items = append(items, DriveItem{Type: DriveItemTypeFile, File: &file})
	}
	items = s.drive.applyDriveListFilter(s.drive.enrichDriveItems(ctx, actor, items), input.Filter)
	results := make([]DriveSearchResult, 0, len(items))
	for _, item := range items {
		if item.File == nil {
			continue
		}
		matches, err := s.matchesForFile(ctx, input.TenantID, item.File.ID, strings.TrimSpace(input.Query))
		if err != nil {
			return nil, err
		}
		if semanticMatch, ok := semanticMatches[item.File.ID]; ok {
			matches = prependLocalSearchMatch(matches, semanticMatch)
		}
		result := DriveSearchResult{Item: item, Matches: matches}
		if len(matches) > 0 {
			result.Snippet = matches[0].Snippet
			result.IndexedAt = matches[0].IndexedAt
		}
		results = append(results, result)
	}
	return results, nil
}

func normalizeDriveSearchMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case DriveSearchModeSemantic:
		return DriveSearchModeSemantic
	case DriveSearchModeHybrid:
		return DriveSearchModeHybrid
	default:
		return DriveSearchModeKeyword
	}
}

func driveFileMatchesSearchInput(file DriveFile, input DriveSearchInput) bool {
	if strings.TrimSpace(input.ContentType) != "" && normalizeContentType(file.ContentType) != normalizeContentType(input.ContentType) {
		return false
	}
	if input.UpdatedAfter != nil && file.UpdatedAt.Before(*input.UpdatedAfter) {
		return false
	}
	if input.UpdatedBefore != nil && file.UpdatedAt.After(*input.UpdatedBefore) {
		return false
	}
	return true
}

func prependLocalSearchMatch(matches []LocalSearchMatch, match LocalSearchMatch) []LocalSearchMatch {
	for _, existing := range matches {
		if existing.ResourceKind == match.ResourceKind && existing.ResourcePublicID == match.ResourcePublicID {
			return matches
		}
	}
	out := make([]LocalSearchMatch, 0, len(matches)+1)
	out = append(out, match)
	out = append(out, matches...)
	return out
}

func (s *LocalSearchService) HandleIndexRequested(ctx context.Context, tenantID, jobID, outboxEventID int64) error {
	job, err := s.markJobProcessing(ctx, tenantID, jobID, outboxEventID)
	if err != nil {
		return err
	}
	if !job.ResourceKind.Valid || !job.ResourceID.Valid {
		return s.failJob(ctx, tenantID, job.ID, "local search index job resource is required")
	}
	summary, err := s.indexResource(ctx, tenantID, job.ResourceKind.String, job.ResourceID.Int64)
	if err != nil {
		_ = s.failJob(ctx, tenantID, job.ID, err.Error())
		return err
	}
	status := "completed"
	if summary.Indexed == 0 && summary.Skipped > 0 {
		status = "skipped"
	}
	_, err = s.queries.CompleteLocalSearchIndexJob(ctx, db.CompleteLocalSearchIndexJobParams{
		ID:           job.ID,
		TenantID:     tenantID,
		Status:       status,
		IndexedCount: summary.Indexed,
		SkippedCount: summary.Skipped,
		FailedCount:  summary.Failed,
	})
	if err != nil {
		return fmt.Errorf("complete local search index job: %w", err)
	}
	return nil
}

func (s *LocalSearchService) HandleRebuildRequested(ctx context.Context, tenantID, jobID, outboxEventID int64) error {
	job, err := s.markJobProcessing(ctx, tenantID, jobID, outboxEventID)
	if err != nil {
		return err
	}
	summary := localSearchIndexSummary{}
	add := func(item localSearchIndexSummary) {
		summary.Indexed += item.Indexed
		summary.Skipped += item.Skipped
		summary.Failed += item.Failed
	}
	files, err := s.queries.ListLocalSearchRebuildDriveFiles(ctx, db.ListLocalSearchRebuildDriveFilesParams{TenantID: tenantID, LimitCount: localSearchRebuildLimit})
	if err != nil {
		return s.failJob(ctx, tenantID, job.ID, fmt.Sprintf("list drive files: %v", err))
	}
	for _, row := range files {
		add(s.indexDriveFile(ctx, driveFileFromDB(row)))
	}
	runs, err := s.queries.ListLocalSearchRebuildOCRRuns(ctx, db.ListLocalSearchRebuildOCRRunsParams{TenantID: tenantID, LimitCount: localSearchRebuildLimit})
	if err != nil {
		return s.failJob(ctx, tenantID, job.ID, fmt.Sprintf("list ocr runs: %v", err))
	}
	for _, row := range runs {
		add(s.indexOCRRun(ctx, row))
	}
	products, err := s.queries.ListLocalSearchRebuildProductExtractionItems(ctx, db.ListLocalSearchRebuildProductExtractionItemsParams{TenantID: tenantID, LimitCount: localSearchRebuildLimit})
	if err != nil {
		return s.failJob(ctx, tenantID, job.ID, fmt.Sprintf("list product extraction items: %v", err))
	}
	for _, row := range products {
		add(s.indexProductExtractionItem(ctx, row))
	}
	gold, err := s.queries.ListLocalSearchRebuildGoldPublications(ctx, db.ListLocalSearchRebuildGoldPublicationsParams{TenantID: tenantID, LimitCount: localSearchRebuildLimit})
	if err != nil {
		return s.failJob(ctx, tenantID, job.ID, fmt.Sprintf("list gold publications: %v", err))
	}
	for _, row := range gold {
		add(s.indexGoldPublication(ctx, row))
	}
	status := "completed"
	if summary.Failed > 0 {
		status = "failed"
	} else if summary.Indexed == 0 && summary.Skipped > 0 {
		status = "skipped"
	}
	_, err = s.queries.CompleteLocalSearchIndexJob(ctx, db.CompleteLocalSearchIndexJobParams{
		ID:           job.ID,
		TenantID:     tenantID,
		Status:       status,
		IndexedCount: summary.Indexed,
		SkippedCount: summary.Skipped,
		FailedCount:  summary.Failed,
	})
	if err != nil {
		return fmt.Errorf("complete local search rebuild job: %w", err)
	}
	return nil
}

func (s *LocalSearchService) HandleEmbeddingRequested(ctx context.Context, tenantID, jobID, outboxEventID int64) error {
	job, err := s.markJobProcessing(ctx, tenantID, jobID, outboxEventID)
	if err != nil {
		return err
	}
	if s.tenantSettings == nil {
		return s.failJob(ctx, tenantID, job.ID, "tenant settings service is not configured")
	}
	policy, policyErr := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if policyErr != nil || !policy.LocalSearch.VectorEnabled || policy.LocalSearch.EmbeddingRuntime == "none" {
		_, err = s.queries.CompleteLocalSearchIndexJob(ctx, db.CompleteLocalSearchIndexJobParams{
			ID:           job.ID,
			TenantID:     tenantID,
			Status:       "skipped",
			SkippedCount: 1,
		})
		if err != nil {
			return fmt.Errorf("skip local search embedding job: %w", err)
		}
		return nil
	}
	if !isValidLocalSearchEmbeddingDimension(policy.LocalSearch.Dimension) {
		return s.failJob(ctx, tenantID, job.ID, "local search embedding dimension must be between 1 and 2000")
	}
	if !job.ResourceKind.Valid || !job.ResourceID.Valid {
		return s.failJob(ctx, tenantID, job.ID, "local search embedding job resource is required")
	}
	document, err := s.queries.GetLocalSearchDocumentForResource(ctx, db.GetLocalSearchDocumentForResourceParams{
		TenantID:     tenantID,
		ResourceKind: job.ResourceKind.String,
		ResourceID:   job.ResourceID.Int64,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_, completeErr := s.queries.CompleteLocalSearchIndexJob(ctx, db.CompleteLocalSearchIndexJobParams{
			ID:           job.ID,
			TenantID:     tenantID,
			Status:       "skipped",
			SkippedCount: 1,
		})
		if completeErr != nil {
			return fmt.Errorf("skip missing local search embedding document: %w", completeErr)
		}
		return nil
	}
	if err != nil {
		_ = s.failJob(ctx, tenantID, job.ID, err.Error())
		return err
	}
	chunks := localSearchChunks(document)
	if len(chunks) == 0 {
		_, completeErr := s.queries.CompleteLocalSearchIndexJob(ctx, db.CompleteLocalSearchIndexJobParams{
			ID:           job.ID,
			TenantID:     tenantID,
			Status:       "skipped",
			SkippedCount: 1,
		})
		if completeErr != nil {
			return fmt.Errorf("skip empty local search embedding document: %w", completeErr)
		}
		return nil
	}
	texts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		texts = append(texts, chunk.Text)
	}
	provider := NewLocalEmbeddingProvider(policy.LocalSearch.EmbeddingRuntime, policy.LocalSearch.RuntimeURL)
	result, err := provider.Embed(ctx, EmbeddingRequest{
		TenantID: tenantID,
		Model:    policy.LocalSearch.Model,
		Texts:    texts,
	})
	if err != nil {
		_ = s.failJob(ctx, tenantID, job.ID, err.Error())
		return err
	}
	if result.Dimension != policy.LocalSearch.Dimension || len(result.Embeddings) != len(chunks) {
		err := fmt.Errorf("embedding runtime returned invalid result shape")
		_ = s.failJob(ctx, tenantID, job.ID, err.Error())
		return err
	}
	store := NewPgVectorStore(s.queries)
	if err := store.DeleteForDocument(ctx, tenantID, document.ID); err != nil {
		_ = s.failJob(ctx, tenantID, job.ID, err.Error())
		return err
	}
	for i, chunk := range chunks {
		if err := store.UpsertEmbedding(ctx, VectorUpsertInput{
			TenantID:         document.TenantID,
			ResourceKind:     document.ResourceKind,
			ResourceID:       document.ResourceID,
			ResourcePublicID: document.ResourcePublicID.String(),
			DocumentID:       document.ID,
			ChunkOrdinal:     chunk.Ordinal,
			SourceText:       chunk.Text,
			Model:            policy.LocalSearch.Model,
			Dimension:        int32(result.Dimension),
			ContentHash:      chunk.ContentHash,
			Embedding:        result.Embeddings[i],
			Metadata: map[string]any{
				"documentContentHash": document.ContentHash,
			},
			Status: "completed",
		}); err != nil {
			_ = s.failJob(ctx, tenantID, job.ID, err.Error())
			return err
		}
	}
	_, err = s.queries.CompleteLocalSearchIndexJob(ctx, db.CompleteLocalSearchIndexJobParams{
		ID:           job.ID,
		TenantID:     tenantID,
		Status:       "completed",
		IndexedCount: int32(len(chunks)),
	})
	if err != nil {
		return fmt.Errorf("complete local search embedding job: %w", err)
	}
	return nil
}

func (s *LocalSearchService) SearchSemantic(ctx context.Context, tenantID int64, resourceKind, query string, limit int32) ([]LocalSearchSemanticHit, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if !localSearchResourceKindValid(resourceKind) {
		return nil, fmt.Errorf("%w: invalid local search resource kind", ErrDriveInvalidInput)
	}
	if s.tenantSettings == nil {
		return nil, nil
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !policy.LocalSearch.VectorEnabled || policy.LocalSearch.EmbeddingRuntime == "none" || strings.TrimSpace(policy.LocalSearch.Model) == "" {
		return nil, nil
	}
	provider := NewLocalEmbeddingProvider(policy.LocalSearch.EmbeddingRuntime, policy.LocalSearch.RuntimeURL)
	semanticQuery := localSearchSemanticQueryText(resourceKind, query)
	result, err := provider.Embed(ctx, EmbeddingRequest{
		TenantID: tenantID,
		Model:    policy.LocalSearch.Model,
		Texts:    []string{semanticQuery},
	})
	if err != nil {
		return nil, err
	}
	if result.Dimension != policy.LocalSearch.Dimension || len(result.Embeddings) != 1 {
		return nil, fmt.Errorf("embedding runtime returned invalid result shape")
	}
	store := NewPgVectorStore(s.queries)
	hits, err := store.Search(ctx, VectorSearchInput{
		TenantID:     tenantID,
		ResourceKind: resourceKind,
		Model:        policy.LocalSearch.Model,
		Dimension:    int32(result.Dimension),
		Embedding:    result.Embeddings[0],
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]LocalSearchSemanticHit, 0, len(hits))
	minScore := localSearchSemanticScoreThreshold(resourceKind)
	for _, hit := range hits {
		if hit.Score < minScore {
			continue
		}
		out = append(out, LocalSearchSemanticHit{
			ResourceKind:     hit.ResourceKind,
			ResourceID:       hit.ResourceID,
			ResourcePublicID: hit.ResourcePublicID,
			SourceText:       hit.SourceText,
			Score:            hit.Score,
		})
	}
	return out, nil
}

func (s *LocalSearchService) SearchDriveFileSemantic(ctx context.Context, tenantID int64, query string, limit int32) ([]LocalSearchSemanticHit, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if s.tenantSettings == nil {
		return nil, nil
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !policy.LocalSearch.VectorEnabled || policy.LocalSearch.EmbeddingRuntime == "none" || strings.TrimSpace(policy.LocalSearch.Model) == "" {
		return nil, nil
	}
	provider := NewLocalEmbeddingProvider(policy.LocalSearch.EmbeddingRuntime, policy.LocalSearch.RuntimeURL)
	result, err := provider.Embed(ctx, EmbeddingRequest{
		TenantID: tenantID,
		Model:    policy.LocalSearch.Model,
		Texts:    []string{localSearchSemanticQueryText(LocalSearchResourceDriveFile, query)},
	})
	if err != nil {
		return nil, err
	}
	if result.Dimension != policy.LocalSearch.Dimension || len(result.Embeddings) != 1 {
		return nil, fmt.Errorf("embedding runtime returned invalid result shape")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		WITH best_per_file AS (
			SELECT DISTINCT ON (d.file_object_id)
				d.resource_kind,
				d.resource_id,
				d.resource_public_id::text AS resource_public_id,
				d.file_object_id,
				e.source_text,
				(1.0 - (e.embedding <=> $1::vector))::float8 AS score
			FROM local_search_embeddings e
			JOIN local_search_documents d ON d.id = e.document_id
			WHERE e.tenant_id = $2
			  AND d.tenant_id = $2
			  AND d.file_object_id IS NOT NULL
			  AND d.resource_kind IN ('drive_file', 'ocr_run', 'product_extraction', 'gold_table')
			  AND NOT (d.resource_kind = 'drive_file' AND btrim(d.body_text) = '')
			  AND e.model = $3
			  AND e.dimension = $4
			  AND e.status = 'completed'
			  AND e.embedding IS NOT NULL
			ORDER BY d.file_object_id, e.embedding <=> $1::vector, e.id
		)
		SELECT resource_kind, resource_id, resource_public_id, file_object_id, source_text, score
		FROM best_per_file
		ORDER BY score DESC, file_object_id DESC
	`, pgvector.NewVector(result.Embeddings[0]), tenantID, policy.LocalSearch.Model, int32(result.Dimension))
	if err != nil {
		return nil, fmt.Errorf("search drive file semantic embeddings: %w", err)
	}
	defer rows.Close()
	hits := make([]LocalSearchSemanticHit, 0, limit)
	minScore := localSearchSemanticScoreThreshold(LocalSearchResourceDriveFile)
	for rows.Next() {
		var hit LocalSearchSemanticHit
		if err := rows.Scan(&hit.ResourceKind, &hit.ResourceID, &hit.ResourcePublicID, &hit.FileObjectID, &hit.SourceText, &hit.Score); err != nil {
			return nil, err
		}
		if hit.Score < minScore {
			continue
		}
		hits = append(hits, hit)
		if int32(len(hits)) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hits, nil
}

func localSearchSemanticScoreThreshold(resourceKind string) float64 {
	switch resourceKind {
	case LocalSearchResourceDriveFile:
		return localSearchDriveMinSemanticScore
	case LocalSearchResourceSchemaColumn:
		return localSearchSchemaColumnMinSemanticScore
	default:
		return localSearchMinSemanticScore
	}
}

func localSearchSemanticQueryText(resourceKind, query string) string {
	query = strings.TrimSpace(query)
	if resourceKind != LocalSearchResourceDriveFile || query == "" {
		return query
	}
	expanded := []string{query}
	if strings.Contains(query, "支払期限") {
		expanded = append(expanded, "振込期限", "支払期日", "入金期限")
	}
	if strings.Contains(query, "支払先") {
		expanded = append(expanded, "振込先", "請求元", "取引先")
	}
	if strings.Contains(query, "請求金額") || strings.Contains(query, "合計金額") {
		expanded = append(expanded, "税込合計", "請求額", "支払金額")
	}
	return strings.Join(uniqueStringList(expanded), " ")
}

func (s *LocalSearchService) indexResource(ctx context.Context, tenantID int64, resourceKind string, resourceID int64) (localSearchIndexSummary, error) {
	switch resourceKind {
	case LocalSearchResourceDriveFile:
		row, err := s.drive.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, ID: resourceID})
		if errors.Is(err, ErrDriveNotFound) {
			return localSearchIndexSummary{Skipped: 1}, nil
		}
		if err != nil {
			return localSearchIndexSummary{}, err
		}
		return s.indexDriveFile(ctx, driveFileFromDB(row)), nil
	case LocalSearchResourceOCRRun:
		row, err := s.queries.GetDriveOCRRunByIDForTenant(ctx, db.GetDriveOCRRunByIDForTenantParams{TenantID: tenantID, ID: resourceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return localSearchIndexSummary{Skipped: 1}, nil
		}
		if err != nil {
			return localSearchIndexSummary{}, err
		}
		return s.indexOCRRun(ctx, row), nil
	case LocalSearchResourceProductExtraction:
		row, err := s.queries.GetDriveProductExtractionItemByIDForTenant(ctx, db.GetDriveProductExtractionItemByIDForTenantParams{TenantID: tenantID, ID: resourceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return localSearchIndexSummary{Skipped: 1}, nil
		}
		if err != nil {
			return localSearchIndexSummary{}, err
		}
		return s.indexProductExtractionItem(ctx, row), nil
	case LocalSearchResourceGoldTable:
		row, err := s.queries.GetDatasetGoldPublicationByIDForTenant(ctx, db.GetDatasetGoldPublicationByIDForTenantParams{TenantID: tenantID, ID: resourceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return localSearchIndexSummary{Skipped: 1}, nil
		}
		if err != nil {
			return localSearchIndexSummary{}, err
		}
		return s.indexGoldPublication(ctx, row), nil
	default:
		return localSearchIndexSummary{}, fmt.Errorf("%w: unsupported local search resource kind", ErrDriveInvalidInput)
	}
}

func (s *LocalSearchService) indexDriveFile(ctx context.Context, file DriveFile) localSearchIndexSummary {
	if !localSearchFileIndexable(file) {
		_ = s.queries.DeleteLocalSearchDocumentsForFile(ctx, db.DeleteLocalSearchDocumentsForFileParams{TenantID: file.TenantID, FileObjectID: pgInt8Value(file.ID)})
		return localSearchIndexSummary{Skipped: 1}
	}
	text := ""
	if s.drive != nil && s.drive.storage != nil && driveSearchCanExtract(file) {
		body, err := s.drive.storage.Open(ctx, file.StorageKey)
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(body, 256*1024))
			_ = body.Close()
			if readErr == nil {
				text = sanitizeSearchText(string(data))
			}
		}
	}
	assetID := int64(0)
	if s.medallion != nil {
		if asset, ok, err := s.medallion.EnsureDriveFileAsset(ctx, file, nil); err == nil && ok {
			assetID = asset.ID
		}
	}
	resourcePublicID, err := uuid.Parse(file.PublicID)
	if err != nil {
		return localSearchIndexSummary{Failed: 1}
	}
	document, err := s.queries.UpsertLocalSearchDocument(ctx, db.UpsertLocalSearchDocumentParams{
		TenantID:         file.TenantID,
		ResourceKind:     LocalSearchResourceDriveFile,
		ResourceID:       file.ID,
		ResourcePublicID: resourcePublicID,
		FileObjectID:     pgInt8Value(file.ID),
		MedallionAssetID: pgInt8Value(assetID),
		Title:            file.OriginalFilename,
		BodyText:         text,
		Snippet:          searchSnippet(text, file.OriginalFilename),
		ContentHash:      localSearchHash(file.SHA256Hex, text),
		SourceUpdatedAt:  pgtype.Timestamptz{Time: file.UpdatedAt, Valid: !file.UpdatedAt.IsZero()},
	})
	if err != nil {
		return localSearchIndexSummary{Failed: 1}
	}
	s.requestEmbeddingBestEffort(ctx, document)
	return localSearchIndexSummary{Indexed: 1}
}

func (s *LocalSearchService) indexOCRRun(ctx context.Context, row db.DriveOcrRun) localSearchIndexSummary {
	if row.Status != "completed" || strings.TrimSpace(row.ExtractedText) == "" {
		_ = s.queries.DeleteLocalSearchDocumentForResource(ctx, db.DeleteLocalSearchDocumentForResourceParams{TenantID: row.TenantID, ResourceKind: LocalSearchResourceOCRRun, ResourceID: row.ID})
		return localSearchIndexSummary{Skipped: 1}
	}
	file, ok := s.fileForSearch(ctx, row.TenantID, row.FileObjectID)
	if !ok {
		return localSearchIndexSummary{Skipped: 1}
	}
	run := driveOCRRunFromDB(row, file.PublicID)
	assetID := int64(0)
	if s.medallion != nil {
		if asset, err := s.medallion.EnsureOCRRunAsset(ctx, file, run, MedallionPipelineStatusCompleted, run.RequestedByUserID); err == nil {
			assetID = asset.ID
		}
	}
	text := sanitizeSearchText(row.ExtractedText)
	document, err := s.queries.UpsertLocalSearchDocument(ctx, db.UpsertLocalSearchDocumentParams{
		TenantID:         row.TenantID,
		ResourceKind:     LocalSearchResourceOCRRun,
		ResourceID:       row.ID,
		ResourcePublicID: row.PublicID,
		FileObjectID:     pgInt8Value(file.ID),
		MedallionAssetID: pgInt8Value(assetID),
		Title:            fmt.Sprintf("OCR: %s", file.OriginalFilename),
		BodyText:         text,
		Snippet:          searchSnippet(text, file.OriginalFilename),
		ContentHash:      localSearchHash(row.ContentSha256, text),
		SourceUpdatedAt:  row.UpdatedAt,
	})
	if err != nil {
		return localSearchIndexSummary{Failed: 1}
	}
	s.requestEmbeddingBestEffort(ctx, document)
	return localSearchIndexSummary{Indexed: 1}
}

func (s *LocalSearchService) indexProductExtractionItem(ctx context.Context, row db.DriveProductExtractionItem) localSearchIndexSummary {
	file, ok := s.fileForSearch(ctx, row.TenantID, row.FileObjectID)
	if !ok {
		return localSearchIndexSummary{Skipped: 1}
	}
	title := strings.TrimSpace(row.Name)
	if title == "" {
		title = fmt.Sprintf("Product: %s", file.OriginalFilename)
	}
	assetID := int64(0)
	if s.medallion != nil {
		if asset, err := s.medallion.EnsureProductExtractionAsset(ctx, file, row, nil); err == nil {
			assetID = asset.ID
		}
	}
	text := sanitizeSearchText(strings.Join(localSearchProductTextParts(row), " "))
	document, err := s.queries.UpsertLocalSearchDocument(ctx, db.UpsertLocalSearchDocumentParams{
		TenantID:         row.TenantID,
		ResourceKind:     LocalSearchResourceProductExtraction,
		ResourceID:       row.ID,
		ResourcePublicID: row.PublicID,
		FileObjectID:     pgInt8Value(file.ID),
		MedallionAssetID: pgInt8Value(assetID),
		Title:            title,
		BodyText:         text,
		Snippet:          searchSnippet(text, title),
		ContentHash:      localSearchHash(row.PublicID.String(), text),
		SourceUpdatedAt:  row.CreatedAt,
	})
	if err != nil {
		return localSearchIndexSummary{Failed: 1}
	}
	s.requestEmbeddingBestEffort(ctx, document)
	return localSearchIndexSummary{Indexed: 1}
}

func (s *LocalSearchService) indexGoldPublication(ctx context.Context, row db.DatasetGoldPublication) localSearchIndexSummary {
	if row.Status != "active" || row.ArchivedAt.Valid {
		_ = s.queries.DeleteLocalSearchDocumentForResource(ctx, db.DeleteLocalSearchDocumentForResourceParams{TenantID: row.TenantID, ResourceKind: LocalSearchResourceGoldTable, ResourceID: row.ID})
		return localSearchIndexSummary{Skipped: 1}
	}
	publication := datasetGoldPublicationFromDB(row)
	if s.datasets != nil {
		s.datasets.hydrateGoldPublication(ctx, row.TenantID, &publication)
	}
	assetID := int64(0)
	if s.medallion != nil {
		if asset, err := s.medallion.EnsureGoldTableAsset(ctx, publication, publication.UpdatedByUserID); err == nil {
			assetID = asset.ID
		}
	}
	schemaText := string(row.SchemaSummary)
	if len(row.SchemaSummary) > 0 {
		var decoded any
		if err := json.Unmarshal(row.SchemaSummary, &decoded); err == nil {
			if encoded, err := json.Marshal(decoded); err == nil {
				schemaText = string(encoded)
			}
		}
	}
	text := sanitizeSearchText(strings.Join([]string{
		row.DisplayName,
		row.Description,
		row.GoldDatabase,
		row.GoldTable,
		schemaText,
	}, " "))
	document, err := s.queries.UpsertLocalSearchDocument(ctx, db.UpsertLocalSearchDocumentParams{
		TenantID:          row.TenantID,
		ResourceKind:      LocalSearchResourceGoldTable,
		ResourceID:        row.ID,
		ResourcePublicID:  row.PublicID,
		MedallionAssetID:  pgInt8Value(assetID),
		GoldPublicationID: pgInt8Value(row.ID),
		Title:             row.DisplayName,
		BodyText:          text,
		Snippet:           searchSnippet(text, row.DisplayName),
		ContentHash:       localSearchHash(row.PublicID.String(), text),
		SourceUpdatedAt:   row.UpdatedAt,
	})
	if err != nil {
		return localSearchIndexSummary{Failed: 1}
	}
	s.requestEmbeddingBestEffort(ctx, document)
	return localSearchIndexSummary{Indexed: 1}
}

func (s *LocalSearchService) fileForSearch(ctx context.Context, tenantID, fileObjectID int64) (DriveFile, bool) {
	if s.drive == nil {
		return DriveFile{}, false
	}
	row, err := s.drive.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, ID: fileObjectID})
	if err != nil {
		return DriveFile{}, false
	}
	file := driveFileFromDB(row)
	if !localSearchFileIndexable(file) {
		_ = s.queries.DeleteLocalSearchDocumentsForFile(ctx, db.DeleteLocalSearchDocumentsForFileParams{TenantID: tenantID, FileObjectID: pgInt8Value(fileObjectID)})
		return DriveFile{}, false
	}
	return file, true
}

func (s *LocalSearchService) matchesForFile(ctx context.Context, tenantID, fileObjectID int64, query string) ([]LocalSearchMatch, error) {
	rows, err := s.queries.ListLocalSearchMatchesForFile(ctx, db.ListLocalSearchMatchesForFileParams{
		TenantID:     tenantID,
		FileObjectID: pgInt8Value(fileObjectID),
		Query:        pgText(query),
		LimitCount:   5,
	})
	if err != nil {
		return nil, fmt.Errorf("list local search matches: %w", err)
	}
	matches := make([]LocalSearchMatch, 0, len(rows))
	for _, row := range rows {
		matches = append(matches, LocalSearchMatch{
			ResourceKind:           row.ResourceKind,
			ResourcePublicID:       row.ResourcePublicID,
			MedallionAssetPublicID: interfaceString(row.MedallionAssetPublicID),
			Layer:                  row.Layer,
			Snippet:                row.Snippet,
			IndexedAt:              optionalPgTime(row.IndexedAt),
		})
	}
	return matches, nil
}

func (s *LocalSearchService) markJobProcessing(ctx context.Context, tenantID, jobID, outboxEventID int64) (db.LocalSearchIndexJob, error) {
	if err := s.ensureConfigured(); err != nil {
		return db.LocalSearchIndexJob{}, err
	}
	job, err := s.queries.MarkLocalSearchIndexJobProcessing(ctx, db.MarkLocalSearchIndexJobProcessingParams{
		TenantID:      tenantID,
		ID:            jobID,
		OutboxEventID: pgtype.Int8{Int64: outboxEventID, Valid: outboxEventID > 0},
	})
	if err != nil {
		return db.LocalSearchIndexJob{}, fmt.Errorf("mark local search index job processing: %w", err)
	}
	return job, nil
}

func (s *LocalSearchService) failJob(ctx context.Context, tenantID, jobID int64, message string) error {
	_, err := s.queries.FailLocalSearchIndexJob(ctx, db.FailLocalSearchIndexJobParams{
		TenantID:  tenantID,
		ID:        jobID,
		LastError: message,
	})
	if err != nil {
		return fmt.Errorf("fail local search index job: %w", err)
	}
	return nil
}

func (s *LocalSearchService) ensureConfigured() error {
	if s == nil || s.pool == nil || s.queries == nil || s.outbox == nil {
		return fmt.Errorf("local search service is not configured")
	}
	return nil
}

func localSearchResourceKindValid(value string) bool {
	switch value {
	case LocalSearchResourceDriveFile, LocalSearchResourceOCRRun, LocalSearchResourceProductExtraction, LocalSearchResourceGoldTable, LocalSearchResourceSchemaColumn, LocalSearchResourceMappingExample:
		return true
	default:
		return false
	}
}

func localSearchFileIndexable(file DriveFile) bool {
	if file.ID <= 0 || file.TenantID <= 0 || file.DeletedAt != nil || file.DLPBlocked {
		return false
	}
	switch file.ScanStatus {
	case "", "clean", "skipped":
		return true
	default:
		return false
	}
}

func localSearchProductTextParts(row db.DriveProductExtractionItem) []string {
	parts := []string{row.ItemType, row.Name, optionalText(row.Brand), optionalText(row.Manufacturer), optionalText(row.Model), optionalText(row.Sku), optionalText(row.JanCode), optionalText(row.Category), optionalText(row.Description), row.SourceText}
	for _, raw := range [][]byte{row.Price, row.Promotion, row.Availability, row.Evidence, row.Attributes} {
		if len(raw) > 0 {
			parts = append(parts, string(raw))
		}
	}
	return parts
}

func localSearchHash(seed, text string) string {
	sum := sha256.Sum256([]byte(seed + "\n" + text))
	return hex.EncodeToString(sum[:])
}

func localSearchChunks(document db.LocalSearchDocument) []LocalSearchChunk {
	text := sanitizeSearchText(strings.TrimSpace(strings.Join([]string{document.Title, document.BodyText}, "\n\n")))
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= localSearchMaxChunkRunes {
		return []LocalSearchChunk{{
			Ordinal:     0,
			Text:        text,
			ContentHash: localSearchHash(document.ContentHash, text),
		}}
	}
	step := localSearchMaxChunkRunes - localSearchChunkOverlap
	if step <= 0 {
		step = localSearchMaxChunkRunes
	}
	chunks := make([]LocalSearchChunk, 0, min(localSearchMaxChunks, 1+(len(runes)/step)))
	for start := 0; start < len(runes) && len(chunks) < localSearchMaxChunks; start += step {
		end := start + localSearchMaxChunkRunes
		if end > len(runes) {
			end = len(runes)
		}
		chunkText := strings.TrimSpace(string(runes[start:end]))
		if chunkText != "" {
			chunks = append(chunks, LocalSearchChunk{
				Ordinal:     int32(len(chunks)),
				Text:        chunkText,
				ContentHash: localSearchHash(document.ContentHash, chunkText),
			})
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

func localSearchIndexJobFromDB(row db.LocalSearchIndexJob) LocalSearchIndexJob {
	return LocalSearchIndexJob{
		ID:               row.ID,
		PublicID:         row.PublicID.String(),
		TenantID:         row.TenantID,
		ResourceKind:     optionalText(row.ResourceKind),
		ResourceID:       optionalPgInt8(row.ResourceID),
		ResourcePublicID: uuidString(row.ResourcePublicID),
		Reason:           row.Reason,
		Status:           row.Status,
		Attempts:         row.Attempts,
		IndexedCount:     row.IndexedCount,
		SkippedCount:     row.SkippedCount,
		FailedCount:      row.FailedCount,
		LastError:        optionalText(row.LastError),
		StartedAt:        optionalPgTime(row.StartedAt),
		CompletedAt:      optionalPgTime(row.CompletedAt),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

func interfaceString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(value)
	}
}
