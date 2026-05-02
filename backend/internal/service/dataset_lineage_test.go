package service

import (
	"errors"
	"testing"
	"time"

	chparser "github.com/AfterShip/clickhouse-sql-parser/parser"
)

func TestNormalizeDatasetLineageOptions(t *testing.T) {
	opts, err := normalizeDatasetLineageOptions(DatasetLineageOptions{})
	if err != nil {
		t.Fatalf("normalizeDatasetLineageOptions(default) error = %v", err)
	}
	if opts.Direction != DatasetLineageDirectionBoth {
		t.Fatalf("Direction = %q, want %q", opts.Direction, DatasetLineageDirectionBoth)
	}
	if opts.Depth != 1 {
		t.Fatalf("Depth = %d, want 1", opts.Depth)
	}
	if opts.Limit != 50 {
		t.Fatalf("Limit = %d, want 50", opts.Limit)
	}

	opts, err = normalizeDatasetLineageOptions(DatasetLineageOptions{
		Direction: DatasetLineageDirectionDownstream,
		Depth:     99,
		Limit:     999,
	})
	if err != nil {
		t.Fatalf("normalizeDatasetLineageOptions(clamp) error = %v", err)
	}
	if opts.Depth != 2 {
		t.Fatalf("Depth = %d, want 2", opts.Depth)
	}
	if opts.Limit != 100 {
		t.Fatalf("Limit = %d, want 100", opts.Limit)
	}
	if opts.Level != DatasetLineageLevelTable {
		t.Fatalf("Level = %q, want table", opts.Level)
	}

	_, err = normalizeDatasetLineageOptions(DatasetLineageOptions{Direction: "sideways"})
	if !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("invalid direction error = %v, want ErrInvalidDatasetInput", err)
	}
	_, err = normalizeDatasetLineageOptions(DatasetLineageOptions{Level: "cell"})
	if !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("invalid level error = %v, want ErrInvalidDatasetInput", err)
	}
	_, err = normalizeDatasetLineageOptions(DatasetLineageOptions{Sources: []string{"metadata", "cloud"}})
	if !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("invalid source error = %v, want ErrInvalidDatasetInput", err)
	}
}

func TestDatasetLineageBuilderDeduplicatesMetadataEdges(t *testing.T) {
	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	builder := newDatasetLineageBuilder()
	dataset := builder.addDatasetNode(Dataset{
		PublicID:  "018f2f05-c6c9-7a49-b32d-04f4dd84ef4a",
		Name:      "Sales",
		Status:    "ready",
		CreatedAt: now,
		UpdatedAt: now,
	})
	query := builder.addQueryJobNode(DatasetQueryJob{
		PublicID:  "018f2f05-c6c9-7a49-b32d-04f4dd84ef4b",
		Statement: "SELECT count() FROM hh_t_1_raw.ds_sales",
		Status:    "completed",
		CreatedAt: now,
		UpdatedAt: now,
	})
	builder.rootID = dataset.ID
	builder.addEdge(dataset, query, DatasetLineageRelationQueryInput, now)
	builder.addEdge(dataset, query, DatasetLineageRelationQueryInput, now)
	builder.addTimeline(query, DatasetLineageRelationQueryInput, query.Status, now, nil)
	builder.addTimeline(query, DatasetLineageRelationQueryInput, query.Status, now, nil)

	graph := builder.graph()
	if len(graph.Nodes) != 2 {
		t.Fatalf("len(Nodes) = %d, want 2", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("len(Edges) = %d, want 1", len(graph.Edges))
	}
	edge := graph.Edges[0]
	if edge.SourceNodeID != dataset.ID || edge.TargetNodeID != query.ID {
		t.Fatalf("edge direction = %s -> %s, want %s -> %s", edge.SourceNodeID, edge.TargetNodeID, dataset.ID, query.ID)
	}
	if edge.Confidence != datasetLineageConfidenceMetadata {
		t.Fatalf("edge confidence = %q, want metadata", edge.Confidence)
	}
	if len(graph.Timeline) != 1 {
		t.Fatalf("len(Timeline) = %d, want 1", len(graph.Timeline))
	}
}

func TestExtractLineageColumnRefsIgnoresFunctionNames(t *testing.T) {
	query := mustParseSelectQuery(t, "SELECT count(*) AS n, sum(t.amount) AS total, t.customer_id FROM hh_t_1_work.orders AS t GROUP BY t.customer_id")
	if len(query.SelectItems) != 3 {
		t.Fatalf("select item count = %d, want 3", len(query.SelectItems))
	}

	countRefs := extractLineageColumnRefs(query.SelectItems[0].Expr)
	if len(countRefs) != 0 {
		t.Fatalf("count refs = %#v, want none", countRefs)
	}

	sumRefs := extractLineageColumnRefs(query.SelectItems[1].Expr)
	if len(sumRefs) != 1 || sumRefs[0].Qualifier != "t" || sumRefs[0].Column != "amount" {
		t.Fatalf("sum refs = %#v, want t.amount", sumRefs)
	}

	directRefs := extractLineageColumnRefs(query.SelectItems[2].Expr)
	if len(directRefs) != 1 || directRefs[0].Qualifier != "t" || directRefs[0].Column != "customer_id" {
		t.Fatalf("direct refs = %#v, want t.customer_id", directRefs)
	}
}

func TestExtractLineageTableRefsCapturesJoinAliases(t *testing.T) {
	query := mustParseSelectQuery(t, "SELECT o.id, c.name FROM hh_t_1_work.orders AS o JOIN hh_t_1_raw.ds_customers AS c ON o.customer_id = c.id")
	refs := extractLineageTableRefs(query)
	if len(refs) != 2 {
		t.Fatalf("refs = %#v, want two physical table refs", refs)
	}
	gotCustomer := false
	for _, ref := range refs {
		if ref.Database == "hh_t_1_raw" && ref.Table == "ds_customers" && ref.Alias == "c" {
			gotCustomer = true
		}
	}
	if !gotCustomer {
		t.Fatalf("refs = %#v, want hh_t_1_raw.ds_customers AS c", refs)
	}
}

func TestEnsureLineageAcyclicRejectsCycle(t *testing.T) {
	nodes := []DatasetLineageNodeInput{{ID: "a"}, {ID: "b"}}
	edges := []DatasetLineageEdgeInput{{SourceNodeID: "a", TargetNodeID: "b"}, {SourceNodeID: "b", TargetNodeID: "a"}}
	if err := ensureLineageAcyclic(nodes, edges); !errors.Is(err, ErrInvalidDatasetInput) {
		t.Fatalf("cycle error = %v, want ErrInvalidDatasetInput", err)
	}
}

func mustParseSelectQuery(t *testing.T, statement string) *chparser.SelectQuery {
	t.Helper()
	stmts, err := chparser.NewParser(statement).ParseStmts()
	if err != nil {
		t.Fatalf("parse statement: %v", err)
	}
	query, ok := findSelectQuery(stmts[0])
	if !ok {
		t.Fatalf("select query not found")
	}
	return query
}
