package service

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateDataPipelineGraph(t *testing.T) {
	graph := dataPipelineTestGraph()
	summary := validateDataPipelineGraph(graph)
	if !summary.Valid {
		t.Fatalf("expected graph to be valid, got errors: %v", summary.Errors)
	}

	graph.Edges = append(graph.Edges, DataPipelineEdge{ID: "cycle", Source: "output_1", Target: "clean_1"})
	summary = validateDataPipelineGraph(graph)
	if summary.Valid || !containsDataPipelineValidationError(summary.Errors, "graph must be acyclic") {
		t.Fatalf("expected cycle error, got valid=%v errors=%v", summary.Valid, summary.Errors)
	}
}

func TestValidateDataPipelineGraphRejectsOrphanExecutableNode(t *testing.T) {
	graph := dataPipelineTestGraph()
	graph.Nodes = append(graph.Nodes, DataPipelineNode{
		ID:   "normalize_orphan",
		Type: "pipelineStep",
		Data: DataPipelineNodeData{StepType: DataPipelineStepNormalize},
	})

	summary := validateDataPipelineGraph(graph)
	if summary.Valid {
		t.Fatalf("expected orphan node to be invalid")
	}
	if !containsDataPipelineValidationError(summary.Errors, "node is not reachable from input: normalize_orphan") {
		t.Fatalf("expected reachability error, got %v", summary.Errors)
	}
	if !containsDataPipelineValidationError(summary.Errors, "node does not reach an output: normalize_orphan") {
		t.Fatalf("expected output reachability error, got %v", summary.Errors)
	}
	if !containsDataPipelineValidationError(summary.Errors, "node has no upstream edge: normalize_orphan") {
		t.Fatalf("expected upstream edge error, got %v", summary.Errors)
	}
}

func TestDataPipelineCompilerRejectsUnsafeIdentifiers(t *testing.T) {
	err := dataPipelineValidateIdentifier("target; DROP TABLE x")
	if !errors.Is(err, ErrInvalidDataPipelineGraph) {
		t.Fatalf("expected invalid graph error, got %v", err)
	}

	compiler := &dataPipelineCompiler{}
	upstream := dataPipelineRelation{
		CTE:     "step_input_1",
		Columns: []string{"name", "amount"},
	}
	_, err = compiler.compileSchemaMapping(DataPipelineNode{
		ID: "schema_mapping_1",
		Data: DataPipelineNodeData{
			StepType: DataPipelineStepSchemaMapping,
			Config: map[string]any{
				"mappings": []any{
					map[string]any{"sourceColumn": "name", "targetColumn": "safe_name"},
					map[string]any{"sourceColumn": "amount", "targetColumn": "amount) FROM system.tables --"},
				},
			},
		},
	}, upstream)
	if !errors.Is(err, ErrInvalidDataPipelineGraph) {
		t.Fatalf("expected invalid graph error for unsafe target column, got %v", err)
	}
}

func TestDataPipelineCompilerGeneratesStructuredFilterSQL(t *testing.T) {
	compiler := &dataPipelineCompiler{}
	upstream := dataPipelineRelation{
		CTE:     "step_input_1",
		Columns: []string{"name", "amount"},
	}
	relation, err := compiler.compileTransform(DataPipelineNode{
		ID: "transform_1",
		Data: DataPipelineNodeData{
			StepType: DataPipelineStepTransform,
			Config: map[string]any{
				"operation": "filter",
				"conditions": []any{
					map[string]any{"column": "amount", "operator": ">=", "value": 10},
				},
			},
		},
	}, upstream)
	if err != nil {
		t.Fatalf("compile transform: %v", err)
	}
	if !strings.Contains(relation.SQL, "FROM `step_input_1`") {
		t.Fatalf("expected upstream CTE to be quoted, got SQL:\n%s", relation.SQL)
	}
	if !strings.Contains(relation.SQL, "`amount` >= 10") {
		t.Fatalf("expected structured condition SQL, got SQL:\n%s", relation.SQL)
	}
}

func TestDataPipelineCompilerRejectsUnsupportedOperatorAndUnknownColumn(t *testing.T) {
	columns := []string{"name", "amount"}
	if _, err := dataPipelineConditionExpr(columns, "amount", map[string]any{"operator": "contains_sql", "value": "x"}); !errors.Is(err, ErrInvalidDataPipelineGraph) {
		t.Fatalf("expected unsupported operator to be rejected, got %v", err)
	}
	if _, err := dataPipelineConditionExpr(columns, "missing", map[string]any{"operator": "=", "value": "x"}); !errors.Is(err, ErrInvalidDataPipelineGraph) {
		t.Fatalf("expected unknown column to be rejected, got %v", err)
	}
}

func dataPipelineTestGraph() DataPipelineGraph {
	return DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{
				ID:   "input_1",
				Type: "pipelineStep",
				Data: DataPipelineNodeData{StepType: DataPipelineStepInput},
			},
			{
				ID:   "clean_1",
				Type: "pipelineStep",
				Data: DataPipelineNodeData{StepType: DataPipelineStepClean},
			},
			{
				ID:   "output_1",
				Type: "pipelineStep",
				Data: DataPipelineNodeData{StepType: DataPipelineStepOutput},
			},
		},
		Edges: []DataPipelineEdge{
			{ID: "input_clean", Source: "input_1", Target: "clean_1"},
			{ID: "clean_output", Source: "clean_1", Target: "output_1"},
		},
	}
}

func containsDataPipelineValidationError(errors []string, want string) bool {
	for _, err := range errors {
		if strings.Contains(err, want) {
			return true
		}
	}
	return false
}
