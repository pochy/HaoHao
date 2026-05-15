package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"example.com/haohao/backend/internal/db"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
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

type dataPipelineRunOutputResult struct {
	Node        DataPipelineNode
	WorkTable   DatasetWorkTable
	Compiled    dataPipelineCompiledSelect
	Err         error
	NodeResults map[string]dataPipelineRunNodeResult
}

type dataPipelineRunNodeResult struct {
	NodeID      string
	StepType    string
	RowCount    int64
	Metadata    map[string]any
	ReviewItems []dataPipelineReviewItemDraft
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
	omitSort bool
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

func (s *DataPipelineService) compileRunSelect(ctx context.Context, tenantID int64, graph DataPipelineGraph, outputNode DataPipelineNode) (dataPipelineCompiledSelect, error) {
	compiled, err := s.compileSelectWithOptions(ctx, tenantID, graph, outputNode.ID, true)
	if err != nil {
		return dataPipelineCompiledSelect{}, err
	}
	return compiled, nil
}

func (s *DataPipelineService) compileSelect(ctx context.Context, tenantID int64, graph DataPipelineGraph, selectedNodeID string) (dataPipelineCompiledSelect, error) {
	return s.compileSelectWithOptions(ctx, tenantID, graph, selectedNodeID, false)
}

func (s *DataPipelineService) compileSelectWithOptions(ctx context.Context, tenantID int64, graph DataPipelineGraph, selectedNodeID string, omitSort bool) (dataPipelineCompiledSelect, error) {
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
	compiler.omitSort = omitSort
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
	if err != nil && node.Data.StepType != DataPipelineStepJoin && node.Data.StepType != DataPipelineStepUnion {
		return dataPipelineRelation{}, err
	}
	switch node.Data.StepType {
	case DataPipelineStepProfile, DataPipelineStepValidate:
		return c.passThrough(node, upstream), nil
	case DataPipelineStepOutput:
		return c.compileOutput(node, upstream)
	case DataPipelineStepClean:
		return c.compileClean(node, upstream)
	case DataPipelineStepNormalize:
		return c.compileNormalize(node, upstream)
	case DataPipelineStepSchemaMapping:
		return c.compileSchemaMapping(node, upstream)
	case DataPipelineStepSchemaCompletion:
		return c.compileSchemaCompletion(node, upstream)
	case DataPipelineStepUnion:
		return c.compileUnion(node, relations)
	case DataPipelineStepJoin:
		return c.compileJoin(node, relations)
	case DataPipelineStepEnrichJoin:
		return c.compileEnrichJoin(ctx, node, upstream)
	case DataPipelineStepTransform:
		return c.compileTransform(node, upstream)
	case DataPipelineStepRouteByCondition:
		return c.compileRouteByCondition(node, upstream)
	case DataPipelineStepPartitionFilter:
		return c.compilePartitionFilter(node, upstream)
	case DataPipelineStepWatermarkFilter:
		return c.compileWatermarkFilter(node, upstream)
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

func (c *dataPipelineCompiler) compileOutput(node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	columns, expressions, err := dataPipelineOutputColumns(node.Data.Config, upstream.Columns)
	if err != nil {
		return dataPipelineRelation{}, err
	}
	sql := fmt.Sprintf("SELECT\n%s\nFROM %s", dataPipelineSelectList(columns, expressions), quoteCHIdent(upstream.CTE))
	return dataPipelineRelation{
		CTE:     dataPipelineCTEName(node.ID),
		SQL:     sql,
		Columns: columns,
		Node:    node,
		Source:  upstream.Source,
	}, nil
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

func (c *dataPipelineCompiler) manyUpstreams(node DataPipelineNode, relations map[string]dataPipelineRelation) ([]dataPipelineRelation, error) {
	sources := c.incoming[node.ID]
	if len(sources) < 2 {
		return nil, fmt.Errorf("%w: node %s must have at least two upstream edges", ErrInvalidDataPipelineGraph, node.ID)
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

func (c *dataPipelineCompiler) compileUnion(node DataPipelineNode, relations map[string]dataPipelineRelation) (dataPipelineRelation, error) {
	upstreams, err := c.manyUpstreams(node, relations)
	if err != nil {
		return dataPipelineRelation{}, err
	}
	columns := dataPipelineUnionColumns(node.Data.Config, relationColumns(upstreams))
	sourceLabelColumn := dataPipelineString(node.Data.Config, "sourceLabelColumn")
	if sourceLabelColumn != "" {
		if err := dataPipelineValidateIdentifier(sourceLabelColumn); err != nil {
			return dataPipelineRelation{}, fmt.Errorf("%w: invalid sourceLabelColumn: %s", ErrInvalidDataPipelineGraph, err.Error())
		}
		columns = dataPipelineUniqueStrings(append(columns, sourceLabelColumn))
	}
	selects := make([]string, 0, len(upstreams))
	for _, upstream := range upstreams {
		expressions := make(map[string]string, len(columns))
		for _, column := range columns {
			switch {
			case sourceLabelColumn != "" && column == sourceLabelColumn:
				expressions[column] = dataPipelineLiteral(upstream.Node.ID)
			case dataPipelineHasColumn(upstream.Columns, column):
				expressions[column] = "toString(" + quoteCHIdent(column) + ")"
			default:
				expressions[column] = "''"
			}
		}
		selects = append(selects, fmt.Sprintf("SELECT\n%s\nFROM %s", dataPipelineSelectList(columns, expressions), quoteCHIdent(upstream.CTE)))
	}
	return dataPipelineRelation{
		CTE:     dataPipelineCTEName(node.ID),
		SQL:     strings.Join(selects, "\nUNION ALL\n"),
		Columns: columns,
		Node:    node,
		Source:  upstreams[0].Source,
	}, nil
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
	if len(selectColumns) == 0 {
		selectColumns = append([]string(nil), rightColumns...)
	}
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
		if c.omitSort {
			return c.passThrough(node, upstream), nil
		}
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

func (s *DataPipelineService) collectStructuredRunNodeResults(ctx context.Context, tenantID int64, runKey string, graph DataPipelineGraph, conn driver.Conn) (map[string]dataPipelineRunNodeResult, error) {
	results := make(map[string]dataPipelineRunNodeResult, len(graph.Nodes))
	order, err := dataPipelineTopologicalOrder(graph)
	if err != nil {
		return nil, err
	}
	incoming := dataPipelineIncomingNodeIDs(graph)
	for _, node := range order {
		compiled, err := s.compileSelectWithOptions(ctx, tenantID, graph, node.ID, true)
		if err != nil {
			return nil, fmt.Errorf("compile data pipeline step metadata for %s: %w", node.ID, err)
		}
		rowCount, queryStats, err := queryDataPipelineCountWithStats(ctx, conn, compiled.SQL, dataPipelineQueryID(runKey, node.ID, "count"))
		if err != nil {
			return nil, fmt.Errorf("count data pipeline step %s: %w", node.ID, err)
		}
		metadata := map[string]any{
			"inputRows":  dataPipelineInputRows(node.ID, incoming, results, rowCount),
			"outputRows": rowCount,
			"queryStats": queryStats,
			"warnings":   []string{},
		}
		switch node.Data.StepType {
		case DataPipelineStepProfile:
			profile, err := s.collectStructuredProfileMetadata(ctx, conn, compiled, node)
			if err != nil {
				return nil, fmt.Errorf("profile data pipeline step %s: %w", node.ID, err)
			}
			metadata["profile"] = profile
		case DataPipelineStepValidate:
			validation, err := s.collectStructuredValidationMetadata(ctx, conn, compiled, node)
			if err != nil {
				return nil, fmt.Errorf("validate data pipeline step %s: %w", node.ID, err)
			}
			metadata["validation"] = validation
			if failedRows, ok := validation["failedRows"].(int64); ok {
				metadata["failedRows"] = failedRows
			}
			if warningCount, ok := validation["warningCount"].(int64); ok {
				metadata["warningCount"] = warningCount
			}
		case DataPipelineStepRouteByCondition:
			spec, err := dataPipelineRouteByConditionSpec(node.Data.Config, compiled.Columns)
			if err != nil {
				return nil, fmt.Errorf("route_by_condition data pipeline step %s: %w", node.ID, err)
			}
			routeCounts, err := queryDataPipelineRouteCounts(ctx, conn, compiled.SQL, spec.RouteColumn)
			if err != nil {
				return nil, fmt.Errorf("route_by_condition data pipeline step %s: %w", node.ID, err)
			}
			metadata["routeColumn"] = spec.RouteColumn
			metadata["defaultRoute"] = spec.DefaultRoute
			metadata["mode"] = spec.Mode
			metadata["selectedRoute"] = spec.SelectedRoute
			metadata["routes"] = spec.Routes
			metadata["routeCounts"] = routeCounts
		case DataPipelineStepPartitionFilter:
			spec, err := dataPipelinePartitionFilterSpec(node.Data.Config, compiled.Columns)
			if err != nil {
				return nil, fmt.Errorf("partition_filter data pipeline step %s: %w", node.ID, err)
			}
			metadata["partitionFilter"] = spec.Metadata()
		case DataPipelineStepWatermarkFilter:
			spec, err := dataPipelineWatermarkFilterSpec(node.Data.Config, compiled.Columns)
			if err != nil {
				return nil, fmt.Errorf("watermark_filter data pipeline step %s: %w", node.ID, err)
			}
			metadata["watermarkFilter"] = spec.Metadata()
		}
		results[node.ID] = dataPipelineRunNodeResult{
			NodeID:   node.ID,
			StepType: node.Data.StepType,
			RowCount: rowCount,
			Metadata: metadata,
		}
	}
	return results, nil
}

func (s *DataPipelineService) collectStructuredProfileMetadata(ctx context.Context, conn driver.Conn, compiled dataPipelineCompiledSelect, node DataPipelineNode) (map[string]any, error) {
	columns := dataPipelineStringSlice(node.Data.Config, "columns")
	if len(columns) == 0 {
		columns = append([]string{}, compiled.Columns...)
	}
	if len(columns) > 20 {
		columns = columns[:20]
	}
	topLimit := int(dataPipelineFloat(node.Data.Config, "topValuesLimit", 10))
	if topLimit <= 0 || topLimit > 20 {
		topLimit = 10
	}
	rowCount, err := queryDataPipelineCount(ctx, conn, compiled.SQL)
	if err != nil {
		return nil, err
	}
	columnSummaries := make([]map[string]any, 0, len(columns))
	for _, column := range columns {
		if err := dataPipelineRequireColumn(compiled.Columns, column); err != nil {
			return nil, err
		}
		nullCount, uniqueCount, minValue, maxValue, err := queryDataPipelineProfileColumn(ctx, conn, compiled.SQL, column)
		if err != nil {
			return nil, err
		}
		topValues, err := queryDataPipelineTopValues(ctx, conn, compiled.SQL, column, topLimit)
		if err != nil {
			return nil, err
		}
		nullRate := 0.0
		if rowCount > 0 {
			nullRate = float64(nullCount) / float64(rowCount)
		}
		columnSummaries = append(columnSummaries, map[string]any{
			"name":        column,
			"nullCount":   nullCount,
			"nullRate":    nullRate,
			"uniqueCount": uniqueCount,
			"min":         minValue,
			"max":         maxValue,
			"topValues":   topValues,
		})
	}
	return map[string]any{
		"rowCount":    rowCount,
		"columnCount": len(compiled.Columns),
		"columns":     columnSummaries,
	}, nil
}

func (s *DataPipelineService) collectStructuredValidationMetadata(ctx context.Context, conn driver.Conn, compiled dataPipelineCompiledSelect, node DataPipelineNode) (map[string]any, error) {
	rules := dataPipelineConfigObjects(node.Data.Config, "rules")
	ruleSummaries := make([]map[string]any, 0, len(rules))
	samples := make([]map[string]any, 0, 5)
	var failedRows int64
	var warningCount int64
	var errorCount int64
	for index, rule := range rules {
		column := dataPipelineString(rule, "column")
		operator := strings.ToLower(firstNonEmpty(dataPipelineString(rule, "operator"), dataPipelineString(rule, "op")))
		if operator == "" {
			operator = "required"
		}
		severity := strings.ToLower(dataPipelineString(rule, "severity"))
		if severity == "" {
			severity = "error"
		}
		failCount, err := queryDataPipelineValidationFailureCount(ctx, conn, compiled.SQL, compiled.Columns, rule)
		if err != nil {
			return nil, err
		}
		if failCount > 0 {
			failedRows += failCount
			if severity == "warning" {
				warningCount += failCount
			} else {
				errorCount += failCount
			}
			if len(samples) < 5 {
				sample, err := queryDataPipelineValidationSample(ctx, conn, compiled.SQL, compiled.Columns, rule, index)
				if err != nil {
					return nil, err
				}
				if sample != nil {
					samples = append(samples, sample)
				}
			}
		}
		ruleSummaries = append(ruleSummaries, map[string]any{
			"column":     column,
			"operator":   operator,
			"severity":   severity,
			"failedRows": failCount,
		})
	}
	return map[string]any{
		"ruleCount":    len(rules),
		"failedRows":   failedRows,
		"errorCount":   errorCount,
		"warningCount": warningCount,
		"rules":        ruleSummaries,
		"samples":      samples,
	}, nil
}

func queryDataPipelineCount(ctx context.Context, conn driver.Conn, sql string) (int64, error) {
	var count uint64
	if err := queryDataPipelineSingle(ctx, conn, fmt.Sprintf("SELECT count() FROM (\n%s\n)", sql), &count); err != nil {
		return 0, err
	}
	return int64(count), nil
}

func queryDataPipelineCountWithStats(ctx context.Context, conn driver.Conn, sql, queryID string) (int64, map[string]any, error) {
	start := time.Now()
	var count uint64
	query := fmt.Sprintf("SELECT count() FROM (\n%s\n)", sql)
	if err := queryDataPipelineSingleWithQueryID(ctx, conn, query, queryID, &count); err != nil {
		return 0, nil, err
	}
	stats := collectDataPipelineQueryStats(ctx, conn, queryID, "count", time.Since(start))
	return int64(count), stats, nil
}

func queryDataPipelineProfileColumn(ctx context.Context, conn driver.Conn, sql, column string) (int64, int64, string, string, error) {
	col := quoteCHIdent(column)
	query := fmt.Sprintf(
		"SELECT countIf(isNull(%[1]s) OR empty(ifNull(toString(%[1]s), ''))), uniqExact(ifNull(toString(%[1]s), '')), ifNull(min(ifNull(toString(%[1]s), '')), ''), ifNull(max(ifNull(toString(%[1]s), '')), '') FROM (\n%[2]s\n)",
		col,
		sql,
	)
	var nullCount uint64
	var uniqueCount uint64
	var minValue string
	var maxValue string
	if err := queryDataPipelineSingle(ctx, conn, query, &nullCount, &uniqueCount, &minValue, &maxValue); err != nil {
		return 0, 0, "", "", err
	}
	return int64(nullCount), int64(uniqueCount), minValue, maxValue, nil
}

func queryDataPipelineTopValues(ctx context.Context, conn driver.Conn, sql, column string, limit int) ([]map[string]any, error) {
	col := quoteCHIdent(column)
	query := fmt.Sprintf("SELECT ifNull(toString(%s), '') AS value, count() AS count FROM (\n%s\n) GROUP BY value ORDER BY count DESC, value ASC LIMIT %d", col, sql, limit)
	rows, err := conn.Query(clickhouse.Context(ctx), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]map[string]any, 0, limit)
	for rows.Next() {
		var value string
		var count uint64
		if err := rows.Scan(&value, &count); err != nil {
			return nil, err
		}
		values = append(values, map[string]any{"value": value, "count": int64(count)})
	}
	return values, rows.Err()
}

func queryDataPipelineRouteCounts(ctx context.Context, conn driver.Conn, sql, routeColumn string) ([]map[string]any, error) {
	if err := dataPipelineValidateIdentifier(routeColumn); err != nil {
		return nil, err
	}
	col := quoteCHIdent(routeColumn)
	query := fmt.Sprintf("SELECT ifNull(toString(%[1]s), '') AS route, count() AS count FROM (\n%[2]s\n) GROUP BY route ORDER BY route ASC", col, sql)
	rows, err := conn.Query(clickhouse.Context(ctx), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var route string
		var count uint64
		if err := rows.Scan(&route, &count); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"route": route, "count": int64(count)})
	}
	return out, rows.Err()
}

func queryDataPipelineValidationFailureCount(ctx context.Context, conn driver.Conn, sql string, columns []string, rule map[string]any) (int64, error) {
	operator := strings.ToLower(firstNonEmpty(dataPipelineString(rule, "operator"), dataPipelineString(rule, "op")))
	if operator == "unique" {
		column := dataPipelineString(rule, "column")
		if err := dataPipelineRequireColumn(columns, column); err != nil {
			return 0, err
		}
		col := quoteCHIdent(column)
		query := fmt.Sprintf("SELECT ifNull(sum(rows), 0) FROM (SELECT count() AS rows FROM (\n%s\n) GROUP BY %s HAVING rows > 1)", sql, col)
		var count uint64
		if err := queryDataPipelineSingle(ctx, conn, query, &count); err != nil {
			return 0, err
		}
		return int64(count), nil
	}
	expr, err := dataPipelineValidationPassExpr(columns, rule)
	if err != nil {
		return 0, err
	}
	query := fmt.Sprintf("SELECT countIf(NOT (%s)) FROM (\n%s\n)", expr, sql)
	var count uint64
	if err := queryDataPipelineSingle(ctx, conn, query, &count); err != nil {
		return 0, err
	}
	return int64(count), nil
}

func queryDataPipelineValidationSample(ctx context.Context, conn driver.Conn, sql string, columns []string, rule map[string]any, ruleIndex int) (map[string]any, error) {
	operator := strings.ToLower(firstNonEmpty(dataPipelineString(rule, "operator"), dataPipelineString(rule, "op")))
	if operator == "" {
		operator = "required"
	}
	column := dataPipelineString(rule, "column")
	if operator == "unique" {
		if err := dataPipelineRequireColumn(columns, column); err != nil {
			return nil, err
		}
		col := quoteCHIdent(column)
		query := fmt.Sprintf("SELECT rowNumberInAllBlocks() + 1 FROM (SELECT %s FROM (\n%s\n) GROUP BY %s HAVING count() > 1) LIMIT 1", col, sql, col)
		var rowNumber uint64
		if err := queryDataPipelineSingle(ctx, conn, query, &rowNumber); err != nil {
			return nil, err
		}
		return map[string]any{
			"rowNumber": int64(rowNumber),
			"column":    column,
			"operator":  operator,
			"severity":  firstNonEmpty(strings.ToLower(dataPipelineString(rule, "severity")), "error"),
			"reason":    operator,
			"ruleIndex": ruleIndex,
		}, nil
	}
	expr, err := dataPipelineValidationPassExpr(columns, rule)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT __dp_row_number FROM (SELECT rowNumberInAllBlocks() + 1 AS __dp_row_number, * FROM (\n%s\n)) WHERE NOT (%s) LIMIT 1", sql, expr)
	var rowNumber uint64
	if err := queryDataPipelineSingle(ctx, conn, query, &rowNumber); err != nil {
		return nil, err
	}
	return map[string]any{
		"rowNumber": int64(rowNumber),
		"column":    column,
		"operator":  operator,
		"severity":  firstNonEmpty(strings.ToLower(dataPipelineString(rule, "severity")), "error"),
		"reason":    operator,
		"ruleIndex": ruleIndex,
	}, nil
}

func dataPipelineValidationPassExpr(columns []string, rule map[string]any) (string, error) {
	operator := strings.ToLower(firstNonEmpty(dataPipelineString(rule, "operator"), dataPipelineString(rule, "op")))
	if operator == "range" {
		column := dataPipelineString(rule, "column")
		if err := dataPipelineRequireColumn(columns, column); err != nil {
			return "", err
		}
		col := "toFloat64OrNull(toString(" + quoteCHIdent(column) + "))"
		parts := []string{}
		if _, ok := rule["min"]; ok {
			parts = append(parts, col+" >= "+dataPipelineLiteral(rule["min"]))
		}
		if _, ok := rule["max"]; ok {
			parts = append(parts, col+" <= "+dataPipelineLiteral(rule["max"]))
		}
		if len(parts) == 0 {
			return "1 = 1", nil
		}
		return strings.Join(parts, " AND "), nil
	}
	if _, ok := rule["values"]; ok && rule["value"] == nil {
		rule = cloneDataPipelineConfig(rule)
		rule["value"] = rule["values"]
	}
	return dataPipelineConditionExpr(columns, dataPipelineString(rule, "column"), rule)
}

func queryDataPipelineSingle(ctx context.Context, conn driver.Conn, sql string, dest ...any) error {
	return queryDataPipelineSingleWithQueryID(ctx, conn, sql, "", dest...)
}

func queryDataPipelineSingleWithQueryID(ctx context.Context, conn driver.Conn, sql, queryID string, dest ...any) error {
	queryCtx := clickhouse.Context(ctx)
	if strings.TrimSpace(queryID) != "" {
		queryCtx = clickhouse.Context(ctx, clickhouse.WithQueryID(queryID))
	}
	rows, err := conn.Query(queryCtx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return fmt.Errorf("query returned no rows")
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	return rows.Err()
}

func collectDataPipelineQueryStats(ctx context.Context, conn driver.Conn, queryID, purpose string, elapsed time.Duration) map[string]any {
	stats := map[string]any{
		"queryId":   queryID,
		"purpose":   purpose,
		"elapsedMs": elapsed.Milliseconds(),
	}
	if strings.TrimSpace(queryID) == "" {
		return stats
	}
	query := fmt.Sprintf("SELECT ifNull(max(query_duration_ms), 0), ifNull(max(read_rows), 0), ifNull(max(read_bytes), 0) FROM system.query_log WHERE query_id = %s AND type = 'QueryFinish'", dataPipelineLiteral(queryID))
	var durationMs uint64
	var readRows uint64
	var readBytes uint64
	if err := queryDataPipelineSingle(ctx, conn, query, &durationMs, &readRows, &readBytes); err != nil {
		return stats
	}
	if durationMs > 0 {
		stats["elapsedMs"] = int64(durationMs)
	}
	stats["readRows"] = int64(readRows)
	stats["readBytes"] = int64(readBytes)
	return stats
}

func dataPipelineQueryID(runKey, nodeID, purpose string) string {
	value := "dp_" + dataPipelineSafeSuffix(runKey) + "_" + dataPipelineSafeSuffix(nodeID) + "_" + dataPipelineSafeSuffix(purpose)
	if len(value) > 120 {
		value = value[:120]
	}
	return value
}

func dataPipelineIncomingNodeIDs(graph DataPipelineGraph) map[string][]string {
	incoming := make(map[string][]string, len(graph.Nodes))
	for _, edge := range graph.Edges {
		incoming[edge.Target] = append(incoming[edge.Target], edge.Source)
	}
	return incoming
}

func dataPipelineInputRows(nodeID string, incoming map[string][]string, results map[string]dataPipelineRunNodeResult, fallback int64) int64 {
	sources := incoming[nodeID]
	if len(sources) == 0 {
		return fallback
	}
	var total int64
	for _, source := range sources {
		total += results[source].RowCount
	}
	return total
}

func cloneDataPipelineConfig(config map[string]any) map[string]any {
	out := make(map[string]any, len(config))
	for key, value := range config {
		out[key] = value
	}
	return out
}

func (s *DataPipelineService) executeRun(ctx context.Context, tenantID int64, run db.DataPipelineRun, version db.DataPipelineVersion) ([]dataPipelineRunOutputResult, error) {
	if s == nil || s.datasets == nil {
		return nil, fmt.Errorf("data pipeline service is not configured")
	}
	graph, err := decodeDataPipelineGraph(version.Graph)
	if err != nil {
		return nil, err
	}
	if dataPipelineGraphNeedsHybrid(graph) {
		return s.executeHybridRun(ctx, tenantID, run, graph)
	}
	if err := s.datasets.ensureTenantSandbox(ctx, tenantID); err != nil {
		return nil, err
	}
	conn, err := s.datasets.openTenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	targetDatabase := datasetWorkDatabaseName(tenantID)
	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.exportQuerySettings()))
	var userID int64
	if run.RequestedByUserID.Valid {
		userID = run.RequestedByUserID.Int64
	}
	nodeResults, err := s.collectStructuredRunNodeResults(ctx, tenantID, run.PublicID.String(), graph, conn)
	if err != nil {
		return nil, err
	}
	outputs := dataPipelineOutputNodes(graph)
	results := make([]dataPipelineRunOutputResult, 0, len(outputs))
	for _, outputNode := range outputs {
		compiled, err := s.compileRunSelect(ctx, tenantID, graph, outputNode)
		result := dataPipelineRunOutputResult{Node: outputNode, Compiled: compiled, Err: err, NodeResults: nodeResults}
		if err == nil {
			targetTable := dataPipelineOutputTableName(outputNode, run)
			stageTable := "__dp_stage_" + strings.ReplaceAll(run.PublicID.String(), "-", "") + "_" + dataPipelineSafeSuffix(outputNode.ID)
			_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(stageTable)))
			createSQL := fmt.Sprintf(
				"CREATE TABLE %s.%s ENGINE = MergeTree ORDER BY %s AS\n%s",
				quoteCHIdent(targetDatabase),
				quoteCHIdent(stageTable),
				dataPipelineOutputOrderBy(outputNode, compiled.Columns),
				compiled.SQL,
			)
			if err := conn.Exec(queryCtx, createSQL); err != nil {
				result.Err = fmt.Errorf("create data pipeline stage table for %s: %w", outputNode.ID, err)
			} else {
				_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(targetTable)))
				if err := conn.Exec(queryCtx, fmt.Sprintf("RENAME TABLE %s.%s TO %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(stageTable), quoteCHIdent(targetDatabase), quoteCHIdent(targetTable))); err != nil {
					_ = conn.Exec(queryCtx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(targetDatabase), quoteCHIdent(stageTable)))
					result.Err = fmt.Errorf("promote data pipeline stage table for %s: %w", outputNode.ID, err)
				} else {
					displayName := dataPipelineString(outputNode.Data.Config, "displayName")
					if displayName == "" {
						displayName = targetTable
					}
					workTable, err := s.datasets.registerDatasetWorkTableForRef(ctx, tenantID, userID, nil, nil, targetDatabase, targetTable, displayName)
					if err != nil {
						result.Err = err
					} else {
						if s.authz != nil {
							if err := s.authz.EnsureResourceOwnerTuples(ctx, tenantID, userID, DataResourceWorkTable, workTable.PublicID); err != nil {
								result.Err = err
								results = append(results, result)
								continue
							}
						}
						result.WorkTable = workTable
					}
				}
			}
		}
		results = append(results, result)
	}
	return results, nil
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
	return "dp_" + raw + "_" + dataPipelineSafeSuffix(node.ID)
}

func dataPipelineOutputOrderBy(node DataPipelineNode, availableColumns ...[]string) string {
	columns := dataPipelineStringSlice(node.Data.Config, "orderBy")
	if len(columns) == 0 {
		return "tuple()"
	}
	var allowed []string
	if len(availableColumns) > 0 {
		allowed = availableColumns[0]
	}
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		if err := dataPipelineValidateIdentifier(column); err != nil {
			continue
		}
		if len(allowed) > 0 && !dataPipelineHasColumn(allowed, column) {
			continue
		}
		parts = append(parts, quoteCHIdent(column))
	}
	if len(parts) == 0 {
		return "tuple()"
	}
	return strings.Join(parts, ", ")
}

func dataPipelineOutputColumns(config map[string]any, upstreamColumns []string) ([]string, map[string]string, error) {
	specs := dataPipelineConfigObjects(config, "columns")
	if len(specs) == 0 {
		columns := append([]string(nil), upstreamColumns...)
		return columns, dataPipelineColumnExpressions(columns), nil
	}
	columns := make([]string, 0, len(specs))
	expressions := make(map[string]string, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		source := strings.TrimSpace(dataPipelineString(spec, "sourceColumn"))
		if source == "" {
			source = strings.TrimSpace(dataPipelineString(spec, "column"))
		}
		if err := dataPipelineRequireColumn(upstreamColumns, source); err != nil {
			return nil, nil, err
		}
		name := strings.TrimSpace(dataPipelineString(spec, "name"))
		if name == "" {
			name = strings.TrimSpace(dataPipelineString(spec, "outputColumn"))
		}
		if name == "" {
			name = source
		}
		if err := dataPipelineValidateIdentifier(name); err != nil {
			return nil, nil, err
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return nil, nil, fmt.Errorf("%w: duplicate output column %s", ErrInvalidDataPipelineGraph, name)
		}
		seen[key] = struct{}{}
		expr, err := dataPipelineOutputCastExpr(quoteCHIdent(source), dataPipelineString(spec, "type"))
		if err != nil {
			return nil, nil, err
		}
		columns = append(columns, name)
		expressions[name] = expr
	}
	return columns, expressions, nil
}

func dataPipelineOutputCastExpr(expr, typ string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "", "string":
		return "toString(" + expr + ")", nil
	case "int", "int64":
		return "toInt64OrNull(toString(" + expr + "))", nil
	case "float", "float64", "number":
		return "toFloat64OrNull(toString(" + expr + "))", nil
	case "bool", "boolean":
		value := "lowerUTF8(toString(" + expr + "))"
		return fmt.Sprintf("multiIf(%s IN ('true', '1', 'yes', 'y'), 1, %s IN ('false', '0', 'no', 'n'), 0, NULL)", value, value), nil
	case "date":
		return "toDate(parseDateTimeBestEffortOrNull(toString(" + expr + ")))", nil
	case "datetime", "datetime64":
		return "parseDateTimeBestEffortOrNull(toString(" + expr + "))", nil
	default:
		return "", fmt.Errorf("%w: unsupported output column type %q", ErrInvalidDataPipelineGraph, typ)
	}
}

func dataPipelineSafeSuffix(value string) string {
	suffix := strings.Trim(dataPipelineCTEName(value), "_")
	suffix = strings.TrimPrefix(suffix, "step_")
	if suffix == "" {
		return "output"
	}
	if len(suffix) > 48 {
		suffix = suffix[:48]
	}
	return suffix
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

func (c *dataPipelineCompiler) compileRouteByCondition(node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	spec, err := dataPipelineRouteByConditionSpec(node.Data.Config, upstream.Columns)
	if err != nil {
		return dataPipelineRelation{}, fmt.Errorf("compile route_by_condition node %s: %w", node.ID, err)
	}

	expressions := dataPipelineColumnExpressions(upstream.Columns)
	columns := dataPipelineUniqueStrings(append(append([]string{}, upstream.Columns...), spec.RouteColumn))
	expressions[spec.RouteColumn] = spec.RouteExpr
	sql := fmt.Sprintf("SELECT\n%s\nFROM %s", dataPipelineSelectList(columns, expressions), quoteCHIdent(upstream.CTE))
	if spec.Mode == "filter_route" {
		sql += "\nWHERE " + spec.RouteExpr + " = " + dataPipelineLiteral(spec.SelectedRoute)
	}
	return dataPipelineRelation{
		CTE:     dataPipelineCTEName(node.ID),
		SQL:     sql,
		Columns: columns,
		Node:    node,
		Source:  upstream.Source,
	}, nil
}

func (c *dataPipelineCompiler) compilePartitionFilter(node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	spec, err := dataPipelinePartitionFilterSpec(node.Data.Config, upstream.Columns)
	if err != nil {
		return dataPipelineRelation{}, fmt.Errorf("compile partition_filter node %s: %w", node.ID, err)
	}
	return c.compileFilterRelation(node, upstream, spec.Conditions), nil
}

func (c *dataPipelineCompiler) compileWatermarkFilter(node DataPipelineNode, upstream dataPipelineRelation) (dataPipelineRelation, error) {
	spec, err := dataPipelineWatermarkFilterSpec(node.Data.Config, upstream.Columns)
	if err != nil {
		return dataPipelineRelation{}, fmt.Errorf("compile watermark_filter node %s: %w", node.ID, err)
	}
	return c.compileFilterRelation(node, upstream, []string{spec.Condition}), nil
}

func (c *dataPipelineCompiler) compileFilterRelation(node DataPipelineNode, upstream dataPipelineRelation, conditions []string) dataPipelineRelation {
	sql := fmt.Sprintf("SELECT\n%s\nFROM %s", dataPipelineSelectList(upstream.Columns, dataPipelineColumnExpressions(upstream.Columns)), quoteCHIdent(upstream.CTE))
	if len(conditions) > 0 {
		sql += "\nWHERE " + strings.Join(conditions, " AND ")
	}
	return dataPipelineRelation{
		CTE:     dataPipelineCTEName(node.ID),
		SQL:     sql,
		Columns: append([]string(nil), upstream.Columns...),
		Node:    node,
		Source:  upstream.Source,
	}
}

type dataPipelineRouteByConditionConfig struct {
	RouteColumn   string
	DefaultRoute  string
	Mode          string
	SelectedRoute string
	RouteExpr     string
	Routes        []string
}

func dataPipelineRouteByConditionSpec(config map[string]any, columns []string) (dataPipelineRouteByConditionConfig, error) {
	routeColumn := firstNonEmpty(dataPipelineString(config, "routeColumn"), "route_key")
	if err := dataPipelineValidateIdentifier(routeColumn); err != nil {
		return dataPipelineRouteByConditionConfig{}, fmt.Errorf("%w: invalid routeColumn: %s", ErrInvalidDataPipelineGraph, err.Error())
	}
	defaultRoute := firstNonEmpty(dataPipelineString(config, "defaultRoute"), "default")
	rules := dataPipelineConfigObjects(config, "rules")
	routeExprParts := make([]string, 0, len(rules)*2+1)
	routes := make([]string, 0, len(rules)+1)
	for _, rule := range rules {
		route := strings.TrimSpace(firstNonEmpty(dataPipelineString(rule, "route"), dataPipelineString(rule, "routeKey")))
		if route == "" {
			continue
		}
		condition, err := dataPipelineConditionExpr(columns, "", rule)
		if err != nil {
			return dataPipelineRouteByConditionConfig{}, err
		}
		routeExprParts = append(routeExprParts, condition, dataPipelineLiteral(route))
		routes = append(routes, route)
	}
	routeExprParts = append(routeExprParts, dataPipelineLiteral(defaultRoute))
	routeExpr := dataPipelineLiteral(defaultRoute)
	if len(routeExprParts) > 1 {
		routeExpr = "multiIf(" + strings.Join(routeExprParts, ", ") + ")"
	}
	mode := firstNonEmpty(dataPipelineString(config, "mode"), "annotate")
	selectedRoute := strings.TrimSpace(firstNonEmpty(dataPipelineString(config, "route"), dataPipelineString(config, "selectedRoute")))
	if mode == "filter_route" && selectedRoute == "" {
		return dataPipelineRouteByConditionConfig{}, fmt.Errorf("%w: route_by_condition filter_route requires route", ErrInvalidDataPipelineGraph)
	}
	if mode != "annotate" && mode != "filter_route" {
		return dataPipelineRouteByConditionConfig{}, fmt.Errorf("%w: route_by_condition mode must be annotate or filter_route", ErrInvalidDataPipelineGraph)
	}
	routes = dataPipelineUniqueStrings(append(routes, defaultRoute))
	return dataPipelineRouteByConditionConfig{
		RouteColumn:   routeColumn,
		DefaultRoute:  defaultRoute,
		Mode:          mode,
		SelectedRoute: selectedRoute,
		RouteExpr:     routeExpr,
		Routes:        routes,
	}, nil
}

func relationColumns(relations []dataPipelineRelation) [][]string {
	out := make([][]string, 0, len(relations))
	for _, relation := range relations {
		out = append(out, relation.Columns)
	}
	return out
}

func materializedRelationColumns(relations []dataPipelineMaterializedRelation) [][]string {
	out := make([][]string, 0, len(relations))
	for _, relation := range relations {
		out = append(out, relation.Columns)
	}
	return out
}

func dataPipelineUnionColumns(config map[string]any, inputs [][]string) []string {
	configured := dataPipelineStringSlice(config, "columns")
	if len(configured) > 0 {
		return dataPipelineUniqueStrings(configured)
	}
	columns := make([]string, 0)
	for _, input := range inputs {
		columns = append(columns, input...)
	}
	return dataPipelineUniqueStrings(columns)
}

type dataPipelinePartitionFilterConfig struct {
	DateColumn     string
	Start          string
	End            string
	ValueType      string
	PartitionKey   string
	PartitionValue string
	Conditions     []string
}

func (c dataPipelinePartitionFilterConfig) Metadata() map[string]any {
	return map[string]any{
		"dateColumn":     c.DateColumn,
		"start":          c.Start,
		"end":            c.End,
		"valueType":      c.ValueType,
		"partitionKey":   c.PartitionKey,
		"partitionValue": c.PartitionValue,
	}
}

func dataPipelinePartitionFilterSpec(config map[string]any, columns []string) (dataPipelinePartitionFilterConfig, error) {
	dateColumn := firstNonEmpty(dataPipelineString(config, "dateColumn"), dataPipelineString(config, "column"))
	if err := dataPipelineRequireColumn(columns, dateColumn); err != nil {
		return dataPipelinePartitionFilterConfig{}, err
	}
	valueType := firstNonEmpty(dataPipelineString(config, "valueType"), "datetime")
	columnExpr, err := dataPipelineComparableColumnExpr(columns, dateColumn, valueType)
	if err != nil {
		return dataPipelinePartitionFilterConfig{}, err
	}
	start := dataPipelineString(config, "start")
	end := dataPipelineString(config, "end")
	conditions := make([]string, 0, 3)
	if start != "" {
		valueExpr, err := dataPipelineComparableLiteral(start, valueType)
		if err != nil {
			return dataPipelinePartitionFilterConfig{}, err
		}
		conditions = append(conditions, columnExpr+" >= "+valueExpr)
	}
	if end != "" {
		valueExpr, err := dataPipelineComparableLiteral(end, valueType)
		if err != nil {
			return dataPipelinePartitionFilterConfig{}, err
		}
		operator := "<"
		if dataPipelineBool(config, "includeEnd", false) {
			operator = "<="
		}
		conditions = append(conditions, columnExpr+" "+operator+" "+valueExpr)
	}
	partitionKey := dataPipelineString(config, "partitionKey")
	partitionValue := dataPipelineString(config, "partitionValue")
	if partitionKey != "" {
		if err := dataPipelineRequireColumn(columns, partitionKey); err != nil {
			return dataPipelinePartitionFilterConfig{}, err
		}
		if partitionValue != "" {
			conditions = append(conditions, "toString("+quoteCHIdent(partitionKey)+") = "+dataPipelineLiteral(partitionValue))
		}
	}
	if len(conditions) == 0 {
		return dataPipelinePartitionFilterConfig{}, fmt.Errorf("%w: partition_filter requires start, end, or partitionValue", ErrInvalidDataPipelineGraph)
	}
	return dataPipelinePartitionFilterConfig{
		DateColumn:     dateColumn,
		Start:          start,
		End:            end,
		ValueType:      valueType,
		PartitionKey:   partitionKey,
		PartitionValue: partitionValue,
		Conditions:     conditions,
	}, nil
}

type dataPipelineWatermarkFilterConfig struct {
	Column         string
	WatermarkValue string
	ValueType      string
	Inclusive      bool
	Condition      string
}

func (c dataPipelineWatermarkFilterConfig) Metadata() map[string]any {
	return map[string]any{
		"column":         c.Column,
		"watermarkValue": c.WatermarkValue,
		"valueType":      c.ValueType,
		"inclusive":      c.Inclusive,
	}
}

func dataPipelineWatermarkFilterSpec(config map[string]any, columns []string) (dataPipelineWatermarkFilterConfig, error) {
	column := firstNonEmpty(dataPipelineString(config, "watermarkColumn"), dataPipelineString(config, "column"))
	if err := dataPipelineRequireColumn(columns, column); err != nil {
		return dataPipelineWatermarkFilterConfig{}, err
	}
	watermarkValue := firstNonEmpty(dataPipelineString(config, "watermarkValue"), dataPipelineString(config, "value"))
	if watermarkValue == "" {
		return dataPipelineWatermarkFilterConfig{}, fmt.Errorf("%w: watermark_filter requires watermarkValue", ErrInvalidDataPipelineGraph)
	}
	valueType := firstNonEmpty(dataPipelineString(config, "valueType"), "datetime")
	columnExpr, err := dataPipelineComparableColumnExpr(columns, column, valueType)
	if err != nil {
		return dataPipelineWatermarkFilterConfig{}, err
	}
	valueExpr, err := dataPipelineComparableLiteral(watermarkValue, valueType)
	if err != nil {
		return dataPipelineWatermarkFilterConfig{}, err
	}
	inclusive := dataPipelineBool(config, "inclusive", false)
	operator := ">"
	if inclusive {
		operator = ">="
	}
	return dataPipelineWatermarkFilterConfig{
		Column:         column,
		WatermarkValue: watermarkValue,
		ValueType:      valueType,
		Inclusive:      inclusive,
		Condition:      columnExpr + " " + operator + " " + valueExpr,
	}, nil
}

func dataPipelineComparableColumnExpr(columns []string, column, valueType string) (string, error) {
	if err := dataPipelineRequireColumn(columns, column); err != nil {
		return "", err
	}
	col := "toString(" + quoteCHIdent(column) + ")"
	switch strings.ToLower(valueType) {
	case "", "datetime", "date":
		return "parseDateTimeBestEffortOrNull(" + col + ")", nil
	case "number", "numeric":
		return "toFloat64OrNull(" + col + ")", nil
	case "string":
		return col, nil
	default:
		return "", fmt.Errorf("%w: unsupported filter valueType %q", ErrInvalidDataPipelineGraph, valueType)
	}
}

func dataPipelineComparableLiteral(value, valueType string) (string, error) {
	switch strings.ToLower(valueType) {
	case "", "datetime", "date":
		return "parseDateTimeBestEffort(" + dataPipelineLiteral(value) + ")", nil
	case "number", "numeric":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return "", fmt.Errorf("%w: invalid numeric filter value %q", ErrInvalidDataPipelineGraph, value)
		}
		return value, nil
	case "string":
		return dataPipelineLiteral(value), nil
	default:
		return "", fmt.Errorf("%w: unsupported filter valueType %q", ErrInvalidDataPipelineGraph, valueType)
	}
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
		return "isNotNull(" + col + ") AND notEmpty(trim(toString(" + col + ")))", nil
	case "=", "!=", ">", ">=", "<", "<=":
		return fmt.Sprintf("%s %s %s", col, operator, dataPipelineLiteral(value)), nil
	case "in":
		values := dataPipelineLiteralList(value)
		if len(values) == 0 {
			return "", fmt.Errorf("%w: in operator requires values array", ErrInvalidDataPipelineGraph)
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(values, ", ")), nil
	case "regex":
		return fmt.Sprintf("match(toString(%s), %s)", col, dataPipelineLiteral(value)), nil
	case "":
		return "1 = 1", nil
	default:
		return "", fmt.Errorf("%w: unsupported operator %q", ErrInvalidDataPipelineGraph, operator)
	}
}

func dataPipelineLiteralList(value any) []string {
	switch raw := value.(type) {
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			out = append(out, dataPipelineLiteral(item))
		}
		return out
	case []string:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			out = append(out, dataPipelineLiteral(item))
		}
		return out
	default:
		return nil
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

func dataPipelineInt(config map[string]any, key string, fallback int) int {
	if config == nil {
		return fallback
	}
	value, ok := config[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
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
