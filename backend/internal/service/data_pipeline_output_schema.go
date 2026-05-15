package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type DataPipelineNodeOutputSchema struct {
	NodeID   string
	StepType string
	Columns  []string
	Warnings []string
}

func (s *DataPipelineService) inferOutputSchemas(ctx context.Context, tenantID int64, graph DataPipelineGraph) ([]DataPipelineNodeOutputSchema, error) {
	ordered, err := dataPipelineTopologicalOrder(graph)
	if err != nil {
		return nil, err
	}
	incoming := make(map[string][]string, len(graph.Nodes))
	for _, edge := range graph.Edges {
		incoming[edge.Target] = append(incoming[edge.Target], edge.Source)
	}
	for nodeID := range incoming {
		sort.Strings(incoming[nodeID])
	}

	columnsByNode := make(map[string][]string, len(graph.Nodes))
	schemas := make([]DataPipelineNodeOutputSchema, 0, len(ordered))
	for _, node := range ordered {
		upstreamColumns := func() []string {
			for _, upstreamID := range incoming[node.ID] {
				if columns := columnsByNode[upstreamID]; len(columns) > 0 {
					return columns
				}
			}
			return nil
		}
		columns, warnings, err := s.inferNodeOutputColumns(ctx, tenantID, node, incoming[node.ID], columnsByNode, upstreamColumns)
		if err != nil {
			return nil, err
		}
		columns = dataPipelineUniqueStrings(columns)
		columnsByNode[node.ID] = columns
		schemas = append(schemas, DataPipelineNodeOutputSchema{
			NodeID:   node.ID,
			StepType: node.Data.StepType,
			Columns:  columns,
			Warnings: dataPipelineUniqueStrings(warnings),
		})
	}
	return schemas, nil
}

func (s *DataPipelineService) inferNodeOutputColumns(
	ctx context.Context,
	tenantID int64,
	node DataPipelineNode,
	upstreamIDs []string,
	columnsByNode map[string][]string,
	upstreamColumns func() []string,
) ([]string, []string, error) {
	config := node.Data.Config
	switch node.Data.StepType {
	case DataPipelineStepInput:
		columns, err := s.inferInputOutputColumns(ctx, tenantID, config, "sourceKind", "datasetPublicId", "workTablePublicId")
		return columns, nil, err
	case DataPipelineStepTransform:
		return inferTransformOutputColumns(config, upstreamColumns()), nil, nil
	case DataPipelineStepJoin:
		leftColumns := columnsByNode[firstString(upstreamIDs)]
		rightColumns := columnsByNode[nthString(upstreamIDs, 1)]
		return inferJoinOutputColumns(config, leftColumns, rightColumns), nil, nil
	case DataPipelineStepUnion:
		upstreamColumnSets := make([][]string, 0, len(upstreamIDs))
		for _, upstreamID := range upstreamIDs {
			upstreamColumnSets = append(upstreamColumnSets, columnsByNode[upstreamID])
		}
		columns := dataPipelineUnionColumns(config, upstreamColumnSets)
		if sourceLabelColumn := dataPipelineString(config, "sourceLabelColumn"); sourceLabelColumn != "" {
			columns = append(columns, sourceLabelColumn)
		}
		return dataPipelineUniqueStrings(columns), nil, nil
	case DataPipelineStepEnrichJoin:
		rightColumns, err := s.inferInputOutputColumns(ctx, tenantID, config, "rightSourceKind", "rightDatasetPublicId", "rightWorkTablePublicId")
		if err != nil {
			return nil, nil, err
		}
		return inferJoinOutputColumns(config, upstreamColumns(), rightColumns), nil, nil
	default:
		columns := inferStepOutputColumns(node.Data.StepType, config, upstreamColumns())
		if columns == nil {
			columns = upstreamColumns()
		}
		return columns, nil, nil
	}
}

func (s *DataPipelineService) inferInputOutputColumns(ctx context.Context, tenantID int64, config map[string]any, kindKey, datasetKey, workTableKey string) ([]string, error) {
	kind := firstNonEmpty(dataPipelineString(config, kindKey), "dataset")
	if kind == dataPipelineDriveFileSource {
		return inferDriveFileInputColumns(config), nil
	}
	if s == nil || s.datasets == nil {
		return nil, fmt.Errorf("dataset service is not configured")
	}
	switch kind {
	case "dataset":
		dataset, err := s.datasets.Get(ctx, tenantID, dataPipelineString(config, datasetKey))
		if err != nil {
			return nil, err
		}
		columns := make([]string, 0, len(dataset.Columns))
		for _, column := range dataset.Columns {
			columns = append(columns, column.ColumnName)
		}
		return columns, nil
	case "work_table":
		workTable, err := s.datasets.GetManagedWorkTable(ctx, tenantID, dataPipelineString(config, workTableKey))
		if err != nil {
			return nil, err
		}
		columns := make([]string, 0, len(workTable.Columns))
		for _, column := range workTable.Columns {
			columns = append(columns, column.ColumnName)
		}
		return columns, nil
	default:
		return nil, fmt.Errorf("%w: sourceKind must be dataset, work_table, or drive_file", ErrInvalidDataPipelineGraph)
	}
}

func inferDriveFileInputColumns(config map[string]any) []string {
	if dataPipelineSpreadsheetInputMode(config) {
		metadataColumns := []string{"file_public_id", "file_name", "mime_type", "file_revision", "sheet_name", "sheet_index", "row_number"}
		if !dataPipelineBool(config, "includeSourceMetadataColumns", true) {
			metadataColumns = nil
		}
		return dataPipelineUniqueStrings(append(metadataColumns, dataPipelineStringList(config["columns"])...))
	}
	if dataPipelineJSONInputMode(config) {
		metadataColumns := []string{"file_public_id", "file_name", "mime_type", "file_revision", "row_number", "record_path"}
		if !dataPipelineBool(config, "includeSourceMetadataColumns", true) {
			metadataColumns = nil
		}
		columns := append([]string{}, metadataColumns...)
		for _, field := range dataPipelineConfigArray(config, "fields") {
			columns = append(columns, dataPipelineString(field, "column"))
		}
		if dataPipelineBool(config, "includeRawRecord", false) {
			columns = append(columns, "raw_record_json")
		}
		return dataPipelineUniqueStrings(columns)
	}
	return []string{"file_public_id", "file_name", "mime_type", "file_revision"}
}

func inferStepOutputColumns(stepType string, config map[string]any, upstreamColumns []string) []string {
	switch stepType {
	case DataPipelineStepProfile,
		DataPipelineStepClean,
		DataPipelineStepNormalize,
		DataPipelineStepValidate,
		DataPipelineStepOutput,
		DataPipelineStepQuarantine:
		return upstreamColumns
	case DataPipelineStepRouteByCondition:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...), firstNonEmpty(dataPipelineString(config, "routeColumn"), "route_key")))
	case DataPipelineStepExtractText:
		return []string{"file_public_id", "ocr_run_public_id", "page_number", "text", "confidence", "layout_json", "boxes_json"}
	case DataPipelineStepJSONExtract:
		return inferJSONExtractOutputColumns(config, upstreamColumns)
	case DataPipelineStepExcelExtract:
		return inferExcelExtractOutputColumns(config, upstreamColumns)
	case DataPipelineStepClassifyDocument:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...),
			firstNonEmpty(dataPipelineString(config, "outputColumn"), "document_type"),
			firstNonEmpty(dataPipelineString(config, "confidenceColumn"), "document_type_confidence"),
			"document_type_reason",
		))
	case DataPipelineStepExtractFields:
		columns := append([]string{}, upstreamColumns...)
		for _, field := range dataPipelineConfigArray(config, "fields") {
			columns = append(columns, dataPipelineString(field, "name"))
		}
		return dataPipelineUniqueStrings(append(columns, "fields_json", "evidence_json", "field_confidence"))
	case DataPipelineStepExtractTable:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...),
			"table_id", "row_number", "row_json", "source_text", "table_column_count", "table_missing_cell_count", "table_confidence",
		))
	case DataPipelineStepProductExtraction:
		columns := []string{}
		if dataPipelineBool(config, "includeSourceColumns", true) {
			columns = append(columns, upstreamColumns...)
		}
		return dataPipelineUniqueStrings(append(columns,
			"product_extraction_item_public_id", "product_item_type", "product_name", "product_brand",
			"product_manufacturer", "product_model", "product_sku", "product_jan_code", "product_category",
			"product_description", "product_price_json", "product_promotion_json", "product_availability_json",
			"product_source_text", "product_evidence_json", "product_attributes_json", "product_confidence",
			"product_extraction_status", "product_extraction_reason",
		))
	case DataPipelineStepQualityReport:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...), "quality_report_json", "missing_rate_json", "validation_summary_json"))
	case DataPipelineStepConfidenceGate:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...), "gate_score", firstNonEmpty(dataPipelineString(config, "statusColumn"), "gate_status"), "gate_reason"))
	case DataPipelineStepDeduplicate:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...), firstNonEmpty(dataPipelineString(config, "groupColumn"), "duplicate_group_id"), firstNonEmpty(dataPipelineString(config, "statusColumn"), "duplicate_status"), "survivor_flag", "match_reason"))
	case DataPipelineStepCanonicalize:
		columns := append([]string{}, upstreamColumns...)
		for _, rule := range dataPipelineConfigArray(config, "rules") {
			columns = append(columns, firstNonEmpty(dataPipelineString(rule, "outputColumn"), dataPipelineString(rule, "column")))
		}
		return dataPipelineUniqueStrings(append(columns, "canonicalization_json"))
	case DataPipelineStepRedactPII:
		columns := append([]string{}, upstreamColumns...)
		suffix := firstNonEmpty(dataPipelineString(config, "outputSuffix"), "_redacted")
		for _, column := range dataPipelineStringList(config["columns"]) {
			columns = append(columns, column+suffix)
		}
		return dataPipelineUniqueStrings(append(columns, "pii_detected", "pii_types_json"))
	case DataPipelineStepDetectLanguage:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...),
			firstNonEmpty(dataPipelineString(config, "languageColumn"), "language"),
			"encoding",
			firstNonEmpty(dataPipelineString(config, "outputTextColumn"), "normalized_text"),
			firstNonEmpty(dataPipelineString(config, "mojibakeScoreColumn"), "mojibake_score"),
			"fixes_applied_json",
		))
	case DataPipelineStepSchemaInference:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...), "schema_inference_json", "schema_field_count", "schema_confidence"))
	case DataPipelineStepEntityResolution:
		column := firstNonEmpty(dataPipelineString(config, "column"), "vendor")
		prefix := firstNonEmpty(dataPipelineString(config, "outputPrefix"), column)
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...), prefix+"_entity_id", prefix+"_match_score", prefix+"_match_method", prefix+"_candidates_json"))
	case DataPipelineStepUnitConversion:
		columns := append([]string{}, upstreamColumns...)
		for _, rule := range dataPipelineConfigArray(config, "rules") {
			valueColumn := dataPipelineString(rule, "valueColumn")
			if valueColumn == "" {
				continue
			}
			columns = append(columns, firstNonEmpty(dataPipelineString(rule, "outputValueColumn"), valueColumn+"_normalized"), firstNonEmpty(dataPipelineString(rule, "outputUnitColumn"), valueColumn+"_unit"))
		}
		return dataPipelineUniqueStrings(append(columns, "conversion_context_json"))
	case DataPipelineStepRelationship:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...), "relationships_json", "relationship_count"))
	case DataPipelineStepSchemaMapping:
		columns := []string{}
		for _, mapping := range dataPipelineConfigArray(config, "mappings") {
			columns = append(columns, dataPipelineString(mapping, "targetColumn"))
		}
		if len(dataPipelineUniqueStrings(columns)) == 0 {
			return upstreamColumns
		}
		if dataPipelineBool(config, "includeSourceColumns", false) {
			columns = append(append([]string{}, upstreamColumns...), columns...)
		}
		return dataPipelineUniqueStrings(append(columns,
			firstNonEmpty(dataPipelineString(config, "scoreColumn"), "schema_mapping_confidence"),
			firstNonEmpty(dataPipelineString(config, "statusColumn"), "schema_mapping_status"),
			firstNonEmpty(dataPipelineString(config, "reasonColumn"), "schema_mapping_reason"),
			firstNonEmpty(dataPipelineString(config, "mappingJSONColumn"), "schema_mapping_json"),
		))
	case DataPipelineStepSchemaCompletion:
		columns := append([]string{}, upstreamColumns...)
		for _, rule := range dataPipelineConfigArray(config, "rules") {
			columns = append(columns, dataPipelineString(rule, "targetColumn"))
		}
		return dataPipelineUniqueStrings(columns)
	case DataPipelineStepHumanReview:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...), firstNonEmpty(dataPipelineString(config, "statusColumn"), "review_status"), firstNonEmpty(dataPipelineString(config, "queueColumn"), "review_queue"), "review_reason_json"))
	case DataPipelineStepSampleCompare:
		return dataPipelineUniqueStrings(append(append([]string{}, upstreamColumns...), "diff_json", "changed_fields", "changed_field_count"))
	default:
		return nil
	}
}

func inferJSONExtractOutputColumns(config map[string]any, upstreamColumns []string) []string {
	columns := []string{}
	if dataPipelineBool(config, "includeSourceColumns", true) {
		columns = append(columns, upstreamColumns...)
	}
	columns = append(columns, "json_row_number", "json_record_path")
	for _, field := range dataPipelineConfigArray(config, "fields") {
		columns = append(columns, dataPipelineString(field, "column"))
	}
	if dataPipelineBool(config, "includeRawRecord", false) {
		columns = append(columns, "raw_record_json")
	}
	return dataPipelineUniqueStrings(columns)
}

func inferExcelExtractOutputColumns(config map[string]any, upstreamColumns []string) []string {
	columns := []string{}
	if dataPipelineBool(config, "includeSourceColumns", true) {
		columns = append(columns, upstreamColumns...)
	}
	if dataPipelineBool(config, "includeSourceMetadataColumns", true) {
		columns = append(columns, "file_public_id", "file_name", "mime_type", "file_revision", "sheet_name", "sheet_index", "row_number")
	}
	columns = append(columns, dataPipelineStringList(config["columns"])...)
	return dataPipelineUniqueStrings(columns)
}

func inferTransformOutputColumns(config map[string]any, upstreamColumns []string) []string {
	operation := firstNonEmpty(dataPipelineString(config, "operation"), dataPipelineString(config, "type"), "select_columns")
	switch operation {
	case "select_columns":
		columns := dataPipelineStringList(config["columns"])
		if len(columns) > 0 {
			return dataPipelineUniqueStrings(columns)
		}
		return upstreamColumns
	case "drop_columns":
		drops := make(map[string]struct{})
		for _, column := range dataPipelineStringList(config["columns"]) {
			drops[column] = struct{}{}
		}
		columns := make([]string, 0, len(upstreamColumns))
		for _, column := range upstreamColumns {
			if _, ok := drops[column]; !ok {
				columns = append(columns, column)
			}
		}
		return columns
	case "rename_columns":
		renames := dataPipelineRecord(config["renames"])
		columns := make([]string, 0, len(upstreamColumns))
		for _, column := range upstreamColumns {
			columns = append(columns, firstNonEmpty(dataPipelineString(renames, column), column))
		}
		return dataPipelineUniqueStrings(columns)
	case "aggregate":
		return inferAggregateOutputColumns(config)
	default:
		return upstreamColumns
	}
}

func inferAggregateOutputColumns(config map[string]any) []string {
	columns := dataPipelineStringList(config["groupBy"])
	for _, aggregation := range dataPipelineConfigArray(config, "aggregations") {
		function := strings.ToLower(firstNonEmpty(dataPipelineString(aggregation, "function"), "count"))
		column := dataPipelineString(aggregation, "column")
		alias := firstNonEmpty(dataPipelineString(aggregation, "alias"), function)
		if column != "" && dataPipelineString(aggregation, "alias") == "" {
			alias = function + "_" + column
		}
		columns = append(columns, alias)
	}
	if len(dataPipelineUniqueStrings(columns)) == 0 {
		return []string{"count"}
	}
	return dataPipelineUniqueStrings(columns)
}

func inferJoinOutputColumns(config map[string]any, leftColumns []string, rightColumns []string) []string {
	rightSelection := dataPipelineStringList(config["selectColumns"])
	if len(rightSelection) == 0 {
		rightSelection = rightColumns
	}
	columns := append([]string{}, leftColumns...)
	for _, column := range rightSelection {
		if dataPipelineContainsString(columns, column) {
			columns = append(columns, column+"_right")
			continue
		}
		columns = append(columns, column)
	}
	return dataPipelineUniqueStrings(columns)
}

func dataPipelineConfigArray(config map[string]any, key string) []map[string]any {
	raw, ok := config[key]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, dataPipelineRecord(item))
	}
	return out
}

func dataPipelineRecord(value any) map[string]any {
	if value == nil {
		return nil
	}
	record, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return record
}

func dataPipelineStringList(value any) []string {
	switch raw := value.(type) {
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			text := strings.TrimSpace(item)
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		text := strings.TrimSpace(raw)
		if text != "" {
			return []string{text}
		}
	}
	return nil
}

func dataPipelineSpreadsheetInputMode(config map[string]any) bool {
	mode := strings.ToLower(firstNonEmpty(dataPipelineString(config, "inputMode"), dataPipelineString(config, "format")))
	return mode == "spreadsheet" || mode == "excel" || mode == "xls" || mode == "xlsx"
}

func dataPipelineJSONInputMode(config map[string]any) bool {
	mode := strings.ToLower(firstNonEmpty(dataPipelineString(config, "inputMode"), dataPipelineString(config, "format")))
	return mode == "json" || mode == "application/json"
}

func dataPipelineContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func firstString(values []string) string {
	return nthString(values, 0)
}

func nthString(values []string, index int) string {
	if index < 0 || index >= len(values) {
		return ""
	}
	return values[index]
}
