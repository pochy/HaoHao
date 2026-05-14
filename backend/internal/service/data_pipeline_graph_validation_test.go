package service

import "testing"

func TestDataPipelineMissingColumnWarningsUsesInferredStepSchemas(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{
				ID: "input",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepInput,
					Config:   map[string]any{"sourceKind": dataPipelineDriveFileSource},
				},
			},
			{ID: "extract_text", Data: DataPipelineNodeData{StepType: DataPipelineStepExtractText}},
			{
				ID: "quality_report",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepQualityReport,
					Config:   map[string]any{"columns": []any{"text", "confidence"}},
				},
			},
		},
		Edges: []DataPipelineEdge{
			{Source: "input", Target: "extract_text"},
			{Source: "extract_text", Target: "quality_report"},
		},
	}
	schemas := []DataPipelineNodeOutputSchema{
		{NodeID: "input", StepType: DataPipelineStepInput, Columns: []string{"file_public_id", "file_name"}},
		{NodeID: "extract_text", StepType: DataPipelineStepExtractText, Columns: []string{"file_public_id", "text", "confidence"}},
		{NodeID: "quality_report", StepType: DataPipelineStepQualityReport, Columns: []string{"file_public_id", "text", "confidence", "quality_report_json"}},
	}

	warnings := dataPipelineMissingColumnWarnings(graph, schemas)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
}

func TestDataPipelineMissingColumnWarningsReportsMissingColumns(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{ID: "input", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{
				ID: "output",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepOutput,
					Config:   map[string]any{"orderBy": []any{"missing_id", "name"}},
				},
			},
		},
		Edges: []DataPipelineEdge{{Source: "input", Target: "output"}},
	}
	schemas := []DataPipelineNodeOutputSchema{
		{NodeID: "input", StepType: DataPipelineStepInput, Columns: []string{"name"}},
		{NodeID: "output", StepType: DataPipelineStepOutput, Columns: []string{"name"}},
	}

	warnings := dataPipelineMissingColumnWarnings(graph, schemas)
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %#v", warnings)
	}
	warning := warnings[0]
	if warning.NodeID != "output" || warning.Code != dataPipelineWarningMissingUpstreamColumns {
		t.Fatalf("unexpected warning metadata: %#v", warning)
	}
	if len(warning.Columns) != 1 || warning.Columns[0] != "missing_id" {
		t.Fatalf("unexpected missing columns: %#v", warning.Columns)
	}
}

func TestDataPipelineMissingColumnWarningsReportsRightJoinColumns(t *testing.T) {
	graph := DataPipelineGraph{
		Nodes: []DataPipelineNode{
			{ID: "left", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{ID: "right", Data: DataPipelineNodeData{StepType: DataPipelineStepInput}},
			{
				ID: "join",
				Data: DataPipelineNodeData{
					StepType: DataPipelineStepJoin,
					Config: map[string]any{
						"leftKeys":  []any{"id"},
						"rightKeys": []any{"missing_id"},
					},
				},
			},
		},
		Edges: []DataPipelineEdge{
			{Source: "left", Target: "join"},
			{Source: "right", Target: "join"},
		},
	}
	schemas := []DataPipelineNodeOutputSchema{
		{NodeID: "left", StepType: DataPipelineStepInput, Columns: []string{"id"}},
		{NodeID: "right", StepType: DataPipelineStepInput, Columns: []string{"other_id"}},
		{NodeID: "join", StepType: DataPipelineStepJoin, Columns: []string{"id", "other_id"}},
	}

	warnings := dataPipelineMissingColumnWarnings(graph, schemas)
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %#v", warnings)
	}
	if warnings[0].Code != dataPipelineWarningMissingRightUpstreamColumns {
		t.Fatalf("unexpected warning code: %#v", warnings[0])
	}
	if len(warnings[0].Columns) != 1 || warnings[0].Columns[0] != "missing_id" {
		t.Fatalf("unexpected missing columns: %#v", warnings[0].Columns)
	}
}
