package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	DatasetLineageDirectionUpstream   = "upstream"
	DatasetLineageDirectionDownstream = "downstream"
	DatasetLineageDirectionBoth       = "both"

	DatasetLineageLevelTable  = "table"
	DatasetLineageLevelColumn = "column"
	DatasetLineageLevelBoth   = "both"

	DatasetLineageSourceMetadata = "metadata"
	DatasetLineageSourceParser   = "parser"
	DatasetLineageSourceManual   = "manual"

	DatasetLineageNodeKindResource = "resource"
	DatasetLineageNodeKindColumn   = "column"
	DatasetLineageNodeKindCustom   = "custom"

	DatasetLineageResourceDataset                 = "dataset"
	DatasetLineageResourceQueryJob                = "dataset_query_job"
	DatasetLineageResourceWorkTable               = "dataset_work_table"
	DatasetLineageResourceWorkTableExport         = "dataset_work_table_export"
	DatasetLineageResourceWorkTableExportSchedule = "dataset_work_table_export_schedule"
	DatasetLineageResourceSyncJob                 = "dataset_sync_job"
	DatasetLineageResourceCustom                  = "custom"

	DatasetLineageRelationQueryInput            = "query_input"
	DatasetLineageRelationQueryCreatedWorkTable = "query_created_work_table"
	DatasetLineageRelationSourceDataset         = "source_dataset"
	DatasetLineageRelationPromotedDataset       = "promoted_dataset"
	DatasetLineageRelationWorkTableExport       = "work_table_export"
	DatasetLineageRelationExportSchedule        = "export_schedule"
	DatasetLineageRelationScheduledExportRun    = "scheduled_export_run"
	DatasetLineageRelationDatasetSyncSource     = "dataset_sync_source"
	DatasetLineageRelationDatasetSyncTarget     = "dataset_sync_target"
	DatasetLineageRelationColumnDerives         = "column_derives"
	DatasetLineageRelationManualDependency      = "manual_dependency"

	datasetLineageConfidenceMetadata      = "metadata"
	datasetLineageConfidenceParserExact   = "parser_exact"
	datasetLineageConfidenceParserPartial = "parser_partial"
	datasetLineageConfidenceManual        = "manual"
)

type DatasetLineageOptions struct {
	Direction         string
	Depth             int32
	IncludeHistory    bool
	Limit             int32
	Level             string
	Sources           []string
	IncludeDraft      bool
	ChangeSetPublicID string
}

type DatasetLineageGraph struct {
	Root     DatasetLineageNode           `json:"root"`
	Nodes    []DatasetLineageNode         `json:"nodes"`
	Edges    []DatasetLineageEdge         `json:"edges"`
	Timeline []DatasetLineageTimelineItem `json:"timeline"`
}

type DatasetLineageNode struct {
	ID           string                  `json:"id"`
	ResourceType string                  `json:"resourceType"`
	PublicID     string                  `json:"publicId,omitempty"`
	DisplayName  string                  `json:"displayName"`
	Status       string                  `json:"status,omitempty"`
	NodeKind     string                  `json:"nodeKind,omitempty"`
	SourceKind   string                  `json:"sourceKind,omitempty"`
	ColumnName   string                  `json:"columnName,omitempty"`
	Description  string                  `json:"description,omitempty"`
	Editable     bool                    `json:"editable"`
	Position     *DatasetLineagePosition `json:"position,omitempty"`
	CreatedAt    *time.Time              `json:"createdAt,omitempty"`
	UpdatedAt    *time.Time              `json:"updatedAt,omitempty"`
	Metadata     map[string]any          `json:"metadata,omitempty"`
}

type DatasetLineagePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type DatasetLineageEdge struct {
	ID           string     `json:"id"`
	SourceNodeID string     `json:"sourceNodeId"`
	TargetNodeID string     `json:"targetNodeId"`
	RelationType string     `json:"relationType"`
	Confidence   string     `json:"confidence"`
	SourceKind   string     `json:"sourceKind,omitempty"`
	Label        string     `json:"label,omitempty"`
	Description  string     `json:"description,omitempty"`
	Expression   string     `json:"expression,omitempty"`
	Editable     bool       `json:"editable"`
	CreatedAt    *time.Time `json:"createdAt,omitempty"`
}

type DatasetLineageTimelineItem struct {
	ID           string         `json:"id"`
	NodeID       string         `json:"nodeId"`
	ResourceType string         `json:"resourceType"`
	PublicID     string         `json:"publicId,omitempty"`
	RelationType string         `json:"relationType"`
	Status       string         `json:"status,omitempty"`
	OccurredAt   *time.Time     `json:"occurredAt,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type datasetLineageBuilder struct {
	rootID      string
	nodes       map[string]DatasetLineageNode
	nodeOrder   []string
	edges       map[string]DatasetLineageEdge
	edgeOrder   []string
	timeline    []DatasetLineageTimelineItem
	timelineIDs map[string]struct{}
}

func (s *DatasetService) GetDatasetLineage(ctx context.Context, tenantID int64, publicID string, opts DatasetLineageOptions) (DatasetLineageGraph, error) {
	if s == nil || s.queries == nil {
		return DatasetLineageGraph{}, fmt.Errorf("dataset service is not configured")
	}
	opts, err := normalizeDatasetLineageOptions(opts)
	if err != nil {
		return DatasetLineageGraph{}, err
	}
	dataset, err := s.Get(ctx, tenantID, publicID)
	if err != nil {
		return DatasetLineageGraph{}, err
	}
	builder := newDatasetLineageBuilder()
	root := builder.addDatasetNode(dataset)
	builder.rootID = root.ID
	if lineageIncludesUpstream(opts.Direction) {
		if err := s.addDatasetLineageUpstream(ctx, tenantID, builder, dataset, opts.Depth, opts); err != nil {
			return DatasetLineageGraph{}, err
		}
	}
	if lineageIncludesDownstream(opts.Direction) {
		if err := s.addDatasetLineageDownstream(ctx, tenantID, builder, dataset, opts.Depth, opts); err != nil {
			return DatasetLineageGraph{}, err
		}
	}
	return s.mergePersistedLineage(ctx, tenantID, builder, opts)
}

func (s *DatasetService) GetWorkTableLineage(ctx context.Context, tenantID int64, publicID string, opts DatasetLineageOptions) (DatasetLineageGraph, error) {
	if s == nil || s.queries == nil {
		return DatasetLineageGraph{}, fmt.Errorf("dataset service is not configured")
	}
	opts, err := normalizeDatasetLineageOptions(opts)
	if err != nil {
		return DatasetLineageGraph{}, err
	}
	row, err := s.getManagedWorkTableRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetLineageGraph{}, err
	}
	workTable := datasetWorkTableFromDB(row)
	builder := newDatasetLineageBuilder()
	root := builder.addWorkTableNode(workTable)
	builder.rootID = root.ID
	if lineageIncludesUpstream(opts.Direction) {
		if err := s.addWorkTableLineageUpstream(ctx, tenantID, builder, workTable, opts.Depth, opts); err != nil {
			return DatasetLineageGraph{}, err
		}
	}
	if lineageIncludesDownstream(opts.Direction) {
		if err := s.addWorkTableLineageDownstream(ctx, tenantID, builder, workTable, opts.Depth, opts); err != nil {
			return DatasetLineageGraph{}, err
		}
	}
	return s.mergePersistedLineage(ctx, tenantID, builder, opts)
}

func (s *DatasetService) GetQueryJobLineage(ctx context.Context, tenantID int64, publicID string, opts DatasetLineageOptions) (DatasetLineageGraph, error) {
	if s == nil || s.queries == nil {
		return DatasetLineageGraph{}, fmt.Errorf("dataset service is not configured")
	}
	opts, err := normalizeDatasetLineageOptions(opts)
	if err != nil {
		return DatasetLineageGraph{}, err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return DatasetLineageGraph{}, ErrDatasetQueryNotFound
	}
	row, err := s.queries.GetDatasetQueryJobForTenant(ctx, db.GetDatasetQueryJobForTenantParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetLineageGraph{}, ErrDatasetQueryNotFound
	}
	if err != nil {
		return DatasetLineageGraph{}, fmt.Errorf("get dataset query job: %w", err)
	}
	job := datasetQueryJobFromDB(row)
	builder := newDatasetLineageBuilder()
	root := builder.addQueryJobNode(job)
	builder.rootID = root.ID
	if lineageIncludesUpstream(opts.Direction) {
		if err := s.addQueryJobLineageUpstream(ctx, tenantID, builder, row, opts.Depth, opts); err != nil {
			return DatasetLineageGraph{}, err
		}
	}
	if lineageIncludesDownstream(opts.Direction) {
		if err := s.addQueryJobLineageDownstream(ctx, tenantID, builder, row, opts.Depth, opts); err != nil {
			return DatasetLineageGraph{}, err
		}
	}
	return s.mergePersistedLineage(ctx, tenantID, builder, opts)
}

func (s *DatasetService) addDatasetLineageUpstream(ctx context.Context, tenantID int64, builder *datasetLineageBuilder, dataset Dataset, depth int32, opts DatasetLineageOptions) error {
	if depth <= 0 {
		return nil
	}
	datasetNode := builder.addDatasetNode(dataset)
	if dataset.SourceWorkTableID != nil {
		workTable, ok, err := s.getLineageWorkTableByID(ctx, tenantID, *dataset.SourceWorkTableID)
		if err != nil {
			return err
		}
		if ok {
			workTableNode := builder.addWorkTableNode(workTable)
			builder.addEdge(workTableNode, datasetNode, DatasetLineageRelationPromotedDataset, dataset.CreatedAt)
			builder.addTimeline(datasetNode, DatasetLineageRelationPromotedDataset, dataset.Status, dataset.CreatedAt, map[string]any{
				"sourceWorkTable": workTable.DisplayName,
			})
			if depth > 1 {
				if err := s.addWorkTableLineageUpstream(ctx, tenantID, builder, workTable, depth-1, opts); err != nil {
					return err
				}
			}
		}
	}
	if opts.IncludeHistory {
		rows, err := s.queries.ListDatasetSyncJobs(ctx, db.ListDatasetSyncJobsParams{
			TenantID:  tenantID,
			DatasetID: dataset.ID,
			Limit:     opts.Limit,
		})
		if err != nil {
			return fmt.Errorf("list dataset sync jobs for lineage: %w", err)
		}
		for _, row := range rows {
			syncJob := datasetSyncJobFromDB(row)
			syncNode := builder.addSyncJobNode(syncJob)
			builder.addTimeline(syncNode, DatasetLineageRelationDatasetSyncTarget, syncJob.Status, syncJobUpdatedAt(syncJob), nil)
			workTable, ok, err := s.getLineageWorkTableByID(ctx, tenantID, row.SourceWorkTableID)
			if err != nil {
				return err
			}
			if ok {
				workTableNode := builder.addWorkTableNode(workTable)
				builder.addEdge(workTableNode, syncNode, DatasetLineageRelationDatasetSyncSource, syncJob.CreatedAt)
				builder.addEdge(syncNode, datasetNode, DatasetLineageRelationDatasetSyncTarget, syncJobUpdatedAt(syncJob))
			}
		}
	}
	return nil
}

func (s *DatasetService) addDatasetLineageDownstream(ctx context.Context, tenantID int64, builder *datasetLineageBuilder, dataset Dataset, depth int32, opts DatasetLineageOptions) error {
	if depth <= 0 {
		return nil
	}
	datasetNode := builder.addDatasetNode(dataset)
	queryRows, err := s.queries.ListDatasetQueryJobsForDataset(ctx, db.ListDatasetQueryJobsForDatasetParams{
		TenantID:  tenantID,
		DatasetID: pgtype.Int8{Int64: dataset.ID, Valid: true},
		Limit:     opts.Limit,
	})
	if err != nil {
		return fmt.Errorf("list dataset query jobs for lineage: %w", err)
	}
	for _, row := range queryRows {
		queryNode := builder.addQueryJobNode(datasetQueryJobFromDB(row))
		builder.addEdge(datasetNode, queryNode, DatasetLineageRelationQueryInput, row.CreatedAt.Time)
		builder.addTimeline(queryNode, DatasetLineageRelationQueryInput, row.Status, row.CreatedAt.Time, nil)
		if depth > 1 {
			if err := s.addQueryJobLineageDownstream(ctx, tenantID, builder, row, depth-1, opts); err != nil {
				return err
			}
		}
	}
	workTableRows, err := s.queries.ListLineageDatasetWorkTablesForDataset(ctx, db.ListLineageDatasetWorkTablesForDatasetParams{
		TenantID:        tenantID,
		SourceDatasetID: pgtype.Int8{Int64: dataset.ID, Valid: true},
		Limit:           opts.Limit,
	})
	if err != nil {
		return fmt.Errorf("list dataset work tables for lineage: %w", err)
	}
	for _, row := range workTableRows {
		workTable := datasetWorkTableFromDB(row)
		workTableNode := builder.addWorkTableNode(workTable)
		builder.addEdge(datasetNode, workTableNode, DatasetLineageRelationSourceDataset, row.CreatedAt.Time)
		builder.addTimeline(workTableNode, DatasetLineageRelationSourceDataset, workTable.Status, row.CreatedAt.Time, nil)
		if depth > 1 {
			if err := s.addWorkTableLineageDownstream(ctx, tenantID, builder, workTable, depth-1, opts); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *DatasetService) addWorkTableLineageUpstream(ctx context.Context, tenantID int64, builder *datasetLineageBuilder, workTable DatasetWorkTable, depth int32, opts DatasetLineageOptions) error {
	if depth <= 0 {
		return nil
	}
	workTableNode := builder.addWorkTableNode(workTable)
	if workTable.SourceDatasetID != nil {
		dataset, ok, err := s.getLineageDatasetByID(ctx, tenantID, *workTable.SourceDatasetID)
		if err != nil {
			return err
		}
		if ok {
			datasetNode := builder.addDatasetNode(dataset)
			builder.addEdge(datasetNode, workTableNode, DatasetLineageRelationSourceDataset, workTable.CreatedAt)
			builder.addTimeline(workTableNode, DatasetLineageRelationSourceDataset, workTable.Status, workTable.CreatedAt, nil)
			if depth > 1 {
				if err := s.addDatasetLineageUpstream(ctx, tenantID, builder, dataset, depth-1, opts); err != nil {
					return err
				}
			}
		}
	}
	if workTable.CreatedFromQueryJobID != nil {
		query, ok, err := s.getLineageQueryJobByID(ctx, tenantID, *workTable.CreatedFromQueryJobID)
		if err != nil {
			return err
		}
		if ok {
			queryNode := builder.addQueryJobNode(query)
			builder.addEdge(queryNode, workTableNode, DatasetLineageRelationQueryCreatedWorkTable, workTable.CreatedAt)
			if depth > 1 && query.DatasetID != nil {
				dataset, ok, err := s.getLineageDatasetByID(ctx, tenantID, *query.DatasetID)
				if err != nil {
					return err
				}
				if ok {
					datasetNode := builder.addDatasetNode(dataset)
					builder.addEdge(datasetNode, queryNode, DatasetLineageRelationQueryInput, query.CreatedAt)
				}
			}
		}
	}
	return nil
}

func (s *DatasetService) addWorkTableLineageDownstream(ctx context.Context, tenantID int64, builder *datasetLineageBuilder, workTable DatasetWorkTable, depth int32, opts DatasetLineageOptions) error {
	if depth <= 0 {
		return nil
	}
	workTableNode := builder.addWorkTableNode(workTable)
	datasetRows, err := s.queries.ListLineageDatasetsBySourceWorkTable(ctx, db.ListLineageDatasetsBySourceWorkTableParams{
		TenantID:          tenantID,
		SourceWorkTableID: pgtype.Int8{Int64: workTable.ID, Valid: true},
		Limit:             opts.Limit,
	})
	if err != nil {
		return fmt.Errorf("list promoted datasets for lineage: %w", err)
	}
	for _, row := range datasetRows {
		dataset := datasetFromDB(row)
		datasetNode := builder.addDatasetNode(dataset)
		builder.addEdge(workTableNode, datasetNode, DatasetLineageRelationPromotedDataset, dataset.CreatedAt)
		builder.addTimeline(datasetNode, DatasetLineageRelationPromotedDataset, dataset.Status, dataset.CreatedAt, nil)
	}
	if !opts.IncludeHistory {
		return nil
	}
	scheduleRows, err := s.queries.ListDatasetWorkTableExportSchedules(ctx, db.ListDatasetWorkTableExportSchedulesParams{
		TenantID:    tenantID,
		WorkTableID: workTable.ID,
	})
	if err != nil {
		return fmt.Errorf("list work table export schedules for lineage: %w", err)
	}
	scheduleNodes := make(map[int64]DatasetLineageNode, len(scheduleRows))
	for _, row := range scheduleRows {
		schedule := datasetWorkTableExportScheduleFromDB(row)
		scheduleNode := builder.addExportScheduleNode(schedule)
		scheduleNodes[row.ID] = scheduleNode
		builder.addEdge(workTableNode, scheduleNode, DatasetLineageRelationExportSchedule, schedule.CreatedAt)
		builder.addTimeline(scheduleNode, DatasetLineageRelationExportSchedule, lineageScheduleStatus(schedule), schedule.UpdatedAt, nil)
	}
	exportRows, err := s.queries.ListLineageDatasetWorkTableExports(ctx, db.ListLineageDatasetWorkTableExportsParams{
		TenantID:    tenantID,
		WorkTableID: workTable.ID,
		Limit:       opts.Limit,
	})
	if err != nil {
		return fmt.Errorf("list work table exports for lineage: %w", err)
	}
	exports := make([]DatasetWorkTableExport, 0, len(exportRows))
	for _, row := range exportRows {
		exports = append(exports, datasetWorkTableExportFromDB(row))
	}
	s.hydrateWorkTableExportSchedulePublicIDs(ctx, tenantID, exports)
	for _, export := range exports {
		exportNode := builder.addExportNode(export)
		builder.addEdge(workTableNode, exportNode, DatasetLineageRelationWorkTableExport, export.CreatedAt)
		builder.addTimeline(exportNode, DatasetLineageRelationWorkTableExport, export.Status, exportUpdatedAt(export), map[string]any{
			"format": export.Format,
			"source": export.Source,
		})
		if export.ScheduleID != nil {
			if scheduleNode, ok := scheduleNodes[*export.ScheduleID]; ok {
				builder.addEdge(scheduleNode, exportNode, DatasetLineageRelationScheduledExportRun, export.CreatedAt)
			}
		}
	}
	syncRows, err := s.queries.ListLineageDatasetSyncJobsBySourceWorkTable(ctx, db.ListLineageDatasetSyncJobsBySourceWorkTableParams{
		TenantID:          tenantID,
		SourceWorkTableID: workTable.ID,
		Limit:             opts.Limit,
	})
	if err != nil {
		return fmt.Errorf("list source work table sync jobs for lineage: %w", err)
	}
	for _, row := range syncRows {
		syncJob := datasetSyncJobFromDB(row)
		syncNode := builder.addSyncJobNode(syncJob)
		builder.addEdge(workTableNode, syncNode, DatasetLineageRelationDatasetSyncSource, syncJob.CreatedAt)
		builder.addTimeline(syncNode, DatasetLineageRelationDatasetSyncSource, syncJob.Status, syncJobUpdatedAt(syncJob), nil)
		dataset, ok, err := s.getLineageDatasetByID(ctx, tenantID, row.DatasetID)
		if err != nil {
			return err
		}
		if ok {
			datasetNode := builder.addDatasetNode(dataset)
			builder.addEdge(syncNode, datasetNode, DatasetLineageRelationDatasetSyncTarget, syncJobUpdatedAt(syncJob))
		}
	}
	return nil
}

func (s *DatasetService) addQueryJobLineageUpstream(ctx context.Context, tenantID int64, builder *datasetLineageBuilder, query db.DatasetQueryJob, depth int32, opts DatasetLineageOptions) error {
	if depth <= 0 || !query.DatasetID.Valid {
		return nil
	}
	queryNode := builder.addQueryJobNode(datasetQueryJobFromDB(query))
	dataset, ok, err := s.getLineageDatasetByID(ctx, tenantID, query.DatasetID.Int64)
	if err != nil {
		return err
	}
	if ok {
		datasetNode := builder.addDatasetNode(dataset)
		builder.addEdge(datasetNode, queryNode, DatasetLineageRelationQueryInput, query.CreatedAt.Time)
		if depth > 1 {
			if err := s.addDatasetLineageUpstream(ctx, tenantID, builder, dataset, depth-1, opts); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *DatasetService) addQueryJobLineageDownstream(ctx context.Context, tenantID int64, builder *datasetLineageBuilder, query db.DatasetQueryJob, depth int32, opts DatasetLineageOptions) error {
	if depth <= 0 {
		return nil
	}
	queryNode := builder.addQueryJobNode(datasetQueryJobFromDB(query))
	rows, err := s.queries.ListLineageDatasetWorkTablesByQueryJob(ctx, db.ListLineageDatasetWorkTablesByQueryJobParams{
		TenantID:              tenantID,
		CreatedFromQueryJobID: pgtype.Int8{Int64: query.ID, Valid: true},
		Limit:                 opts.Limit,
	})
	if err != nil {
		return fmt.Errorf("list query work tables for lineage: %w", err)
	}
	for _, row := range rows {
		workTable := datasetWorkTableFromDB(row)
		workTableNode := builder.addWorkTableNode(workTable)
		builder.addEdge(queryNode, workTableNode, DatasetLineageRelationQueryCreatedWorkTable, workTable.CreatedAt)
		builder.addTimeline(workTableNode, DatasetLineageRelationQueryCreatedWorkTable, workTable.Status, workTable.CreatedAt, nil)
		if depth > 1 {
			if err := s.addWorkTableLineageDownstream(ctx, tenantID, builder, workTable, depth-1, opts); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *DatasetService) getLineageDatasetByID(ctx context.Context, tenantID, datasetID int64) (Dataset, bool, error) {
	row, err := s.queries.GetDatasetByIDForTenant(ctx, db.GetDatasetByIDForTenantParams{ID: datasetID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Dataset{}, false, nil
	}
	if err != nil {
		return Dataset{}, false, fmt.Errorf("get lineage dataset: %w", err)
	}
	return datasetFromDB(row), true, nil
}

func (s *DatasetService) getLineageWorkTableByID(ctx context.Context, tenantID, workTableID int64) (DatasetWorkTable, bool, error) {
	row, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: workTableID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetWorkTable{}, false, nil
	}
	if err != nil {
		return DatasetWorkTable{}, false, fmt.Errorf("get lineage work table: %w", err)
	}
	return datasetWorkTableFromDB(row), true, nil
}

func (s *DatasetService) getLineageQueryJobByID(ctx context.Context, tenantID, queryJobID int64) (DatasetQueryJob, bool, error) {
	row, err := s.queries.GetDatasetQueryJobByIDForTenant(ctx, db.GetDatasetQueryJobByIDForTenantParams{ID: queryJobID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetQueryJob{}, false, nil
	}
	if err != nil {
		return DatasetQueryJob{}, false, fmt.Errorf("get lineage query job: %w", err)
	}
	return datasetQueryJobFromDB(row), true, nil
}

func newDatasetLineageBuilder() *datasetLineageBuilder {
	return &datasetLineageBuilder{
		nodes:       make(map[string]DatasetLineageNode),
		edges:       make(map[string]DatasetLineageEdge),
		timelineIDs: make(map[string]struct{}),
	}
}

func (b *datasetLineageBuilder) graph() DatasetLineageGraph {
	nodes := make([]DatasetLineageNode, 0, len(b.nodeOrder))
	for _, id := range b.nodeOrder {
		nodes = append(nodes, b.nodes[id])
	}
	edges := make([]DatasetLineageEdge, 0, len(b.edgeOrder))
	for _, id := range b.edgeOrder {
		edges = append(edges, b.edges[id])
	}
	return DatasetLineageGraph{
		Root:     b.nodes[b.rootID],
		Nodes:    nodes,
		Edges:    edges,
		Timeline: b.timeline,
	}
}

func (b *datasetLineageBuilder) addDatasetNode(item Dataset) DatasetLineageNode {
	return b.addNode(DatasetLineageNode{
		ID:           lineageNodeID(DatasetLineageResourceDataset, item.PublicID),
		ResourceType: DatasetLineageResourceDataset,
		PublicID:     item.PublicID,
		DisplayName:  item.Name,
		Status:       item.Status,
		NodeKind:     DatasetLineageNodeKindResource,
		SourceKind:   DatasetLineageSourceMetadata,
		CreatedAt:    timePtr(item.CreatedAt),
		UpdatedAt:    timePtr(item.UpdatedAt),
		Metadata: compactLineageMetadata(map[string]any{
			"sourceKind": item.SourceKind,
			"rowCount":   item.RowCount,
			"rawTable":   item.RawTable,
		}),
	})
}

func (b *datasetLineageBuilder) addQueryJobNode(item DatasetQueryJob) DatasetLineageNode {
	return b.addNode(DatasetLineageNode{
		ID:           lineageNodeID(DatasetLineageResourceQueryJob, item.PublicID),
		ResourceType: DatasetLineageResourceQueryJob,
		PublicID:     item.PublicID,
		DisplayName:  queryStatementPreview(item.Statement),
		Status:       item.Status,
		NodeKind:     DatasetLineageNodeKindResource,
		SourceKind:   DatasetLineageSourceMetadata,
		CreatedAt:    timePtr(item.CreatedAt),
		UpdatedAt:    timePtr(item.UpdatedAt),
		Metadata: compactLineageMetadata(map[string]any{
			"statementPreview": queryStatementPreview(item.Statement),
			"rowCount":         item.RowCount,
			"durationMs":       item.DurationMs,
			"errorSummary":     item.ErrorSummary,
		}),
	})
}

func (b *datasetLineageBuilder) addWorkTableNode(item DatasetWorkTable) DatasetLineageNode {
	return b.addNode(DatasetLineageNode{
		ID:           lineageNodeID(DatasetLineageResourceWorkTable, item.PublicID),
		ResourceType: DatasetLineageResourceWorkTable,
		PublicID:     item.PublicID,
		DisplayName:  firstNonEmpty(item.DisplayName, item.Table),
		Status:       item.Status,
		NodeKind:     DatasetLineageNodeKindResource,
		SourceKind:   DatasetLineageSourceMetadata,
		CreatedAt:    timePtr(item.CreatedAt),
		UpdatedAt:    timePtr(item.UpdatedAt),
		Metadata: compactLineageMetadata(map[string]any{
			"database": item.Database,
			"table":    item.Table,
			"engine":   item.Engine,
			"rowCount": item.TotalRows,
		}),
	})
}

func (b *datasetLineageBuilder) addExportNode(item DatasetWorkTableExport) DatasetLineageNode {
	return b.addNode(DatasetLineageNode{
		ID:           lineageNodeID(DatasetLineageResourceWorkTableExport, item.PublicID),
		ResourceType: DatasetLineageResourceWorkTableExport,
		PublicID:     item.PublicID,
		DisplayName:  strings.ToUpper(item.Format) + " export",
		Status:       item.Status,
		NodeKind:     DatasetLineageNodeKindResource,
		SourceKind:   DatasetLineageSourceMetadata,
		CreatedAt:    timePtr(item.CreatedAt),
		UpdatedAt:    timePtr(item.UpdatedAt),
		Metadata: compactLineageMetadata(map[string]any{
			"format":           item.Format,
			"source":           item.Source,
			"schedulePublicId": item.SchedulePublicID,
			"scheduledFor":     timeValue(item.ScheduledFor),
			"expiresAt":        item.ExpiresAt,
			"errorSummary":     item.ErrorSummary,
		}),
	})
}

func (b *datasetLineageBuilder) addExportScheduleNode(item DatasetWorkTableExportSchedule) DatasetLineageNode {
	return b.addNode(DatasetLineageNode{
		ID:           lineageNodeID(DatasetLineageResourceWorkTableExportSchedule, item.PublicID),
		ResourceType: DatasetLineageResourceWorkTableExportSchedule,
		PublicID:     item.PublicID,
		DisplayName:  item.Frequency + " " + strings.ToUpper(item.Format),
		Status:       lineageScheduleStatus(item),
		NodeKind:     DatasetLineageNodeKindResource,
		SourceKind:   DatasetLineageSourceMetadata,
		CreatedAt:    timePtr(item.CreatedAt),
		UpdatedAt:    timePtr(item.UpdatedAt),
		Metadata: compactLineageMetadata(map[string]any{
			"format":        item.Format,
			"frequency":     item.Frequency,
			"timezone":      item.Timezone,
			"runTime":       item.RunTime,
			"retentionDays": item.RetentionDays,
			"nextRunAt":     item.NextRunAt,
			"lastStatus":    item.LastStatus,
		}),
	})
}

func (b *datasetLineageBuilder) addSyncJobNode(item DatasetSyncJob) DatasetLineageNode {
	return b.addNode(DatasetLineageNode{
		ID:           lineageNodeID(DatasetLineageResourceSyncJob, item.PublicID),
		ResourceType: DatasetLineageResourceSyncJob,
		PublicID:     item.PublicID,
		DisplayName:  item.Mode,
		Status:       item.Status,
		NodeKind:     DatasetLineageNodeKindResource,
		SourceKind:   DatasetLineageSourceMetadata,
		CreatedAt:    timePtr(item.CreatedAt),
		UpdatedAt:    timePtr(item.UpdatedAt),
		Metadata: compactLineageMetadata(map[string]any{
			"mode":         item.Mode,
			"rowCount":     item.RowCount,
			"totalBytes":   item.TotalBytes,
			"errorSummary": item.ErrorSummary,
		}),
	})
}

func (b *datasetLineageBuilder) addNode(node DatasetLineageNode) DatasetLineageNode {
	if existing, ok := b.nodes[node.ID]; ok {
		return existing
	}
	if node.DisplayName == "" {
		node.DisplayName = node.PublicID
	}
	if node.NodeKind == "" {
		node.NodeKind = DatasetLineageNodeKindResource
	}
	if node.SourceKind == "" {
		node.SourceKind = DatasetLineageSourceMetadata
	}
	if node.Metadata != nil && len(node.Metadata) == 0 {
		node.Metadata = nil
	}
	b.nodes[node.ID] = node
	b.nodeOrder = append(b.nodeOrder, node.ID)
	return node
}

func (b *datasetLineageBuilder) addEdge(source, target DatasetLineageNode, relationType string, createdAt time.Time) DatasetLineageEdge {
	id := relationType + ":" + source.ID + "->" + target.ID
	if existing, ok := b.edges[id]; ok {
		return existing
	}
	edge := DatasetLineageEdge{
		ID:           id,
		SourceNodeID: source.ID,
		TargetNodeID: target.ID,
		RelationType: relationType,
		Confidence:   datasetLineageConfidenceMetadata,
		SourceKind:   DatasetLineageSourceMetadata,
		CreatedAt:    timePtr(createdAt),
	}
	b.edges[id] = edge
	b.edgeOrder = append(b.edgeOrder, id)
	return edge
}

func (b *datasetLineageBuilder) addTimeline(node DatasetLineageNode, relationType, status string, occurredAt time.Time, metadata map[string]any) {
	id := relationType + ":" + node.ID + ":" + occurredAt.UTC().Format(time.RFC3339Nano)
	if _, ok := b.timelineIDs[id]; ok {
		return
	}
	b.timelineIDs[id] = struct{}{}
	b.timeline = append(b.timeline, DatasetLineageTimelineItem{
		ID:           id,
		NodeID:       node.ID,
		ResourceType: node.ResourceType,
		PublicID:     node.PublicID,
		RelationType: relationType,
		Status:       status,
		OccurredAt:   timePtr(occurredAt),
		Metadata:     compactLineageMetadata(metadata),
	})
}

func normalizeDatasetLineageOptions(opts DatasetLineageOptions) (DatasetLineageOptions, error) {
	opts.Direction = strings.ToLower(strings.TrimSpace(opts.Direction))
	if opts.Direction == "" {
		opts.Direction = DatasetLineageDirectionBoth
	}
	switch opts.Direction {
	case DatasetLineageDirectionUpstream, DatasetLineageDirectionDownstream, DatasetLineageDirectionBoth:
	default:
		return opts, fmt.Errorf("%w: unsupported lineage direction", ErrInvalidDatasetInput)
	}
	if opts.Depth <= 0 {
		opts.Depth = 1
	}
	if opts.Depth > 2 {
		opts.Depth = 2
	}
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	opts.Level = strings.ToLower(strings.TrimSpace(opts.Level))
	if opts.Level == "" {
		opts.Level = DatasetLineageLevelTable
	}
	switch opts.Level {
	case DatasetLineageLevelTable, DatasetLineageLevelColumn, DatasetLineageLevelBoth:
	default:
		return opts, fmt.Errorf("%w: unsupported lineage level", ErrInvalidDatasetInput)
	}
	if len(opts.Sources) == 0 {
		opts.Sources = []string{DatasetLineageSourceMetadata, DatasetLineageSourceParser, DatasetLineageSourceManual}
	}
	normalizedSources := make([]string, 0, len(opts.Sources))
	seen := map[string]struct{}{}
	for _, source := range opts.Sources {
		source = strings.ToLower(strings.TrimSpace(source))
		if source == "" {
			continue
		}
		switch source {
		case DatasetLineageSourceMetadata, DatasetLineageSourceParser, DatasetLineageSourceManual:
		default:
			return opts, fmt.Errorf("%w: unsupported lineage source", ErrInvalidDatasetInput)
		}
		if _, ok := seen[source]; !ok {
			seen[source] = struct{}{}
			normalizedSources = append(normalizedSources, source)
		}
	}
	if len(normalizedSources) == 0 {
		normalizedSources = []string{DatasetLineageSourceMetadata}
	}
	opts.Sources = normalizedSources
	opts.ChangeSetPublicID = strings.TrimSpace(opts.ChangeSetPublicID)
	return opts, nil
}

func lineageIncludesUpstream(direction string) bool {
	return direction == DatasetLineageDirectionUpstream || direction == DatasetLineageDirectionBoth
}

func lineageIncludesDownstream(direction string) bool {
	return direction == DatasetLineageDirectionDownstream || direction == DatasetLineageDirectionBoth
}

func lineageNodeID(resourceType, publicID string) string {
	return resourceType + ":" + publicID
}

func queryStatementPreview(statement string) string {
	value := strings.Join(strings.Fields(statement), " ")
	if len(value) <= 180 {
		return value
	}
	return value[:177] + "..."
}

func compactLineageMetadata(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				out[key] = typed
			}
		case int32:
			out[key] = typed
		case int64:
			out[key] = typed
		case time.Time:
			if !typed.IsZero() {
				out[key] = typed
			}
		case nil:
		default:
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func timeValue(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

func syncJobUpdatedAt(job DatasetSyncJob) time.Time {
	if job.CompletedAt != nil {
		return *job.CompletedAt
	}
	return job.UpdatedAt
}

func exportUpdatedAt(item DatasetWorkTableExport) time.Time {
	if item.CompletedAt != nil {
		return *item.CompletedAt
	}
	return item.UpdatedAt
}

func lineageScheduleStatus(item DatasetWorkTableExportSchedule) string {
	if item.Enabled {
		return "enabled"
	}
	return "disabled"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
