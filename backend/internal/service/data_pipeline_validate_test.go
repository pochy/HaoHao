package service

import (
	"strings"
	"testing"
)

func TestCompileValidateAnnotatesRows(t *testing.T) {
	compiler := &dataPipelineCompiler{}
	node := DataPipelineNode{
		ID: "validate",
		Data: DataPipelineNodeData{
			StepType: DataPipelineStepValidate,
			Config: map[string]any{"rules": []any{
				map[string]any{"column": "name", "operator": "required"},
				map[string]any{"column": "amount", "operator": "range", "min": 0, "severity": "warning"},
			}},
		},
	}
	upstream := dataPipelineRelation{
		CTE:     "input",
		Columns: []string{"name", "amount"},
	}

	relation, err := compiler.compileValidate(node, upstream)
	if err != nil {
		t.Fatalf("compileValidate() error = %v", err)
	}
	for _, column := range []string{"name", "amount", "validation_status", "validation_errors_json"} {
		if !dataPipelineContainsString(relation.Columns, column) {
			t.Fatalf("compiled columns missing %q: %#v", column, relation.Columns)
		}
	}
	for _, fragment := range []string{"multiIf", "validation_status", "validation_errors_json", "arrayStringConcat"} {
		if !strings.Contains(relation.SQL, fragment) {
			t.Fatalf("compiled SQL missing %q:\n%s", fragment, relation.SQL)
		}
	}
}

func TestValidationUniqueRuleCanAnnotateRows(t *testing.T) {
	statusExpr, errorsExpr, err := dataPipelineValidationRowExpressions([]string{"id"}, map[string]any{"rules": []any{
		map[string]any{"column": "id", "operator": "unique"},
	}})
	if err != nil {
		t.Fatalf("dataPipelineValidationRowExpressions() error = %v", err)
	}
	if !strings.Contains(statusExpr, "count() OVER (PARTITION BY `id`) = 1") {
		t.Fatalf("status expression does not use row-level unique check: %s", statusExpr)
	}
	if !strings.Contains(errorsExpr, `"operator":"unique"`) {
		t.Fatalf("errors expression does not include unique detail: %s", errorsExpr)
	}
}
