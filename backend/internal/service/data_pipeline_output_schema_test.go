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

func schemaColumnsByNode(schemas []DataPipelineNodeOutputSchema) map[string][]string {
	out := make(map[string][]string, len(schemas))
	for _, schema := range schemas {
		out[schema.NodeID] = schema.Columns
	}
	return out
}
