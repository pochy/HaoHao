package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DriveOCRRun struct {
	ID                  int64
	PublicID            string
	FilePublicID        string
	TenantID            int64
	FileObjectID        int64
	FileRevision        string
	ContentSHA256       string
	Engine              string
	Languages           []string
	StructuredExtractor string
	Status              string
	Reason              string
	PageCount           int
	ProcessedPageCount  int
	AverageConfidence   *float64
	ExtractedText       string
	ErrorCode           string
	ErrorMessage        string
	StartedAt           *time.Time
	CompletedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type DriveOCRPage struct {
	PageNumber        int
	RawText           string
	AverageConfidence *float64
	LayoutJSON        []byte
	BoxesJSON         []byte
	CreatedAt         time.Time
}

type DriveOCRResult struct {
	Run   DriveOCRRun
	Pages []DriveOCRPage
}

type DriveProductExtractionItem struct {
	PublicID     string
	TenantID     int64
	FileObjectID int64
	FilePublicID string
	ItemType     string
	Name         string
	Brand        string
	Manufacturer string
	Model        string
	SKU          string
	JANCode      string
	Category     string
	Description  string
	Price        map[string]any
	Promotion    map[string]any
	Availability map[string]any
	SourceText   string
	Evidence     []map[string]any
	Attributes   map[string]any
	Confidence   *float64
	CreatedAt    time.Time
}

type DriveOCRService struct {
	pool           *pgxpool.Pool
	queries        *db.Queries
	drive          *DriveService
	storage        FileStorage
	tenantSettings *TenantSettingsService
	audit          AuditRecorder
	provider       DriveOCRProvider
	extractor      DriveProductExtractor
}

func NewDriveOCRService(pool *pgxpool.Pool, queries *db.Queries, drive *DriveService, storage FileStorage, tenantSettings *TenantSettingsService, audit AuditRecorder, provider DriveOCRProvider, extractor DriveProductExtractor) *DriveOCRService {
	if storage == nil && drive != nil {
		storage = drive.storage
	}
	if provider == nil {
		p := NewLocalDriveOCRProvider()
		provider = p
	}
	if extractor == nil {
		extractor = NewDriveProductExtractorRouter(
			NewRulesDriveProductExtractor(),
			NewOllamaDriveProductExtractor(nil),
			NewLMStudioDriveProductExtractor(nil),
			DefaultLocalCommandDriveProductExtractors()...,
		)
	}
	return &DriveOCRService{
		pool:           pool,
		queries:        queries,
		drive:          drive,
		storage:        storage,
		tenantSettings: tenantSettings,
		audit:          audit,
		provider:       provider,
		extractor:      extractor,
	}
}

func (s *DriveOCRService) RequestJob(ctx context.Context, tenantID, actorUserID int64, filePublicID, reason string, auditCtx AuditContext) (DriveOCRRun, error) {
	if err := s.ensureConfigured(); err != nil {
		return DriveOCRRun{}, err
	}
	if reason == "" {
		reason = "manual"
	}
	actor, file, err := s.fileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveOCRRun{}, err
	}
	if err := s.drive.authz.CanEditFile(ctx, actor, file); err != nil {
		s.drive.auditDenied(ctx, actor, "drive.ocr.job.create", "drive_file", file.PublicID, err, auditCtx)
		return DriveOCRRun{}, err
	}
	policy, err := s.driveOCRPolicy(ctx, tenantID)
	if err != nil {
		return DriveOCRRun{}, err
	}
	if !policy.Enabled {
		return DriveOCRRun{}, ErrDrivePolicyDenied
	}
	run, err := s.createRun(ctx, file, policy, actorUserID, reason, 0)
	if err != nil {
		return DriveOCRRun{}, err
	}
	if run.Status != "completed" && s.drive.outbox != nil {
		event, err := s.enqueue(ctx, file, actorUserID, reason)
		if err != nil {
			return DriveOCRRun{}, err
		}
		row, err := s.queries.LinkDriveOCRRunOutboxEvent(ctx, db.LinkDriveOCRRunOutboxEventParams{
			OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
			ID:            run.ID,
			TenantID:      tenantID,
		})
		if err == nil {
			run = driveOCRRunFromDB(row, file.PublicID)
		}
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.ocr.job.create", file.PublicID, map[string]any{"reason": reason})
	return run, nil
}

func (s *DriveOCRService) HandleRequested(ctx context.Context, tenantID, fileObjectID, actorUserID int64, reason string, outboxEventID int64) error {
	if err := s.ensureConfigured(); err != nil {
		return err
	}
	row, err := s.drive.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, ID: fileObjectID})
	if errors.Is(err, ErrDriveNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	file := driveFileFromDB(row)
	policy, err := s.driveOCRPolicy(ctx, tenantID)
	if err != nil {
		return err
	}
	run, err := s.createRun(ctx, file, policy, actorUserID, reason, outboxEventID)
	if err != nil {
		return err
	}
	if run.Status == "completed" {
		return s.indexOCRText(ctx, file, run.ExtractedText)
	}
	if code, message := s.skipReason(ctx, file, policy); code != "" {
		_, err := s.queries.MarkDriveOCRRunSkipped(ctx, db.MarkDriveOCRRunSkippedParams{
			ErrorCode:    pgtype.Text{String: code, Valid: true},
			ErrorMessage: message,
			ID:           run.ID,
			TenantID:     tenantID,
		})
		return err
	}
	if _, err := s.queries.MarkDriveOCRRunRunning(ctx, db.MarkDriveOCRRunRunningParams{ID: run.ID, TenantID: tenantID}); err != nil {
		return err
	}
	body, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return s.markRunFailed(ctx, run, "storage_open_failed", err)
	}
	defer body.Close()
	result, err := s.provider.Extract(ctx, DriveOCRProviderInput{TenantID: tenantID, File: file, Body: body, Policy: policy})
	if err != nil {
		if isDriveOCRUnsupported(err) {
			_, markErr := s.queries.MarkDriveOCRRunSkipped(ctx, db.MarkDriveOCRRunSkippedParams{
				ErrorCode:    pgtype.Text{String: "unsupported_file", Valid: true},
				ErrorMessage: trimOCRProcessError(err),
				ID:           run.ID,
				TenantID:     tenantID,
			})
			return markErr
		}
		return s.markRunFailed(ctx, run, "provider_failed", err)
	}
	if err := s.replacePages(ctx, run, file, result.Pages); err != nil {
		return s.markRunFailed(ctx, run, "save_pages_failed", err)
	}
	completed, err := s.queries.MarkDriveOCRRunCompleted(ctx, db.MarkDriveOCRRunCompletedParams{
		PageCount:          int32(len(result.Pages)),
		ProcessedPageCount: int32(len(result.Pages)),
		AverageConfidence:  pgNumericFromFloat(result.AverageConfidence),
		ExtractedText:      result.FullText,
		ID:                 run.ID,
		TenantID:           tenantID,
	})
	if err != nil {
		return err
	}
	run = driveOCRRunFromDB(completed, file.PublicID)
	if err := s.replaceProductItems(ctx, policy, run, file, result); err != nil {
		return s.markRunFailed(ctx, run, "product_extraction_failed", err)
	}
	if err := s.indexOCRText(ctx, file, result.FullText); err != nil {
		return err
	}
	return nil
}

func (s *DriveOCRService) GetLatest(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext) (DriveOCRResult, error) {
	if err := s.ensureConfigured(); err != nil {
		return DriveOCRResult{}, err
	}
	actor, file, err := s.fileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveOCRResult{}, err
	}
	if err := s.drive.authz.CanViewFile(ctx, actor, file); err != nil {
		s.drive.auditDenied(ctx, actor, "drive.ocr.view", "drive_file", file.PublicID, err, auditCtx)
		return DriveOCRResult{}, err
	}
	row, err := s.queries.GetLatestDriveOCRRunForFile(ctx, db.GetLatestDriveOCRRunForFileParams{TenantID: tenantID, FileObjectID: file.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveOCRResult{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveOCRResult{}, fmt.Errorf("get latest drive ocr run: %w", err)
	}
	pageRows, err := s.queries.ListDriveOCRPages(ctx, db.ListDriveOCRPagesParams{TenantID: tenantID, OcrRunID: row.ID})
	if err != nil {
		return DriveOCRResult{}, fmt.Errorf("list drive ocr pages: %w", err)
	}
	pages := make([]DriveOCRPage, 0, len(pageRows))
	for _, page := range pageRows {
		pages = append(pages, driveOCRPageFromDB(page))
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.ocr.view", file.PublicID, nil)
	return DriveOCRResult{Run: driveOCRRunFromDB(row, file.PublicID), Pages: pages}, nil
}

func (s *DriveOCRService) ListProductExtractions(ctx context.Context, tenantID, actorUserID int64, filePublicID string) ([]DriveProductExtractionItem, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}
	actor, file, err := s.fileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return nil, err
	}
	if err := s.drive.authz.CanViewFile(ctx, actor, file); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDriveProductExtractionItems(ctx, db.ListDriveProductExtractionItemsParams{
		TenantID:     tenantID,
		FileObjectID: file.ID,
		LimitCount:   100,
	})
	if err != nil {
		return nil, fmt.Errorf("list drive product extraction items: %w", err)
	}
	items := make([]DriveProductExtractionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, driveProductExtractionItemFromDB(row, file.PublicID))
	}
	return items, nil
}

func (s *DriveOCRService) RequestProductExtraction(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext) (DriveOCRRun, []DriveProductExtractionItem, error) {
	if err := s.ensureConfigured(); err != nil {
		return DriveOCRRun{}, nil, err
	}
	actor, file, err := s.fileForActor(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveOCRRun{}, nil, err
	}
	if err := s.drive.authz.CanEditFile(ctx, actor, file); err != nil {
		s.drive.auditDenied(ctx, actor, "drive.product_extraction.job.create", "drive_file", file.PublicID, err, auditCtx)
		return DriveOCRRun{}, nil, err
	}
	policy, err := s.driveOCRPolicy(ctx, tenantID)
	if err != nil {
		return DriveOCRRun{}, nil, err
	}
	if !policy.Enabled || !policy.StructuredExtractionEnabled {
		return DriveOCRRun{}, nil, ErrDrivePolicyDenied
	}
	row, err := s.queries.GetLatestDriveOCRRunForFile(ctx, db.GetLatestDriveOCRRunForFileParams{TenantID: tenantID, FileObjectID: file.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveOCRRun{}, nil, fmt.Errorf("%w: OCR must complete before product extraction", ErrDriveInvalidInput)
	}
	if err != nil {
		return DriveOCRRun{}, nil, fmt.Errorf("get latest drive ocr run: %w", err)
	}
	run := driveOCRRunFromDB(row, file.PublicID)
	if run.Status != "completed" {
		return DriveOCRRun{}, nil, fmt.Errorf("%w: OCR must complete before product extraction", ErrDriveInvalidInput)
	}
	pageRows, err := s.queries.ListDriveOCRPages(ctx, db.ListDriveOCRPagesParams{TenantID: tenantID, OcrRunID: row.ID})
	if err != nil {
		return DriveOCRRun{}, nil, fmt.Errorf("list drive ocr pages: %w", err)
	}
	pages := make([]DriveOCRPageResult, 0, len(pageRows))
	for _, page := range pageRows {
		pages = append(pages, DriveOCRPageResult{
			PageNumber:        int(page.PageNumber),
			RawText:           page.RawText,
			AverageConfidence: floatFromPgNumeric(page.AverageConfidence),
			LayoutJSON:        append([]byte{}, page.LayoutJson...),
			BoxesJSON:         append([]byte{}, page.BoxesJson...),
		})
	}
	if err := s.replaceProductItems(ctx, policy, run, file, DriveOCRProviderResult{Pages: pages, FullText: run.ExtractedText, AverageConfidence: run.AverageConfidence}); err != nil {
		return DriveOCRRun{}, nil, fmt.Errorf("extract drive product items: %w", err)
	}
	items, err := s.ListProductExtractions(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return DriveOCRRun{}, nil, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.product_extraction.job.create", file.PublicID, map[string]any{"ocrRunPublicId": run.PublicID})
	return run, items, nil
}

func (s *DriveOCRService) RuntimeStatus(ctx context.Context, tenantID int64) (DriveOCRRuntimeStatus, error) {
	policy, err := s.driveOCRPolicy(ctx, tenantID)
	if err != nil {
		return DriveOCRRuntimeStatus{}, err
	}
	status := DriveOCRRuntimeStatus{
		Enabled:             policy.Enabled,
		OCREngine:           policy.OCREngine,
		StructuredExtractor: policy.StructuredExtractor,
		StatusCounts:        map[string]int64{},
	}
	if s.provider != nil {
		status.Dependencies = s.provider.Check(ctx, policy)
	}
	if policy.StructuredExtractor == "ollama" || strings.TrimSpace(policy.OllamaModel) != "" {
		status.Ollama = CheckDriveOCROllama(ctx, policy)
	}
	if policy.StructuredExtractor == "lmstudio" || strings.TrimSpace(policy.LMStudioModel) != "" {
		status.LMStudio = CheckDriveOCRLMStudio(ctx, policy)
	}
	status.LocalCommands = CheckDriveOCRLocalCommands(ctx, policy)
	if s.queries != nil {
		rows, err := s.queries.CountDriveOCRRunsByStatus(ctx, tenantID)
		if err != nil {
			return DriveOCRRuntimeStatus{}, err
		}
		for _, row := range rows {
			status.StatusCounts[row.Status] = row.Count
		}
	}
	return status, nil
}

func (s *DriveOCRService) createRun(ctx context.Context, file DriveFile, policy DriveOCRPolicy, actorUserID int64, reason string, outboxEventID int64) (DriveOCRRun, error) {
	if reason == "" {
		reason = "upload"
	}
	row, err := s.queries.CreateDriveOCRRun(ctx, db.CreateDriveOCRRunParams{
		TenantID:            file.TenantID,
		FileObjectID:        file.ID,
		FileRevision:        fileOCRRevision(file),
		ContentSha256:       file.SHA256Hex,
		Engine:              policy.OCREngine,
		Languages:           policy.OCRLanguages,
		StructuredExtractor: policy.StructuredExtractor,
		Reason:              reason,
		RequestedByUserID:   pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
		OutboxEventID:       pgtype.Int8{Int64: outboxEventID, Valid: outboxEventID > 0},
	})
	if err != nil {
		return DriveOCRRun{}, fmt.Errorf("create drive ocr run: %w", err)
	}
	return driveOCRRunFromDB(row, file.PublicID), nil
}

func (s *DriveOCRService) enqueue(ctx context.Context, file DriveFile, actorUserID int64, reason string) (db.OutboxEvent, error) {
	if s.drive == nil || s.drive.outbox == nil {
		return db.OutboxEvent{}, fmt.Errorf("drive ocr outbox is not configured")
	}
	tenantID := file.TenantID
	return s.drive.outbox.Enqueue(ctx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "drive_file",
		AggregateID:   file.PublicID,
		EventType:     "drive.ocr.requested",
		Payload: map[string]any{
			"tenantId":     file.TenantID,
			"fileObjectId": file.ID,
			"filePublicId": file.PublicID,
			"actorUserId":  actorUserID,
			"reason":       reason,
		},
	})
}

func (s *DriveOCRService) replacePages(ctx context.Context, run DriveOCRRun, file DriveFile, pages []DriveOCRPageResult) error {
	if err := s.queries.DeleteDriveOCRPagesForRun(ctx, db.DeleteDriveOCRPagesForRunParams{TenantID: file.TenantID, OcrRunID: run.ID}); err != nil {
		return err
	}
	for _, page := range pages {
		layout := page.LayoutJSON
		if len(layout) == 0 {
			layout = []byte("{}")
		}
		boxes := page.BoxesJSON
		if len(boxes) == 0 {
			boxes = []byte("[]")
		}
		if _, err := s.queries.UpsertDriveOCRPage(ctx, db.UpsertDriveOCRPageParams{
			TenantID:          file.TenantID,
			OcrRunID:          run.ID,
			FileObjectID:      file.ID,
			PageNumber:        int32(page.PageNumber),
			RawText:           page.RawText,
			AverageConfidence: pgNumericFromFloat(page.AverageConfidence),
			LayoutJson:        layout,
			BoxesJson:         boxes,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *DriveOCRService) replaceProductItems(ctx context.Context, policy DriveOCRPolicy, run DriveOCRRun, file DriveFile, result DriveOCRProviderResult) error {
	if !policy.StructuredExtractionEnabled {
		return nil
	}
	switch policy.StructuredExtractor {
	case "rules", "ollama", "lmstudio", "gemini", "codex", "claude":
	default:
		return nil
	}
	if s.extractor == nil {
		return nil
	}
	extracted, err := s.extractor.ExtractProducts(ctx, DriveProductExtractionInput{
		TenantID: file.TenantID,
		File:     file,
		Run:      run,
		Pages:    result.Pages,
		FullText: result.FullText,
		Policy:   policy,
	})
	if err != nil {
		return err
	}
	if err := s.queries.DeleteDriveProductExtractionItemsForRun(ctx, db.DeleteDriveProductExtractionItemsForRunParams{TenantID: file.TenantID, OcrRunID: run.ID}); err != nil {
		return err
	}
	for _, item := range extracted.Items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		if _, err := s.queries.CreateDriveProductExtractionItem(ctx, db.CreateDriveProductExtractionItemParams{
			TenantID:     file.TenantID,
			OcrRunID:     run.ID,
			FileObjectID: file.ID,
			ItemType:     defaultString(item.ItemType, "unknown"),
			Name:         strings.TrimSpace(item.Name),
			Brand:        pgText(item.Brand),
			Manufacturer: pgText(item.Manufacturer),
			Model:        pgText(item.Model),
			Sku:          pgText(item.SKU),
			JanCode:      pgText(item.JANCode),
			Category:     pgText(item.Category),
			Description:  pgText(item.Description),
			Price:        jsonBytesOrEmptyObject(item.Price),
			Promotion:    jsonBytesOrEmptyObject(item.Promotion),
			Availability: jsonBytesOrEmptyObject(item.Availability),
			SourceText:   item.SourceText,
			Evidence:     jsonBytesOrEmptyArray(item.Evidence),
			Attributes:   jsonBytesOrEmptyObject(item.Attributes),
			Confidence:   pgNumericFromFloat(item.Confidence),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *DriveOCRService) indexOCRText(ctx context.Context, file DriveFile, text string) error {
	if s.queries == nil {
		return nil
	}
	if text == "" {
		return nil
	}
	snippet := searchSnippet(text, file.OriginalFilename)
	_, err := s.queries.UpsertDriveSearchDocument(ctx, db.UpsertDriveSearchDocumentParams{
		TenantID:        file.TenantID,
		WorkspaceID:     pgInt8(file.WorkspaceID),
		FileObjectID:    file.ID,
		Title:           file.OriginalFilename,
		ContentType:     file.ContentType,
		ExtractedText:   sanitizeSearchText(text),
		Snippet:         snippet,
		ContentSha256:   pgText(file.SHA256Hex),
		ObjectUpdatedAt: pgtype.Timestamptz{Time: file.UpdatedAt, Valid: !file.UpdatedAt.IsZero()},
	})
	if err != nil {
		return fmt.Errorf("upsert drive search document with ocr: %w", err)
	}
	return nil
}

func (s *DriveOCRService) markRunFailed(ctx context.Context, run DriveOCRRun, code string, cause error) error {
	_, err := s.queries.MarkDriveOCRRunFailed(ctx, db.MarkDriveOCRRunFailedParams{
		ErrorCode:    pgtype.Text{String: code, Valid: true},
		ErrorMessage: trimOCRProcessError(cause),
		ID:           run.ID,
		TenantID:     run.TenantID,
	})
	if err != nil {
		return err
	}
	return cause
}

func (s *DriveOCRService) skipReason(ctx context.Context, file DriveFile, policy DriveOCRPolicy) (string, string) {
	if !policy.Enabled {
		return "policy_disabled", "drive ocr is disabled"
	}
	if file.DeletedAt != nil {
		return "file_deleted", "file is deleted"
	}
	if file.ScanStatus == "infected" || file.ScanStatus == "blocked" {
		return "scan_blocked", "file scan status blocks ocr"
	}
	if file.DLPBlocked {
		return "dlp_blocked", "file is dlp blocked"
	}
	if s.drive != nil && s.drive.drivePhase9IsZeroKnowledge(ctx, file.TenantID, file.ID) {
		return "zero_knowledge", "zero-knowledge encrypted files are not readable by local ocr"
	}
	if policy.OCREngine != "tesseract" {
		return "engine_not_configured", "optional ocr engine is not configured in this runtime"
	}
	if !driveOCRSupportedFile(file) {
		return "unsupported_file", "file type is not supported by drive ocr"
	}
	return "", ""
}

func driveOCRSupportedFile(file DriveFile) bool {
	contentType := strings.ToLower(strings.TrimSpace(file.ContentType))
	ext := strings.ToLower(strings.TrimSpace(fileOCRExt(file.OriginalFilename)))
	if contentType == "application/pdf" || strings.HasPrefix(contentType, "text/") || strings.Contains(contentType, "json") || strings.Contains(contentType, "xml") || strings.HasPrefix(contentType, "image/") {
		return true
	}
	switch ext {
	case ".pdf", ".png", ".jpg", ".jpeg", ".tif", ".tiff", ".webp", ".txt", ".md", ".csv", ".json", ".xml", ".log":
		return true
	default:
		return false
	}
}

func fileOCRExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}

func (s *DriveOCRService) fileForActor(ctx context.Context, tenantID, actorUserID int64, filePublicID string) (DriveActor, DriveFile, error) {
	if s.drive == nil {
		return DriveActor{}, DriveFile{}, fmt.Errorf("drive service is not configured")
	}
	actor, err := s.drive.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveActor{}, DriveFile{}, err
	}
	row, err := s.drive.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return DriveActor{}, DriveFile{}, err
	}
	return actor, driveFileFromDB(row), nil
}

func (s *DriveOCRService) driveOCRPolicy(ctx context.Context, tenantID int64) (DriveOCRPolicy, error) {
	if s.tenantSettings == nil {
		return defaultDriveOCRPolicy(), nil
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, tenantID)
	if err != nil {
		return DriveOCRPolicy{}, err
	}
	return policy.OCR, nil
}

func (s *DriveOCRService) ensureConfigured() error {
	if s == nil || s.queries == nil || s.drive == nil || s.storage == nil {
		return fmt.Errorf("drive ocr service is not configured")
	}
	return nil
}

func (s *DriveOCRService) recordAuditBestEffort(ctx context.Context, actor DriveActor, auditCtx AuditContext, action, targetID string, metadata map[string]any) {
	if s.drive == nil {
		return
	}
	s.drive.recordAuditBestEffort(ctx, actor, auditCtx, action, "drive_file", targetID, metadata)
}

func driveOCRRunFromDB(row db.DriveOcrRun, filePublicID string) DriveOCRRun {
	return DriveOCRRun{
		ID:                  row.ID,
		PublicID:            row.PublicID.String(),
		FilePublicID:        filePublicID,
		TenantID:            row.TenantID,
		FileObjectID:        row.FileObjectID,
		FileRevision:        row.FileRevision,
		ContentSHA256:       row.ContentSha256,
		Engine:              row.Engine,
		Languages:           append([]string{}, row.Languages...),
		StructuredExtractor: row.StructuredExtractor,
		Status:              row.Status,
		Reason:              row.Reason,
		PageCount:           int(row.PageCount),
		ProcessedPageCount:  int(row.ProcessedPageCount),
		AverageConfidence:   floatFromPgNumeric(row.AverageConfidence),
		ExtractedText:       row.ExtractedText,
		ErrorCode:           optionalText(row.ErrorCode),
		ErrorMessage:        optionalText(row.ErrorMessage),
		StartedAt:           optionalPgTime(row.StartedAt),
		CompletedAt:         optionalPgTime(row.CompletedAt),
		CreatedAt:           row.CreatedAt.Time,
		UpdatedAt:           row.UpdatedAt.Time,
	}
}

func driveOCRPageFromDB(row db.DriveOcrPage) DriveOCRPage {
	return DriveOCRPage{
		PageNumber:        int(row.PageNumber),
		RawText:           row.RawText,
		AverageConfidence: floatFromPgNumeric(row.AverageConfidence),
		LayoutJSON:        append([]byte{}, row.LayoutJson...),
		BoxesJSON:         append([]byte{}, row.BoxesJson...),
		CreatedAt:         row.CreatedAt.Time,
	}
}

func driveProductExtractionItemFromDB(row db.DriveProductExtractionItem, filePublicID string) DriveProductExtractionItem {
	return DriveProductExtractionItem{
		PublicID:     row.PublicID.String(),
		TenantID:     row.TenantID,
		FileObjectID: row.FileObjectID,
		FilePublicID: filePublicID,
		ItemType:     row.ItemType,
		Name:         row.Name,
		Brand:        optionalText(row.Brand),
		Manufacturer: optionalText(row.Manufacturer),
		Model:        optionalText(row.Model),
		SKU:          optionalText(row.Sku),
		JANCode:      optionalText(row.JanCode),
		Category:     optionalText(row.Category),
		Description:  optionalText(row.Description),
		Price:        jsonObjectFromBytes(row.Price),
		Promotion:    jsonObjectFromBytes(row.Promotion),
		Availability: jsonObjectFromBytes(row.Availability),
		SourceText:   row.SourceText,
		Evidence:     jsonArrayObjectsFromBytes(row.Evidence),
		Attributes:   jsonObjectFromBytes(row.Attributes),
		Confidence:   floatFromPgNumeric(row.Confidence),
		CreatedAt:    row.CreatedAt.Time,
	}
}

func fileOCRRevision(file DriveFile) string {
	if file.SHA256Hex != "" {
		return file.SHA256Hex
	}
	return filePhase9Revision(file)
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func pgNumericFromFloat(value *float64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}
	scaled := int64(math.Round(*value * 10000))
	return pgtype.Numeric{Int: big.NewInt(scaled), Exp: -4, Valid: true}
}

func floatFromPgNumeric(value pgtype.Numeric) *float64 {
	if !value.Valid || value.Int == nil {
		return nil
	}
	f, _ := new(big.Float).SetInt(value.Int).Float64()
	if value.Exp < 0 {
		f = f / math.Pow10(int(-value.Exp))
	} else if value.Exp > 0 {
		f = f * math.Pow10(int(value.Exp))
	}
	return &f
}

func jsonObjectFromBytes(data []byte) map[string]any {
	out := map[string]any{}
	_ = jsonUnmarshal(data, &out)
	return out
}

func jsonArrayObjectsFromBytes(data []byte) []map[string]any {
	out := []map[string]any{}
	_ = jsonUnmarshal(data, &out)
	return out
}

func jsonUnmarshal(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}

func parseDriveOCRRunPublicID(value string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(value))
}
