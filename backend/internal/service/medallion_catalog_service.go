package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	MedallionLayerBronze = "bronze"
	MedallionLayerSilver = "silver"
	MedallionLayerGold   = "gold"

	MedallionResourceDriveFile         = "drive_file"
	MedallionResourceDataset           = "dataset"
	MedallionResourceWorkTable         = "work_table"
	MedallionResourceOCRRun            = "ocr_run"
	MedallionResourceProductExtraction = "product_extraction"
	MedallionResourceGoldTable         = "gold_table"

	MedallionAssetStatusActive   = "active"
	MedallionAssetStatusBuilding = "building"
	MedallionAssetStatusFailed   = "failed"
	MedallionAssetStatusSkipped  = "skipped"
	MedallionAssetStatusArchived = "archived"

	MedallionPipelineDatasetImport     = "dataset_import"
	MedallionPipelineWorkTableRegister = "work_table_register"
	MedallionPipelineWorkTablePromote  = "work_table_promote"
	MedallionPipelineDatasetSync       = "dataset_sync"
	MedallionPipelineDriveOCR          = "drive_ocr"
	MedallionPipelineProductExtraction = "product_extraction"
	MedallionPipelineGoldPublish       = "gold_publish"

	MedallionPipelineStatusPending    = "pending"
	MedallionPipelineStatusProcessing = "processing"
	MedallionPipelineStatusCompleted  = "completed"
	MedallionPipelineStatusFailed     = "failed"
	MedallionPipelineStatusSkipped    = "skipped"

	MedallionTriggerManual     = "manual"
	MedallionTriggerUpload     = "upload"
	MedallionTriggerScheduled  = "scheduled"
	MedallionTriggerSystem     = "system"
	MedallionTriggerReadRepair = "read_repair"
)

var ErrMedallionAssetNotFound = errors.New("medallion asset not found")

type MedallionAsset struct {
	ID               int64
	PublicID         string
	TenantID         int64
	Layer            string
	ResourceKind     string
	ResourceID       int64
	ResourcePublicID string
	DisplayName      string
	Status           string
	RowCount         *int64
	ByteSize         *int64
	SchemaSummary    map[string]any
	Metadata         map[string]any
	CreatedByUserID  *int64
	UpdatedByUserID  *int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ArchivedAt       *time.Time
}

type MedallionPipelineRun struct {
	ID                     int64
	PublicID               string
	TenantID               int64
	PipelineType           string
	Status                 string
	Runtime                string
	TriggerKind            string
	Retryable              bool
	ErrorSummary           string
	Metadata               map[string]any
	RequestedByUserID      *int64
	StartedAt              *time.Time
	CompletedAt            *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
	SourceAssetPublicIDs   []string
	TargetAssetPublicIDs   []string
	SourceResourceKind     string
	SourceResourcePublicID string
	TargetResourceKind     string
	TargetResourcePublicID string
}

type MedallionCatalog struct {
	Asset        *MedallionAsset
	Upstream     []MedallionAsset
	Downstream   []MedallionAsset
	PipelineRuns []MedallionPipelineRun
}

type MedallionCatalogService struct {
	queries  *db.Queries
	drive    *DriveService
	datasets *DatasetService
}

type medallionPipelineRunInput struct {
	TenantID               int64
	PipelineType           string
	RunKey                 string
	SourceResourceKind     string
	SourceResourceID       int64
	SourceResourcePublicID string
	TargetResourceKind     string
	TargetResourceID       int64
	TargetResourcePublicID string
	Status                 string
	Runtime                string
	TriggerKind            string
	Retryable              bool
	ErrorSummary           string
	Metadata               map[string]any
	RequestedByUserID      *int64
	StartedAt              *time.Time
	CompletedAt            *time.Time
	SourceAssets           []MedallionAsset
	TargetAssets           []MedallionAsset
}

func NewMedallionCatalogService(queries *db.Queries, drive *DriveService, datasets *DatasetService) *MedallionCatalogService {
	return &MedallionCatalogService{queries: queries, drive: drive, datasets: datasets}
}

func (s *MedallionCatalogService) ListAssets(ctx context.Context, tenantID, actorUserID int64, layer, resourceKind string, limit int32) ([]MedallionAsset, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	layer = strings.TrimSpace(layer)
	resourceKind = strings.TrimSpace(resourceKind)
	rows, err := s.queries.ListMedallionAssets(ctx, db.ListMedallionAssetsParams{
		TenantID:     tenantID,
		Layer:        pgText(layer),
		ResourceKind: pgText(resourceKind),
		LimitCount:   limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list medallion assets: %w", err)
	}
	items := make([]MedallionAsset, 0, len(rows))
	for _, row := range rows {
		item := medallionAssetFromDB(row)
		ok, err := s.canViewAsset(ctx, tenantID, actorUserID, item)
		if err != nil {
			return nil, err
		}
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *MedallionCatalogService) GetAssetCatalog(ctx context.Context, tenantID, actorUserID int64, assetPublicID string) (MedallionCatalog, error) {
	if err := s.ensureConfigured(); err != nil {
		return MedallionCatalog{}, err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(assetPublicID))
	if err != nil {
		return MedallionCatalog{}, ErrMedallionAssetNotFound
	}
	row, err := s.queries.GetMedallionAssetByPublicIDForTenant(ctx, db.GetMedallionAssetByPublicIDForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return MedallionCatalog{}, ErrMedallionAssetNotFound
	}
	if err != nil {
		return MedallionCatalog{}, fmt.Errorf("get medallion asset: %w", err)
	}
	asset := medallionAssetFromDB(row)
	ok, err := s.canViewAsset(ctx, tenantID, actorUserID, asset)
	if err != nil {
		return MedallionCatalog{}, err
	}
	if !ok {
		return MedallionCatalog{}, ErrDrivePermissionDenied
	}
	return s.catalogForAsset(ctx, tenantID, asset)
}

func (s *MedallionCatalogService) GetResourceCatalog(ctx context.Context, tenantID, actorUserID int64, resourceKind, resourcePublicID string) (MedallionCatalog, error) {
	if err := s.ensureConfigured(); err != nil {
		return MedallionCatalog{}, err
	}
	asset, ok, err := s.ensureResourceAsset(ctx, tenantID, actorUserID, strings.TrimSpace(resourceKind), strings.TrimSpace(resourcePublicID), MedallionTriggerReadRepair)
	if err != nil || !ok {
		return MedallionCatalog{}, err
	}
	return s.catalogForAsset(ctx, tenantID, asset)
}

func (s *MedallionCatalogService) EnsureDriveFileAsset(ctx context.Context, file DriveFile, actorUserID *int64) (MedallionAsset, bool, error) {
	if err := s.ensureConfigured(); err != nil {
		return MedallionAsset{}, false, err
	}
	if !medallionDriveFileEligible(file) {
		return MedallionAsset{}, false, nil
	}
	status := MedallionAssetStatusActive
	if file.DeletedAt != nil || file.Status == "deleted" {
		status = MedallionAssetStatusArchived
	} else if file.DLPBlocked || file.ScanStatus == "infected" || file.ScanStatus == "blocked" || file.Status == "blocked" {
		status = MedallionAssetStatusFailed
	}
	asset, err := s.upsertAsset(ctx, medallionAssetInput{
		TenantID:         file.TenantID,
		Layer:            MedallionLayerBronze,
		ResourceKind:     MedallionResourceDriveFile,
		ResourceID:       file.ID,
		ResourcePublicID: file.PublicID,
		DisplayName:      file.OriginalFilename,
		Status:           status,
		ByteSize:         &file.ByteSize,
		Metadata: map[string]any{
			"contentType": file.ContentType,
			"sha256":      file.SHA256Hex,
			"scanStatus":  file.ScanStatus,
			"dlpBlocked":  file.DLPBlocked,
		},
		CreatedByUserID: file.UploadedByUserID,
		UpdatedByUserID: actorUserID,
	})
	return asset, true, err
}

func (s *MedallionCatalogService) EnsureDatasetAsset(ctx context.Context, dataset Dataset, actorUserID *int64) (MedallionAsset, error) {
	if err := s.ensureConfigured(); err != nil {
		return MedallionAsset{}, err
	}
	layer := medallionDatasetLayer(dataset)
	status := medallionAssetStatusFromDatasetStatus(dataset.Status)
	metadata := map[string]any{
		"sourceKind":  dataset.SourceKind,
		"contentType": dataset.ContentType,
		"rawDatabase": dataset.RawDatabase,
		"rawTable":    dataset.RawTable,
	}
	if dataset.SourceWorkTablePublicID != "" {
		metadata["sourceWorkTablePublicId"] = dataset.SourceWorkTablePublicID
	}
	asset, err := s.upsertAsset(ctx, medallionAssetInput{
		TenantID:         dataset.TenantID,
		Layer:            layer,
		ResourceKind:     MedallionResourceDataset,
		ResourceID:       dataset.ID,
		ResourcePublicID: dataset.PublicID,
		DisplayName:      dataset.Name,
		Status:           status,
		RowCount:         &dataset.RowCount,
		ByteSize:         &dataset.ByteSize,
		SchemaSummary:    medallionDatasetSchemaSummary(dataset.Columns),
		Metadata:         metadata,
		CreatedByUserID:  dataset.CreatedByUserID,
		UpdatedByUserID:  actorUserID,
	})
	return asset, err
}

func (s *MedallionCatalogService) EnsureWorkTableAsset(ctx context.Context, table DatasetWorkTable, actorUserID *int64) (MedallionAsset, error) {
	if err := s.ensureConfigured(); err != nil {
		return MedallionAsset{}, err
	}
	status := MedallionAssetStatusActive
	if table.Status == "dropped" || table.DroppedAt != nil {
		status = MedallionAssetStatusArchived
	}
	return s.upsertAsset(ctx, medallionAssetInput{
		TenantID:         table.TenantID,
		Layer:            MedallionLayerSilver,
		ResourceKind:     MedallionResourceWorkTable,
		ResourceID:       table.ID,
		ResourcePublicID: table.PublicID,
		DisplayName:      table.DisplayName,
		Status:           status,
		RowCount:         &table.TotalRows,
		ByteSize:         &table.TotalBytes,
		SchemaSummary:    medallionWorkTableSchemaSummary(table.Columns),
		Metadata: map[string]any{
			"database": table.Database,
			"table":    table.Table,
			"engine":   table.Engine,
		},
		CreatedByUserID: table.CreatedByUserID,
		UpdatedByUserID: actorUserID,
	})
}

func (s *MedallionCatalogService) EnsureOCRRunAsset(ctx context.Context, file DriveFile, run DriveOCRRun, status string, actorUserID *int64) (MedallionAsset, error) {
	if err := s.ensureConfigured(); err != nil {
		return MedallionAsset{}, err
	}
	rowCount := int64(run.ProcessedPageCount)
	return s.upsertAsset(ctx, medallionAssetInput{
		TenantID:         run.TenantID,
		Layer:            MedallionLayerSilver,
		ResourceKind:     MedallionResourceOCRRun,
		ResourceID:       run.ID,
		ResourcePublicID: run.PublicID,
		DisplayName:      fmt.Sprintf("OCR: %s", file.OriginalFilename),
		Status:           medallionAssetStatusFromPipelineStatus(status),
		RowCount:         &rowCount,
		Metadata: map[string]any{
			"filePublicId":          file.PublicID,
			"engine":                run.Engine,
			"structuredExtractor":   run.StructuredExtractor,
			"artifactSchemaVersion": run.ArtifactSchemaVersion,
			"pipelineConfigHash":    run.PipelineConfigHash,
			"pageCount":             run.PageCount,
			"processedPageCount":    run.ProcessedPageCount,
			"reason":                run.Reason,
		},
		CreatedByUserID: run.RequestedByUserID,
		UpdatedByUserID: actorUserID,
	})
}

func (s *MedallionCatalogService) EnsureProductExtractionAsset(ctx context.Context, file DriveFile, item db.DriveProductExtractionItem, actorUserID *int64) (MedallionAsset, error) {
	if err := s.ensureConfigured(); err != nil {
		return MedallionAsset{}, err
	}
	return s.upsertAsset(ctx, medallionAssetInput{
		TenantID:         item.TenantID,
		Layer:            MedallionLayerSilver,
		ResourceKind:     MedallionResourceProductExtraction,
		ResourceID:       item.ID,
		ResourcePublicID: item.PublicID.String(),
		DisplayName:      item.Name,
		Status:           MedallionAssetStatusActive,
		Metadata: map[string]any{
			"filePublicId": file.PublicID,
			"itemType":     item.ItemType,
			"brand":        optionalText(item.Brand),
			"manufacturer": optionalText(item.Manufacturer),
			"model":        optionalText(item.Model),
			"sku":          optionalText(item.Sku),
			"janCode":      optionalText(item.JanCode),
			"category":     optionalText(item.Category),
		},
		UpdatedByUserID: actorUserID,
	})
}

func (s *MedallionCatalogService) EnsureGoldTableAsset(ctx context.Context, publication DatasetGoldPublication, actorUserID *int64) (MedallionAsset, error) {
	if err := s.ensureConfigured(); err != nil {
		return MedallionAsset{}, err
	}
	metadata := map[string]any{
		"goldDatabase":  publication.GoldDatabase,
		"goldTable":     publication.GoldTable,
		"refreshPolicy": publication.RefreshPolicy,
	}
	if publication.SourceWorkTablePublicID != "" {
		metadata["sourceWorkTablePublicId"] = publication.SourceWorkTablePublicID
	}
	return s.upsertAsset(ctx, medallionAssetInput{
		TenantID:         publication.TenantID,
		Layer:            MedallionLayerGold,
		ResourceKind:     MedallionResourceGoldTable,
		ResourceID:       publication.ID,
		ResourcePublicID: publication.PublicID,
		DisplayName:      publication.DisplayName,
		Status:           medallionAssetStatusFromGoldPublicationStatus(publication.Status),
		RowCount:         &publication.RowCount,
		ByteSize:         &publication.TotalBytes,
		SchemaSummary:    publication.SchemaSummary,
		Metadata:         metadata,
		CreatedByUserID:  publication.CreatedByUserID,
		UpdatedByUserID:  actorUserID,
	})
}

func (s *MedallionCatalogService) RecordPipelineRun(ctx context.Context, input medallionPipelineRunInput) (MedallionPipelineRun, error) {
	if err := s.ensureConfigured(); err != nil {
		return MedallionPipelineRun{}, err
	}
	if input.Status == "" {
		input.Status = MedallionPipelineStatusPending
	}
	if input.TriggerKind == "" {
		input.TriggerKind = MedallionTriggerSystem
	}
	if input.RunKey == "" {
		return MedallionPipelineRun{}, fmt.Errorf("medallion pipeline run key is required")
	}
	row, err := s.queries.UpsertMedallionPipelineRun(ctx, db.UpsertMedallionPipelineRunParams{
		TenantID:               input.TenantID,
		PipelineType:           input.PipelineType,
		RunKey:                 input.RunKey,
		SourceResourceKind:     pgText(input.SourceResourceKind),
		SourceResourceID:       pgInt8Value(input.SourceResourceID),
		SourceResourcePublicID: pgUUID(input.SourceResourcePublicID),
		TargetResourceKind:     pgText(input.TargetResourceKind),
		TargetResourceID:       pgInt8Value(input.TargetResourceID),
		TargetResourcePublicID: pgUUID(input.TargetResourcePublicID),
		Status:                 input.Status,
		Runtime:                input.Runtime,
		TriggerKind:            input.TriggerKind,
		Retryable:              input.Retryable,
		ErrorSummary:           pgText(input.ErrorSummary),
		Metadata:               jsonBytes(input.Metadata),
		RequestedByUserID:      pgInt8(input.RequestedByUserID),
		StartedAt:              pgTimestampPtr(input.StartedAt),
		CompletedAt:            pgTimestampPtr(input.CompletedAt),
	})
	if err != nil {
		return MedallionPipelineRun{}, fmt.Errorf("upsert medallion pipeline run: %w", err)
	}
	for _, asset := range input.SourceAssets {
		_, _ = s.queries.LinkMedallionPipelineRunAsset(ctx, db.LinkMedallionPipelineRunAssetParams{TenantID: input.TenantID, PipelineRunID: row.ID, AssetID: asset.ID, Role: "source"})
	}
	for _, asset := range input.TargetAssets {
		_, _ = s.queries.LinkMedallionPipelineRunAsset(ctx, db.LinkMedallionPipelineRunAssetParams{TenantID: input.TenantID, PipelineRunID: row.ID, AssetID: asset.ID, Role: "target"})
	}
	run := medallionPipelineRunFromDB(row)
	s.hydratePipelineRunAssetIDs(ctx, input.TenantID, &run)
	return run, nil
}

func (s *MedallionCatalogService) LinkAssets(ctx context.Context, tenantID int64, source, target MedallionAsset, relationType string, metadata map[string]any) {
	if s == nil || s.queries == nil || source.ID <= 0 || target.ID <= 0 || strings.TrimSpace(relationType) == "" {
		return
	}
	_, _ = s.queries.UpsertMedallionAssetEdge(ctx, db.UpsertMedallionAssetEdgeParams{
		TenantID:      tenantID,
		SourceAssetID: source.ID,
		TargetAssetID: target.ID,
		RelationType:  relationType,
		Metadata:      jsonBytes(metadata),
	})
}

func (s *MedallionCatalogService) ensureDatasetEdges(ctx context.Context, dataset Dataset, asset MedallionAsset) {
	if s == nil || s.queries == nil {
		return
	}
	if dataset.SourceKind == "file" && dataset.SourceFileObjectID != nil && s.drive != nil {
		row, err := s.drive.getDriveFileRow(ctx, dataset.TenantID, DriveResourceRef{Type: DriveResourceTypeFile, ID: *dataset.SourceFileObjectID})
		if err == nil {
			file := driveFileFromDB(row)
			if source, ok, err := s.EnsureDriveFileAsset(ctx, file, nil); err == nil && ok {
				s.LinkAssets(ctx, dataset.TenantID, source, asset, "source_file", nil)
			}
		}
		return
	}
	if dataset.SourceKind == "work_table" && dataset.SourceWorkTableID != nil && s.datasets != nil {
		row, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: *dataset.SourceWorkTableID, TenantID: dataset.TenantID})
		if err == nil {
			table := datasetWorkTableFromDB(row)
			if source, err := s.EnsureWorkTableAsset(ctx, table, nil); err == nil {
				s.LinkAssets(ctx, dataset.TenantID, source, asset, "promoted_dataset", nil)
			}
		}
	}
}

func (s *MedallionCatalogService) ensureWorkTableEdges(ctx context.Context, table DatasetWorkTable, asset MedallionAsset) {
	if s == nil || s.queries == nil || s.datasets == nil || table.SourceDatasetID == nil {
		return
	}
	row, err := s.queries.GetDatasetByIDForTenant(ctx, db.GetDatasetByIDForTenantParams{ID: *table.SourceDatasetID, TenantID: table.TenantID})
	if err != nil {
		return
	}
	dataset, err := s.datasets.inflateDataset(ctx, row)
	if err != nil {
		return
	}
	source, err := s.EnsureDatasetAsset(ctx, dataset, nil)
	if err != nil {
		return
	}
	s.LinkAssets(ctx, table.TenantID, source, asset, "source_dataset", nil)
}

func (s *MedallionCatalogService) ensureResourceAsset(ctx context.Context, tenantID, actorUserID int64, resourceKind, publicID, trigger string) (MedallionAsset, bool, error) {
	switch resourceKind {
	case MedallionResourceDriveFile:
		if s.drive == nil {
			return MedallionAsset{}, false, fmt.Errorf("drive service is not configured")
		}
		actor, file, err := s.driveFileForView(ctx, tenantID, actorUserID, publicID)
		if err != nil {
			return MedallionAsset{}, false, err
		}
		return s.EnsureDriveFileAsset(ctx, file, &actor.UserID)
	case MedallionResourceDataset:
		if s.datasets == nil {
			return MedallionAsset{}, false, fmt.Errorf("dataset service is not configured")
		}
		dataset, err := s.datasets.Get(ctx, tenantID, publicID)
		if err != nil {
			return MedallionAsset{}, false, err
		}
		asset, err := s.EnsureDatasetAsset(ctx, dataset, nil)
		if err == nil {
			s.ensureDatasetEdges(ctx, dataset, asset)
		}
		return asset, true, err
	case MedallionResourceWorkTable:
		if s.datasets == nil {
			return MedallionAsset{}, false, fmt.Errorf("dataset service is not configured")
		}
		table, err := s.datasets.GetManagedWorkTable(ctx, tenantID, publicID)
		if err != nil {
			return MedallionAsset{}, false, err
		}
		asset, err := s.EnsureWorkTableAsset(ctx, table, nil)
		if err == nil {
			s.ensureWorkTableEdges(ctx, table, asset)
		}
		return asset, true, err
	case MedallionResourceOCRRun:
		if s.drive == nil {
			return MedallionAsset{}, false, fmt.Errorf("drive service is not configured")
		}
		runID, err := uuid.Parse(publicID)
		if err != nil {
			return MedallionAsset{}, false, ErrMedallionAssetNotFound
		}
		run, err := s.queries.GetDriveOCRRunByPublicID(ctx, db.GetDriveOCRRunByPublicIDParams{TenantID: tenantID, PublicID: runID})
		if errors.Is(err, pgx.ErrNoRows) {
			return MedallionAsset{}, false, ErrMedallionAssetNotFound
		}
		if err != nil {
			return MedallionAsset{}, false, err
		}
		_, file, err := s.driveFileForViewByID(ctx, tenantID, actorUserID, run.FileObjectID)
		if err != nil {
			return MedallionAsset{}, false, err
		}
		item := driveOCRRunFromDB(run, file.PublicID)
		asset, err := s.EnsureOCRRunAsset(ctx, file, item, medallionPipelineStatusFromOCRStatus(item.Status), item.RequestedByUserID)
		return asset, true, err
	case MedallionResourceProductExtraction:
		itemID, err := uuid.Parse(publicID)
		if err != nil {
			return MedallionAsset{}, false, ErrMedallionAssetNotFound
		}
		item, err := s.queries.GetDriveProductExtractionItemByPublicIDForTenant(ctx, db.GetDriveProductExtractionItemByPublicIDForTenantParams{TenantID: tenantID, PublicID: itemID})
		if errors.Is(err, pgx.ErrNoRows) {
			return MedallionAsset{}, false, ErrMedallionAssetNotFound
		}
		if err != nil {
			return MedallionAsset{}, false, err
		}
		_, file, err := s.driveFileForViewByID(ctx, tenantID, actorUserID, item.FileObjectID)
		if err != nil {
			return MedallionAsset{}, false, err
		}
		asset, err := s.EnsureProductExtractionAsset(ctx, file, item, nil)
		return asset, true, err
	case MedallionResourceGoldTable:
		if s.datasets == nil {
			return MedallionAsset{}, false, fmt.Errorf("dataset service is not configured")
		}
		publication, err := s.datasets.GetGoldPublication(ctx, tenantID, publicID)
		if err != nil {
			return MedallionAsset{}, false, err
		}
		asset, err := s.EnsureGoldTableAsset(ctx, publication, nil)
		return asset, true, err
	default:
		return MedallionAsset{}, false, ErrMedallionAssetNotFound
	}
}

func (s *MedallionCatalogService) catalogForAsset(ctx context.Context, tenantID int64, asset MedallionAsset) (MedallionCatalog, error) {
	upstreamRows, err := s.queries.ListMedallionUpstreamAssets(ctx, db.ListMedallionUpstreamAssetsParams{TenantID: tenantID, AssetID: asset.ID, LimitCount: 50})
	if err != nil {
		return MedallionCatalog{}, fmt.Errorf("list upstream medallion assets: %w", err)
	}
	downstreamRows, err := s.queries.ListMedallionDownstreamAssets(ctx, db.ListMedallionDownstreamAssetsParams{TenantID: tenantID, AssetID: asset.ID, LimitCount: 50})
	if err != nil {
		return MedallionCatalog{}, fmt.Errorf("list downstream medallion assets: %w", err)
	}
	runRows, err := s.queries.ListMedallionPipelineRunsByAsset(ctx, db.ListMedallionPipelineRunsByAssetParams{TenantID: tenantID, AssetID: asset.ID, LimitCount: 50})
	if err != nil {
		return MedallionCatalog{}, fmt.Errorf("list medallion pipeline runs: %w", err)
	}
	runs := make([]MedallionPipelineRun, 0, len(runRows))
	for _, row := range runRows {
		run := medallionPipelineRunFromDB(row)
		s.hydratePipelineRunAssetIDs(ctx, tenantID, &run)
		runs = append(runs, run)
	}
	upstream := make([]MedallionAsset, 0, len(upstreamRows))
	for _, row := range upstreamRows {
		upstream = append(upstream, medallionAssetFromDB(row))
	}
	downstream := make([]MedallionAsset, 0, len(downstreamRows))
	for _, row := range downstreamRows {
		downstream = append(downstream, medallionAssetFromDB(row))
	}
	return MedallionCatalog{Asset: &asset, Upstream: upstream, Downstream: downstream, PipelineRuns: runs}, nil
}

func (s *MedallionCatalogService) canViewAsset(ctx context.Context, tenantID, actorUserID int64, asset MedallionAsset) (bool, error) {
	switch asset.ResourceKind {
	case MedallionResourceDriveFile:
		_, _, err := s.driveFileForViewByID(ctx, tenantID, actorUserID, asset.ResourceID)
		if errors.Is(err, ErrDrivePermissionDenied) {
			return false, nil
		}
		return err == nil, err
	case MedallionResourceOCRRun:
		parsed, err := uuid.Parse(asset.ResourcePublicID)
		if err != nil {
			return false, nil
		}
		row, err := s.queries.GetDriveOCRRunByPublicID(ctx, db.GetDriveOCRRunByPublicIDParams{TenantID: tenantID, PublicID: parsed})
		if err != nil {
			return false, nil
		}
		_, _, err = s.driveFileForViewByID(ctx, tenantID, actorUserID, row.FileObjectID)
		if errors.Is(err, ErrDrivePermissionDenied) {
			return false, nil
		}
		return err == nil, err
	case MedallionResourceProductExtraction:
		parsed, err := uuid.Parse(asset.ResourcePublicID)
		if err != nil {
			return false, nil
		}
		row, err := s.queries.GetDriveProductExtractionItemByPublicIDForTenant(ctx, db.GetDriveProductExtractionItemByPublicIDForTenantParams{TenantID: tenantID, PublicID: parsed})
		if err != nil {
			return false, nil
		}
		_, _, err = s.driveFileForViewByID(ctx, tenantID, actorUserID, row.FileObjectID)
		if errors.Is(err, ErrDrivePermissionDenied) {
			return false, nil
		}
		return err == nil, err
	default:
		return true, nil
	}
}

func (s *MedallionCatalogService) driveFileForView(ctx context.Context, tenantID, actorUserID int64, filePublicID string) (DriveActor, DriveFile, error) {
	actor, err := s.drive.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveActor{}, DriveFile{}, err
	}
	row, err := s.drive.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return DriveActor{}, DriveFile{}, err
	}
	file := driveFileFromDB(row)
	if err := s.drive.authz.CanViewFile(ctx, actor, file); err != nil {
		return DriveActor{}, DriveFile{}, err
	}
	return actor, file, nil
}

func (s *MedallionCatalogService) driveFileForViewByID(ctx context.Context, tenantID, actorUserID, fileID int64) (DriveActor, DriveFile, error) {
	actor, err := s.drive.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveActor{}, DriveFile{}, err
	}
	row, err := s.drive.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, ID: fileID})
	if err != nil {
		return DriveActor{}, DriveFile{}, err
	}
	file := driveFileFromDB(row)
	if err := s.drive.authz.CanViewFile(ctx, actor, file); err != nil {
		return DriveActor{}, DriveFile{}, err
	}
	return actor, file, nil
}

func (s *MedallionCatalogService) ensureConfigured() error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("medallion catalog service is not configured")
	}
	return nil
}

type medallionAssetInput struct {
	TenantID         int64
	Layer            string
	ResourceKind     string
	ResourceID       int64
	ResourcePublicID string
	DisplayName      string
	Status           string
	RowCount         *int64
	ByteSize         *int64
	SchemaSummary    map[string]any
	Metadata         map[string]any
	CreatedByUserID  *int64
	UpdatedByUserID  *int64
}

func (s *MedallionCatalogService) upsertAsset(ctx context.Context, input medallionAssetInput) (MedallionAsset, error) {
	if input.Status == "" {
		input.Status = MedallionAssetStatusActive
	}
	parsed, err := uuid.Parse(input.ResourcePublicID)
	if err != nil {
		return MedallionAsset{}, err
	}
	row, err := s.queries.UpsertMedallionAsset(ctx, db.UpsertMedallionAssetParams{
		TenantID:         input.TenantID,
		Layer:            input.Layer,
		ResourceKind:     input.ResourceKind,
		ResourceID:       input.ResourceID,
		ResourcePublicID: parsed,
		DisplayName:      strings.TrimSpace(input.DisplayName),
		Status:           input.Status,
		RowCount:         pgInt8(input.RowCount),
		ByteSize:         pgInt8(input.ByteSize),
		SchemaSummary:    jsonBytes(input.SchemaSummary),
		Metadata:         jsonBytes(input.Metadata),
		CreatedByUserID:  pgInt8(input.CreatedByUserID),
		UpdatedByUserID:  pgInt8(input.UpdatedByUserID),
		ArchivedAt:       pgtype.Timestamptz{},
	})
	if err != nil {
		return MedallionAsset{}, fmt.Errorf("upsert medallion asset: %w", err)
	}
	return medallionAssetFromDB(row), nil
}

func medallionAssetFromDB(row db.MedallionAsset) MedallionAsset {
	return MedallionAsset{
		ID:               row.ID,
		PublicID:         row.PublicID.String(),
		TenantID:         row.TenantID,
		Layer:            row.Layer,
		ResourceKind:     row.ResourceKind,
		ResourceID:       row.ResourceID,
		ResourcePublicID: row.ResourcePublicID.String(),
		DisplayName:      row.DisplayName,
		Status:           row.Status,
		RowCount:         optionalPgInt8(row.RowCount),
		ByteSize:         optionalPgInt8(row.ByteSize),
		SchemaSummary:    jsonObjectFromBytes(row.SchemaSummary),
		Metadata:         jsonObjectFromBytes(row.Metadata),
		CreatedByUserID:  optionalPgInt8(row.CreatedByUserID),
		UpdatedByUserID:  optionalPgInt8(row.UpdatedByUserID),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
		ArchivedAt:       optionalPgTime(row.ArchivedAt),
	}
}

func medallionPipelineRunFromDB(row db.MedallionPipelineRun) MedallionPipelineRun {
	return MedallionPipelineRun{
		ID:                     row.ID,
		PublicID:               row.PublicID.String(),
		TenantID:               row.TenantID,
		PipelineType:           row.PipelineType,
		Status:                 row.Status,
		Runtime:                row.Runtime,
		TriggerKind:            row.TriggerKind,
		Retryable:              row.Retryable,
		ErrorSummary:           optionalText(row.ErrorSummary),
		Metadata:               jsonObjectFromBytes(row.Metadata),
		RequestedByUserID:      optionalPgInt8(row.RequestedByUserID),
		StartedAt:              optionalPgTime(row.StartedAt),
		CompletedAt:            optionalPgTime(row.CompletedAt),
		CreatedAt:              row.CreatedAt.Time,
		UpdatedAt:              row.UpdatedAt.Time,
		SourceResourceKind:     optionalText(row.SourceResourceKind),
		SourceResourcePublicID: uuidString(row.SourceResourcePublicID),
		TargetResourceKind:     optionalText(row.TargetResourceKind),
		TargetResourcePublicID: uuidString(row.TargetResourcePublicID),
	}
}

func (s *MedallionCatalogService) hydratePipelineRunAssetIDs(ctx context.Context, tenantID int64, run *MedallionPipelineRun) {
	if s == nil || s.queries == nil || run == nil || run.ID <= 0 {
		return
	}
	rows, err := s.queries.ListMedallionPipelineRunAssetLinks(ctx, db.ListMedallionPipelineRunAssetLinksParams{TenantID: tenantID, PipelineRunID: run.ID})
	if err != nil {
		return
	}
	for _, row := range rows {
		switch row.Role {
		case "source":
			run.SourceAssetPublicIDs = append(run.SourceAssetPublicIDs, row.PublicID.String())
		case "target":
			run.TargetAssetPublicIDs = append(run.TargetAssetPublicIDs, row.PublicID.String())
		}
	}
}

func medallionDriveFileEligible(file DriveFile) bool {
	contentType := strings.ToLower(strings.TrimSpace(file.ContentType))
	name := strings.ToLower(strings.TrimSpace(file.OriginalFilename))
	if contentType == "application/pdf" || strings.HasPrefix(contentType, "image/") {
		return true
	}
	if isDatasetCSVSource(name, contentType) {
		return true
	}
	return strings.HasSuffix(name, ".pdf") || strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".tif") || strings.HasSuffix(name, ".tiff") || strings.HasSuffix(name, ".webp")
}

func medallionDatasetLayer(dataset Dataset) string {
	if dataset.SourceKind == "work_table" {
		return MedallionLayerSilver
	}
	return MedallionLayerBronze
}

func medallionAssetStatusFromDatasetStatus(status string) string {
	switch status {
	case "pending", "importing":
		return MedallionAssetStatusBuilding
	case "failed":
		return MedallionAssetStatusFailed
	case "deleted":
		return MedallionAssetStatusArchived
	default:
		return MedallionAssetStatusActive
	}
}

func medallionPipelineStatusFromOCRStatus(status string) string {
	switch status {
	case "completed":
		return MedallionPipelineStatusCompleted
	case "failed":
		return MedallionPipelineStatusFailed
	case "skipped":
		return MedallionPipelineStatusSkipped
	case "running":
		return MedallionPipelineStatusProcessing
	default:
		return MedallionPipelineStatusPending
	}
}

func medallionAssetStatusFromPipelineStatus(status string) string {
	switch status {
	case MedallionPipelineStatusCompleted:
		return MedallionAssetStatusActive
	case MedallionPipelineStatusFailed:
		return MedallionAssetStatusFailed
	case MedallionPipelineStatusSkipped:
		return MedallionAssetStatusSkipped
	default:
		return MedallionAssetStatusBuilding
	}
}

func medallionAssetStatusFromGoldPublicationStatus(status string) string {
	switch status {
	case "active":
		return MedallionAssetStatusActive
	case "pending":
		return MedallionAssetStatusBuilding
	case "failed":
		return MedallionAssetStatusFailed
	default:
		return MedallionAssetStatusArchived
	}
}

func medallionDatasetSchemaSummary(columns []DatasetColumn) map[string]any {
	out := map[string]any{"columns": len(columns)}
	names := make([]map[string]any, 0, len(columns))
	for _, column := range columns {
		names = append(names, map[string]any{"name": column.ColumnName, "type": column.ClickHouseType, "ordinal": column.Ordinal})
	}
	if len(names) > 0 {
		out["items"] = names
	}
	return out
}

func medallionWorkTableSchemaSummary(columns []DatasetWorkTableColumn) map[string]any {
	out := map[string]any{"columns": len(columns)}
	names := make([]map[string]any, 0, len(columns))
	for _, column := range columns {
		names = append(names, map[string]any{"name": column.ColumnName, "type": column.ClickHouseType, "ordinal": column.Ordinal})
	}
	if len(names) > 0 {
		out["items"] = names
	}
	return out
}

func jsonBytes(value map[string]any) []byte {
	if value == nil {
		value = map[string]any{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func pgInt8Value(value int64) pgtype.Int8 {
	if value <= 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: value, Valid: true}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
