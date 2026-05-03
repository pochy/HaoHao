package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"example.com/haohao/backend/internal/db"

	"github.com/ClickHouse/clickhouse-go/v2"
)

var dataPipelineIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

type dataPipelineCompiledSelect struct {
	SQL      string
	Columns  []string
	NodeID   string
	StepType string
	Source   *dataPipelineSource
}

type dataPipelineSource struct {
	Kind     string
	ID       int64
	PublicID string
	Database string
	Table    string
	Columns  []string
}

type dataPipelineRelation struct {
	CTE     string
	SQL     string
	Columns []string
	Node    DataPipelineNode
	Source  *dataPipelineSource
}

type dataPipelineCompiler struct {
	service  *DataPipelineService
	tenantID int64
	graph    DataPipelineGraph
	nodes    map[string]DataPipelineNode
	incoming map[string][]string
}

func (s *DataPipelineService) compilePreviewSelect(ctx context.Context, tenantID int64, graph DataPipelineGraph, selectedNodeID string, limit int32) (dataPipelineCompiledSelect, error) {
	if limit <= 0 || limit > datasetPreviewRowLimit {
		limit = 100
	}
	compiled, err := s.compileSelect(ctx, tenantID, graph, selectedNodeID)
	if err != nil {
		return dataPipelineCompiledSelect{}, err
	}
	compiled.SQL = fmt.Sprintf("%s\nLIMIT %d", compiled.SQL, limit)
	return compiled, nil
}

func (s *DataPipelineService) compileRunSelect(ctx context.Context, tenantID int64, graph DataPipelineGraph) (dataPipelineCompiledSelect, DataPipelineNode, error) {
	outputs := make([]DataPipelineNode, 0, 1)
	for _, node := range graph.Nodes {
		if node.Data.StepType == DataPipelineStepOutput {
			outputs = append(outputs, node)
		}
	}
	if len(outputs) != 1 {
		return dataPipelineCompiledSelect{}, DataPipelineNode{}, fmt.Errorf("%w: run requires exactly one output node in v1", ErrInvalidDataPipelineGraph)
	}
	compiled, err := s.compileSelect(ctx, tenantID, graph, outputs[0].ID)
	if err != nil {
		return dataPipelineCompiledSelect{}, DataPipelineNode{}, err
	}
	return compiled, outputs[0], nil
}

func (s *DataPipelineService) compileSelect(ctx context.Context, tenantID int64, graph DataPipelineGraph, selectedNodeID string) (dataPipelineCompiledSelect, error) {
	selectedNodeID = strings.TrimSpace(selectedNodeID)
	if selectedNodeID == "" {
		for _, node := range graph.Nodes {
			if node.Data.StepType == DataPipelineStepOutput {
				selectedNodeID = node.ID
				break
			}
		}
	}
	compiler := newDataPipelineCompiler(s, tenantID, graph)
	relation, ctes, err := compiler.compileToNode(ctx, selectedNodeID)
	if err != nil {
		return dataPipelineCompiledSelect{}, err
	}
	sql := "WITH\n" + strings.Join(ctes, ",\n") + "\nSELECT *\nFROM " + quoteCHIdent(relation.CTE)
	return dataPipelineCompiledSelect{
		SQL:      sql,
		Columns:  relation.Columns,
		NodeID:   relation.Node.ID,
		StepType: relation.Node.Data.StepType,
		Source:   relation.Source,
	}, nil
}

func newDataPipelineCompiler(service *DataPipelineService, tenantID int64, graph DataPipelineGraph) *dataPipelineCompiler {
	nodes := make(map[string]DataPipelineNode, len(graph.Nodes))
	incoming := make(map[string][]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	for _, edge := range graph.Edges {
		incoming[edge.Target] = append(incoming[edge.Target], edge.Source)
	}
	return &dataPipelineCompiler{
		service:  service,
		tenantID: tenantID,
		graph:    graph,
		nodes:    nodes,
		incoming: incoming,
	}
}

func (c *dataPipelineCompiler) compileToNode(ctx context.Context, selectedNodeID string) (dataPipelineRelation, []string, error) {
	order, err := dataPipelineTopologicalOrder(c.graph)
	if err != nil {
		return dataPipelineRelation{}, nil, err
	}
	relations := make(map[string]dataPipelineRelation, len(order))
	ctes := make([]string, 0, len(order))
	for _, node := range order {
		relation, err := c.compileNode(ctx, node, relations)
		if err != nil {
			return dataPipelineRelation{}, nil, err
		}
		relations[node.ID] = relation
		ctes = append(ctes, fmt.Sprintf("%s AS (\n%s\n)", quoteCHIdent(relation.CTE), relation.SQL))
		if node.ID == selectedNodeID {
			return relation, ctes, nil
		}
	}
	return dataPipelineRelation{}, nil, fmt.Errorf("%w: selected node not found", ErrInvalidDataPipelineGraph)
}

func (c *dataPipelineCompiler) compileNode(ctx context.Context, node DataPipelineNode, relations map[string]dataPipelineRelation) (dataPipelineRelation, error) {
	cte := dataPipelineCTEName(node.ID)
	if node.Data.StepType == DataPipelineStepInput {
		source, err := c.resolveInputSource(ctx, node.Data.Config)
		if err != nil {
			return dataPipelineRelation{}, err
		}
		return dataPipelineRelation{
			CTE:     cte,
			SQL:     fmt.Sprintf("SELECT *\nFROM %s.%s", quoteCHIdent(source.Database), quoteCHIdent(source.Table)),
			Columns: source.Columns,
			Node:    node,
			Source:  source,
		}, nil
	}

	upstream, err := c.singleUpstream(node, relations)
	if err != nil && node.Data.StepType != DataPipelineStepJoin {
		return dataPipelineRelation{}, err
	}
	switch node.Data.StepType {
	case DataPipelineStepProfile, DataPipelineStepValidate, DataPipelineStepOutput:
		return c.passThrough(node, upstream), nil
	case DataPipelineStepClean:
		return c.compileClean(node, upstream)
	case DataPipelineStepNormalize:
		return c.compileNormalize(node, upstream)
	case DataPipelineStepSchemaMapping:
		return c.compileSchemaMapping(node, upstream)
	case DataPipelineStepSchemaCompletion:
		return c.compileSchemaCompletion(node, upstream)
	case DataPipelineStepJoin:
		return c.compileJoin(node, relations)
	case DataPipelineStepEnrichJoin:
		return c.compileEnrichJoin(ctx, node, upstream)
	case DataPipelineStepTransform:
		return c.compileTransform(node, upstream)
	default:
		return dataPipelineRelation{}, fmt.Errorf("%w: unsupported step type %q", ErrInvalidDataPipelineGraph, node.Data.StepType)
	}
}

func (c *dataPipelineCompiler) passThrough(node DataPipelineNode, upstream dataPipelineRelation) dataPipelineRelation {
	return dataPipelineRelation{
		CTE:     dataPipelineCTEName(node.ID),
		SQL:     "SELECT *\nFROM " + quoteCHIdent(upstream.CTE),
		Columns: append([]string(nil), upstream.Columns...),
		Node:    node,
		Source:  upstream.Source,
	}
}

func (c *dataPipelineCompiler) singleUpstream(node DataPipelineNode, relations map[string]dataPipelineRelation) (dataPipelineRelation, error) {
	sources := c.incoming[node.ID]
	if len(sources) != 1 {
		return dataPipelineRelation{}, fmt.Errorf("%w: node %s must have exactly one upstream edge", ErrInvalidDataPipelineGraph, node.ID)
	}
	upstream, ok := relations[sources[0]]
	if !ok {
		return dataPipelineRelation{}, fmt.Errorf("%w: upstream node is not compiled: %s", ErrInvalidDataPipelineGraph, sources[0])
	}
	return upstream, nil
}

func (c *dataPipelineCompiler) joinUpstreams(node DataPipelineNode, relations map[string]dataPipelineRelation) ([]dataPipelineRelation, error) {
	sources := c.incoming[node.ID]
	if len(sources) != 2 {
		return nil, fmt.Errorf("%w: node %s must have exactly two upstream edges", ErrInvalidDataPipelineGraph, node.ID)
	}
	upstreams := make([]dataPipelineRelation, 0, len(sources))
	for _, source := range sources {
		upstream, ok := relations[source]
		if !ok {
			return nil, fmt.Errorf("%w: upstream node is not compiled: %s", ErrInvalidDataPipelineGraph, source)
		}
		upstreams = append(upstreams, upstream)
	}
	return upstreams, nil
}

func (c *dataPipelineCompiler) compileClean(node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	expressions := dataPipelineColumnExpressions(upstream.Columns)
	filters := make([]string, 0)
	var dedupeKeys []string
	dedupeOrder := ""
	for _, rule := range dataPipelineConfigObjects(node.Data.Config, "rules") {
		operation := dataPipelineString(rule, "operation")
		column := dataPipelineString(rule, "column")
		switch operation {
		case "drop_null_rows":
			if column == "" {
				for _, value := range dataPipelineStringSlice(rule, "columns") {
					if err := dataPipelineRequireColumn(upstream.Columns, value); err != nil {
						return dataPipelineRelation{}, err
					}
					filters = append(filters, "isNotNull("+quoteCHIdent(value)+")")
				}
				continue
			}
			if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
				return dataPipelineRelation{}, err
			}
			filters = append(filters, "isNotNull("+quoteCHIdent(column)+")")
		case "fill_null":
			if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
				return dataPipelineRelation{}, err
			}
			expressions[column] = fmt.Sprintf("ifNull(%s, %s)", quoteCHIdent(column), dataPipelineLiteral(rule["value"]))
		case "null_if":
			if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
				return dataPipelineRelation{}, err
			}
			condition, err := dataPipelineConditionExpr(upstream.Columns, column, rule["condition"])
			if err != nil {
				return dataPipelineRelation{}, err
			}
			expressions[column] = fmt.Sprintf("if(%s, NULL, %s)", condition, quoteCHIdent(column))
		case "clamp":
			if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
				return dataPipelineRelation{}, err
			}
			expr := quoteCHIdent(column)
			if _, ok := rule["min"]; ok {
				expr = fmt.Sprintf("greatest(%s, %s)", expr, dataPipelineLiteral(rule["min"]))
			}
			if _, ok := rule["max"]; ok {
				expr = fmt.Sprintf("least(%s, %s)", expr, dataPipelineLiteral(rule["max"]))
			}
			expressions[column] = expr
		case "trim_control_chars":
			if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
				return dataPipelineRelation{}, err
			}
			expressions[column] = fmt.Sprintf("replaceRegexpAll(toString(%s), '[[:cntrl:]]+', '')", quoteCHIdent(column))
		case "dedupe":
			dedupeKeys = dataPipelineStringSlice(rule, "keys")
			for _, key := range dedupeKeys {
				if err := dataPipelineRequireColumn(upstream.Columns, key); err != nil {
					return dataPipelineRelation{}, err
				}
			}
			dedupeOrder = dataPipelineString(rule, "orderBy")
			if dedupeOrder != "" {
				if err := dataPipelineRequireColumn(upstream.Columns, dedupeOrder); err != nil {
					return dataPipelineRelation{}, err
				}
			}
		case "":
			continue
		default:
			return dataPipelineRelation{}, fmt.Errorf("%w: unsupported clean operation %q", ErrInvalidDataPipelineGraph, operation)
		}
	}

	sql := fmt.Sprintf("SELECT\n%s\nFROM %s", dataPipelineSelectList(upstream.Columns, expressions), quoteCHIdent(upstream.CTE))
	if len(filters) > 0 {
		sql += "\nWHERE " + strings.Join(filters, " AND ")
	}
	if len(dedupeKeys) > 0 {
		partitions := make([]string, 0, len(dedupeKeys))
		for _, key := range dedupeKeys {
			partitions = append(partitions, quoteCHIdent(key))
		}
		orderBy := "tuple()"
		if dedupeOrder != "" {
			orderBy = quoteCHIdent(dedupeOrder) + " DESC"
		}
		sql = fmt.Sprintf("SELECT\n%s\nFROM (\nSELECT *, row_number() OVER (PARTITION BY %s ORDER BY %s) AS %s\nFROM (\n%s\n)\n)\nWHERE %s = 1",
			dataPipelineSelectList(upstream.Columns, dataPipelineColumnExpressions(upstream.Columns)),
			strings.Join(partitions, ", "),
			orderBy,
			quoteCHIdent("__dp_row_number"),
			sql,
			quoteCHIdent("__dp_row_number"),
		)
	}
	return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: upstream.Columns, Node: node, Source: upstream.Source}, nil
}

func (c *dataPipelineCompiler) compileNormalize(node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	expressions := dataPipelineColumnExpressions(upstream.Columns)
	for _, rule := range dataPipelineConfigObjects(node.Data.Config, "rules") {
		column := dataPipelineString(rule, "column")
		if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
			return dataPipelineRelation{}, err
		}
		operation := dataPipelineString(rule, "operation")
		col := quoteCHIdent(column)
		switch operation {
		case "trim":
			expressions[column] = "trimBoth(toString(" + col + "))"
		case "lowercase":
			expressions[column] = "lowerUTF8(toString(" + col + "))"
		case "uppercase":
			expressions[column] = "upperUTF8(toString(" + col + "))"
		case "normalize_spaces":
			expressions[column] = "replaceRegexpAll(trimBoth(toString(" + col + ")), '\\\\s+', ' ')"
		case "remove_symbols":
			expressions[column] = "replaceRegexpAll(toString(" + col + "), '[[:punct:]]+', '')"
		case "cast_decimal":
			scale := int(dataPipelineFloat(rule, "scale", 2))
			expressions[column] = fmt.Sprintf("toDecimal64OrNull(%s, %d)", col, scale)
		case "round":
			precision := int(dataPipelineFloat(rule, "precision", 0))
			expressions[column] = fmt.Sprintf("round(toFloat64OrNull(%s), %d)", col, precision)
		case "scale":
			expressions[column] = fmt.Sprintf("toFloat64OrNull(%s) * %s", col, dataPipelineLiteral(rule["factor"]))
		case "parse_date":
			expressions[column] = "parseDateTimeBestEffortOrNull(toString(" + col + "))"
		case "to_date":
			expressions[column] = "toDateOrNull(toString(" + col + "))"
		case "map_values":
			values, _ := rule["values"].(map[string]any)
			parts := make([]string, 0, len(values)*2+1)
			keys := make([]string, 0, len(values))
			for key := range values {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				parts = append(parts, fmt.Sprintf("%s = %s", col, dataPipelineLiteral(key)), dataPipelineLiteral(values[key]))
			}
			parts = append(parts, col)
			expressions[column] = "multiIf(" + strings.Join(parts, ", ") + ")"
		case "", "timezone":
			continue
		default:
			return dataPipelineRelation{}, fmt.Errorf("%w: unsupported normalize operation %q", ErrInvalidDataPipelineGraph, operation)
		}
	}
	sql := fmt.Sprintf("SELECT\n%s\nFROM %s", dataPipelineSelectList(upstream.Columns, expressions), quoteCHIdent(upstream.CTE))
	return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: upstream.Columns, Node: node, Source: upstream.Source}, nil
}

func (c *dataPipelineCompiler) compileSchemaMapping(node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	mappings := dataPipelineConfigObjects(node.Data.Config, "mappings")
	if len(mappings) == 0 {
		return c.passThrough(node, upstream), nil
	}
	selects := make([]string, 0, len(mappings))
	columns := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		target := dataPipelineString(mapping, "targetColumn")
		if err := dataPipelineValidateIdentifier(target); err != nil {
			return dataPipelineRelation{}, err
		}
		source := dataPipelineString(mapping, "sourceColumn")
		expr := ""
		if source != "" {
			if err := dataPipelineRequireColumn(upstream.Columns, source); err != nil {
				return dataPipelineRelation{}, err
			}
			expr = quoteCHIdent(source)
		} else if _, ok := mapping["defaultValue"]; ok {
			expr = dataPipelineLiteral(mapping["defaultValue"])
		} else if dataPipelineBool(mapping, "required", false) {
			return dataPipelineRelation{}, fmt.Errorf("%w: required mapping %s has no source or default", ErrInvalidDataPipelineGraph, target)
		} else {
			expr = "NULL"
		}
		if cast := dataPipelineString(mapping, "cast"); cast != "" {
			casted, err := dataPipelineCastExpr(expr, cast)
			if err != nil {
				return dataPipelineRelation{}, err
			}
			expr = casted
		}
		selects = append(selects, fmt.Sprintf("  %s AS %s", expr, quoteCHIdent(target)))
		columns = append(columns, target)
	}
	sql := fmt.Sprintf("SELECT\n%s\nFROM %s", strings.Join(selects, ",\n"), quoteCHIdent(upstream.CTE))
	return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: columns, Node: node, Source: upstream.Source}, nil
}

func (c *dataPipelineCompiler) compileSchemaCompletion(node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	columns := append([]string(nil), upstream.Columns...)
	expressions := dataPipelineColumnExpressions(upstream.Columns)
	for _, rule := range dataPipelineConfigObjects(node.Data.Config, "rules") {
		target := dataPipelineString(rule, "targetColumn")
		if err := dataPipelineValidateIdentifier(target); err != nil {
			return dataPipelineRelation{}, err
		}
		if dataPipelineHasColumn(columns, target) {
			return dataPipelineRelation{}, fmt.Errorf("%w: duplicate completion column %s", ErrInvalidDataPipelineGraph, target)
		}
		expr, err := dataPipelineCompletionExpr(upstream.Columns, rule)
		if err != nil {
			return dataPipelineRelation{}, err
		}
		columns = append(columns, target)
		expressions[target] = expr
	}
	sql := fmt.Sprintf("SELECT\n%s\nFROM %s", dataPipelineSelectList(columns, expressions), quoteCHIdent(upstream.CTE))
	return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: columns, Node: node, Source: upstream.Source}, nil
}

func (c *dataPipelineCompiler) compileJoin(node DataPipelineNode, relations map[string]dataPipelineRelation) (dataPipelineRelation, error) {
	upstreams, err := c.joinUpstreams(node, relations)
	if err != nil {
		return dataPipelineRelation{}, err
	}
	return c.compileJoinRelation(node, upstreams[0], upstreams[1].Columns, quoteCHIdent(upstreams[1].CTE))
}

func (c *dataPipelineCompiler) compileEnrichJoin(ctx context.Context, node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	config := node.Data.Config
	right, err := c.resolveRightSource(ctx, config)
	if err != nil {
		return dataPipelineRelation{}, err
	}
	return c.compileJoinRelation(node, upstream, right.Columns, quoteCHIdent(right.Database)+"."+quoteCHIdent(right.Table))
}

func (c *dataPipelineCompiler) compileJoinRelation(node DataPipelineNode, upstream dataPipelineRelation, rightColumns []string, rightTable string) (dataPipelineRelation, error) {
	config := node.Data.Config
	joinType, err := dataPipelineJoinType(config)
	if err != nil {
		return dataPipelineRelation{}, err
	}
	joinStrictness, err := dataPipelineJoinStrictness(config)
	if err != nil {
		return dataPipelineRelation{}, err
	}
	leftKeys := dataPipelineStringSlice(config, "leftKeys")
	rightKeys := dataPipelineStringSlice(config, "rightKeys")
	if joinType != "cross" {
		if len(leftKeys) == 0 || len(leftKeys) != len(rightKeys) || len(leftKeys) > 5 {
			return dataPipelineRelation{}, fmt.Errorf("%w: join keys must contain 1 to 5 matching columns", ErrInvalidDataPipelineGraph)
		}
		for _, key := range leftKeys {
			if err := dataPipelineRequireColumn(upstream.Columns, key); err != nil {
				return dataPipelineRelation{}, err
			}
		}
		for _, key := range rightKeys {
			if err := dataPipelineRequireColumn(rightColumns, key); err != nil {
				return dataPipelineRelation{}, err
			}
		}
	}
	selectColumns := dataPipelineStringSlice(config, "selectColumns")
	columns := append([]string(nil), upstream.Columns...)
	selects := []string{"  l.*"}
	for _, column := range selectColumns {
		if err := dataPipelineRequireColumn(rightColumns, column); err != nil {
			return dataPipelineRelation{}, err
		}
		alias := column
		if dataPipelineHasColumn(columns, alias) {
			alias = alias + "_right"
		}
		columns = append(columns, alias)
		selects = append(selects, fmt.Sprintf("  r.%s AS %s", quoteCHIdent(column), quoteCHIdent(alias)))
	}
	conditions := make([]string, 0, len(leftKeys))
	for i := range leftKeys {
		conditions = append(conditions, fmt.Sprintf("l.%s = r.%s", quoteCHIdent(leftKeys[i]), quoteCHIdent(rightKeys[i])))
	}
	sql := fmt.Sprintf("SELECT\n%s\nFROM %s AS l\n%s %s AS r",
		strings.Join(selects, ",\n"),
		quoteCHIdent(upstream.CTE),
		dataPipelineJoinSQL(joinType, joinStrictness),
		rightTable,
	)
	if joinType != "cross" {
		sql += " ON " + strings.Join(conditions, " AND ")
	}
	return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: columns, Node: node, Source: upstream.Source}, nil
}

func dataPipelineJoinType(config map[string]any) (string, error) {
	joinType := dataPipelineString(config, "joinType")
	if joinType == "" {
		joinType = "left"
	}
	switch joinType {
	case "inner", "left", "right", "full", "cross":
		return joinType, nil
	default:
		return "", fmt.Errorf("%w: unsupported join type %q", ErrInvalidDataPipelineGraph, joinType)
	}
}

func dataPipelineJoinStrictness(config map[string]any) (string, error) {
	joinStrictness := dataPipelineString(config, "joinStrictness")
	if joinStrictness == "" {
		joinStrictness = "all"
	}
	switch joinStrictness {
	case "all", "any":
		return joinStrictness, nil
	default:
		return "", fmt.Errorf("%w: unsupported join strictness %q", ErrInvalidDataPipelineGraph, joinStrictness)
	}
}

func dataPipelineJoinSQL(joinType, joinStrictness string) string {
	if joinType == "cross" {
		return "CROSS JOIN"
	}
	strictness := "ALL"
	if joinStrictness == "any" {
		strictness = "ANY"
	}
	switch joinType {
	case "inner":
		return "INNER " + strictness + " JOIN"
	case "right":
		return "RIGHT " + strictness + " JOIN"
	case "full":
		return "FULL " + strictness + " JOIN"
	default:
		return "LEFT " + strictness + " JOIN"
	}
}

func (c *dataPipelineCompiler) compileTransform(node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	operation := dataPipelineString(node.Data.Config, "operation")
	if operation == "" {
		operation = dataPipelineString(node.Data.Config, "type")
	}
	switch operation {
	case "", "select_columns":
		columns := dataPipelineStringSlice(node.Data.Config, "columns")
		if len(columns) == 0 {
			return c.passThrough(node, upstream), nil
		}
		for _, column := range columns {
			if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
				return dataPipelineRelation{}, err
			}
		}
		sql := fmt.Sprintf("SELECT\n%s\nFROM %s", dataPipelineSelectList(columns, dataPipelineColumnExpressions(columns)), quoteCHIdent(upstream.CTE))
		return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: columns, Node: node, Source: upstream.Source}, nil
	case "drop_columns":
		drops := dataPipelineStringSet(dataPipelineStringSlice(node.Data.Config, "columns"))
		columns := make([]string, 0, len(upstream.Columns))
		for _, column := range upstream.Columns {
			if _, ok := drops[column]; !ok {
				columns = append(columns, column)
			}
		}
		sql := fmt.Sprintf("SELECT\n%s\nFROM %s", dataPipelineSelectList(columns, dataPipelineColumnExpressions(columns)), quoteCHIdent(upstream.CTE))
		return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: columns, Node: node, Source: upstream.Source}, nil
	case "rename_columns":
		renames, _ := node.Data.Config["renames"].(map[string]any)
		columns := make([]string, 0, len(upstream.Columns))
		selects := make([]string, 0, len(upstream.Columns))
		for _, column := range upstream.Columns {
			alias := column
			if value, ok := renames[column]; ok {
				alias = strings.TrimSpace(fmt.Sprint(value))
				if err := dataPipelineValidateIdentifier(alias); err != nil {
					return dataPipelineRelation{}, err
				}
			}
			columns = append(columns, alias)
			selects = append(selects, fmt.Sprintf("  %s AS %s", quoteCHIdent(column), quoteCHIdent(alias)))
		}
		sql := fmt.Sprintf("SELECT\n%s\nFROM %s", strings.Join(selects, ",\n"), quoteCHIdent(upstream.CTE))
		return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: columns, Node: node, Source: upstream.Source}, nil
	case "filter":
		filters := make([]string, 0)
		for _, condition := range dataPipelineConfigObjects(node.Data.Config, "conditions") {
			column := dataPipelineString(condition, "column")
			expr, err := dataPipelineConditionExpr(upstream.Columns, column, condition)
			if err != nil {
				return dataPipelineRelation{}, err
			}
			filters = append(filters, expr)
		}
		if len(filters) == 0 {
			return c.passThrough(node, upstream), nil
		}
		sql := fmt.Sprintf("SELECT *\nFROM %s\nWHERE %s", quoteCHIdent(upstream.CTE), strings.Join(filters, " AND "))
		return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: upstream.Columns, Node: node, Source: upstream.Source}, nil
	case "sort":
		sorts := dataPipelineConfigObjects(node.Data.Config, "sorts")
		orderParts := make([]string, 0, len(sorts))
		for _, item := range sorts {
			column := dataPipelineString(item, "column")
			if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
				return dataPipelineRelation{}, err
			}
			direction := strings.ToUpper(dataPipelineString(item, "direction"))
			if direction != "DESC" {
				direction = "ASC"
			}
			orderParts = append(orderParts, quoteCHIdent(column)+" "+direction)
		}
		if len(orderParts) == 0 {
			return c.passThrough(node, upstream), nil
		}
		sql := fmt.Sprintf("SELECT *\nFROM %s\nORDER BY %s", quoteCHIdent(upstream.CTE), strings.Join(orderParts, ", "))
		return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: upstream.Columns, Node: node, Source: upstream.Source}, nil
	case "aggregate":
		return c.compileAggregate(node, upstream)
	default:
		return dataPipelineRelation{}, fmt.Errorf("%w: unsupported transform operation %q", ErrInvalidDataPipelineGraph, operation)
	}
}

func (c *dataPipelineCompiler) compileAggregate(node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	groupBy := dataPipelineStringSlice(node.Data.Config, "groupBy")
	for _, column := range groupBy {
		if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
			return dataPipelineRelation{}, err
		}
	}
	selects := make([]string, 0, len(groupBy)+4)
	columns := make([]string, 0, len(groupBy)+4)
	for _, column := range groupBy {
		selects = append(selects, fmt.Sprintf("  %s", quoteCHIdent(column)))
		columns = append(columns, column)
	}
	for _, agg := range dataPipelineConfigObjects(node.Data.Config, "aggregations") {
		function := strings.ToLower(dataPipelineString(agg, "function"))
		if function == "" {
			function = "count"
		}
		switch function {
		case "count", "sum", "avg", "min", "max":
		default:
			return dataPipelineRelation{}, fmt.Errorf("%w: unsupported aggregate function %q", ErrInvalidDataPipelineGraph, function)
		}
		column := dataPipelineString(agg, "column")
		expr := "*"
		if function != "count" || column != "" {
			if err := dataPipelineRequireColumn(upstream.Columns, column); err != nil {
				return dataPipelineRelation{}, err
			}
			expr = quoteCHIdent(column)
		}
		alias := dataPipelineString(agg, "alias")
		if alias == "" {
			alias = function
			if column != "" {
				alias += "_" + column
			}
		}
		if err := dataPipelineValidateIdentifier(alias); err != nil {
			return dataPipelineRelation{}, err
		}
		selects = append(selects, fmt.Sprintf("  %s(%s) AS %s", function, expr, quoteCHIdent(alias)))
		columns = append(columns, alias)
	}
	if len(selects) == 0 {
		selects = append(selects, "  count() AS "+quoteCHIdent("count"))
		columns = append(columns, "count")
	}
	sql := fmt.Sprintf("SELECT\n%s\nFROM %s", strings.Join(selects, ",\n"), quoteCHIdent(upstream.CTE))
	if len(groupBy) > 0 {
		parts := make([]string, 0, len(groupBy))
		for _, column := range groupBy {
			parts = append(parts, quoteCHIdent(column))
		}
		sql += "\nGROUP BY " + strings.Join(parts, ", ")
	}
	return dataPipelineRelation{CTE: dataPipelineCTEName(node.ID), SQL: sql, Columns: columns, Node: node, Source: upstream.Source}, nil
}

func (c *dataPipelineCompiler) resolveInputSource(ctx context.Context, config map[string]any) (*dataPipelineSource, error) {
	return c.resolveSource(ctx, dataPipelineString(config, "sourceKind"), dataPipelineString(config, "datasetPublicId"), dataPipelineString(config, "workTablePublicId"))
}

func (c *dataPipelineCompiler) resolveRightSource(ctx context.Context, config map[string]any) (*dataPipelineSource, error) {
	return c.resolveSource(ctx, dataPipelineString(config, "rightSourceKind"), dataPipelineString(config, "rightDatasetPublicId"), dataPipelineString(config, "rightWorkTablePublicId"))
}

func (c *dataPipelineCompiler) resolveSource(ctx context.Context, kind, datasetPublicID, workTablePublicID string) (*dataPipelineSource, error) {
	if c.service == nil || c.service.datasets == nil {
		return nil, fmt.Errorf("dataset service is not configured")
	}
	switch strings.TrimSpace(kind) {
	case "dataset":
		dataset, err := c.service.datasets.Get(ctx, c.tenantID, datasetPublicID)
		if err != nil {
			return nil, err
		}
		columns := make([]string, 0, len(dataset.Columns))
		for _, column := range dataset.Columns {
			columns = append(columns, column.ColumnName)
		}
		return &dataPipelineSource{
			Kind:     "dataset",
			ID:       dataset.ID,
			PublicID: dataset.PublicID,
			Database: dataset.RawDatabase,
			Table:    dataset.RawTable,
			Columns:  columns,
		}, nil
	case "work_table":
		workTable, err := c.service.datasets.GetManagedWorkTable(ctx, c.tenantID, workTablePublicID)
		if err != nil {
			return nil, err
		}
		columns := make([]string, 0, len(workTable.Columns))
		for _, column := range workTable.Columns {
			columns = append(columns, column.ColumnName)
		}
		return &dataPipelineSource{
			Kind:     "work_table",
			ID:       workTable.ID,
			PublicID: workTable.PublicID,
			Database: workTable.Database,
			Table:    workTable.Table,
			Columns:  columns,
		}, nil
	default:
		return nil, fmt.Errorf("%w: sourceKind must be dataset or work_table", ErrInvalidDataPipelineGraph)
	}
}

func (s *DataPipelineService) executeRun(ctx context.Context, tenantID int64, run db.DataPipelineRun, version db.DataPipelineVersion) (DatasetWorkTable, dataPipelineCompiledSelect, error) {
	if s == nil || s.datasets == nil {
		return DatasetWorkTable{}, dataPipelineCompiledSelect{}, fmt.Errorf("data pipeline service is not configured")
	}
	graph, err := decodeDataPipelineGraph(version.Graph)
	if err != nil {
		return DatasetWorkTable{}, dataPipelineCompiledSelect{}, err
	}
	if dataPipelineGraphNeedsHybrid(graph) {
		return s.executeHybridRun(ctx, tenantID, run, graph)
	}
	compiled, outputNode, err := s.compileRunSelect(ctx, tenantID, graph)
	if err != nil {
		return DatasetWorkTable{}, dataPipelineCompiledSelect{}, err
	}
	if err := s.datasets.ensureTenantSandbox(ctx, tenantID); err != nil {
		return DatasetWorkTable{}, dataPipelineCompiledSelect{}, err
	}
	conn, err := s.datasets.openTenantConn(ctx, tenantID)
	if err != nil {
		return DatasetWorkTable{}, dataPipelineCompiledSelect{}, err
	}
	defer conn.Close()

	targetDatabase := datasetWorkDatabaseName(tenantID)
	targetTable := dataPipelineOutputTableName(outputNode, run)
	stageTable := "__dp_stage_" + strings.ReplaceAll(run.PublicID.String(), "-", "")
	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.exportQuerySettings()))
	_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(stageTable)))
	createSQL := fmt.Sprintf(
		"CREATE TABLE %s.%s ENGINE = MergeTree ORDER BY tuple() AS\n%s",
		quoteCHIdent(targetDatabase),
		quoteCHIdent(stageTable),
		compiled.SQL,
	)
	if err := conn.Exec(queryCtx, createSQL); err != nil {
		return DatasetWorkTable{}, compiled, fmt.Errorf("create data pipeline stage table: %w", err)
	}
	_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(targetTable)))
	if err := conn.Exec(queryCtx, fmt.Sprintf("RENAME TABLE %s.%s TO %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(stageTable), quoteCHIdent(targetDatabase), quoteCHIdent(targetTable))); err != nil {
		_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(stageTable)))
		return DatasetWorkTable{}, compiled, fmt.Errorf("promote data pipeline stage table: %w", err)
	}

	displayName := dataPipelineString(outputNode.Data.Config, "displayName")
	if displayName == "" {
		displayName = targetTable
	}
	var userID int64
	if run.RequestedByUserID.Valid {
		userID = run.RequestedByUserID.Int64
	}
	workTable, err := s.datasets.registerDatasetWorkTableForRef(ctx, tenantID, userID, nil, nil, targetDatabase, targetTable, displayName)
	if err != nil {
		return DatasetWorkTable{}, compiled, err
	}
	return workTable, compiled, nil
}

func dataPipelineOutputTableName(node DataPipelineNode, run db.DataPipelineRun) string {
	tableName := strings.TrimSpace(dataPipelineString(node.Data.Config, "tableName"))
	if tableName != "" && dataPipelineIdentifierPattern.MatchString(tableName) {
		return tableName
	}
	raw := strings.ReplaceAll(run.PublicID.String(), "-", "")
	if len(raw) > 20 {
		raw = raw[:20]
	}
	return "dp_" + raw
}

func dataPipelineCTEName(nodeID string) string {
	var b strings.Builder
	b.WriteString("step_")
	for _, r := range strings.ToLower(strings.TrimSpace(nodeID)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	if b.Len() == len("step_") {
		b.WriteString("node")
	}
	return b.String()
}

func dataPipelineColumnExpressions(columns []string) map[string]string {
	expressions := make(map[string]string, len(columns))
	for _, column := range columns {
		expressions[column] = quoteCHIdent(column)
	}
	return expressions
}

func dataPipelineSelectList(columns []string, expressions map[string]string) string {
	if len(columns) == 0 {
		return "  *"
	}
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		expr := expressions[column]
		if expr == "" {
			expr = quoteCHIdent(column)
		}
		parts = append(parts, fmt.Sprintf("  %s AS %s", expr, quoteCHIdent(column)))
	}
	return strings.Join(parts, ",\n")
}

func dataPipelineRequireColumn(columns []string, column string) error {
	column = strings.TrimSpace(column)
	if column == "" {
		return fmt.Errorf("%w: column is required", ErrInvalidDataPipelineGraph)
	}
	if !dataPipelineHasColumn(columns, column) {
		return fmt.Errorf("%w: unknown column %s", ErrInvalidDataPipelineGraph, column)
	}
	return nil
}

func dataPipelineHasColumn(columns []string, column string) bool {
	for _, item := range columns {
		if item == column {
			return true
		}
	}
	return false
}

func dataPipelineValidateIdentifier(value string) error {
	value = strings.TrimSpace(value)
	if !dataPipelineIdentifierPattern.MatchString(value) {
		return fmt.Errorf("%w: unsafe identifier %q", ErrInvalidDataPipelineGraph, value)
	}
	return nil
}

func dataPipelineCastExpr(expr, cast string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(cast)) {
	case "string":
		return "toString(" + expr + ")", nil
	case "int", "int64":
		return "toInt64OrNull(" + expr + ")", nil
	case "float", "float64":
		return "toFloat64OrNull(" + expr + ")", nil
	case "decimal":
		return "toDecimal64OrNull(" + expr + ", 2)", nil
	case "date":
		return "toDateOrNull(toString(" + expr + "))", nil
	case "datetime":
		return "parseDateTimeBestEffortOrNull(toString(" + expr + "))", nil
	default:
		return "", fmt.Errorf("%w: unsupported cast %q", ErrInvalidDataPipelineGraph, cast)
	}
}

func dataPipelineCompletionExpr(columns []string, rule map[string]any) (string, error) {
	method := dataPipelineString(rule, "method")
	switch method {
	case "", "literal":
		return dataPipelineLiteral(rule["value"]), nil
	case "copy_column":
		column := dataPipelineString(rule, "sourceColumn")
		if err := dataPipelineRequireColumn(columns, column); err != nil {
			return "", err
		}
		return quoteCHIdent(column), nil
	case "coalesce":
		sourceColumns := dataPipelineStringSlice(rule, "sourceColumns")
		parts := make([]string, 0, len(sourceColumns)+1)
		for _, column := range sourceColumns {
			if err := dataPipelineRequireColumn(columns, column); err != nil {
				return "", err
			}
			parts = append(parts, quoteCHIdent(column))
		}
		if _, ok := rule["defaultValue"]; ok {
			parts = append(parts, dataPipelineLiteral(rule["defaultValue"]))
		}
		if len(parts) == 0 {
			return "NULL", nil
		}
		return "coalesce(" + strings.Join(parts, ", ") + ")", nil
	case "concat":
		sourceColumns := dataPipelineStringSlice(rule, "sourceColumns")
		parts := make([]string, 0, len(sourceColumns))
		for _, column := range sourceColumns {
			if err := dataPipelineRequireColumn(columns, column); err != nil {
				return "", err
			}
			parts = append(parts, "toString("+quoteCHIdent(column)+")")
		}
		return "concat(" + strings.Join(parts, ", ") + ")", nil
	case "case_when":
		return dataPipelineLiteral(rule["defaultValue"]), nil
	default:
		return "", fmt.Errorf("%w: unsupported completion method %q", ErrInvalidDataPipelineGraph, method)
	}
}

func dataPipelineConditionExpr(columns []string, defaultColumn string, raw any) (string, error) {
	condition, _ := raw.(map[string]any)
	if condition == nil {
		condition, _ = raw.(map[string]interface{})
	}
	if condition == nil {
		condition = map[string]any{}
	}
	column := dataPipelineString(condition, "column")
	if column == "" {
		column = strings.TrimSpace(defaultColumn)
	}
	if err := dataPipelineRequireColumn(columns, column); err != nil {
		return "", err
	}
	operator := strings.ToLower(dataPipelineString(condition, "operator"))
	if operator == "" {
		operator = strings.ToLower(dataPipelineString(condition, "op"))
	}
	value := condition["value"]
	col := quoteCHIdent(column)
	switch operator {
	case "required":
		return "isNotNull(" + col + ")", nil
	case "=", "!=", ">", ">=", "<", "<=":
		return fmt.Sprintf("%s %s %s", col, operator, dataPipelineLiteral(value)), nil
	case "in":
		values, ok := value.([]any)
		if !ok {
			return "", fmt.Errorf("%w: in operator requires values array", ErrInvalidDataPipelineGraph)
		}
		parts := make([]string, 0, len(values))
		for _, item := range values {
			parts = append(parts, dataPipelineLiteral(item))
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(parts, ", ")), nil
	case "regex":
		return fmt.Sprintf("match(toString(%s), %s)", col, dataPipelineLiteral(value)), nil
	case "":
		return "1 = 1", nil
	default:
		return "", fmt.Errorf("%w: unsupported operator %q", ErrInvalidDataPipelineGraph, operator)
	}
}

func dataPipelineLiteral(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case string:
		return quoteCHString(v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 64)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return quoteCHString(fmt.Sprint(v))
	}
}

func dataPipelineString(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	value, ok := config[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func dataPipelineBool(config map[string]any, key string, fallback bool) bool {
	if config == nil {
		return fallback
	}
	value, ok := config[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(v)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func dataPipelineFloat(config map[string]any, key string, fallback float64) float64 {
	if config == nil {
		return fallback
	}
	value, ok := config[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func dataPipelineConfigObjects(config map[string]any, key string) []map[string]any {
	if config == nil {
		return nil
	}
	raw, ok := config[key]
	if !ok || raw == nil {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if object, ok := item.(map[string]any); ok {
			out = append(out, object)
		}
	}
	return out
}

func dataPipelineStringSlice(config map[string]any, key string) []string {
	if config == nil {
		return nil
	}
	raw, ok := config[key]
	if !ok || raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case []string:
		return value
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []string{strings.TrimSpace(value)}
	default:
		return nil
	}
}

func dataPipelineStringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
