package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"example.com/haohao/backend/internal/db"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
)

const dataPipelineDriveFileSource = "drive_file"

type dataPipelineMaterializedRelation struct {
	Database string
	Table    string
	Columns  []string
}

type dataPipelineHybridResult struct {
	Relation dataPipelineMaterializedRelation
	Compiled dataPipelineCompiledSelect
	Tables   []string
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
		DataPipelineStepClassifyDocument,
		DataPipelineStepExtractFields,
		DataPipelineStepExtractTable,
		DataPipelineStepConfidenceGate:
		return true
	default:
		return false
	}
}

func (s *DataPipelineService) executeHybridRun(ctx context.Context, tenantID int64, run db.DataPipelineRun, graph DataPipelineGraph) (DatasetWorkTable, dataPipelineCompiledSelect, error) {
	actorUserID := int64(0)
	if run.RequestedByUserID.Valid {
		actorUserID = run.RequestedByUserID.Int64
	}
	if actorUserID <= 0 {
		return DatasetWorkTable{}, dataPipelineCompiledSelect{}, fmt.Errorf("%w: data pipeline OCR nodes require requested_by_user_id or schedule creator", ErrInvalidDataPipelineGraph)
	}
	result, err := s.executeHybridGraph(ctx, tenantID, graph, run.PublicID.String(), actorUserID, "", "data_pipeline")
	if err != nil {
		return DatasetWorkTable{}, result.Compiled, err
	}
	if err := s.datasets.ensureTenantSandbox(ctx, tenantID); err != nil {
		return DatasetWorkTable{}, result.Compiled, err
	}
	conn, err := s.datasets.openTenantConn(ctx, tenantID)
	if err != nil {
		return DatasetWorkTable{}, result.Compiled, err
	}
	defer conn.Close()

	outputNode := dataPipelineOutputNode(graph)
	targetDatabase := datasetWorkDatabaseName(tenantID)
	targetTable := dataPipelineOutputTableName(outputNode, run)
	stageTable := "__dp_stage_" + strings.ReplaceAll(run.PublicID.String(), "-", "")
	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.exportQuerySettings()))
	_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(stageTable)))
	createSQL := fmt.Sprintf(
		"CREATE TABLE %s.%s ENGINE = MergeTree ORDER BY tuple() AS\nSELECT * FROM %s.%s",
		quoteCHIdent(targetDatabase),
		quoteCHIdent(stageTable),
		quoteCHIdent(result.Relation.Database),
		quoteCHIdent(result.Relation.Table),
	)
	if err := conn.Exec(queryCtx, createSQL); err != nil {
		return DatasetWorkTable{}, result.Compiled, fmt.Errorf("create data pipeline hybrid stage table: %w", err)
	}
	_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(targetTable)))
	if err := conn.Exec(queryCtx, fmt.Sprintf("RENAME TABLE %s.%s TO %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(stageTable), quoteCHIdent(targetDatabase), quoteCHIdent(targetTable))); err != nil {
		_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(stageTable)))
		return DatasetWorkTable{}, result.Compiled, fmt.Errorf("promote data pipeline hybrid stage table: %w", err)
	}
	for _, table := range result.Tables {
		if table != targetTable {
			_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(table)))
		}
	}

	displayName := dataPipelineString(outputNode.Data.Config, "displayName")
	if displayName == "" {
		displayName = targetTable
	}
	workTable, err := s.datasets.registerDatasetWorkTableForRef(ctx, tenantID, actorUserID, nil, nil, targetDatabase, targetTable, displayName)
	if err != nil {
		return DatasetWorkTable{}, result.Compiled, err
	}
	return workTable, result.Compiled, nil
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
	}, nil
}

func (s *DataPipelineService) materializeHybridNode(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, relations map[string]dataPipelineMaterializedRelation, compiler *dataPipelineCompiler, tenantID, actorUserID int64, ocrReason string) (dataPipelineMaterializedRelation, error) {
	switch node.Data.StepType {
	case DataPipelineStepInput:
		if dataPipelineString(node.Data.Config, "sourceKind") == dataPipelineDriveFileSource {
			return s.materializeDriveFileInput(ctx, conn, database, table, node)
		}
	case DataPipelineStepExtractText:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeExtractText(ctx, conn, database, table, node, upstream, tenantID, actorUserID, ocrReason)
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
	case DataPipelineStepConfidenceGate:
		upstream, err := materializedSingleUpstream(node, compiler.incoming, relations)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		return s.materializeConfidenceGate(ctx, conn, database, table, node, upstream)
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
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: relation.Columns}, nil
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
	for _, row := range rows {
		next := cloneRow(row)
		text := fmt.Sprint(row["text"])
		fieldsJSON := map[string]any{}
		evidence := []map[string]any{}
		scoreTotal := 0.0
		scoreCount := 0.0
		for _, field := range fields {
			name := dataPipelineString(field, "name")
			value, ok, source := extractFieldValue(text, field)
			if ok {
				next[name] = value
				fieldsJSON[name] = value
				evidence = append(evidence, map[string]any{"field": name, "sourceText": source})
				scoreTotal += 1
			} else {
				next[name] = ""
				fieldsJSON[name] = nil
				if dataPipelineBool(field, "required", false) {
					evidence = append(evidence, map[string]any{"field": name, "missing": true})
				}
			}
			scoreCount++
		}
		next["fields_json"] = jsonString(fieldsJSON)
		next["evidence_json"] = jsonString(evidence)
		if scoreCount > 0 {
			next["field_confidence"] = fmt.Sprintf("%.4f", scoreTotal/scoreCount)
		} else {
			next["field_confidence"] = "1.0000"
		}
		out = append(out, next)
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
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
	tableColumns := []string{"table_id", "row_number", "row_json", "source_text"}
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), tableColumns...))
	out := make([]map[string]any, 0)
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
			values := map[string]any{}
			for i, part := range parts {
				values[fmt.Sprintf("column_%d", i+1)] = strings.TrimSpace(part)
			}
			next := cloneRow(row)
			next["table_id"] = fmt.Sprintf("%s:%v", row["file_public_id"], row["page_number"])
			next["row_number"] = strconv.Itoa(rowNumber)
			next["row_json"] = jsonString(values)
			next["source_text"] = line
			out = append(out, next)
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeConfidenceGate(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	threshold := dataPipelineFloat(node.Data.Config, "threshold", 0.8)
	statusCol := firstNonEmpty(dataPipelineString(node.Data.Config, "statusColumn"), "gate_status")
	scoreCol := "gate_score"
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
	if len(scoreColumns) > 0 {
		parts := make([]string, 0, len(scoreColumns))
		for _, column := range scoreColumns {
			if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
				return dataPipelineMaterializedRelation{}, err
			}
			parts = append(parts, "toFloat64OrZero("+quoteCHIdent(column)+")")
		}
		scoreExpr = "(" + strings.Join(parts, " + ") + ") / " + strconv.Itoa(len(parts))
	}
	columns := uniqueStringList(append(append([]string{}, upstream.Columns...), scoreCol, statusCol))
	exprs[scoreCol] = scoreExpr
	exprs[statusCol] = fmt.Sprintf("if(%s >= %s, 'pass', 'needs_review')", scoreExpr, dataPipelineLiteral(threshold))
	selectSQL := fmt.Sprintf("SELECT\n%s\nFROM %s.%s", dataPipelineSelectList(columns, exprs), quoteCHIdent(upstream.Database), quoteCHIdent(upstream.Table))
	if dataPipelineString(node.Data.Config, "mode") == "filter_pass" {
		selectSQL += fmt.Sprintf("\nWHERE %s >= %s", scoreExpr, dataPipelineLiteral(threshold))
	}
	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.exportQuerySettings()))
	sql := fmt.Sprintf("CREATE TABLE %s.%s ENGINE = MergeTree ORDER BY tuple() AS\n%s", quoteCHIdent(database), quoteCHIdent(table), selectSQL)
	if err := conn.Exec(queryCtx, sql); err != nil {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("materialize confidence_gate: %w", err)
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
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
	_, items, err := scanDatasetRows(rows, int(limit))
	return items, err
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
