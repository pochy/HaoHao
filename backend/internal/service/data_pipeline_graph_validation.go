package service

import (
	"context"
	"fmt"
	"strings"
)

const (
	dataPipelineWarningMissingUpstreamColumns      = "missing_upstream_columns"
	dataPipelineWarningMissingRightUpstreamColumns = "missing_right_upstream_columns"
)

func (s *DataPipelineService) ValidateDraft(ctx context.Context, tenantID, actorUserID int64, pipelinePublicID string, graph DataPipelineGraph) (DataPipelineGraphValidation, error) {
	if _, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID); err != nil {
		return DataPipelineGraphValidation{}, err
	}
	if s.authz != nil {
		if err := s.authz.CheckResourceAction(ctx, tenantID, actorUserID, DataResourceDataPipeline, pipelinePublicID, DataActionPreview); err != nil {
			return DataPipelineGraphValidation{}, err
		}
	}
	summary := validateDataPipelineDraftGraph(graph)
	result := DataPipelineGraphValidation{ValidationSummary: summary}
	if !summary.Valid {
		return result, nil
	}
	if err := s.checkGraphInputPermissions(ctx, tenantID, actorUserID, graph); err != nil {
		return DataPipelineGraphValidation{}, err
	}
	outputSchemas, err := s.inferOutputSchemas(ctx, tenantID, graph)
	if err != nil {
		return DataPipelineGraphValidation{}, err
	}
	result.OutputSchemas = outputSchemas
	result.NodeWarnings = dataPipelineMissingColumnWarnings(graph, outputSchemas)
	return result, nil
}

func dataPipelineMissingColumnWarnings(graph DataPipelineGraph, outputSchemas []DataPipelineNodeOutputSchema) []DataPipelineNodeWarning {
	columnsByNode := make(map[string][]string, len(outputSchemas))
	for _, schema := range outputSchemas {
		columnsByNode[schema.NodeID] = schema.Columns
	}
	incoming := make(map[string][]string, len(graph.Nodes))
	for _, edge := range graph.Edges {
		incoming[edge.Target] = append(incoming[edge.Target], edge.Source)
	}
	warnings := make([]DataPipelineNodeWarning, 0)
	for _, node := range graph.Nodes {
		if node.Data.StepType == DataPipelineStepInput {
			continue
		}
		upstreamIDs := incoming[node.ID]
		primaryColumns := dataPipelineFirstAvailableColumns(upstreamIDs, columnsByNode)
		primaryRefs := dataPipelineConfiguredPrimaryColumnRefs(node.Data.StepType, node.Data.Config)
		if missing := dataPipelineMissingColumns(primaryRefs.Columns, primaryColumns); len(missing) > 0 {
			warnings = append(warnings, DataPipelineNodeWarning{
				NodeID:     node.ID,
				StepType:   node.Data.StepType,
				Code:       dataPipelineWarningMissingUpstreamColumns,
				Severity:   "warning",
				Message:    fmt.Sprintf("Configured columns are not available from the upstream step: %s", strings.Join(missing, ", ")),
				Columns:    missing,
				ConfigKeys: primaryRefs.ConfigKeys,
			})
		}
		if node.Data.StepType == DataPipelineStepJoin {
			rightColumns := columnsByNode[nthString(upstreamIDs, 1)]
			rightRefs := dataPipelineColumnRefs{Columns: dataPipelineStringList(node.Data.Config["rightKeys"]), ConfigKeys: []string{"rightKeys"}}
			if missing := dataPipelineMissingColumns(rightRefs.Columns, rightColumns); len(missing) > 0 {
				warnings = append(warnings, DataPipelineNodeWarning{
					NodeID:     node.ID,
					StepType:   node.Data.StepType,
					Code:       dataPipelineWarningMissingRightUpstreamColumns,
					Severity:   "warning",
					Message:    fmt.Sprintf("Configured right-side columns are not available from the upstream step: %s", strings.Join(missing, ", ")),
					Columns:    missing,
					ConfigKeys: rightRefs.ConfigKeys,
				})
			}
		}
	}
	return warnings
}

type dataPipelineColumnRefs struct {
	Columns    []string
	ConfigKeys []string
}

func dataPipelineConfiguredPrimaryColumnRefs(stepType string, config map[string]any) dataPipelineColumnRefs {
	switch stepType {
	case DataPipelineStepJSONExtract:
		return dataPipelineColumnRefs{Columns: []string{dataPipelineString(config, "sourceColumn")}, ConfigKeys: []string{"sourceColumn"}}
	case DataPipelineStepExcelExtract:
		return dataPipelineColumnRefs{Columns: []string{dataPipelineString(config, "sourceFileColumn")}, ConfigKeys: []string{"sourceFileColumn"}}
	case DataPipelineStepClean:
		return dataPipelineCleanRuleColumnRefs(config)
	case DataPipelineStepNormalize, DataPipelineStepValidate:
		return dataPipelineColumnRefs{Columns: dataPipelineRuleStringFieldRefs(config, "rules", "column"), ConfigKeys: []string{"rules[].column"}}
	case DataPipelineStepSchemaMapping:
		return dataPipelineColumnRefs{Columns: dataPipelineRuleStringFieldRefs(config, "mappings", "sourceColumn"), ConfigKeys: []string{"mappings[].sourceColumn"}}
	case DataPipelineStepSchemaCompletion:
		return dataPipelineSchemaCompletionColumnRefs(config)
	case DataPipelineStepJoin:
		return dataPipelineColumnRefs{Columns: dataPipelineStringList(config["leftKeys"]), ConfigKeys: []string{"leftKeys"}}
	case DataPipelineStepTransform:
		return dataPipelineTransformColumnRefs(config)
	case DataPipelineStepSchemaInference, DataPipelineStepQualityReport, DataPipelineStepDeduplicate, DataPipelineStepRedactPII:
		return dataPipelineColumnRefs{Columns: dataPipelineStringList(config["columns"]), ConfigKeys: []string{"columns"}}
	case DataPipelineStepQuarantine:
		return dataPipelineColumnRefs{Columns: []string{dataPipelineString(config, "statusColumn")}, ConfigKeys: []string{"statusColumn"}}
	case DataPipelineStepCanonicalize:
		return dataPipelineColumnRefs{Columns: dataPipelineRuleStringFieldRefs(config, "rules", "column"), ConfigKeys: []string{"rules[].column"}}
	case DataPipelineStepClassifyDocument, DataPipelineStepRelationship, DataPipelineStepDetectLanguage:
		return dataPipelineColumnRefs{Columns: []string{dataPipelineString(config, "textColumn")}, ConfigKeys: []string{"textColumn"}}
	case DataPipelineStepUnitConversion:
		refs := make([]string, 0)
		for _, rule := range dataPipelineConfigArray(config, "rules") {
			refs = append(refs, dataPipelineString(rule, "valueColumn"), dataPipelineString(rule, "unitColumn"))
		}
		return dataPipelineColumnRefs{Columns: refs, ConfigKeys: []string{"rules[].valueColumn", "rules[].unitColumn"}}
	case DataPipelineStepSampleCompare:
		refs := make([]string, 0)
		for _, pair := range dataPipelineConfigArray(config, "pairs") {
			refs = append(refs, dataPipelineString(pair, "beforeColumn"), dataPipelineString(pair, "afterColumn"))
		}
		return dataPipelineColumnRefs{Columns: refs, ConfigKeys: []string{"pairs[].beforeColumn", "pairs[].afterColumn"}}
	case DataPipelineStepOutput:
		refs := make([]string, 0)
		for _, column := range dataPipelineConfigArray(config, "columns") {
			refs = append(refs, firstNonEmpty(dataPipelineString(column, "sourceColumn"), dataPipelineString(column, "column")))
		}
		return dataPipelineColumnRefs{Columns: refs, ConfigKeys: []string{"columns[].sourceColumn", "columns[].column"}}
	default:
		return dataPipelineColumnRefs{}
	}
}

func dataPipelineCleanRuleColumnRefs(config map[string]any) dataPipelineColumnRefs {
	refs := make([]string, 0)
	for _, rule := range dataPipelineConfigArray(config, "rules") {
		switch dataPipelineString(rule, "operation") {
		case "dedupe":
			refs = append(refs, dataPipelineStringList(rule["keys"])...)
			refs = append(refs, dataPipelineString(rule, "orderBy"))
		case "drop_null_rows":
			refs = append(refs, dataPipelineStringList(firstNonNil(rule["columns"], rule["column"]))...)
		default:
			refs = append(refs, dataPipelineString(rule, "column"))
		}
	}
	return dataPipelineColumnRefs{Columns: refs, ConfigKeys: []string{"rules[].column", "rules[].columns", "rules[].keys", "rules[].orderBy"}}
}

func dataPipelineSchemaCompletionColumnRefs(config map[string]any) dataPipelineColumnRefs {
	refs := make([]string, 0)
	for _, rule := range dataPipelineConfigArray(config, "rules") {
		refs = append(refs, dataPipelineString(rule, "sourceColumn"))
		refs = append(refs, dataPipelineStringList(rule["sourceColumns"])...)
	}
	return dataPipelineColumnRefs{Columns: refs, ConfigKeys: []string{"rules[].sourceColumn", "rules[].sourceColumns"}}
}

func dataPipelineTransformColumnRefs(config map[string]any) dataPipelineColumnRefs {
	operation := firstNonEmpty(dataPipelineString(config, "operation"), dataPipelineString(config, "type"), "select_columns")
	switch operation {
	case "select_columns", "drop_columns":
		return dataPipelineColumnRefs{Columns: dataPipelineStringList(config["columns"]), ConfigKeys: []string{"columns"}}
	case "rename_columns":
		renames := dataPipelineRecord(config["renames"])
		refs := make([]string, 0, len(renames))
		for column := range renames {
			refs = append(refs, column)
		}
		return dataPipelineColumnRefs{Columns: refs, ConfigKeys: []string{"renames"}}
	case "filter":
		return dataPipelineColumnRefs{Columns: dataPipelineRuleStringFieldRefs(config, "conditions", "column"), ConfigKeys: []string{"conditions[].column"}}
	case "sort":
		return dataPipelineColumnRefs{Columns: dataPipelineRuleStringFieldRefs(config, "sorts", "column"), ConfigKeys: []string{"sorts[].column"}}
	case "aggregate":
		refs := append([]string{}, dataPipelineStringList(config["groupBy"])...)
		refs = append(refs, dataPipelineRuleStringFieldRefs(config, "aggregations", "column")...)
		return dataPipelineColumnRefs{Columns: refs, ConfigKeys: []string{"groupBy", "aggregations[].column"}}
	default:
		return dataPipelineColumnRefs{}
	}
}

func dataPipelineRuleStringFieldRefs(config map[string]any, arrayKey, fieldKey string) []string {
	refs := make([]string, 0)
	for _, item := range dataPipelineConfigArray(config, arrayKey) {
		refs = append(refs, dataPipelineString(item, fieldKey))
	}
	return refs
}

func dataPipelineFirstAvailableColumns(upstreamIDs []string, columnsByNode map[string][]string) []string {
	for _, upstreamID := range upstreamIDs {
		if columns := columnsByNode[upstreamID]; len(columns) > 0 {
			return columns
		}
	}
	return nil
}

func dataPipelineMissingColumns(columns []string, availableColumns []string) []string {
	available := make(map[string]struct{}, len(availableColumns))
	for _, column := range availableColumns {
		available[column] = struct{}{}
	}
	missing := make([]string, 0)
	for _, column := range dataPipelineUniqueStrings(columns) {
		column = strings.TrimSpace(column)
		if column == "" {
			continue
		}
		if _, ok := available[column]; !ok {
			missing = append(missing, column)
		}
	}
	return missing
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
