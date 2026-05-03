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

func TestValidateDataPipelineGraphAllowsMultipleInputs(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{ID: "input_1", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{ID: "input_2", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{ID: "join_1", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepJoin}},
			{ID: "output_1", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{
			{ID: "input_1_join", Source: "input_1", Target: "join_1"},
			{ID: "input_2_join", Source: "input_2", Target: "join_1"},
			{ID: "join_output", Source: "join_1", Target: "output_1"},
		},
	}

	summary := validateDataPipelineGraph(graph)
	if !summary.Valid {
		t.Fatalf("expected multiple input graph to be valid, got errors: %v", summary.Errors)
	}
}

func TestValidateDataPipelineGraphRejectsDirectMultipleUpstreamOutput(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{ID: "input_1", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{ID: "input_2", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{ID: "output_1", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{
			{ID: "input_1_output", Source: "input_1", Target: "output_1"},
			{ID: "input_2_output", Source: "input_2", Target: "output_1"},
		},
	}

	summary := validateDataPipelineGraph(graph)
	if summary.Valid {
		t.Fatalf("expected direct multiple upstream output to be invalid")
	}
	if !containsDataPipelineValidationError(summary.Errors, "use a join node to combine multiple inputs: output_1") {
		t.Fatalf("expected join guidance error, got %v", summary.Errors)
	}
}

func TestDataPipelinePreviewSubgraphIgnoresUnrelatedInput(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{ID: "input_1", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{ID: "input_2", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{ID: "normalize_1", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepNormalize}},
			{ID: "output_1", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{
			{ID: "input_1_normalize", Source: "input_1", Target: "normalize_1"},
			{ID: "normalize_output", Source: "normalize_1", Target: "output_1"},
		},
	}

	subgraph, selectedNodeID, err := dataPipelinePreviewSubgraph(graph, "normalize_1")
	if err != nil {
		t.Fatalf("preview subgraph: %v", err)
	}
	if selectedNodeID != "normalize_1" {
		t.Fatalf("selected node id = %q, want normalize_1", selectedNodeID)
	}
	if len(subgraph.Nodes) != 2 {
		t.Fatalf("expected only selected node and its input ancestor, got %#v", subgraph.Nodes)
	}
	summary := validateDataPipelinePreviewGraph(subgraph)
	if !summary.Valid {
		t.Fatalf("expected preview subgraph to be valid, got %v", summary.Errors)
	}
}

func TestValidateDataPipelineDraftGraphAllowsUnrelatedInput(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{ID: "input_1", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{ID: "input_2", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{ID: "output_1", Type: "pipelineStep", Data: DataPipelineNodeData{StepType: DataPipelineStepOutput}},
		},
		Edges: []DataPipelineEdge{
			{ID: "input_1_output", Source: "input_1", Target: "output_1"},
		},
	}

	draftSummary := validateDataPipelineDraftGraph(graph)
	if !draftSummary.Valid {
		t.Fatalf("expected draft graph to allow unrelated input, got %v", draftSummary.Errors)
	}
	publishSummary := validateDataPipelineGraph(graph)
	if publishSummary.Valid {
		t.Fatalf("expected publish graph to reject unrelated input")
	}
	if !containsDataPipelineValidationError(publishSummary.Errors, "node does not reach an output: input_2") {
		t.Fatalf("expected output reachability error, got %v", publishSummary.Errors)
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

func TestDataPipelineCompilerJoinsGraphInput(t *testing.T) {
	relation := compileDataPipelineJoinForTest(t, map[string]any{
		"leftKeys":      []any{"customer_id"},
		"rightKeys":     []any{"customer_id"},
		"selectColumns": []any{"segment"},
	})
	if !strings.Contains(relation.SQL, "LEFT ALL JOIN `step_input_2` AS r") {
		t.Fatalf("expected graph input CTE join, got SQL:\n%s", relation.SQL)
	}
	if !strings.Contains(relation.SQL, "r.`segment` AS `segment`") {
		t.Fatalf("expected selected right column, got SQL:\n%s", relation.SQL)
	}
}

func TestDataPipelineCompilerJoinTypes(t *testing.T) {
	tests := []struct {
		joinType string
		wantSQL  string
	}{
		{joinType: "inner", wantSQL: "INNER ALL JOIN `step_input_2` AS r ON"},
		{joinType: "left", wantSQL: "LEFT ALL JOIN `step_input_2` AS r ON"},
		{joinType: "right", wantSQL: "RIGHT ALL JOIN `step_input_2` AS r ON"},
		{joinType: "full", wantSQL: "FULL ALL JOIN `step_input_2` AS r ON"},
		{joinType: "cross", wantSQL: "CROSS JOIN `step_input_2` AS r"},
	}
	for _, tt := range tests {
		t.Run(tt.joinType, func(t *testing.T) {
			config := map[string]any{
				"joinType":      tt.joinType,
				"selectColumns": []any{"segment"},
			}
			if tt.joinType != "cross" {
				config["leftKeys"] = []any{"customer_id"}
				config["rightKeys"] = []any{"customer_id"}
			}
			relation := compileDataPipelineJoinForTest(t, config)
			if !strings.Contains(relation.SQL, tt.wantSQL) {
				t.Fatalf("expected %q, got SQL:\n%s", tt.wantSQL, relation.SQL)
			}
			if tt.joinType == "cross" && strings.Contains(relation.SQL, " ON ") {
				t.Fatalf("cross join must not emit ON clause, got SQL:\n%s", relation.SQL)
			}
		})
	}
}

func TestDataPipelineCompilerAnyJoinStrictness(t *testing.T) {
	relation := compileDataPipelineJoinForTest(t, map[string]any{
		"joinType":       "left",
		"joinStrictness": "any",
		"leftKeys":       []any{"customer_id"},
		"rightKeys":      []any{"customer_id"},
		"selectColumns":  []any{"segment"},
	})
	if !strings.Contains(relation.SQL, "LEFT ANY JOIN `step_input_2` AS r ON") {
		t.Fatalf("expected ANY join strictness, got SQL:\n%s", relation.SQL)
	}
}

func compileDataPipelineJoinForTest(t *testing.T, config map[string]any) dataPipelineRelation {
	t.Helper()
	compiler := &dataPipelineCompiler{
		incoming: map[string][]string{
			"join_1": {"input_1", "input_2"},
		},
	}
	relation, err := compiler.compileJoin(DataPipelineNode{
		ID: "join_1",
		Data: DataPipelineNodeData{
			StepType: DataPipelineStepJoin,
			Config:   config,
		},
	}, map[string]dataPipelineRelation{
		"input_1": {CTE: "step_input_1", Columns: []string{"customer_id", "amount"}},
		"input_2": {CTE: "step_input_2", Columns: []string{"customer_id", "segment"}},
	})
	if err != nil {
		t.Fatalf("compile graph input join: %v", err)
	}
	return relation
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
