package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"example.com/haohao/backend/internal/db"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
)

const dataPipelineDriveFileSource = "drive_file"

type dataPipelineMaterializedRelation struct {
	Database    string
	Table       string
	Columns     []string
	Metadata    map[string]any
	ReviewItems []dataPipelineReviewItemDraft
}

type dataPipelineHybridResult struct {
	Relation dataPipelineMaterializedRelation
	Compiled dataPipelineCompiledSelect
	Tables   []string
	Nodes    map[string]dataPipelineRunNodeResult
}

func dataPipelineGraphNeedsHybrid(graph DataPipelineGraph) bool {
	for _, node := range graph.Nodes {
		if node.Data.StepType == DataPipelineStepInput && dataPipelineString(node.Data.Config, "sourceKind") == dataPipelineDriveFileSource {
			return true
		}
		if dataPipelineUnstructuredStep(node.Data.StepType) {
			return true
		}
	}
	return false
}

func dataPipelineUnstructuredStep(stepType string) bool {
	switch stepType {
	case DataPipelineStepExtractText,
		DataPipelineStepJSONExtract,
		DataPipelineStepExcelExtract,
		DataPipelineStepClassifyDocument,
		DataPipelineStepExtractFields,
		DataPipelineStepExtractTable,
		DataPipelineStepProductExtraction,
		DataPipelineStepConfidenceGate,
		DataPipelineStepQuarantine,
		DataPipelineStepRouteByCondition,
		DataPipelineStepDeduplicate,
		DataPipelineStepCanonicalize,
		DataPipelineStepRedactPII,
		DataPipelineStepDetectLanguage,
		DataPipelineStepSchemaInference,
		DataPipelineStepEntityResolution,
		DataPipelineStepUnitConversion,
		DataPipelineStepRelationship,
		DataPipelineStepHumanReview,
		DataPipelineStepSampleCompare,
		DataPipelineStepQualityReport:
		return true
	default:
		return false
	}
}

func (s *DataPipelineService) executeHybridRun(ctx context.Context, tenantID int64, run db.DataPipelineRun, graph DataPipelineGraph) ([]dataPipelineRunOutputResult, error) {
	actorUserID := int64(0)
	if run.RequestedByUserID.Valid {
		actorUserID = run.RequestedByUserID.Int64
	}
	if actorUserID <= 0 {
		return nil, fmt.Errorf("%w: data pipeline OCR nodes require requested_by_user_id or schedule creator", ErrInvalidDataPipelineGraph)
	}
	if err := s.datasets.ensureTenantSandbox(ctx, tenantID); err != nil {
		return nil, err
	}
	conn, err := s.datasets.openTenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	targetDatabase := datasetWorkDatabaseName(tenantID)
	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.exportQuerySettings()))
	outputs := dataPipelineOutputNodes(graph)
	results := make([]dataPipelineRunOutputResult, 0, len(outputs))
	for _, outputNode := range outputs {
		hybrid, err := s.executeHybridGraph(ctx, tenantID, graph, run.PublicID.String()+":"+outputNode.ID, actorUserID, outputNode.ID, "data_pipeline")
		result := dataPipelineRunOutputResult{Node: outputNode, Compiled: hybrid.Compiled, Err: err, NodeResults: hybrid.Nodes}
		if err == nil {
			targetTable := dataPipelineOutputTableName(outputNode, run)
			stageTable := "__dp_stage_" + strings.ReplaceAll(run.PublicID.String(), "-", "") + "_" + dataPipelineSafeSuffix(outputNode.ID)
			_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(stageTable)))
			createSQL := fmt.Sprintf(
				"CREATE TABLE %s.%s ENGINE = MergeTree ORDER BY %s AS\nSELECT * FROM %s.%s",
				quoteCHIdent(targetDatabase),
				quoteCHIdent(stageTable),
				dataPipelineOutputOrderBy(outputNode, hybrid.Relation.Columns),
				quoteCHIdent(hybrid.Relation.Database),
				quoteCHIdent(hybrid.Relation.Table),
			)
			if err := conn.Exec(queryCtx, createSQL); err != nil {
				result.Err = fmt.Errorf("create data pipeline hybrid stage table for %s: %w", outputNode.ID, err)
			} else if err := promoteDataPipelineOutputTable(queryCtx, conn, targetDatabase, targetTable, stageTable, dataPipelineOutputWriteMode(outputNode)); err != nil {
				result.Err = fmt.Errorf("promote data pipeline hybrid stage table for %s: %w", outputNode.ID, err)
			} else {
				displayName := dataPipelineString(outputNode.Data.Config, "displayName")
				if displayName == "" {
					displayName = targetTable
				}
				workTable, err := s.datasets.registerDatasetWorkTableForRef(ctx, tenantID, actorUserID, nil, nil, targetDatabase, targetTable, displayName)
				if err != nil {
					result.Err = err
				} else {
					result.WorkTable = workTable
				}
			}
		}
		for _, table := range hybrid.Tables {
			_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(table)))
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *DataPipelineService) previewHybridGraph(ctx context.Context, tenantID, actorUserID int64, graph DataPipelineGraph, nodeID string, limit int32) (DataPipelinePreview, error) {
	if actorUserID <= 0 {
		return DataPipelinePreview{}, fmt.Errorf("%w: data pipeline OCR preview requires an actor user", ErrInvalidDataPipelineGraph)
	}
	if limit <= 0 || limit > datasetPreviewRowLimit {
		limit = 100
	}
	result, err := s.executeHybridGraph(ctx, tenantID, graph, "preview:"+uuid.NewString(), actorUserID, nodeID, "data_pipeline_preview")
	if err != nil {
		return DataPipelinePreview{}, err
	}
	defer s.dropHybridTables(context.Background(), tenantID, result.Tables)
	if err := s.datasets.ensureTenantSandbox(ctx, tenantID); err != nil {
		return DataPipelinePreview{}, err
	}
	conn, err := s.datasets.openTenantConn(ctx, tenantID)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	defer conn.Close()
	sql := fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d", quoteCHIdent(result.Relation.Database), quoteCHIdent(result.Relation.Table), limit)
	rows, err := conn.Query(clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.querySettings())), sql)
	if err != nil {
		return DataPipelinePreview{}, fmt.Errorf("preview hybrid data pipeline: %w", err)
	}
	defer rows.Close()
	columns, previewRows, err := scanDatasetRows(rows, int(limit))
	if err != nil {
		return DataPipelinePreview{}, err
	}
	return DataPipelinePreview{
		NodeID:      result.Compiled.NodeID,
		StepType:    result.Compiled.StepType,
		Columns:     columns,
		PreviewRows: previewRows,
	}, nil
}

func (s *DataPipelineService) dropHybridTables(ctx context.Context, tenantID int64, tables []string) {
	if s == nil || s.datasets == nil || len(tables) == 0 {
		return
	}
	conn, err := s.datasets.openTenantConn(ctx, tenantID)
	if err != nil {
		return
	}
	defer conn.Close()
	database := datasetWorkDatabaseName(tenantID)
	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.exportQuerySettings()))
	for _, table := range tables {
		_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(database), quoteCHIdent(table)))
	}
}

func (s *DataPipelineService) executeHybridGraph(ctx context.Context, tenantID int64, graph DataPipelineGraph, runKey string, actorUserID int64, selectedNodeID, ocrReason string) (dataPipelineHybridResult, error) {
	if s == nil || s.datasets == nil {
		return dataPipelineHybridResult{}, fmt.Errorf("data pipeline service is not configured")
	}
	if err := s.datasets.ensureTenantSandbox(ctx, tenantID); err != nil {
		return dataPipelineHybridResult{}, err
	}
	conn, err := s.datasets.openTenantConn(ctx, tenantID)
	if err != nil {
		return dataPipelineHybridResult{}, err
	}
	defer conn.Close()

	order, err := dataPipelineTopologicalOrder(graph)
	if err != nil {
		return dataPipelineHybridResult{}, err
	}
	database := datasetWorkDatabaseName(tenantID)
	prefix := dataPipelineHybridTablePrefix(runKey)
	relations := make(map[string]dataPipelineMaterializedRelation, len(order))
	tables := make([]string, 0, len(order))
	nodeResults := make(map[string]dataPipelineRunNodeResult, len(order))
	incoming := dataPipelineIncomingNodeIDs(graph)
	compiler := newDataPipelineCompiler(s, tenantID, graph)
	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.exportQuerySettings()))
	for _, node := range order {
		table := prefix + "_" + dataPipelineCTEName(node.ID)
		_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(database), quoteCHIdent(table)))
		relation, err := s.materializeHybridNode(ctx, conn, database, table, node, relations, compiler, tenantID, actorUserID, ocrReason)
		if err != nil {
			for _, cleanup := range tables {
				_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(database), quoteCHIdent(cleanup)))
			}
			return dataPipelineHybridResult{}, err
		}
		rowCount, queryStats, err := countHybridRelationRowsWithStats(ctx, conn, relation, dataPipelineQueryID(runKey, node.ID, "count"))
		if err != nil {
			_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(database), quoteCHIdent(table)))
			for _, cleanup := range tables {
				_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(database), quoteCHIdent(cleanup)))
			}
			return dataPipelineHybridResult{}, err
		}
		metadata := relation.Metadata
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["inputRows"] = dataPipelineInputRows(node.ID, incoming, nodeResults, rowCount)
		metadata["outputRows"] = rowCount
		metadata["queryStats"] = queryStats
		if _, ok := metadata["warnings"]; !ok {
			metadata["warnings"] = []string{}
		}
		compiled := dataPipelineCompiledSelect{
			SQL:      fmt.Sprintf("SELECT * FROM %s.%s", quoteCHIdent(relation.Database), quoteCHIdent(relation.Table)),
			Columns:  relation.Columns,
			NodeID:   node.ID,
			StepType: node.Data.StepType,
		}
		switch node.Data.StepType {
		case DataPipelineStepProfile:
			profile, err := s.collectStructuredProfileMetadata(ctx, conn, compiled, node)
			if err != nil {
				_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(database), quoteCHIdent(table)))
				for _, cleanup := range tables {
					_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(database), quoteCHIdent(cleanup)))
				}
				return dataPipelineHybridResult{}, fmt.Errorf("profile data pipeline step %s: %w", node.ID, err)
			}
			metadata["profile"] = profile
		case DataPipelineStepValidate:
			validation, err := s.collectStructuredValidationMetadata(ctx, conn, compiled, node)
			if err != nil {
				_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(database), quoteCHIdent(table)))
				for _, cleanup := range tables {
					_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(database), quoteCHIdent(cleanup)))
				}
				return dataPipelineHybridResult{}, fmt.Errorf("validate data pipeline step %s: %w", node.ID, err)
			}
			metadata["validation"] = validation
			if failedRows, ok := validation["failedRows"].(int64); ok {
				metadata["failedRows"] = failedRows
			}
			if warningCount, ok := validation["warningCount"].(int64); ok {
				metadata["warningCount"] = warningCount
			}
		}
		nodeResults[node.ID] = dataPipelineRunNodeResult{
			NodeID:      node.ID,
			StepType:    node.Data.StepType,
			RowCount:    rowCount,
			Metadata:    metadata,
			ReviewItems: relation.ReviewItems,
		}
		relations[node.ID] = relation
		tables = append(tables, table)
		if selectedNodeID != "" && node.ID == selectedNodeID {
			return dataPipelineHybridResult{
				Relation: relation,
				Compiled: dataPipelineCompiledSelect{
					SQL:      fmt.Sprintf("SELECT * FROM %s.%s", quoteCHIdent(relation.Database), quoteCHIdent(relation.Table)),
					Columns:  relation.Columns,
					NodeID:   node.ID,
					StepType: node.Data.StepType,
				},
				Tables: tables,
				Nodes:  nodeResults,
			}, nil
		}
	}
	if selectedNodeID != "" {
		for _, cleanup := range tables {
			_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(database), quoteCHIdent(cleanup)))
		}
		return dataPipelineHybridResult{}, fmt.Errorf("%w: selected node not found", ErrInvalidDataPipelineGraph)
	}
	output := dataPipelineOutputNode(graph)
	relation, ok := relations[output.ID]
	if !ok {
		return dataPipelineHybridResult{}, fmt.Errorf("%w: selected output node not found", ErrInvalidDataPipelineGraph)
	}
	return dataPipelineHybridResult{
		Relation: relation,
		Compiled: dataPipelineCompiledSelect{
			SQL:      fmt.Sprintf("SELECT * FROM %s.%s", quoteCHIdent(relation.Database), quoteCHIdent(relation.Table)),
			Columns:  relation.Columns,
			NodeID:   output.ID,
			StepType: output.Data.StepType,
		},
		Tables: tables,
		Nodes:  nodeResults,
	}, nil
}

func (s *DataPipelineService) materializeHybridNode(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, relations map[string]dataPipelineMaterializedRelation, compiler *dataPipelineCompiler, tenantID, actorUserID int64, ocrReason string) (dataPipelineMaterializedRelation, error) {
	switch node.Data.StepType {
	case DataPipelineStepInput:
		if dataPipelineString(node.Data.Config, "sourceKind") == dataPipelineDriveFileSource {
			switch dataPipelineDriveInputMode(node.Data.Config) {
			case dataPipelineDriveInputModeSpreadsheet:
				return s.materializeDriveSpreadsheetInput(ctx, conn, database, table, node, tenantID, actorUserID)
			case dataPipelineDriveInputModeJSON:
				return s.materializeDriveJSONInput(ctx, conn, database, table, node, tenantID, actorUserID)
			}
			return s.materializeDriveFileInput(ctx, conn, database, table, node)
		}
	case DataPipelineStepExtractText:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeExtractText(ctx, conn, database, table, node, upstream, tenantID, actorUserID, ocrReason)
	case DataPipelineStepJSONExtract:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeJSONExtract(ctx, conn, database, table, node, upstream)
	case DataPipelineStepExcelExtract:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeExcelExtract(ctx, conn, database, table, node, upstream, tenantID, actorUserID)
	case DataPipelineStepClassifyDocument:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeClassifyDocument(ctx, conn, database, table, node, upstream)
	case DataPipelineStepExtractFields:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeExtractFields(ctx, conn, database, table, node, upstream)
	case DataPipelineStepExtractTable:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeExtractTable(ctx, conn, database, table, node, upstream)
	case DataPipelineStepProductExtraction:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeProductExtraction(ctx, conn, database, table, node, upstream, tenantID, actorUserID)
	case DataPipelineStepSchemaMapping:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeSchemaMapping(ctx, conn, database, table, node, upstream)
	case DataPipelineStepConfidenceGate:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeConfidenceGate(ctx, conn, database, table, node, upstream)
	case DataPipelineStepQuarantine:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeQuarantine(ctx, conn, database, table, node, upstream)
	case DataPipelineStepRouteByCondition:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeRouteByCondition(ctx, conn, database, table, node, upstream)
	case DataPipelineStepDeduplicate:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeDeduplicate(ctx, conn, database, table, node, upstream)
	case DataPipelineStepCanonicalize:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeCanonicalize(ctx, conn, database, table, node, upstream)
	case DataPipelineStepRedactPII:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeRedactPII(ctx, conn, database, table, node, upstream)
	case DataPipelineStepDetectLanguage:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeDetectLanguage(ctx, conn, database, table, node, upstream)
	case DataPipelineStepSchemaInference:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeSchemaInference(ctx, conn, database, table, node, upstream)
	case DataPipelineStepEntityResolution:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeEntityResolution(ctx, conn, database, table, node, upstream)
	case DataPipelineStepUnitConversion:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeUnitConversion(ctx, conn, database, table, node, upstream)
	case DataPipelineStepRelationship:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeRelationshipExtraction(ctx, conn, database, table, node, upstream)
	case DataPipelineStepHumanReview:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeHumanReview(ctx, conn, database, table, node, upstream)
	case DataPipelineStepSampleCompare:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeSampleCompare(ctx, conn, database, table, node, upstream)
	case DataPipelineStepQualityReport:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeQualityReport(ctx, conn, database, table, node, upstream)
	}

	sqlRelations := make(map[string]dataPipelineRelation, len(relations))
	for id, relation := range relations {
		sqlRelations[id] = dataPipelineRelation{
			CTE:     relation.Table,
			Columns: relation.Columns,
			Node:    compiler.nodes[id],
		}
	}
	relation, err := compiler.compileNode(ctx, node, sqlRelations)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.exportQuerySettings()))
	sql := fmt.Sprintf("CREATE TABLE %s.%s ENGINE = MergeTree ORDER BY tuple() AS\n%s", quoteCHIdent(database), quoteCHIdent(table), relation.SQL)
	if err := conn.Exec(queryCtx, sql); err != nil {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("materialize data pipeline node %s: %w", node.ID, err)
	}
	metadata, err := dataPipelineHybridCompiledNodeMetadata(ctx, conn, node, relation.Columns, database, table)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: relation.Columns, Metadata: metadata}, nil
}

func dataPipelineHybridCompiledNodeMetadata(ctx context.Context, conn driver.Conn, node DataPipelineNode, columns []string, database, table string) (map[string]any, error) {
	switch node.Data.StepType {
	case DataPipelineStepPartitionFilter:
		spec, err := dataPipelinePartitionFilterSpec(node.Data.Config, columns)
		if err != nil {
			return nil, err
		}
		return map[string]any{"partitionFilter": spec.Metadata()}, nil
	case DataPipelineStepWatermarkFilter:
		spec, err := dataPipelineWatermarkFilterSpec(node.Data.Config, columns)
		if err != nil {
			return nil, err
		}
		metadata := spec.Metadata()
		nextWatermark, err := queryDataPipelineMaxValue(ctx, conn, fmt.Sprintf("SELECT * FROM %s.%s", quoteCHIdent(database), quoteCHIdent(table)), spec.Column, spec.ValueType)
		if err != nil {
			return nil, err
		}
		if nextWatermark == "" {
			nextWatermark = spec.WatermarkValue
		}
		metadata["nextWatermarkValue"] = nextWatermark
		return map[string]any{"watermarkFilter": metadata}, nil
	case DataPipelineStepSnapshotSCD2:
		spec, err := dataPipelineSnapshotSCD2Spec(node.Data.Config, dataPipelineSnapshotSCD2SourceColumns(node.Data.Config, columns))
		if err != nil {
			return nil, err
		}
		return map[string]any{"snapshotSCD2": spec.Metadata()}, nil
	default:
		return nil, nil
	}
}

func materializedSingleUpstream(node DataPipelineNode, incoming map[string][]string, relations map[string]dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	sources := incoming[node.ID]
	if len(sources) != 1 {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: node %s must have exactly one upstream edge", ErrInvalidDataPipelineGraph, node.ID)
	}
	relation, ok := relations[sources[0]]
	if !ok {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: upstream node is not materialized: %s", ErrInvalidDataPipelineGraph, sources[0])
	}
	return relation, nil
}

func dataPipelineOutputNode(graph DataPipelineGraph) DataPipelineNode {
	for _, node := range graph.Nodes {
		if node.Data.StepType == DataPipelineStepOutput {
			return node
		}
	}
	if len(graph.Nodes) > 0 {
		return graph.Nodes[len(graph.Nodes)-1]
	}
	return DataPipelineNode{ID: "output", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}}
}

func dataPipelineHybridTablePrefix(runKey string) string {
	raw := strings.ReplaceAll(strings.TrimSpace(runKey), "-", "")
	if strings.HasPrefix(raw, "preview:") {
		raw = strings.TrimPrefix(raw, "preview:")
		if raw == "" {
			raw = strings.ReplaceAll(uuid.NewString(), "-", "")
		}
		if len(raw) > 18 {
			raw = raw[:18]
		}
		return "__dp_preview_" + raw
	}
	if raw == "" {
		raw = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	if len(raw) > 18 {
		raw = raw[:18]
	}
	return "__dp_node_" + raw
}

func (s *DataPipelineService) materializeDriveFileInput(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode) (dataPipelineMaterializedRelation, error) {
	columns := []string{"file_public_id", "file_name", "mime_type", "file_revision"}
	rows := make([]map[string]any, 0)
	for _, publicID := range dataPipelineStringSlice(node.Data.Config, "filePublicIds") {
		rows = append(rows, map[string]any{
			"file_public_id": publicID,
			"file_name":      "",
			"mime_type":      "",
			"file_revision":  "",
		})
	}
	if len(rows) == 0 {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: drive_file input requires filePublicIds", ErrInvalidDataPipelineGraph)
	}
	if len(rows) > 50 {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: drive_file input cannot contain more than 50 files", ErrInvalidDataPipelineGraph)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, rows); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeExtractText(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation, tenantID, actorUserID int64, ocrReason string) (dataPipelineMaterializedRelation, error) {
	if s == nil || s.driveOCR == nil {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("Drive OCR service is not configured")
	}
	rows, err := readHybridRows(ctx, conn, upstream, 5000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	columns := []string{"file_public_id", "ocr_run_public_id", "page_number", "text", "confidence", "layout_json", "boxes_json"}
	out := make([]map[string]any, 0)
	seen := map[string]struct{}{}
	for _, row := range rows {
		filePublicID := strings.TrimSpace(fmt.Sprint(row["file_public_id"]))
		if filePublicID == "" {
			continue
		}
		if _, ok := seen[filePublicID]; ok {
			continue
		}
		seen[filePublicID] = struct{}{}
		result, err := s.driveOCR.EnsureCompletedForPipeline(ctx, DriveOCRPipelineRequest{
			TenantID:     tenantID,
			ActorUserID:  actorUserID,
			FilePublicID: filePublicID,
			Reason:       firstNonEmpty(ocrReason, "data_pipeline"),
			OCREngine:    dataPipelineString(node.Data.Config, "ocrEngine"),
			OCRLanguages: dataPipelineStringSlice(node.Data.Config, "languages"),
			IncludeBoxes: dataPipelineBool(node.Data.Config, "includeBoxes", true),
		})
		if err != nil {
			return dataPipelineMaterializedRelation{}, fmt.Errorf("extract_text %s: %w", filePublicID, err)
		}
		chunkMode := dataPipelineString(node.Data.Config, "chunkMode")
		if chunkMode == "full_text" {
			out = append(out, map[string]any{
				"file_public_id":    filePublicID,
				"ocr_run_public_id": result.Run.PublicID,
				"page_number":       "0",
				"text":              result.Run.ExtractedText,
				"confidence":        floatString(result.Run.AverageConfidence),
				"layout_json":       "{}",
				"boxes_json":        "[]",
			})
			continue
		}
		for _, page := range result.Pages {
			boxesJSON := []byte("[]")
			if dataPipelineBool(node.Data.Config, "includeBoxes", true) {
				boxesJSON = defaultJSON(page.BoxesJSON, "[]")
			}
			out = append(out, map[string]any{
				"file_public_id":    filePublicID,
				"ocr_run_public_id": result.Run.PublicID,
				"page_number":       strconv.Itoa(page.PageNumber),
				"text":              page.RawText,
				"confidence":        floatString(page.AverageConfidence),
				"layout_json":       string(defaultJSON(page.LayoutJSON, "{}")),
				"boxes_json":        string(boxesJSON),
			})
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeClassifyDocument(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	labelCol := firstNonEmpty(dataPipelineString(node.Data.Config, "outputColumn"), "document_type")
	confCol := firstNonEmpty(dataPipelineString(node.Data.Config, "confidenceColumn"), "document_type_confidence")
	reasonCol := "document_type_reason"
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), labelCol, confCol, reasonCol))
	out := make([]map[string]any, 0, len(rows))
	classes := dataPipelineConfigObjects(node.Data.Config, "classes")
	for _, row := range rows {
		next := cloneRow(row)
		label, confidence, reason := classifyText(fmt.Sprint(row["text"]), classes)
		next[labelCol] = label
		next[confCol] = fmt.Sprintf("%.4f", confidence)
		next[reasonCol] = reason
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeExtractFields(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	fields := dataPipelineConfigObjects(node.Data.Config, "fields")
	fieldNames := make([]string, 0, len(fields))
	for _, field := range fields {
		name := dataPipelineString(field, "name")
		if err := dataPipelineValidateIdentifier(name); err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		fieldNames = append(fieldNames, name)
	}
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), append(fieldNames, "fields_json", "evidence_json", "field_confidence")...))
	out := make([]map[string]any, 0, len(rows))
	fieldStats := make(map[string]map[string]any, len(fieldNames))
	for _, name := range fieldNames {
		fieldStats[name] = map[string]any{"extractedRows": int64(0), "missingRows": int64(0)}
	}
	lowConfidenceSamples := make([]map[string]any, 0)
	lowConfidenceRows := int64(0)
	missingRequiredRows := int64(0)
	confidenceTotal := 0.0
	for _, row := range rows {
		next := cloneRow(row)
		text := fmt.Sprint(row["text"])
		fieldsJSON := map[string]any{}
		evidence := []map[string]any{}
		scoreTotal := 0.0
		scoreCount := 0.0
		missingRequired := []string{}
		for _, field := range fields {
			name := dataPipelineString(field, "name")
			value, ok, source := extractFieldValue(text, field)
			if ok {
				next[name] = value
				fieldsJSON[name] = value
				evidence = append(evidence, map[string]any{"field": name, "sourceText": source})
				if stats, ok := fieldStats[name]; ok {
					stats["extractedRows"] = stats["extractedRows"].(int64) + 1
				}
				scoreTotal += 1
			} else {
				next[name] = ""
				fieldsJSON[name] = nil
				if stats, ok := fieldStats[name]; ok {
					stats["missingRows"] = stats["missingRows"].(int64) + 1
				}
				if dataPipelineBool(field, "required", false) {
					evidence = append(evidence, map[string]any{"field": name, "missing": true})
					missingRequired = append(missingRequired, name)
				}
			}
			scoreCount++
		}
		next["fields_json"] = jsonString(fieldsJSON)
		next["evidence_json"] = jsonString(evidence)
		confidence := 1.0
		if scoreCount > 0 {
			confidence = scoreTotal / scoreCount
		}
		next["field_confidence"] = fmt.Sprintf("%.4f", confidence)
		confidenceTotal += confidence
		if confidence < 1 {
			lowConfidenceRows++
			if len(lowConfidenceSamples) < 5 {
				lowConfidenceSamples = append(lowConfidenceSamples, map[string]any{
					"field_confidence": fmt.Sprintf("%.4f", confidence),
					"missingRequired":  missingRequired,
					"evidence":         evidence,
				})
			}
		}
		if len(missingRequired) > 0 {
			missingRequiredRows++
		}
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	avgConfidence := 1.0
	if len(rows) > 0 {
		avgConfidence = confidenceTotal / float64(len(rows))
	}
	metadata := map[string]any{
		"fieldExtraction": map[string]any{
			"fieldCount":           len(fieldNames),
			"rowCount":             len(rows),
			"avgConfidence":        avgConfidence,
			"lowConfidenceRows":    lowConfidenceRows,
			"missingRequiredRows":  missingRequiredRows,
			"fields":               fieldStats,
			"lowConfidenceSamples": lowConfidenceSamples,
		},
	}
	if lowConfidenceRows > 0 {
		metadata["warningCount"] = lowConfidenceRows
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns, Metadata: metadata}, nil
}

func (s *DataPipelineService) materializeExtractTable(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	delimiter := dataPipelineString(node.Data.Config, "delimiter")
	if delimiter == "" {
		delimiter = ","
	}
	expectedColumnCount := dataPipelineInt(node.Data.Config, "expectedColumnCount", 0)
	tableColumns := []string{"table_id", "row_number", "row_json", "source_text", "table_column_count", "table_missing_cell_count", "table_confidence"}
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), tableColumns...))
	out := make([]map[string]any, 0)
	lowConfidenceRows := int64(0)
	confidenceTotal := 0.0
	lowConfidenceSamples := make([]map[string]any, 0)
	for _, row := range rows {
		lines := strings.Split(fmt.Sprint(row["text"]), "\n")
		rowNumber := 0
		for _, line := range lines {
			if !strings.Contains(line, delimiter) {
				continue
			}
			parts := strings.Split(line, delimiter)
			if len(parts) < 2 {
				continue
			}
			rowNumber++
			columnCount := len(parts)
			effectiveColumnCount := expectedColumnCount
			if effectiveColumnCount <= 0 {
				effectiveColumnCount = columnCount
			}
			values := map[string]any{}
			nonEmptyCells := 0
			for i, part := range parts {
				value := strings.TrimSpace(part)
				values[fmt.Sprintf("column_%d", i+1)] = value
				if value != "" {
					nonEmptyCells++
				}
			}
			missingCells := effectiveColumnCount - nonEmptyCells
			if missingCells < 0 {
				missingCells = 0
			}
			confidence := 1.0
			if effectiveColumnCount > 0 {
				confidence = float64(nonEmptyCells) / float64(effectiveColumnCount)
				if confidence > 1 {
					confidence = 1
				}
			}
			confidenceTotal += confidence
			if confidence < 1 {
				lowConfidenceRows++
				if len(lowConfidenceSamples) < 5 {
					lowConfidenceSamples = append(lowConfidenceSamples, map[string]any{
						"row_number":               rowNumber,
						"table_confidence":         fmt.Sprintf("%.4f", confidence),
						"table_column_count":       columnCount,
						"table_missing_cell_count": missingCells,
						"source_text":              line,
					})
				}
			}
			next := cloneRow(row)
			next["table_id"] = fmt.Sprintf("%s:%v", row["file_public_id"], row["page_number"])
			next["row_number"] = strconv.Itoa(rowNumber)
			next["row_json"] = jsonString(values)
			next["source_text"] = line
			next["table_column_count"] = strconv.Itoa(columnCount)
			next["table_missing_cell_count"] = strconv.Itoa(missingCells)
			next["table_confidence"] = fmt.Sprintf("%.4f", confidence)
			out = append(out, next)
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	avgConfidence := 1.0
	if len(out) > 0 {
		avgConfidence = confidenceTotal / float64(len(out))
	}
	metadata := map[string]any{
		"tableExtraction": map[string]any{
			"rowCount":             len(out),
			"expectedColumnCount":  expectedColumnCount,
			"avgConfidence":        avgConfidence,
			"lowConfidenceRows":    lowConfidenceRows,
			"lowConfidenceSamples": lowConfidenceSamples,
		},
	}
	if lowConfidenceRows > 0 {
		metadata["warningCount"] = lowConfidenceRows
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns, Metadata: metadata}, nil
}

func (s *DataPipelineService) materializeProductExtraction(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation, tenantID, actorUserID int64) (dataPipelineMaterializedRelation, error) {
	if s.driveOCR == nil {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("drive product extraction service is not configured")
	}
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	sourceFileColumn := firstNonEmpty(dataPipelineString(node.Data.Config, "sourceFileColumn"), "file_public_id")
	if err := dataPipelineRequireColumn(upstream.Columns, sourceFileColumn); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	includeSourceColumns := dataPipelineBool(node.Data.Config, "includeSourceColumns", true)
	maxItems := dataPipelineInt(node.Data.Config, "maxItems", 1000)
	if maxItems <= 0 || maxItems > 10000 {
		maxItems = 1000
	}
	threshold := dataPipelineFloat(node.Data.Config, "confidenceThreshold", 0.8)
	productColumns := []string{
		"product_extraction_item_public_id",
		"product_item_type",
		"product_name",
		"product_brand",
		"product_manufacturer",
		"product_model",
		"product_sku",
		"product_jan_code",
		"product_category",
		"product_description",
		"product_price_json",
		"product_promotion_json",
		"product_availability_json",
		"product_source_text",
		"product_evidence_json",
		"product_attributes_json",
		"product_confidence",
		"product_extraction_status",
		"product_extraction_reason",
	}
	columns := append([]string{}, productColumns...)
	if includeSourceColumns {
		columns = uniqueStringList(append(append([]string{}, upstream.Columns...), productColumns...))
	}
	out := make([]map[string]any, 0)
	itemCount := 0
	fileCount := 0
	lowConfidenceItems := 0
	missingConfidenceItems := 0
	confidenceTotal := 0.0
	confidenceCount := 0
	lowConfidenceSamples := make([]map[string]any, 0, 5)
	cache := map[string][]DriveProductExtractionItem{}
	for _, row := range rows {
		filePublicID := strings.TrimSpace(fmt.Sprint(row[sourceFileColumn]))
		if filePublicID == "" {
			continue
		}
		items, ok := cache[filePublicID]
		if !ok {
			items, err = s.driveOCR.ListProductExtractions(ctx, tenantID, actorUserID, filePublicID)
			if err != nil {
				return dataPipelineMaterializedRelation{}, err
			}
			cache[filePublicID] = items
			fileCount++
		}
		for _, item := range items {
			if itemCount >= maxItems {
				break
			}
			next := make(map[string]any, len(columns))
			if includeSourceColumns {
				next = cloneRow(row)
			}
			confidence := ""
			status := "needs_review"
			reason := "confidence_missing"
			if item.Confidence != nil {
				confidence = fmt.Sprintf("%.4f", clampConfidence(*item.Confidence))
				confidenceTotal += clampConfidence(*item.Confidence)
				confidenceCount++
				if clampConfidence(*item.Confidence) >= threshold {
					status = "pass"
					reason = "passed"
				} else {
					reason = "below_threshold"
				}
			} else {
				missingConfidenceItems++
			}
			next["product_extraction_item_public_id"] = item.PublicID
			next["product_item_type"] = item.ItemType
			next["product_name"] = item.Name
			next["product_brand"] = item.Brand
			next["product_manufacturer"] = item.Manufacturer
			next["product_model"] = item.Model
			next["product_sku"] = item.SKU
			next["product_jan_code"] = item.JANCode
			next["product_category"] = item.Category
			next["product_description"] = item.Description
			next["product_price_json"] = jsonString(item.Price)
			next["product_promotion_json"] = jsonString(item.Promotion)
			next["product_availability_json"] = jsonString(item.Availability)
			next["product_source_text"] = item.SourceText
			next["product_evidence_json"] = jsonString(item.Evidence)
			next["product_attributes_json"] = jsonString(item.Attributes)
			next["product_confidence"] = confidence
			next["product_extraction_status"] = status
			next["product_extraction_reason"] = reason
			if status == "needs_review" {
				lowConfidenceItems++
				if len(lowConfidenceSamples) < 5 {
					lowConfidenceSamples = append(lowConfidenceSamples, map[string]any{
						"file_public_id":                    filePublicID,
						"product_extraction_item_public_id": item.PublicID,
						"product_name":                      item.Name,
						"product_confidence":                confidence,
						"product_extraction_reason":         reason,
					})
				}
			}
			out = append(out, next)
			itemCount++
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	avgConfidence := 0.0
	if confidenceCount > 0 {
		avgConfidence = confidenceTotal / float64(confidenceCount)
	}
	metadata := map[string]any{
		"productExtraction": map[string]any{
			"fileCount":              fileCount,
			"itemCount":              itemCount,
			"threshold":              threshold,
			"avgConfidence":          avgConfidence,
			"lowConfidenceItems":     lowConfidenceItems,
			"missingConfidenceItems": missingConfidenceItems,
			"lowConfidenceSamples":   lowConfidenceSamples,
			"sourceFileColumn":       sourceFileColumn,
			"includeSourceColumns":   includeSourceColumns,
			"maxItems":               maxItems,
		},
	}
	if lowConfidenceItems > 0 {
		metadata["warningCount"] = lowConfidenceItems
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns, Metadata: metadata}, nil
}

func (s *DataPipelineService) materializeSchemaMapping(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	mappings := dataPipelineConfigObjects(node.Data.Config, "mappings")
	if len(mappings) == 0 {
		return materializePassThrough(ctx, conn, database, table, upstream)
	}
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	threshold := dataPipelineFloat(node.Data.Config, "confidenceThreshold", 0.8)
	scoreCol := firstNonEmpty(dataPipelineString(node.Data.Config, "scoreColumn"), "schema_mapping_confidence")
	statusCol := firstNonEmpty(dataPipelineString(node.Data.Config, "statusColumn"), "schema_mapping_status")
	reasonCol := firstNonEmpty(dataPipelineString(node.Data.Config, "reasonColumn"), "schema_mapping_reason")
	mappingJSONCol := firstNonEmpty(dataPipelineString(node.Data.Config, "mappingJSONColumn"), "schema_mapping_json")
	includeSourceColumns := dataPipelineBool(node.Data.Config, "includeSourceColumns", false)
	columns := make([]string, 0, len(mappings)+4)
	if includeSourceColumns {
		columns = append(columns, upstream.Columns...)
	}
	for _, mapping := range mappings {
		target := dataPipelineString(mapping, "targetColumn")
		if err := dataPipelineValidateIdentifier(target); err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		if source := dataPipelineString(mapping, "sourceColumn"); source != "" {
			if err := dataPipelineRequireColumn(upstream.Columns, source); err != nil {
				return dataPipelineMaterializedRelation{}, err
			}
		}
		columns = append(columns, target)
	}
	columns = uniqueStringList(append(columns, scoreCol, statusCol, reasonCol, mappingJSONCol))

	out := make([]map[string]any, 0, len(rows))
	lowConfidenceRows := 0
	missingRequiredRows := 0
	lowConfidenceSamples := make([]map[string]any, 0, 5)
	for _, row := range rows {
		next := make(map[string]any, len(columns))
		if includeSourceColumns {
			next = cloneRow(row)
		}
		mappingDetails := make([]map[string]any, 0, len(mappings))
		confidenceTotal := 0.0
		requiredMissing := false
		for _, mapping := range mappings {
			target := dataPipelineString(mapping, "targetColumn")
			source := dataPipelineString(mapping, "sourceColumn")
			value := any("")
			sourceFound := false
			if source != "" {
				value = row[source]
				sourceFound = true
			} else if defaultValue, ok := mapping["defaultValue"]; ok {
				value = defaultValue
			}
			required := dataPipelineBool(mapping, "required", false)
			valueText := strings.TrimSpace(fmt.Sprint(value))
			missing := required && valueText == ""
			confidence := dataPipelineFloat(mapping, "confidence", dataPipelineFloat(mapping, "mappingConfidence", 1.0))
			reason := "mapped"
			if !sourceFound && source == "" {
				reason = "default_value"
			}
			if missing {
				confidence = 0
				reason = "required_value_missing"
				requiredMissing = true
			}
			confidence = clampConfidence(confidence)
			next[target] = value
			confidenceTotal += confidence
			mappingDetails = append(mappingDetails, map[string]any{
				"targetColumn": target,
				"sourceColumn": source,
				"confidence":   fmt.Sprintf("%.4f", confidence),
				"reason":       reason,
				"required":     required,
			})
		}
		avgConfidence := 1.0
		if len(mappings) > 0 {
			avgConfidence = confidenceTotal / float64(len(mappings))
		}
		status := "pass"
		reason := "passed"
		if requiredMissing {
			status = "needs_review"
			reason = "required_mapping_missing"
			missingRequiredRows++
		} else if avgConfidence < threshold {
			status = "needs_review"
			reason = "below_threshold"
		}
		if status == "needs_review" {
			lowConfidenceRows++
			if len(lowConfidenceSamples) < 5 {
				lowConfidenceSamples = append(lowConfidenceSamples, map[string]any{
					scoreCol:   fmt.Sprintf("%.4f", avgConfidence),
					statusCol:  status,
					reasonCol:  reason,
					"mappings": mappingDetails,
				})
			}
		}
		next[scoreCol] = fmt.Sprintf("%.4f", avgConfidence)
		next[statusCol] = status
		next[reasonCol] = reason
		next[mappingJSONCol] = jsonString(mappingDetails)
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	metadata := map[string]any{
		"schemaMapping": map[string]any{
			"rowCount":             len(out),
			"mappingCount":         len(mappings),
			"threshold":            threshold,
			"lowConfidenceRows":    lowConfidenceRows,
			"missingRequiredRows":  missingRequiredRows,
			"lowConfidenceSamples": lowConfidenceSamples,
		},
	}
	if lowConfidenceRows > 0 {
		metadata["warningCount"] = lowConfidenceRows
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns, Metadata: metadata}, nil
}

func (s *DataPipelineService) materializeConfidenceGate(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	threshold := dataPipelineFloat(node.Data.Config, "threshold", 0.8)
	statusCol := firstNonEmpty(dataPipelineString(node.Data.Config, "statusColumn"), "gate_status")
	scoreCol := "gate_score"
	reasonCol := "gate_reason"
	scoreColumns := dataPipelineStringSlice(node.Data.Config, "scoreColumns")
	if len(scoreColumns) == 0 {
		for _, candidate := range []string{"confidence", "field_confidence", "document_type_confidence"} {
			if dataPipelineHasColumn(upstream.Columns, candidate) {
				scoreColumns = append(scoreColumns, candidate)
			}
		}
	}
	exprs := dataPipelineColumnExpressions(upstream.Columns)
	scoreExpr := "1.0"
	missingScoreExpr := "0"
	invalidScoreExpr := "0"
	if len(scoreColumns) > 0 {
		parts := make([]string, 0, len(scoreColumns))
		missingParts := make([]string, 0, len(scoreColumns))
		invalidParts := make([]string, 0, len(scoreColumns))
		for _, column := range scoreColumns {
			if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
				return dataPipelineMaterializedRelation{}, err
			}
			valueExpr := "toString(" + quoteCHIdent(column) + ")"
			trimmedExpr := "trim(" + valueExpr + ")"
			parts = append(parts, "toFloat64OrZero("+valueExpr+")")
			missingParts = append(missingParts, trimmedExpr+" = ''")
			invalidParts = append(invalidParts, trimmedExpr+" != '' AND isNull(toFloat64OrNull("+valueExpr+"))")
		}
		scoreExpr = "(" + strings.Join(parts, " + ") + ") / " + strconv.Itoa(len(parts))
		missingScoreExpr = strings.Join(missingParts, " AND ")
		invalidScoreExpr = strings.Join(invalidParts, " OR ")
	}
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), scoreCol, statusCol, reasonCol))
	exprs[scoreCol] = scoreExpr
	exprs[statusCol] = fmt.Sprintf("if(%s >= %s, 'pass', 'needs_review')", scoreExpr, dataPipelineLiteral(threshold))
	exprs[reasonCol] = fmt.Sprintf("multiIf(%s, 'score_missing', %s, 'score_invalid', %s >= %s, 'passed', 'below_threshold')", missingScoreExpr, invalidScoreExpr, scoreExpr, dataPipelineLiteral(threshold))
	selectSQL := fmt.Sprintf("SELECT\n%s\nFROM %s.%s", dataPipelineSelectList(columns, exprs), quoteCHIdent(upstream.Database), quoteCHIdent(upstream.Table))
	if dataPipelineString(node.Data.Config, "mode") == "filter_pass" {
		selectSQL += fmt.Sprintf("\nWHERE %s >= %s", scoreExpr, dataPipelineLiteral(threshold))
	}
	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.exportQuerySettings()))
	sql := fmt.Sprintf("CREATE TABLE %s.%s ENGINE = MergeTree ORDER BY tuple() AS\n%s", quoteCHIdent(database), quoteCHIdent(table), selectSQL)
	if err := conn.Exec(queryCtx, sql); err != nil {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("materialize confidence_gate: %w", err)
	}
	metadata, err := collectConfidenceGateMetadata(ctx, conn, database, table, statusCol, scoreCol, reasonCol, threshold, scoreColumns)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns, Metadata: metadata}, nil
}

func (s *DataPipelineService) materializeQuarantine(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	statusCol := firstNonEmpty(dataPipelineString(node.Data.Config, "statusColumn"), "gate_status")
	if err := dataPipelineRequireColumn(upstream.Columns, statusCol); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	matchValues := dataPipelineStringSlice(node.Data.Config, "matchValues")
	if len(matchValues) == 0 {
		matchValues = []string{"needs_review"}
	}
	matchSet := make(map[string]struct{}, len(matchValues))
	for _, value := range matchValues {
		matchSet[value] = struct{}{}
	}
	outputMode := firstNonEmpty(dataPipelineString(node.Data.Config, "outputMode"), "quarantine_only")
	if outputMode != "quarantine_only" && outputMode != "pass_only" {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: quarantine outputMode must be quarantine_only or pass_only", ErrInvalidDataPipelineGraph)
	}
	out := make([]map[string]any, 0, len(rows))
	quarantinedRows := int64(0)
	passedRows := int64(0)
	for _, row := range rows {
		_, quarantined := matchSet[fmt.Sprint(row[statusCol])]
		if quarantined {
			quarantinedRows++
		} else {
			passedRows++
		}
		if (outputMode == "quarantine_only" && quarantined) || (outputMode == "pass_only" && !quarantined) {
			out = append(out, cloneRow(row))
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, upstream.Columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	metadata := map[string]any{
		"quarantinedRows": quarantinedRows,
		"passedRows":      passedRows,
		"statusColumn":    statusCol,
		"matchValues":     matchValues,
		"outputMode":      outputMode,
	}
	if quarantinedRows > 0 {
		metadata["failedRows"] = quarantinedRows
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: upstream.Columns, Metadata: metadata}, nil
}

func (s *DataPipelineService) materializeRouteByCondition(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	spec, err := dataPipelineRouteByConditionSpec(node.Data.Config, upstream.Columns)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	exprs := dataPipelineColumnExpressions(upstream.Columns)
	columns := dataPipelineUniqueStrings(append(append([]string{}, upstream.Columns...), spec.RouteColumn))
	exprs[spec.RouteColumn] = spec.RouteExpr
	selectSQL := fmt.Sprintf("SELECT\n%s\nFROM %s.%s", dataPipelineSelectList(columns, exprs), quoteCHIdent(upstream.Database), quoteCHIdent(upstream.Table))
	if spec.Mode == "filter_route" {
		selectSQL += "\nWHERE " + spec.RouteExpr + " = " + dataPipelineLiteral(spec.SelectedRoute)
	}
	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.exportQuerySettings()))
	sql := fmt.Sprintf("CREATE TABLE %s.%s ENGINE = MergeTree ORDER BY tuple() AS\n%s", quoteCHIdent(database), quoteCHIdent(table), selectSQL)
	if err := conn.Exec(queryCtx, sql); err != nil {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("materialize route_by_condition: %w", err)
	}
	routeCounts, err := queryDataPipelineRouteCounts(ctx, conn, fmt.Sprintf("SELECT * FROM %s.%s", quoteCHIdent(database), quoteCHIdent(table)), spec.RouteColumn)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	metadata := map[string]any{
		"routeColumn":   spec.RouteColumn,
		"defaultRoute":  spec.DefaultRoute,
		"mode":          spec.Mode,
		"selectedRoute": spec.SelectedRoute,
		"routes":        spec.Routes,
		"routeCounts":   routeCounts,
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns, Metadata: metadata}, nil
}

func (s *DataPipelineService) materializeDeduplicate(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	keyColumns := dataPipelineStringSlice(node.Data.Config, "keyColumns")
	if len(keyColumns) == 0 {
		keyColumns = append(keyColumns, firstNonEmpty(dataPipelineString(node.Data.Config, "keyColumn"), "file_public_id"))
	}
	for _, column := range keyColumns {
		if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
	}
	groupCol := firstNonEmpty(dataPipelineString(node.Data.Config, "groupColumn"), "duplicate_group_id")
	statusCol := firstNonEmpty(dataPipelineString(node.Data.Config, "statusColumn"), "duplicate_status")
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), groupCol, statusCol, "survivor_flag", "match_reason"))
	seen := map[string]int{}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		key := dataPipelineCompositeKey(row, keyColumns)
		seen[key]++
		next := cloneRow(row)
		next[groupCol] = "dup_" + shortHash(key)
		if seen[key] == 1 {
			next[statusCol] = "unique"
			next["survivor_flag"] = "true"
		} else {
			next[statusCol] = "duplicate"
			next["survivor_flag"] = "false"
		}
		next["match_reason"] = "key_columns:" + strings.Join(keyColumns, ",")
		if dataPipelineString(node.Data.Config, "mode") != "keep_first" || seen[key] == 1 {
			out = append(out, next)
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeCanonicalize(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	rules := dataPipelineConfigObjects(node.Data.Config, "rules")
	columns := append([]string{}, upstream.Columns...)
	out := make([]map[string]any, 0, len(rows))
	for _, rule := range rules {
		column := dataPipelineString(rule, "column")
		if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		outputColumn := firstNonEmpty(dataPipelineString(rule, "outputColumn"), column)
		if err := dataPipelineValidateIdentifier(outputColumn); err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		columns = append(columns, outputColumn)
	}
	columns = uniqueStringList(append(columns, "canonicalization_json"))
	for _, row := range rows {
		next := cloneRow(row)
		evidence := make([]map[string]any, 0, len(rules))
		for _, rule := range rules {
			column := dataPipelineString(rule, "column")
			outputColumn := firstNonEmpty(dataPipelineString(rule, "outputColumn"), column)
			original := fmt.Sprint(row[column])
			value := canonicalizeValue(original, dataPipelineStringSlice(rule, "operations"), dataPipelineStringMap(rule, "mappings"))
			next[outputColumn] = value
			evidence = append(evidence, map[string]any{"column": column, "outputColumn": outputColumn, "originalValue": original, "canonicalValue": value})
		}
		next["canonicalization_json"] = jsonString(evidence)
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeRedactPII(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	targetColumns := dataPipelineStringSlice(node.Data.Config, "columns")
	if len(targetColumns) == 0 && dataPipelineHasColumn(upstream.Columns, "text") {
		targetColumns = []string{"text"}
	}
	for _, column := range targetColumns {
		if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
	}
	suffix := firstNonEmpty(dataPipelineString(node.Data.Config, "outputSuffix"), "_redacted")
	columns := append([]string{}, upstream.Columns...)
	for _, column := range targetColumns {
		columns = append(columns, column+suffix)
	}
	columns = uniqueStringList(append(columns, "pii_detected", "pii_types_json"))
	types := dataPipelineStringSlice(node.Data.Config, "types")
	if len(types) == 0 {
		types = []string{"email", "phone", "postal_code", "api_key_like", "credit_card_like"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := cloneRow(row)
		allTypes := map[string]struct{}{}
		for _, column := range targetColumns {
			value, matched := redactPII(fmt.Sprint(row[column]), types, dataPipelineString(node.Data.Config, "mode"))
			next[column+suffix] = value
			for _, item := range matched {
				allTypes[item] = struct{}{}
			}
		}
		matchedTypes := keysOfSet(allTypes)
		next["pii_detected"] = strconv.FormatBool(len(matchedTypes) > 0)
		next["pii_types_json"] = jsonString(matchedTypes)
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeDetectLanguage(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	textCol := firstNonEmpty(dataPipelineString(node.Data.Config, "textColumn"), "text")
	if err := dataPipelineRequireColumn(upstream.Columns, textCol); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	outputTextCol := firstNonEmpty(dataPipelineString(node.Data.Config, "outputTextColumn"), "normalized_text")
	languageCol := firstNonEmpty(dataPipelineString(node.Data.Config, "languageColumn"), "language")
	mojibakeCol := firstNonEmpty(dataPipelineString(node.Data.Config, "mojibakeScoreColumn"), "mojibake_score")
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), languageCol, "encoding", outputTextCol, mojibakeCol, "fixes_applied_json"))
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := cloneRow(row)
		normalized, fixes := normalizeTextForPipeline(fmt.Sprint(row[textCol]))
		next[languageCol] = detectBasicLanguage(normalized)
		next["encoding"] = "utf-8"
		next[outputTextCol] = normalized
		next[mojibakeCol] = fmt.Sprintf("%.4f", mojibakeScore(normalized))
		next["fixes_applied_json"] = jsonString(fixes)
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeSchemaInference(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, int32(dataPipelineFloat(node.Data.Config, "sampleLimit", 1000)))
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	targetColumns := dataPipelineStringSlice(node.Data.Config, "columns")
	if len(targetColumns) == 0 {
		targetColumns = append([]string{}, upstream.Columns...)
	}
	for _, column := range targetColumns {
		if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
	}
	fields := make([]map[string]any, 0, len(targetColumns))
	for _, column := range targetColumns {
		fields = append(fields, inferSchemaField(column, rows))
	}
	schema := map[string]any{"fields": fields, "sampleRows": len(rows)}
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), "schema_inference_json", "schema_field_count", "schema_confidence"))
	out := make([]map[string]any, 0, max(1, len(rows)))
	if len(rows) == 0 {
		out = append(out, map[string]any{"schema_inference_json": jsonString(schema), "schema_field_count": strconv.Itoa(len(fields)), "schema_confidence": "0.0000"})
	} else {
		for _, row := range rows {
			next := cloneRow(row)
			next["schema_inference_json"] = jsonString(schema)
			next["schema_field_count"] = strconv.Itoa(len(fields))
			next["schema_confidence"] = "0.7500"
			out = append(out, next)
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeEntityResolution(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	column := firstNonEmpty(dataPipelineString(node.Data.Config, "column"), "vendor")
	if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	prefix := firstNonEmpty(dataPipelineString(node.Data.Config, "outputPrefix"), column)
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), prefix+"_entity_id", prefix+"_match_score", prefix+"_match_method", prefix+"_candidates_json"))
	dictionary := dataPipelineConfigObjects(node.Data.Config, "dictionary")
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := cloneRow(row)
		match := resolveEntity(fmt.Sprint(row[column]), dictionary)
		next[prefix+"_entity_id"] = fmt.Sprint(match["entity_id"])
		next[prefix+"_match_score"] = fmt.Sprint(match["match_score"])
		next[prefix+"_match_method"] = fmt.Sprint(match["match_method"])
		next[prefix+"_candidates_json"] = jsonString(match["candidates"])
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeUnitConversion(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	rules := dataPipelineConfigObjects(node.Data.Config, "rules")
	columns := append([]string{}, upstream.Columns...)
	for _, rule := range rules {
		valueCol := dataPipelineString(rule, "valueColumn")
		if err := dataPipelineRequireColumn(upstream.Columns, valueCol); err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		unitCol := dataPipelineString(rule, "unitColumn")
		if unitCol != "" {
			if err := dataPipelineRequireColumn(upstream.Columns, unitCol); err != nil {
				return dataPipelineMaterializedRelation{}, err
			}
		}
		columns = append(columns, firstNonEmpty(dataPipelineString(rule, "outputValueColumn"), valueCol+"_normalized"), firstNonEmpty(dataPipelineString(rule, "outputUnitColumn"), valueCol+"_unit"))
	}
	columns = uniqueStringList(append(columns, "conversion_context_json"))
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := cloneRow(row)
		contextItems := make([]map[string]any, 0, len(rules))
		for _, rule := range rules {
			valueCol := dataPipelineString(rule, "valueColumn")
			unitCol := dataPipelineString(rule, "unitColumn")
			inputUnit := firstNonEmpty(dataPipelineString(rule, "inputUnit"), fmt.Sprint(row[unitCol]))
			outputUnit := dataPipelineString(rule, "outputUnit")
			rate := conversionRate(inputUnit, outputUnit, dataPipelineConfigObjects(rule, "conversions"))
			outputValueCol := firstNonEmpty(dataPipelineString(rule, "outputValueColumn"), valueCol+"_normalized")
			outputUnitCol := firstNonEmpty(dataPipelineString(rule, "outputUnitColumn"), valueCol+"_unit")
			value := parseFloatString(fmt.Sprint(row[valueCol]))
			next[outputValueCol] = strconv.FormatFloat(value*rate, 'f', -1, 64)
			next[outputUnitCol] = outputUnit
			contextItems = append(contextItems, map[string]any{"valueColumn": valueCol, "inputUnit": inputUnit, "outputUnit": outputUnit, "rate": rate})
		}
		next["conversion_context_json"] = jsonString(contextItems)
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeRelationshipExtraction(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	textCol := firstNonEmpty(dataPipelineString(node.Data.Config, "textColumn"), "text")
	if err := dataPipelineRequireColumn(upstream.Columns, textCol); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	patterns := dataPipelineConfigObjects(node.Data.Config, "patterns")
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), "relationships_json", "relationship_count"))
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := cloneRow(row)
		relationships := extractRelationships(fmt.Sprint(row[textCol]), patterns)
		next["relationships_json"] = jsonString(relationships)
		next["relationship_count"] = strconv.Itoa(len(relationships))
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeHumanReview(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	reasonColumns := dataPipelineStringSlice(node.Data.Config, "reasonColumns")
	for _, column := range reasonColumns {
		if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
	}
	statusCol := firstNonEmpty(dataPipelineString(node.Data.Config, "statusColumn"), "review_status")
	queueCol := firstNonEmpty(dataPipelineString(node.Data.Config, "queueColumn"), "review_queue")
	queue := firstNonEmpty(dataPipelineString(node.Data.Config, "queue"), "default")
	createReviewItems := dataPipelineBool(node.Data.Config, "createReviewItems", false)
	reviewItemLimit := dataPipelineReviewItemLimit(node.Data.Config)
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), statusCol, queueCol, "review_reason_json"))
	out := make([]map[string]any, 0, len(rows))
	reviewItems := make([]dataPipelineReviewItemDraft, 0)
	reviewItemOverflow := 0
	for _, row := range rows {
		next := cloneRow(row)
		reasons := reviewReasons(row, reasonColumns)
		status := "not_required"
		if len(reasons) > 0 {
			status = "needs_review"
		}
		next[statusCol] = status
		next[queueCol] = queue
		next["review_reason_json"] = jsonString(reasons)
		if createReviewItems && status == "needs_review" {
			if len(reviewItems) < reviewItemLimit {
				reviewItems = append(reviewItems, dataPipelineReviewItemDraft{
					NodeID:            node.ID,
					Queue:             queue,
					Reason:            reasons,
					SourceSnapshot:    cloneRow(row),
					SourceFingerprint: dataPipelineReviewSourceFingerprint(node.ID, row),
				})
			} else {
				reviewItemOverflow++
			}
		}
		if dataPipelineString(node.Data.Config, "mode") != "filter_review" || status == "needs_review" {
			out = append(out, next)
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	metadata := map[string]any{}
	if createReviewItems {
		metadata["reviewItemCount"] = len(reviewItems)
		if reviewItemOverflow > 0 {
			metadata["reviewItemOverflowCount"] = reviewItemOverflow
			metadata["warnings"] = []string{"review_item_limit_exceeded"}
		}
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns, Metadata: metadata, ReviewItems: reviewItems}, nil
}

func (s *DataPipelineService) materializeSampleCompare(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	pairs := dataPipelineConfigObjects(node.Data.Config, "pairs")
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), "diff_json", "changed_fields", "changed_field_count"))
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := cloneRow(row)
		diffs := make([]map[string]any, 0)
		changed := make([]string, 0)
		for _, pair := range pairs {
			beforeCol := dataPipelineString(pair, "beforeColumn")
			afterCol := dataPipelineString(pair, "afterColumn")
			if err := dataPipelineRequireColumn(upstream.Columns, beforeCol); err != nil {
				return dataPipelineMaterializedRelation{}, err
			}
			if err := dataPipelineRequireColumn(upstream.Columns, afterCol); err != nil {
				return dataPipelineMaterializedRelation{}, err
			}
			beforeValue := fmt.Sprint(row[beforeCol])
			afterValue := fmt.Sprint(row[afterCol])
			if beforeValue != afterValue {
				field := firstNonEmpty(dataPipelineString(pair, "field"), afterCol)
				changed = append(changed, field)
				diffs = append(diffs, map[string]any{"field": field, "before": beforeValue, "after": afterValue})
			}
		}
		next["diff_json"] = jsonString(diffs)
		next["changed_fields"] = strings.Join(changed, ",")
		next["changed_field_count"] = strconv.Itoa(len(changed))
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeQualityReport(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	targetColumns := dataPipelineStringSlice(node.Data.Config, "columns")
	if len(targetColumns) == 0 {
		targetColumns = append([]string{}, upstream.Columns...)
	}
	for _, column := range targetColumns {
		if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
	}
	report := qualityReport(rows, targetColumns)
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), "quality_report_json", "missing_rate_json", "validation_summary_json"))
	out := make([]map[string]any, 0, max(1, len(rows)))
	if dataPipelineString(node.Data.Config, "outputMode") == "dataset_summary" {
		out = append(out, map[string]any{
			"quality_report_json":     jsonString(report),
			"missing_rate_json":       jsonString(report["missing_rate"]),
			"validation_summary_json": jsonString(report["summary"]),
		})
	} else {
		for _, row := range rows {
			next := cloneRow(row)
			next["quality_report_json"] = jsonString(report)
			next["missing_rate_json"] = jsonString(report["missing_rate"])
			next["validation_summary_json"] = jsonString(report["summary"])
			out = append(out, next)
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	rowCount := len(rows)
	missingRate, _ := report["missing_rate"].(map[string]float64)
	warningThreshold := dataPipelineFloat(node.Data.Config, "missingRateWarningThreshold", 0)
	warningCount := int64(0)
	warnings := make([]map[string]any, 0)
	for column, rate := range missingRate {
		if rate > warningThreshold {
			warningCount++
			warnings = append(warnings, map[string]any{
				"column":    column,
				"rate":      rate,
				"threshold": warningThreshold,
				"reason":    "missing_rate_threshold",
			})
		}
	}
	metadata := map[string]any{
		"quality": map[string]any{
			"rowCount":    rowCount,
			"columnCount": len(targetColumns),
			"missingRate": report["missing_rate"],
			"summary":     report["summary"],
			"warnings":    warnings,
		},
		"warningCount": warningCount,
		"warnings":     qualityWarningMessages(warnings),
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns, Metadata: metadata}, nil
}

func countHybridRelationRows(ctx context.Context, conn driver.Conn, relation dataPipelineMaterializedRelation) (int64, error) {
	count, _, err := countHybridRelationRowsWithStats(ctx, conn, relation, "")
	return count, err
}

func countHybridRelationRowsWithStats(ctx context.Context, conn driver.Conn, relation dataPipelineMaterializedRelation, queryID string) (int64, map[string]any, error) {
	var count uint64
	query := fmt.Sprintf("SELECT count() FROM %s.%s", quoteCHIdent(relation.Database), quoteCHIdent(relation.Table))
	start := time.Now()
	if err := queryDataPipelineSingleWithQueryID(ctx, conn, query, queryID, &count); err != nil {
		return 0, nil, err
	}
	return int64(count), collectDataPipelineQueryStats(ctx, conn, queryID, "count", time.Since(start)), nil
}

func collectConfidenceGateMetadata(ctx context.Context, conn driver.Conn, database, table, statusColumn, scoreColumn, reasonColumn string, threshold float64, scoreColumns []string) (map[string]any, error) {
	query := fmt.Sprintf(
		"SELECT countIf(%[1]s = 'pass'), countIf(%[1]s = 'needs_review'), ifNull(min(toFloat64OrZero(toString(%[2]s))), 0), ifNull(avg(toFloat64OrZero(toString(%[2]s))), 0) FROM %[3]s.%[4]s",
		quoteCHIdent(statusColumn),
		quoteCHIdent(scoreColumn),
		quoteCHIdent(database),
		quoteCHIdent(table),
	)
	var passRows uint64
	var needsReviewRows uint64
	var minScore float64
	var avgScore float64
	if err := queryDataPipelineSingle(ctx, conn, query, &passRows, &needsReviewRows, &minScore, &avgScore); err != nil {
		return nil, err
	}
	samples, err := collectConfidenceGateLowConfidenceSamples(ctx, conn, database, table, statusColumn, scoreColumn, reasonColumn, scoreColumns)
	if err != nil {
		return nil, err
	}
	failedRows := int64(needsReviewRows)
	return map[string]any{
		"failedRows": failedRows,
		"confidenceGate": map[string]any{
			"threshold":            threshold,
			"scoreColumns":         scoreColumns,
			"passRows":             int64(passRows),
			"needsReviewRows":      int64(needsReviewRows),
			"failedRows":           failedRows,
			"minScore":             minScore,
			"avgScore":             avgScore,
			"lowConfidenceSamples": samples,
		},
	}, nil
}

func collectConfidenceGateLowConfidenceSamples(ctx context.Context, conn driver.Conn, database, table, statusColumn, scoreColumn, reasonColumn string, scoreColumns []string) ([]map[string]any, error) {
	columns := uniqueStringList(append([]string{statusColumn, scoreColumn, reasonColumn}, scoreColumns...))
	query := fmt.Sprintf("SELECT %s FROM %s.%s WHERE %s = 'needs_review' ORDER BY %s ASC LIMIT 5", quoteCHIdentList(columns), quoteCHIdent(database), quoteCHIdent(table), quoteCHIdent(statusColumn), quoteCHIdent(scoreColumn))
	rows, err := conn.Query(clickhouse.Context(ctx), query)
	if err != nil {
		return nil, fmt.Errorf("collect confidence gate samples: %w", err)
	}
	defer rows.Close()
	_, samples, err := scanHybridRows(rows, 5)
	if err != nil {
		return nil, err
	}
	return samples, nil
}

func qualityWarningMessages(warnings []map[string]any) []string {
	messages := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		messages = append(messages, fmt.Sprintf("%s missing rate %.4f exceeded %.4f", warning["column"], warning["rate"], warning["threshold"]))
	}
	return messages
}

func createHybridStringTable(ctx context.Context, conn driver.Conn, database, table string, columns []string, rows []map[string]any) error {
	defs := make([]string, 0, len(columns))
	for _, column := range columns {
		if err := dataPipelineValidateIdentifier(column); err != nil {
			return err
		}
		defs = append(defs, quoteCHIdent(column)+" String")
	}
	queryCtx := clickhouse.Context(ctx)
	createSQL := fmt.Sprintf("CREATE TABLE %s.%s (%s) ENGINE = MergeTree ORDER BY tuple()", quoteCHIdent(database), quoteCHIdent(table), strings.Join(defs, ", "))
	if err := conn.Exec(queryCtx, createSQL); err != nil {
		return fmt.Errorf("create hybrid data pipeline table: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	insertSQL := fmt.Sprintf("INSERT INTO %s.%s (%s)", quoteCHIdent(database), quoteCHIdent(table), quoteCHIdentList(columns))
	batch, err := conn.PrepareBatch(queryCtx, insertSQL)
	if err != nil {
		return fmt.Errorf("prepare hybrid data pipeline insert: %w", err)
	}
	defer func() {
		if !batch.IsSent() {
			_ = batch.Abort()
		}
	}()
	for _, row := range rows {
		values := make([]any, 0, len(columns))
		for _, column := range columns {
			values = append(values, stringifyHybridValue(row[column]))
		}
		if err := batch.Append(values...); err != nil {
			return fmt.Errorf("append hybrid data pipeline row: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send hybrid data pipeline rows: %w", err)
	}
	return nil
}

func materializePassThrough(ctx context.Context, conn driver.Conn, database, table string, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, 100000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	if err := createHybridStringTable(ctx, conn, database, table, upstream.Columns, rows); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: upstream.Columns}, nil
}

func readHybridRows(ctx context.Context, conn driver.Conn, relation dataPipelineMaterializedRelation, limit int32) ([]map[string]any, error) {
	sql := fmt.Sprintf("SELECT * FROM %s.%s", quoteCHIdent(relation.Database), quoteCHIdent(relation.Table))
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := conn.Query(clickhouse.Context(ctx), sql)
	if err != nil {
		return nil, fmt.Errorf("read hybrid data pipeline rows: %w", err)
	}
	defer rows.Close()
	_, items, err := scanHybridRows(rows, int(limit))
	return items, err
}

func scanHybridRows(rows driver.Rows, limit int) ([]string, []map[string]any, error) {
	if limit <= 0 || limit > dataPipelineJSONMaxRows {
		limit = dataPipelineJSONMaxRows
	}
	columns := rows.Columns()
	columnTypes := rows.ColumnTypes()
	resultRows := make([]map[string]any, 0)
	for rows.Next() {
		holders, dest := datasetScanDestinations(columnTypes, len(columns))
		if err := rows.Scan(dest...); err != nil {
			return columns, resultRows, err
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = datasetJSONValue(datasetScannedValue(holders[i]))
		}
		resultRows = append(resultRows, row)
		if len(resultRows) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return columns, resultRows, err
	}
	return columns, resultRows, nil
}

func quoteCHIdentList(columns []string) string {
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, quoteCHIdent(column))
	}
	return strings.Join(parts, ", ")
}

func stringifyHybridValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case fmt.Stringer:
		return typed.String()
	default:
		if data, err := json.Marshal(typed); err == nil && (strings.HasPrefix(string(data), "{") || strings.HasPrefix(string(data), "[")) {
			return string(data)
		}
		return fmt.Sprint(typed)
	}
}

func cloneRow(row map[string]any) map[string]any {
	next := make(map[string]any, len(row)+4)
	for key, value := range row {
		next[key] = value
	}
	return next
}

func classifyText(text string, classes []map[string]any) (string, float64, string) {
	lower := strings.ToLower(text)
	bestLabel := "unknown"
	bestScore := 0.0
	bestReason := "no rule matched"
	bestPriority := -1.0
	for _, class := range classes {
		label := dataPipelineString(class, "label")
		if label == "" {
			continue
		}
		priority := dataPipelineFloat(class, "priority", 0)
		score := 0.0
		reasons := make([]string, 0)
		for _, keyword := range dataPipelineStringSlice(class, "keywords") {
			if strings.Contains(lower, strings.ToLower(keyword)) {
				score += 0.25
				reasons = append(reasons, "keyword:"+keyword)
			}
		}
		for _, pattern := range dataPipelineStringSlice(class, "regexes") {
			if re, err := regexp.Compile(pattern); err == nil && re.MatchString(text) {
				score += 0.5
				reasons = append(reasons, "regex:"+pattern)
			}
		}
		if score > 1 {
			score = 1
		}
		if score > 0 && (score > bestScore || (score == bestScore && priority > bestPriority)) {
			bestLabel = label
			bestScore = score
			bestPriority = priority
			bestReason = strings.Join(reasons, ", ")
		}
	}
	return bestLabel, bestScore, bestReason
}

func extractFieldValue(text string, field map[string]any) (string, bool, string) {
	for _, pattern := range dataPipelineStringSlice(field, "patterns") {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		match := re.FindStringSubmatch(text)
		if len(match) == 0 {
			continue
		}
		value := match[0]
		if len(match) > 1 {
			value = match[1]
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		return coerceFieldValue(value, dataPipelineString(field, "type")), true, match[0]
	}
	return "", false, ""
}

func coerceFieldValue(value, fieldType string) string {
	switch fieldType {
	case "number":
		clean := strings.ReplaceAll(value, ",", "")
		if parsed, err := strconv.ParseFloat(clean, 64); err == nil {
			return strconv.FormatFloat(parsed, 'f', -1, 64)
		}
	case "boolean":
		lower := strings.ToLower(value)
		if lower == "true" || lower == "yes" || lower == "1" {
			return "true"
		}
		if lower == "false" || lower == "no" || lower == "0" {
			return "false"
		}
	}
	return value
}

func dataPipelineCompositeKey(row map[string]any, columns []string) string {
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, strings.TrimSpace(fmt.Sprint(row[column])))
	}
	return strings.Join(parts, "\x1f")
}

func shortHash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}

func dataPipelineReviewItemLimit(config map[string]any) int {
	limit := 1000
	raw := dataPipelineString(config, "reviewItemLimit")
	if raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit <= 0 {
		return 0
	}
	if limit > 10000 {
		return 10000
	}
	return limit
}

func dataPipelineReviewSourceFingerprint(nodeID string, row map[string]any) string {
	payload, err := json.Marshal(row)
	if err != nil {
		return shortHash(nodeID + ":" + fmt.Sprint(row))
	}
	sum := sha1.Sum(append([]byte(nodeID+":"), payload...))
	return hex.EncodeToString(sum[:])
}

func canonicalizeValue(value string, operations []string, mappings map[string]string) string {
	out := value
	for _, operation := range operations {
		switch operation {
		case "trim":
			out = strings.TrimSpace(out)
		case "lowercase":
			out = strings.ToLower(out)
		case "uppercase":
			out = strings.ToUpper(out)
		case "normalize_spaces":
			out = strings.Join(strings.Fields(out), " ")
		case "remove_symbols":
			out = strings.Map(func(r rune) rune {
				if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
					return r
				}
				return -1
			}, out)
		case "zenkaku_to_hankaku_basic":
			out = zenkakuToHankakuBasic(out)
		case "normalize_date":
			out = normalizeDataPipelineDate(out)
		}
	}
	if mapped, ok := mappings[out]; ok {
		return mapped
	}
	return out
}

func normalizeDataPipelineDate(value string) string {
	value = strings.TrimSpace(zenkakuToHankakuBasic(value))
	re := regexp.MustCompile(`^([0-9]{4})[年/-]([0-9]{1,2})[月/-]([0-9]{1,2})日?$`)
	matches := re.FindStringSubmatch(value)
	if len(matches) != 4 {
		return value
	}
	year, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	day, _ := strconv.Atoi(matches[3])
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return value
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

func zenkakuToHankakuBasic(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= '！' && r <= '～' {
			return r - 0xFEE0
		}
		if r == '　' {
			return ' '
		}
		return r
	}, value)
}

func dataPipelineStringMap(record map[string]any, key string) map[string]string {
	raw, ok := record[key]
	if !ok || raw == nil {
		return nil
	}
	out := map[string]string{}
	if typed, ok := raw.(map[string]any); ok {
		for k, v := range typed {
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

func redactPII(value string, types []string, mode string) (string, []string) {
	out := value
	matched := make([]string, 0)
	patterns := map[string]*regexp.Regexp{
		"email":            regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
		"phone":            regexp.MustCompile(`(?:\+?\d[\d\s\-()]{7,}\d)`),
		"postal_code":      regexp.MustCompile(`\b\d{3}-?\d{4}\b`),
		"api_key_like":     regexp.MustCompile(`(?i)\b(?:api[_-]?key|secret|token)[=:]\s*[A-Za-z0-9_\-]{8,}\b`),
		"credit_card_like": regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`),
	}
	for _, piiType := range types {
		re := patterns[piiType]
		if re == nil || !re.MatchString(out) {
			continue
		}
		matched = append(matched, piiType)
		replacement := "[REDACTED:" + piiType + "]"
		if mode == "remove" {
			replacement = ""
		}
		out = re.ReplaceAllString(out, replacement)
	}
	return out, matched
}

func keysOfSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeTextForPipeline(value string) (string, []string) {
	fixes := make([]string, 0)
	out := strings.ReplaceAll(value, "\r\n", "\n")
	if out != value {
		fixes = append(fixes, "normalize_crlf")
	}
	trimmedLines := make([]string, 0)
	for _, line := range strings.Split(out, "\n") {
		trimmedLines = append(trimmedLines, strings.TrimRightFunc(line, unicode.IsSpace))
	}
	normalized := strings.Join(trimmedLines, "\n")
	if normalized != out {
		fixes = append(fixes, "trim_line_trailing_space")
	}
	return normalized, fixes
}

func detectBasicLanguage(value string) string {
	japanese := 0
	latin := 0
	for _, r := range value {
		switch {
		case unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han):
			japanese++
		case r <= unicode.MaxASCII && unicode.IsLetter(r):
			latin++
		}
	}
	if japanese > 0 && japanese >= latin/2 {
		return "ja"
	}
	if latin > 0 {
		return "en"
	}
	return "unknown"
}

func mojibakeScore(value string) float64 {
	if value == "" {
		return 0
	}
	suspicious := strings.Count(value, "�") + strings.Count(value, "Ã") + strings.Count(value, "ã")
	return float64(suspicious) / float64(max(1, len([]rune(value))))
}

func inferSchemaField(column string, rows []map[string]any) map[string]any {
	nonEmpty := 0
	counts := map[string]int{"number": 0, "date": 0, "boolean": 0}
	values := map[string]struct{}{}
	for _, row := range rows {
		value := strings.TrimSpace(fmt.Sprint(row[column]))
		if value == "" {
			continue
		}
		nonEmpty++
		if len(values) < 20 {
			values[value] = struct{}{}
		}
		if _, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64); err == nil {
			counts["number"]++
		}
		if _, ok := parseDateLike(value); ok {
			counts["date"]++
		}
		if isBoolLike(value) {
			counts["boolean"]++
		}
	}
	inferredType := "string"
	if nonEmpty > 0 {
		for _, candidate := range []string{"number", "date", "boolean"} {
			if counts[candidate] == nonEmpty {
				inferredType = candidate
				break
			}
		}
	}
	return map[string]any{
		"name":       column,
		"type":       inferredType,
		"required":   nonEmpty == len(rows) && len(rows) > 0,
		"missing":    len(rows) - nonEmpty,
		"enumValues": keysOfSet(values),
	}
}

func parseDateLike(value string) (string, bool) {
	for _, pattern := range []string{`^\d{4}-\d{1,2}-\d{1,2}$`, `^\d{4}/\d{1,2}/\d{1,2}$`} {
		if regexp.MustCompile(pattern).MatchString(value) {
			return value, true
		}
	}
	return "", false
}

func isBoolLike(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "false", "yes", "no", "1", "0":
		return true
	default:
		return false
	}
}

func resolveEntity(value string, dictionary []map[string]any) map[string]any {
	normalized := strings.ToLower(strings.TrimSpace(value))
	candidates := make([]map[string]any, 0)
	best := map[string]any{"entity_id": "", "match_score": "0.0000", "match_method": "none", "candidates": candidates}
	for _, item := range dictionary {
		id := dataPipelineString(item, "entityId")
		if id == "" {
			id = dataPipelineString(item, "id")
		}
		name := strings.ToLower(strings.TrimSpace(firstNonEmpty(dataPipelineString(item, "name"), dataPipelineString(item, "canonicalValue"))))
		aliases := append([]string{name}, dataPipelineStringSlice(item, "aliases")...)
		for _, alias := range aliases {
			alias = strings.ToLower(strings.TrimSpace(alias))
			score := 0.0
			method := "none"
			if alias != "" && normalized == alias {
				score = 1
				method = "exact"
			} else if alias != "" && (strings.Contains(normalized, alias) || strings.Contains(alias, normalized)) {
				score = 0.75
				method = "contains"
			}
			if score > 0 {
				candidates = append(candidates, map[string]any{"entityId": id, "name": name, "score": score, "method": method})
			}
			if score > parseFloatString(fmt.Sprint(best["match_score"])) {
				best["entity_id"] = id
				best["match_score"] = fmt.Sprintf("%.4f", score)
				best["match_method"] = method
			}
		}
	}
	best["candidates"] = candidates
	return best
}

func conversionRate(inputUnit, outputUnit string, conversions []map[string]any) float64 {
	if strings.EqualFold(inputUnit, outputUnit) || outputUnit == "" {
		return 1
	}
	for _, conversion := range conversions {
		if strings.EqualFold(dataPipelineString(conversion, "from"), inputUnit) && strings.EqualFold(dataPipelineString(conversion, "to"), outputUnit) {
			return dataPipelineFloat(conversion, "rate", 1)
		}
	}
	return 1
}

func parseFloatString(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", ""), 64)
	return parsed
}

func extractRelationships(text string, patterns []map[string]any) []map[string]any {
	out := make([]map[string]any, 0)
	for _, pattern := range patterns {
		reText := dataPipelineString(pattern, "pattern")
		if reText == "" {
			continue
		}
		re, err := regexp.Compile(reText)
		if err != nil {
			continue
		}
		for _, match := range re.FindAllStringSubmatch(text, -1) {
			source := ""
			target := ""
			if len(match) > 1 {
				source = strings.TrimSpace(match[1])
			}
			if len(match) > 2 {
				target = strings.TrimSpace(match[2])
			}
			out = append(out, map[string]any{
				"relation_type": firstNonEmpty(dataPipelineString(pattern, "relationType"), "related_to"),
				"source":        source,
				"target":        target,
				"source_text":   match[0],
				"confidence":    0.7,
			})
		}
	}
	return out
}

func reviewReasons(row map[string]any, columns []string) []map[string]any {
	out := make([]map[string]any, 0)
	for _, column := range columns {
		value := strings.TrimSpace(fmt.Sprint(row[column]))
		if value == "" || strings.EqualFold(value, "needs_review") || strings.EqualFold(value, "false") || value == "0" {
			out = append(out, map[string]any{"column": column, "value": value})
		}
	}
	return out
}

func qualityReport(rows []map[string]any, columns []string) map[string]any {
	missing := map[string]float64{}
	for _, column := range columns {
		empty := 0
		for _, row := range rows {
			if strings.TrimSpace(fmt.Sprint(row[column])) == "" {
				empty++
			}
		}
		missing[column] = float64(empty) / float64(max(1, len(rows)))
	}
	return map[string]any{
		"summary": map[string]any{
			"row_count":    len(rows),
			"column_count": len(columns),
		},
		"missing_rate": missing,
	}
}

func uniqueStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func jsonString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func defaultJSON(value []byte, fallback string) []byte {
	if len(value) == 0 {
		return []byte(fallback)
	}
	return value
}

func floatString(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%.4f", *value)
}
