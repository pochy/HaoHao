package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	DataPipelineStepInput             = "input"
	DataPipelineStepProfile           = "profile"
	DataPipelineStepClean             = "clean"
	DataPipelineStepNormalize         = "normalize"
	DataPipelineStepValidate          = "validate"
	DataPipelineStepSchemaMapping     = "schema_mapping"
	DataPipelineStepSchemaCompletion  = "schema_completion"
	DataPipelineStepUnion             = "union"
	DataPipelineStepJoin              = "join"
	DataPipelineStepEnrichJoin        = "enrich_join"
	DataPipelineStepTransform         = "transform"
	DataPipelineStepOutput            = "output"
	DataPipelineStepExtractText       = "extract_text"
	DataPipelineStepJSONExtract       = "json_extract"
	DataPipelineStepExcelExtract      = "excel_extract"
	DataPipelineStepClassifyDocument  = "classify_document"
	DataPipelineStepExtractFields     = "extract_fields"
	DataPipelineStepExtractTable      = "extract_table"
	DataPipelineStepProductExtraction = "product_extraction"
	DataPipelineStepConfidenceGate    = "confidence_gate"
	DataPipelineStepQuarantine        = "quarantine"
	DataPipelineStepRouteByCondition  = "route_by_condition"
	DataPipelineStepPartitionFilter   = "partition_filter"
	DataPipelineStepWatermarkFilter   = "watermark_filter"
	DataPipelineStepSnapshotSCD2      = "snapshot_scd2"
	DataPipelineStepDeduplicate       = "deduplicate"
	DataPipelineStepCanonicalize      = "canonicalize"
	DataPipelineStepRedactPII         = "redact_pii"
	DataPipelineStepDetectLanguage    = "detect_language_encoding"
	DataPipelineStepSchemaInference   = "schema_inference"
	DataPipelineStepEntityResolution  = "entity_resolution"
	DataPipelineStepUnitConversion    = "unit_conversion"
	DataPipelineStepRelationship      = "relationship_extraction"
	DataPipelineStepHumanReview       = "human_review"
	DataPipelineStepSampleCompare     = "sample_compare"
	DataPipelineStepQualityReport     = "quality_report"

	dataPipelineMaxNodes = 50
	dataPipelineMaxEdges = 80
)

var dataPipelineStepCatalog = map[string]struct{}{
	DataPipelineStepInput:             {},
	DataPipelineStepProfile:           {},
	DataPipelineStepClean:             {},
	DataPipelineStepNormalize:         {},
	DataPipelineStepValidate:          {},
	DataPipelineStepSchemaMapping:     {},
	DataPipelineStepSchemaCompletion:  {},
	DataPipelineStepUnion:             {},
	DataPipelineStepJoin:              {},
	DataPipelineStepEnrichJoin:        {},
	DataPipelineStepTransform:         {},
	DataPipelineStepOutput:            {},
	DataPipelineStepExtractText:       {},
	DataPipelineStepJSONExtract:       {},
	DataPipelineStepExcelExtract:      {},
	DataPipelineStepClassifyDocument:  {},
	DataPipelineStepExtractFields:     {},
	DataPipelineStepExtractTable:      {},
	DataPipelineStepProductExtraction: {},
	DataPipelineStepConfidenceGate:    {},
	DataPipelineStepQuarantine:        {},
	DataPipelineStepRouteByCondition:  {},
	DataPipelineStepPartitionFilter:   {},
	DataPipelineStepWatermarkFilter:   {},
	DataPipelineStepSnapshotSCD2:      {},
	DataPipelineStepDeduplicate:       {},
	DataPipelineStepCanonicalize:      {},
	DataPipelineStepRedactPII:         {},
	DataPipelineStepDetectLanguage:    {},
	DataPipelineStepSchemaInference:   {},
	DataPipelineStepEntityResolution:  {},
	DataPipelineStepUnitConversion:    {},
	DataPipelineStepRelationship:      {},
	DataPipelineStepHumanReview:       {},
	DataPipelineStepSampleCompare:     {},
	DataPipelineStepQualityReport:     {},
}

type DataPipelineGraph struct {
	Nodes []DataPipelineNode `json:"nodes"`
	Edges []DataPipelineEdge `json:"edges"`
}

type DataPipelineNode struct {
	ID       string               `json:"id"`
	Type     string               `json:"type,omitempty"`
	Position map[string]float64   `json:"position,omitempty"`
	Data     DataPipelineNodeData `json:"data"`
}

type DataPipelineNodeData struct {
	Label    string         `json:"label,omitempty"`
	StepType string         `json:"stepType"`
	Config   map[string]any `json:"config,omitempty"`
}

type DataPipelineEdge struct {
	ID     string `json:"id,omitempty"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type DataPipelineValidationSummary struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

func validateDataPipelineGraph(graph DataPipelineGraph) DataPipelineValidationSummary {
	return validateDataPipelineGraphForUse(graph, true)
}

func validateDataPipelineDraftGraph(graph DataPipelineGraph) DataPipelineValidationSummary {
	return validateDataPipelineGraphForUse(graph, false)
}

func validateDataPipelinePreviewGraph(graph DataPipelineGraph) DataPipelineValidationSummary {
	return validateDataPipelineGraphForUse(graph, false)
}

func validateDataPipelineGraphForUse(graph DataPipelineGraph, requireOutput bool) DataPipelineValidationSummary {
	var errors []string
	if len(graph.Nodes) == 0 {
		errors = append(errors, "graph must contain nodes")
	}
	if len(graph.Nodes) > dataPipelineMaxNodes {
		errors = append(errors, fmt.Sprintf("graph cannot contain more than %d nodes", dataPipelineMaxNodes))
	}
	if len(graph.Edges) > dataPipelineMaxEdges {
		errors = append(errors, fmt.Sprintf("graph cannot contain more than %d edges", dataPipelineMaxEdges))
	}

	nodes := make(map[string]DataPipelineNode, len(graph.Nodes))
	inputCount := 0
	outputCount := 0
	for _, node := range graph.Nodes {
		nodeID := strings.TrimSpace(node.ID)
		if nodeID == "" {
			errors = append(errors, "node id is required")
			continue
		}
		if _, exists := nodes[nodeID]; exists {
			errors = append(errors, "node id must be unique: "+nodeID)
			continue
		}
		stepType := strings.TrimSpace(node.Data.StepType)
		if _, ok := dataPipelineStepCatalog[stepType]; !ok {
			errors = append(errors, "unsupported step type for node "+nodeID+": "+stepType)
		}
		switch stepType {
		case DataPipelineStepInput:
			inputCount++
		case DataPipelineStepOutput:
			outputCount++
		}
		nodes[nodeID] = node
	}
	if inputCount < 1 {
		errors = append(errors, "graph must contain at least one input node")
	}
	if requireOutput && outputCount < 1 {
		errors = append(errors, "graph must contain at least one output node")
	}
	if requireOutput {
		errors = append(errors, validateDataPipelineOutputConfigs(graph)...)
	}

	outgoing := make(map[string][]string, len(nodes))
	incoming := make(map[string][]string, len(nodes))
	for _, edge := range graph.Edges {
		source := strings.TrimSpace(edge.Source)
		target := strings.TrimSpace(edge.Target)
		if source == "" || target == "" {
			errors = append(errors, "edge source and target are required")
			continue
		}
		if source == target {
			errors = append(errors, "self-loop is not allowed: "+source)
			continue
		}
		if _, ok := nodes[source]; !ok {
			errors = append(errors, "edge source node does not exist: "+source)
			continue
		}
		if _, ok := nodes[target]; !ok {
			errors = append(errors, "edge target node does not exist: "+target)
			continue
		}
		outgoing[source] = append(outgoing[source], target)
		incoming[target] = append(incoming[target], source)
	}

	if len(errors) == 0 {
		if _, err := dataPipelineTopologicalOrder(graph); err != nil {
			errors = append(errors, err.Error())
		}
	}

	inputIDs := make([]string, 0)
	outputIDs := make(map[string]struct{})
	for _, node := range graph.Nodes {
		switch node.Data.StepType {
		case DataPipelineStepInput:
			inputIDs = append(inputIDs, node.ID)
		case DataPipelineStepOutput:
			outputIDs[node.ID] = struct{}{}
		}
	}
	if len(inputIDs) > 0 {
		reachableFromInput := dataPipelineReachableAny(inputIDs, outgoing)
		canReachOutput := dataPipelineReverseReachable(outputIDs, incoming)
		for _, node := range graph.Nodes {
			if _, ok := reachableFromInput[node.ID]; !ok {
				errors = append(errors, "node is not reachable from input: "+node.ID)
			}
			if requireOutput {
				if _, ok := canReachOutput[node.ID]; !ok {
					errors = append(errors, "node does not reach an output: "+node.ID)
				}
			}
			upstreamCount := len(incoming[node.ID])
			switch node.Data.StepType {
			case DataPipelineStepInput:
				if upstreamCount != 0 {
					errors = append(errors, "input node cannot have upstream edges: "+node.ID)
				}
			case DataPipelineStepUnion:
				if upstreamCount < 2 {
					errors = append(errors, "union node must have at least two upstream edges: "+node.ID)
				}
			case DataPipelineStepJoin:
				if upstreamCount != 2 {
					errors = append(errors, "join node must have exactly two upstream edges: "+node.ID)
				}
			case DataPipelineStepEnrichJoin:
				if upstreamCount != 1 {
					errors = append(errors, "enrich join node must have exactly one upstream edge: "+node.ID)
				}
			default:
				if upstreamCount == 0 {
					errors = append(errors, "node has no upstream edge: "+node.ID)
				}
				if upstreamCount > 1 {
					errors = append(errors, "node must have exactly one upstream edge; use a join node to combine multiple inputs: "+node.ID)
				}
			}
		}
	}

	return DataPipelineValidationSummary{
		Valid:  len(errors) == 0,
		Errors: dataPipelineUniqueStrings(errors),
	}
}

func validateDataPipelineOutputConfigs(graph DataPipelineGraph) []string {
	var errors []string
	tableNames := make(map[string]string)
	for _, node := range graph.Nodes {
		if node.Data.StepType != DataPipelineStepOutput {
			continue
		}
		writeMode := strings.TrimSpace(dataPipelineString(node.Data.Config, "writeMode"))
		if writeMode != "" && writeMode != "replace" {
			errors = append(errors, "output node only supports replace writeMode: "+node.ID)
		}
		tableName := strings.TrimSpace(dataPipelineString(node.Data.Config, "tableName"))
		if tableName != "" {
			if err := dataPipelineValidateIdentifier(tableName); err != nil {
				errors = append(errors, "invalid output tableName for node "+node.ID+": "+err.Error())
				continue
			}
			key := strings.ToLower(tableName)
			if previous, ok := tableNames[key]; ok {
				errors = append(errors, "output tableName must be unique: "+previous+" and "+node.ID)
				continue
			}
			tableNames[key] = node.ID
		}
		for _, column := range dataPipelineStringSlice(node.Data.Config, "orderBy") {
			if err := dataPipelineValidateIdentifier(column); err != nil {
				errors = append(errors, "invalid output orderBy for node "+node.ID+": "+err.Error())
			}
		}
		seenColumns := make(map[string]struct{})
		for _, column := range dataPipelineConfigObjects(node.Data.Config, "columns") {
			name := strings.TrimSpace(dataPipelineString(column, "name"))
			if name == "" {
				name = strings.TrimSpace(dataPipelineString(column, "outputColumn"))
			}
			if name == "" {
				name = strings.TrimSpace(dataPipelineString(column, "sourceColumn"))
			}
			if name == "" {
				name = strings.TrimSpace(dataPipelineString(column, "column"))
			}
			if name == "" {
				errors = append(errors, "output column name is required for node "+node.ID)
				continue
			}
			if err := dataPipelineValidateIdentifier(name); err != nil {
				errors = append(errors, "invalid output column for node "+node.ID+": "+err.Error())
				continue
			}
			key := strings.ToLower(name)
			if _, ok := seenColumns[key]; ok {
				errors = append(errors, "duplicate output column for node "+node.ID+": "+name)
			}
			seenColumns[key] = struct{}{}
			if _, err := dataPipelineOutputCastExpr("dummy", dataPipelineString(column, "type")); err != nil {
				errors = append(errors, "invalid output column type for node "+node.ID+": "+err.Error())
			}
		}
	}
	return errors
}

func dataPipelineOutputNodes(graph DataPipelineGraph) []DataPipelineNode {
	order, err := dataPipelineTopologicalOrder(graph)
	if err != nil {
		order = graph.Nodes
	}
	outputs := make([]DataPipelineNode, 0)
	for _, node := range order {
		if node.Data.StepType == DataPipelineStepOutput {
			outputs = append(outputs, node)
		}
	}
	return outputs
}

func dataPipelineTopologicalOrder(graph DataPipelineGraph) ([]DataPipelineNode, error) {
	nodes := make(map[string]DataPipelineNode, len(graph.Nodes))
	indegree := make(map[string]int, len(graph.Nodes))
	outgoing := make(map[string][]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
		indegree[node.ID] = 0
	}
	for _, edge := range graph.Edges {
		if _, ok := nodes[edge.Source]; !ok {
			continue
		}
		if _, ok := nodes[edge.Target]; !ok {
			continue
		}
		outgoing[edge.Source] = append(outgoing[edge.Source], edge.Target)
		indegree[edge.Target]++
	}

	ready := make([]string, 0)
	for id, degree := range indegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	ordered := make([]DataPipelineNode, 0, len(nodes))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, nodes[id])
		next := append([]string(nil), outgoing[id]...)
		sort.Strings(next)
		for _, target := range next {
			indegree[target]--
			if indegree[target] == 0 {
				ready = append(ready, target)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) != len(nodes) {
		return nil, fmt.Errorf("graph must be acyclic")
	}
	return ordered, nil
}

func dataPipelinePreviewSubgraph(graph DataPipelineGraph, selectedNodeID string) (DataPipelineGraph, string, error) {
	selectedNodeID = strings.TrimSpace(selectedNodeID)
	if selectedNodeID == "" {
		for _, node := range graph.Nodes {
			if node.Data.StepType == DataPipelineStepOutput {
				selectedNodeID = node.ID
				break
			}
		}
	}
	if selectedNodeID == "" {
		return DataPipelineGraph{}, "", fmt.Errorf("%w: selected node not found", ErrInvalidDataPipelineGraph)
	}

	nodes := make(map[string]DataPipelineNode, len(graph.Nodes))
	incoming := make(map[string][]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	if _, ok := nodes[selectedNodeID]; !ok {
		return DataPipelineGraph{}, "", fmt.Errorf("%w: selected node not found: %s", ErrInvalidDataPipelineGraph, selectedNodeID)
	}
	for _, edge := range graph.Edges {
		if _, ok := nodes[edge.Source]; !ok {
			continue
		}
		if _, ok := nodes[edge.Target]; !ok {
			continue
		}
		incoming[edge.Target] = append(incoming[edge.Target], edge.Source)
	}

	included := map[string]struct{}{selectedNodeID: {}}
	queue := []string{selectedNodeID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, source := range incoming[id] {
			if _, ok := included[source]; ok {
				continue
			}
			included[source] = struct{}{}
			queue = append(queue, source)
		}
	}

	subgraph := DataPipelineGraph{
		Nodes: make([]DataPipelineNode, 0, len(included)),
		Edges: make([]DataPipelineEdge, 0, len(graph.Edges)),
	}
	for _, node := range graph.Nodes {
		if _, ok := included[node.ID]; ok {
			subgraph.Nodes = append(subgraph.Nodes, node)
		}
	}
	for _, edge := range graph.Edges {
		if _, sourceOK := included[edge.Source]; !sourceOK {
			continue
		}
		if _, targetOK := included[edge.Target]; !targetOK {
			continue
		}
		subgraph.Edges = append(subgraph.Edges, edge)
	}
	return subgraph, selectedNodeID, nil
}

func dataPipelineReachable(start string, outgoing map[string][]string) map[string]struct{} {
	seen := map[string]struct{}{start: {}}
	queue := []string{start}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, next := range outgoing[id] {
			if _, ok := seen[next]; ok {
				continue
			}
			seen[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	return seen
}

func dataPipelineReachableAny(starts []string, outgoing map[string][]string) map[string]struct{} {
	seen := make(map[string]struct{}, len(starts))
	queue := make([]string, 0, len(starts))
	for _, start := range starts {
		if start == "" {
			continue
		}
		if _, ok := seen[start]; ok {
			continue
		}
		seen[start] = struct{}{}
		queue = append(queue, start)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, next := range outgoing[id] {
			if _, ok := seen[next]; ok {
				continue
			}
			seen[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	return seen
}

func dataPipelineReverseReachable(starts map[string]struct{}, incoming map[string][]string) map[string]struct{} {
	seen := make(map[string]struct{}, len(starts))
	queue := make([]string, 0, len(starts))
	for start := range starts {
		seen[start] = struct{}{}
		queue = append(queue, start)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, prev := range incoming[id] {
			if _, ok := seen[prev]; ok {
				continue
			}
			seen[prev] = struct{}{}
			queue = append(queue, prev)
		}
	}
	return seen
}

func dataPipelineUniqueStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := set[value]; ok {
			continue
		}
		set[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func encodeDataPipelineJSON(value any) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode data pipeline json: %w", err)
	}
	return payload, nil
}

func decodeDataPipelineGraph(payload []byte) (DataPipelineGraph, error) {
	var graph DataPipelineGraph
	if len(payload) == 0 {
		return graph, nil
	}
	if err := json.Unmarshal(payload, &graph); err != nil {
		return DataPipelineGraph{}, fmt.Errorf("%w: graph json is invalid", ErrInvalidDataPipelineGraph)
	}
	return graph, nil
}

func decodeDataPipelineJSONMap(payload []byte) map[string]any {
	if len(payload) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func decodeDataPipelineJSONStringList(payload []byte) []string {
	if len(payload) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(payload, &out); err == nil && out != nil {
		return out
	}
	var values []any
	if err := json.Unmarshal(payload, &values); err != nil {
		return []string{}
	}
	out = make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func nextDataPipelineScheduleRunAfter(frequency, timezoneName, runTime string, weekday, monthDay *int32, after time.Time) (time.Time, error) {
	return nextWorkTableExportScheduleRunAfter(frequency, timezoneName, runTime, weekday, monthDay, after)
}
