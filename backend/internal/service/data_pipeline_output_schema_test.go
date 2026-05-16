package service

import (
	"context"
	"testing"
)

func TestInferOutputSchemasForFieldReviewPipeline(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{
				ID: "input",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepInput,
					Config: map[string]any{
						"sourceKind":    dataPipelineDriveFileSource,
						"filePublicIds": []any{"file_1"},
					},
				},
			},
			{ID: "extract_text", Data: DataPipelineNodeData{StepType: DataPipelineStepExtractText}},
			{
				ID: "extract_fields",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepExtractFields,
					Config: map[string]any{
						"fields": []any{
							map[string]any{"name": "invoice_id"},
							map[string]any{"name": "customer"},
							map[string]any{"name": "amount"},
						},
					},
				},
			},
			{
				ID: "confidence_gate",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepConfidenceGate,
					Config:   map[string]any{"statusColumn": "gate_status"},
				},
			},
			{
				ID: "human_review",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepHumanReview,
					Config:   map[string]any{"queueColumn": "review_queue"},
				},
			},
			{ID: "output", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{
			{Source: "input", Target: "extract_text"},
			{Source: "extract_text", Target: "extract_fields"},
			{Source: "extract_fields", Target: "confidence_gate"},
			{Source: "confidence_gate", Target: "human_review"},
			{Source: "human_review", Target: "output"},
		},
	}

	schemas, err := (&DataPipelineService{}).inferOutputSchemas(context.Background(), 1, graph)
	if err != nil {
		t.Fatalf("inferOutputSchemas() error = %v", err)
	}
	output := schemaColumnsByNode(schemas)["output"]
	for _, column := range []string{"file_public_id", "text", "confidence", "invoice_id", "customer", "amount", "field_confidence", "gate_score", "gate_status", "review_status", "review_queue"} {
		if !dataPipelineContainsString(output, column) {
			t.Fatalf("output schema missing %q: %#v", column, output)
		}
	}
}

func TestInferOutputSchemasForProductReviewPipeline(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{
				ID: "input",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepInput,
					Config: map[string]any{
						"sourceKind": dataPipelineDriveFileSource,
					},
				},
			},
			{
				ID: "product_extraction",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepProductExtraction,
					Config:   map[string]any{"includeSourceColumns": true},
				},
			},
			{
				ID: "confidence_gate",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepConfidenceGate,
					Config:   map[string]any{"statusColumn": "gate_status"},
				},
			},
			{ID: "output", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{
			{Source: "input", Target: "product_extraction"},
			{Source: "product_extraction", Target: "confidence_gate"},
			{Source: "confidence_gate", Target: "output"},
		},
	}

	schemas, err := (&DataPipelineService{}).inferOutputSchemas(context.Background(), 1, graph)
	if err != nil {
		t.Fatalf("inferOutputSchemas() error = %v", err)
	}
	output := schemaColumnsByNode(schemas)["output"]
	for _, column := range []string{"file_public_id", "product_name", "product_confidence", "product_extraction_status", "gate_score", "gate_status", "gate_reason"} {
		if !dataPipelineContainsString(output, column) {
			t.Fatalf("output schema missing %q: %#v", column, output)
		}
	}
}

func TestInferOutputSchemasForSchemaMappingReviewPipeline(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{
				ID: "input",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepInput,
					Config: map[string]any{
						"sourceKind": dataPipelineDriveFileSource,
						"inputMode":  "json",
						"fields": []any{
							map[string]any{"column": "invoice_number"},
							map[string]any{"column": "total"},
							map[string]any{"column": "state"},
						},
						"includeRawRecord": true,
					},
				},
			},
			{
				ID: "schema_mapping",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepSchemaMapping,
					Config: map[string]any{
						"includeSourceColumns": true,
						"mappings": []any{
							map[string]any{"sourceColumn": "invoice_number", "targetColumn": "invoice_id"},
							map[string]any{"sourceColumn": "total", "targetColumn": "amount"},
							map[string]any{"sourceColumn": "state", "targetColumn": "status"},
						},
					},
				},
			},
			{ID: "output", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{
			{Source: "input", Target: "schema_mapping"},
			{Source: "schema_mapping", Target: "output"},
		},
	}

	schemas, err := (&DataPipelineService{}).inferOutputSchemas(context.Background(), 1, graph)
	if err != nil {
		t.Fatalf("inferOutputSchemas() error = %v", err)
	}
	output := schemaColumnsByNode(schemas)["output"]
	for _, column := range []string{"file_public_id", "invoice_id", "amount", "status", "schema_mapping_confidence", "schema_mapping_status", "schema_mapping_reason", "schema_mapping_json"} {
		if !dataPipelineContainsString(output, column) {
			t.Fatalf("output schema missing %q: %#v", column, output)
		}
	}
}

func TestInferOutputSchemasForRouteByConditionPipeline(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{
				ID: "input",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepInput,
					Config: map[string]any{
						"sourceKind": dataPipelineDriveFileSource,
					},
				},
			},
			{
				ID: "route",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepRouteByCondition,
					Config: map[string]any{
						"routeColumn": "review_route",
						"rules": []any{
							map[string]any{"column": "file_name", "operator": "regex", "value": "invoice", "route": "invoice"},
						},
					},
				},
			},
			{ID: "output", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{
			{Source: "input", Target: "route"},
			{Source: "route", Target: "output"},
		},
	}

	schemas, err := (&DataPipelineService{}).inferOutputSchemas(context.Background(), 1, graph)
	if err != nil {
		t.Fatalf("inferOutputSchemas() error = %v", err)
	}
	output := schemaColumnsByNode(schemas)["output"]
	for _, column := range []string{"file_public_id", "file_name", "review_route"} {
		if !dataPipelineContainsString(output, column) {
			t.Fatalf("output schema missing %q: %#v", column, output)
		}
	}
}

func TestInferOutputSchemasForUnionPipeline(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{
				ID: "input_a",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepInput,
					Config: map[string]any{
						"sourceKind": dataPipelineDriveFileSource,
						"inputMode":  "json",
						"fields":     []any{map[string]any{"column": "name"}, map[string]any{"column": "amount"}},
					},
				},
			},
			{
				ID: "input_b",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepInput,
					Config: map[string]any{
						"sourceKind": dataPipelineDriveFileSource,
						"inputMode":  "json",
						"fields":     []any{map[string]any{"column": "name"}, map[string]any{"column": "status"}},
					},
				},
			},
			{
				ID: "union",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepUnion,
					Config:   map[string]any{"sourceLabelColumn": "source_node_id"},
				},
			},
			{ID: "output", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{
			{Source: "input_a", Target: "union"},
			{Source: "input_b", Target: "union"},
			{Source: "union", Target: "output"},
		},
	}

	schemas, err := (&DataPipelineService{}).inferOutputSchemas(context.Background(), 1, graph)
	if err != nil {
		t.Fatalf("inferOutputSchemas() error = %v", err)
	}
	output := schemaColumnsByNode(schemas)["output"]
	for _, column := range []string{"file_public_id", "name", "amount", "status", "source_node_id"} {
		if !dataPipelineContainsString(output, column) {
			t.Fatalf("output schema missing %q: %#v", column, output)
		}
	}
}

func TestInferOutputSchemasForTypedOutputPipeline(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{
				ID: "input",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepInput,
					Config: map[string]any{
						"sourceKind": dataPipelineDriveFileSource,
						"inputMode":  "json",
						"fields": []any{
							map[string]any{"column": "id"},
							map[string]any{"column": "amount"},
							map[string]any{"column": "updated_at"},
						},
					},
				},
			},
			{
				ID: "output",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepOutput,
					Config: map[string]any{"columns": []any{
						map[string]any{"sourceColumn": "id", "name": "id", "type": "string"},
						map[string]any{"sourceColumn": "amount", "name": "amount_value", "type": "float64"},
						map[string]any{"sourceColumn": "updated_at", "name": "updated_at", "type": "datetime"},
					}},
				},
			},
		},
		Edges: []DataPipelineEdge{{Source: "input", Target: "output"}},
	}

	schemas, err := (&DataPipelineService{}).inferOutputSchemas(context.Background(), 1, graph)
	if err != nil {
		t.Fatalf("inferOutputSchemas() error = %v", err)
	}
	output := schemaColumnsByNode(schemas)["output"]
	want := []string{"id", "amount_value", "updated_at"}
	if len(output) != len(want) {
		t.Fatalf("output schema = %#v, want %#v", output, want)
	}
	for i, column := range want {
		if output[i] != column {
			t.Fatalf("output schema = %#v, want %#v", output, want)
		}
	}
}

func TestInferOutputSchemasForValidateQuarantinePipeline(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{
				ID: "input",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepInput,
					Config: map[string]any{
						"sourceKind": dataPipelineDriveFileSource,
						"inputMode":  "json",
						"fields":     []any{map[string]any{"column": "id"}, map[string]any{"column": "amount"}},
					},
				},
			},
			{
				ID: "validate",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepValidate,
					Config: map[string]any{"rules": []any{
						map[string]any{"column": "id", "operator": "required"},
						map[string]any{"column": "amount", "operator": "range", "min": 0},
					}},
				},
			},
			{ID: "quarantine", Data: DataPipelineNodeData{StepType: DataPipelineStepQuarantine}},
			{ID: "output", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{
			{Source: "input", Target: "validate"},
			{Source: "validate", Target: "quarantine"},
			{Source: "quarantine", Target: "output"},
		},
	}

	schemas, err := (&DataPipelineService{}).inferOutputSchemas(context.Background(), 1, graph)
	if err != nil {
		t.Fatalf("inferOutputSchemas() error = %v", err)
	}
	output := schemaColumnsByNode(schemas)["output"]
	for _, column := range []string{"id", "amount", "validation_status", "validation_errors_json"} {
		if !dataPipelineContainsString(output, column) {
			t.Fatalf("output schema missing %q: %#v", column, output)
		}
	}
}

func TestInferOutputSchemasForSnapshotSCD2Pipeline(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{
				ID: "input",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepInput,
					Config: map[string]any{
						"sourceKind": dataPipelineDriveFileSource,
						"inputMode":  "json",
						"fields": []any{
							map[string]any{"column": "id"},
							map[string]any{"column": "status"},
							map[string]any{"column": "updated_at"},
						},
					},
				},
			},
			{
				ID: "snapshot",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepSnapshotSCD2,
					Config: map[string]any{
						"uniqueKeys":      []any{"id"},
						"updatedAtColumn": "updated_at",
						"watchedColumns":  []any{"status"},
					},
				},
			},
			{ID: "output", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{{Source: "input", Target: "snapshot"}, {Source: "snapshot", Target: "output"}},
	}

	schemas, err := (&DataPipelineService{}).inferOutputSchemas(context.Background(), 1, graph)
	if err != nil {
		t.Fatalf("inferOutputSchemas() error = %v", err)
	}
	output := schemaColumnsByNode(schemas)["output"]
	for _, column := range []string{"id", "status", "updated_at", "valid_from", "valid_to", "is_current", "change_hash"} {
		if !dataPipelineContainsString(output, column) {
			t.Fatalf("output schema missing %q: %#v", column, output)
		}
	}
}

func TestInferOutputSchemasCoversEveryCatalogStep(t *testing.T) {
	for _, entry := range DataPipelineStepCatalog() {
		stepType := entry.Type
		t.Run(stepType, func(t *testing.T) {
			graph := outputSchemaCoverageGraph(stepType)
			schemas, err := (&DataPipelineService{}).inferOutputSchemas(context.Background(), 1, graph)
			if err != nil {
				t.Fatalf("inferOutputSchemas() error = %v", err)
			}
			columnsByNode := schemaColumnsByNode(schemas)
			nodeID := stepType
			if stepType == DataPipelineStepInput {
				nodeID = "input"
			}
			if stepType == DataPipelineStepOutput {
				nodeID = "output"
			}
			columns := columnsByNode[nodeID]
			if len(columns) == 0 {
				t.Fatalf("schema for %q is empty: %#v", nodeID, schemas)
			}
		})
	}
}

func outputSchemaCoverageGraph(stepType string) DataPipelineGraph {
	leftInput := DataPipelineNode{
		ID: "input",
		Data: DataPipelineNodeData{
			StepType: DataPipelineStepInput,
			Config: map[string]any{
				"sourceKind": dataPipelineDriveFileSource,
				"inputMode":  "json",
				"fields": []any{
					map[string]any{"column": "id"},
					map[string]any{"column": "name"},
					map[string]any{"column": "amount"},
					map[string]any{"column": "status"},
					map[string]any{"column": "updated_at"},
					map[string]any{"column": "raw_text"},
				},
				"includeRawRecord": true,
			},
		},
	}
	rightInput := DataPipelineNode{
		ID: "right_input",
		Data: DataPipelineNodeData{
			StepType: DataPipelineStepInput,
			Config: map[string]any{
				"sourceKind": dataPipelineDriveFileSource,
				"inputMode":  "json",
				"fields": []any{
					map[string]any{"column": "id"},
					map[string]any{"column": "category"},
				},
			},
		},
	}
	if stepType == DataPipelineStepInput {
		return DataPipelineGraph{Nodes: []DataPipelineNode{leftInput}}
	}
	if stepType == DataPipelineStepUnion {
		return DataPipelineGraph{
			Nodes: []DataPipelineNode{
				leftInput,
				rightInput,
				{ID: stepType, Data: DataPipelineNodeData{StepType: stepType, Config: map[string]any{"sourceLabelColumn": "source_node_id"}}},
			},
			Edges: []DataPipelineEdge{
				{Source: "input", Target: stepType},
				{Source: "right_input", Target: stepType},
			},
		}
	}
	if stepType == DataPipelineStepJoin {
		return DataPipelineGraph{
			Nodes: []DataPipelineNode{
				leftInput,
				rightInput,
				{ID: stepType, Data: DataPipelineNodeData{StepType: stepType, Config: map[string]any{"leftKeys": []any{"id"}, "rightKeys": []any{"id"}}}},
			},
			Edges: []DataPipelineEdge{
				{Source: "input", Target: stepType},
				{Source: "right_input", Target: stepType},
			},
		}
	}
	config := outputSchemaCoverageConfig(stepType)
	return DataPipelineGraph{
		Nodes: []DataPipelineNode{
			leftInput,
			{ID: stepType, Data: DataPipelineNodeData{StepType: stepType, Config: config}},
		},
		Edges: []DataPipelineEdge{{Source: "input", Target: stepType}},
	}
}

func outputSchemaCoverageConfig(stepType string) map[string]any {
	switch stepType {
	case DataPipelineStepEnrichJoin:
		return map[string]any{
			"rightSourceKind": dataPipelineDriveFileSource,
			"rightInputMode":  "json",
			"rightFields":     []any{map[string]any{"column": "category"}},
			"leftKeys":        []any{"id"},
			"rightKeys":       []any{"id"},
		}
	case DataPipelineStepTransform:
		return map[string]any{"operation": "rename_columns", "renames": map[string]any{"name": "customer_name"}}
	case DataPipelineStepOutput:
		return map[string]any{"columns": []any{map[string]any{"sourceColumn": "id", "name": "id"}}}
	case DataPipelineStepJSONExtract:
		return map[string]any{"fields": []any{map[string]any{"column": "json_name"}}, "includeSourceColumns": true}
	case DataPipelineStepExcelExtract:
		return map[string]any{"columns": []any{"sheet_name", "sheet_amount"}, "includeSourceColumns": true}
	case DataPipelineStepClassifyDocument:
		return map[string]any{"outputColumn": "document_type", "confidenceColumn": "document_confidence"}
	case DataPipelineStepExtractFields:
		return map[string]any{"fields": []any{map[string]any{"name": "invoice_id"}}}
	case DataPipelineStepProductExtraction:
		return map[string]any{"includeSourceColumns": true}
	case DataPipelineStepRouteByCondition:
		return map[string]any{"routeColumn": "route_key"}
	case DataPipelineStepDeduplicate:
		return map[string]any{"groupColumn": "duplicate_group_id", "statusColumn": "duplicate_status"}
	case DataPipelineStepCanonicalize:
		return map[string]any{"rules": []any{map[string]any{"column": "name", "outputColumn": "canonical_name"}}}
	case DataPipelineStepRedactPII:
		return map[string]any{"columns": []any{"raw_text"}, "outputSuffix": "_redacted"}
	case DataPipelineStepDetectLanguage:
		return map[string]any{"outputTextColumn": "normalized_text"}
	case DataPipelineStepEntityResolution:
		return map[string]any{"column": "name", "outputPrefix": "customer"}
	case DataPipelineStepUnitConversion:
		return map[string]any{"rules": []any{map[string]any{"valueColumn": "amount"}}}
	case DataPipelineStepSchemaMapping:
		return map[string]any{"includeSourceColumns": true, "mappings": []any{map[string]any{"targetColumn": "mapped_name"}}}
	case DataPipelineStepSchemaCompletion:
		return map[string]any{"rules": []any{map[string]any{"targetColumn": "completed_status"}}}
	case DataPipelineStepHumanReview:
		return map[string]any{"statusColumn": "review_status", "queueColumn": "review_queue"}
	case DataPipelineStepSnapshotSCD2:
		return map[string]any{"uniqueKeys": []any{"id"}, "updatedAtColumn": "updated_at"}
	default:
		return map[string]any{}
	}
}

func schemaColumnsByNode(schemas []DataPipelineNodeOutputSchema) map[string][]string {
	out := make(map[string][]string, len(schemas))
	for _, schema := range schemas {
		out[schema.NodeID] = schema.Columns
	}
	return out
}
