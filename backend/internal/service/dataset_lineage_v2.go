package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"example.com/haohao/backend/internal/db"

	chparser "github.com/AfterShip/clickhouse-sql-parser/parser"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	datasetLineageMaxPersistedNodes = 200
	datasetLineageMaxPersistedEdges = 400
	datasetLineageLoadLimit         = 1000
)

type DatasetLineageChangeSet struct {
	ID                   int64
	PublicID             string
	QueryJobPublicID     string
	RootResourceType     string
	RootResourcePublicID string
	SourceKind           string
	Status               string
	Title                string
	Description          string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	PublishedAt          *time.Time
	RejectedAt           *time.Time
	ArchivedAt           *time.Time
}

type DatasetLineageParseRun struct {
	PublicID          string
	QueryJobPublicID  string
	ChangeSetPublicID string
	Status            string
	TableRefCount     int32
	ColumnEdgeCount   int32
	ErrorSummary      string
	CreatedAt         time.Time
	CompletedAt       *time.Time
}

type DatasetLineageChangeSetWithGraph struct {
	ChangeSet DatasetLineageChangeSet
	Nodes     []DatasetLineageNode
	Edges     []DatasetLineageEdge
}

type DatasetLineageChangeSetInput struct {
	RootResourceType     string
	RootResourcePublicID string
	SourceKind           string
	Title                string
	Description          string
}

type DatasetLineageGraphInput struct {
	Nodes []DatasetLineageNodeInput
	Edges []DatasetLineageEdgeInput
}

type DatasetLineageNodeInput struct {
	ID           string
	ResourceType string
	PublicID     string
	DisplayName  string
	NodeKind     string
	SourceKind   string
	ColumnName   string
	Description  string
	Position     *DatasetLineagePosition
	Metadata     map[string]any
}

type DatasetLineageEdgeInput struct {
	ID           string
	SourceNodeID string
	TargetNodeID string
	RelationType string
	Confidence   string
	SourceKind   string
	Label        string
	Description  string
	Expression   string
	Metadata     map[string]any
}

func (s *DatasetService) mergePersistedLineage(ctx context.Context, tenantID int64, builder *datasetLineageBuilder, opts DatasetLineageOptions) (DatasetLineageGraph, error) {
	graph := builder.graph()
	if !lineageSourceAllowed(opts, DatasetLineageSourceParser) && !lineageSourceAllowed(opts, DatasetLineageSourceManual) {
		return filterLineageGraph(graph, opts), nil
	}

	nodes, edges, err := s.loadPersistedLineageRows(ctx, tenantID, opts)
	if err != nil {
		return DatasetLineageGraph{}, err
	}
	if len(nodes) == 0 {
		return filterLineageGraph(graph, opts), nil
	}
	nodeByKey := make(map[string]db.DatasetLineageNode, len(nodes))
	for _, node := range nodes {
		nodeByKey[node.NodeKey] = node
	}

	includeAll := opts.ChangeSetPublicID != ""
	included := make(map[string]struct{}, len(builder.nodes))
	for id := range builder.nodes {
		included[id] = struct{}{}
	}
	if includeAll {
		for _, node := range nodes {
			included[node.NodeKey] = struct{}{}
		}
	} else {
		changed := true
		for changed {
			changed = false
			for _, edge := range edges {
				if !lineageSourceAllowed(opts, edge.SourceKind) {
					continue
				}
				_, sourceIncluded := included[edge.SourceNodeKey]
				_, targetIncluded := included[edge.TargetNodeKey]
				if sourceIncluded || targetIncluded {
					if _, ok := included[edge.SourceNodeKey]; !ok {
						included[edge.SourceNodeKey] = struct{}{}
						changed = true
					}
					if _, ok := included[edge.TargetNodeKey]; !ok {
						included[edge.TargetNodeKey] = struct{}{}
						changed = true
					}
				}
			}
		}
	}

	for key := range included {
		node, ok := nodeByKey[key]
		if !ok {
			continue
		}
		if !lineageSourceAllowed(opts, node.SourceKind) {
			continue
		}
		builder.addNode(datasetLineageNodeFromPersisted(node))
	}
	for _, edge := range edges {
		if !lineageSourceAllowed(opts, edge.SourceKind) {
			continue
		}
		if _, ok := included[edge.SourceNodeKey]; !ok {
			continue
		}
		if _, ok := included[edge.TargetNodeKey]; !ok {
			continue
		}
		if _, ok := builder.nodes[edge.SourceNodeKey]; !ok {
			continue
		}
		if _, ok := builder.nodes[edge.TargetNodeKey]; !ok {
			continue
		}
		builder.addPersistedEdge(datasetLineageEdgeFromPersisted(edge))
	}
	return filterLineageGraph(builder.graph(), opts), nil
}

func (s *DatasetService) loadPersistedLineageRows(ctx context.Context, tenantID int64, opts DatasetLineageOptions) ([]db.DatasetLineageNode, []db.DatasetLineageEdge, error) {
	if opts.ChangeSetPublicID != "" {
		changeSet, err := s.getLineageChangeSetRow(ctx, tenantID, opts.ChangeSetPublicID)
		if err != nil {
			return nil, nil, err
		}
		nodes, err := s.queries.ListDatasetLineageNodesForChangeSet(ctx, db.ListDatasetLineageNodesForChangeSetParams{TenantID: tenantID, ChangeSetID: changeSet.ID})
		if err != nil {
			return nil, nil, fmt.Errorf("list lineage change set nodes: %w", err)
		}
		edges, err := s.queries.ListDatasetLineageEdgesForChangeSet(ctx, db.ListDatasetLineageEdgesForChangeSetParams{TenantID: tenantID, ChangeSetID: changeSet.ID})
		if err != nil {
			return nil, nil, fmt.Errorf("list lineage change set edges: %w", err)
		}
		return nodes, edges, nil
	}
	statuses := []string{"published"}
	if opts.IncludeDraft {
		statuses = append(statuses, "draft")
	}
	nodes, err := s.queries.ListDatasetLineageNodesForChangeSets(ctx, db.ListDatasetLineageNodesForChangeSetsParams{TenantID: tenantID, Column2: statuses, Limit: datasetLineageLoadLimit})
	if err != nil {
		return nil, nil, fmt.Errorf("list persisted lineage nodes: %w", err)
	}
	edges, err := s.queries.ListDatasetLineageEdgesForChangeSets(ctx, db.ListDatasetLineageEdgesForChangeSetsParams{TenantID: tenantID, Column2: statuses, Limit: datasetLineageLoadLimit})
	if err != nil {
		return nil, nil, fmt.Errorf("list persisted lineage edges: %w", err)
	}
	return nodes, edges, nil
}

func (s *DatasetService) ParseQueryJobLineage(ctx context.Context, tenantID, userID int64, queryJobPublicID string, auditCtx AuditContext) (DatasetLineageParseRun, DatasetLineageChangeSetWithGraph, error) {
	if s == nil || s.queries == nil {
		return DatasetLineageParseRun{}, DatasetLineageChangeSetWithGraph{}, fmt.Errorf("dataset service is not configured")
	}
	queryID, err := uuid.Parse(strings.TrimSpace(queryJobPublicID))
	if err != nil {
		return DatasetLineageParseRun{}, DatasetLineageChangeSetWithGraph{}, ErrDatasetQueryNotFound
	}
	query, err := s.queries.GetDatasetQueryJobForTenant(ctx, db.GetDatasetQueryJobForTenantParams{PublicID: queryID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetLineageParseRun{}, DatasetLineageChangeSetWithGraph{}, ErrDatasetQueryNotFound
	}
	if err != nil {
		return DatasetLineageParseRun{}, DatasetLineageChangeSetWithGraph{}, fmt.Errorf("get query job for lineage parse: %w", err)
	}
	run, err := s.queries.CreateDatasetLineageParseRun(ctx, db.CreateDatasetLineageParseRunParams{
		TenantID:          tenantID,
		QueryJobID:        query.ID,
		RequestedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
	})
	if err != nil {
		return DatasetLineageParseRun{}, DatasetLineageChangeSetWithGraph{}, fmt.Errorf("create lineage parse run: %w", err)
	}
	parsed, parseErr := s.parseQueryJobLineage(ctx, tenantID, query)
	if parseErr != nil {
		failed, failErr := s.queries.FailDatasetLineageParseRun(ctx, db.FailDatasetLineageParseRunParams{ID: run.ID, TenantID: tenantID, Left: parseErr.Error()})
		if failErr != nil {
			return DatasetLineageParseRun{}, DatasetLineageChangeSetWithGraph{}, fmt.Errorf("fail lineage parse run: %w", failErr)
		}
		return datasetLineageParseRunFromDB(failed, query.PublicID.String(), ""), DatasetLineageChangeSetWithGraph{}, nil
	}

	changeSet, err := s.CreateLineageChangeSet(ctx, tenantID, userID, DatasetLineageChangeSetInput{
		RootResourceType:     DatasetLineageResourceQueryJob,
		RootResourcePublicID: query.PublicID.String(),
		SourceKind:           DatasetLineageSourceParser,
		Title:                "Parsed SQL lineage",
		Description:          queryStatementPreview(query.Statement),
	}, auditCtx)
	if err != nil {
		return DatasetLineageParseRun{}, DatasetLineageChangeSetWithGraph{}, err
	}
	graph, err := s.SaveLineageChangeSetGraph(ctx, tenantID, changeSet.PublicID, parsed, auditCtx)
	if err != nil {
		return DatasetLineageParseRun{}, DatasetLineageChangeSetWithGraph{}, err
	}
	completed, err := s.queries.CompleteDatasetLineageParseRun(ctx, db.CompleteDatasetLineageParseRunParams{
		ID:              run.ID,
		TenantID:        tenantID,
		ChangeSetID:     pgtype.Int8{Int64: changeSet.ID, Valid: true},
		TableRefCount:   int32(len(parsedTableNodes(parsed.Nodes))),
		ColumnEdgeCount: int32(countColumnEdges(parsed.Edges)),
	})
	if err != nil {
		return DatasetLineageParseRun{}, DatasetLineageChangeSetWithGraph{}, fmt.Errorf("complete lineage parse run: %w", err)
	}
	s.recordDatasetLineageAudit(ctx, auditCtx, "dataset_lineage.parse", "dataset_query_job", query.PublicID.String(), map[string]any{
		"changeSetPublicId": graph.ChangeSet.PublicID,
		"tableRefCount":     len(parsedTableNodes(parsed.Nodes)),
		"columnEdgeCount":   countColumnEdges(parsed.Edges),
	})
	return datasetLineageParseRunFromDB(completed, query.PublicID.String(), graph.ChangeSet.PublicID), graph, nil
}

func (s *DatasetService) ListLineageParseRuns(ctx context.Context, tenantID int64, queryJobPublicID string, limit int32) ([]DatasetLineageParseRun, error) {
	queryID, err := uuid.Parse(strings.TrimSpace(queryJobPublicID))
	if err != nil {
		return nil, ErrDatasetQueryNotFound
	}
	query, err := s.queries.GetDatasetQueryJobForTenant(ctx, db.GetDatasetQueryJobForTenantParams{PublicID: queryID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDatasetQueryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get query job for parse runs: %w", err)
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.queries.ListDatasetLineageParseRuns(ctx, db.ListDatasetLineageParseRunsParams{TenantID: tenantID, QueryJobID: query.ID, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("list lineage parse runs: %w", err)
	}
	items := make([]DatasetLineageParseRun, 0, len(rows))
	for _, row := range rows {
		changeSetPublicID := ""
		if row.ChangeSetID.Valid {
			if cs, ok, _ := s.getLineageChangeSetByID(ctx, tenantID, row.ChangeSetID.Int64); ok {
				changeSetPublicID = cs.PublicID.String()
			}
		}
		items = append(items, datasetLineageParseRunFromDB(row, query.PublicID.String(), changeSetPublicID))
	}
	return items, nil
}

func (s *DatasetService) CreateLineageChangeSet(ctx context.Context, tenantID, userID int64, input DatasetLineageChangeSetInput, auditCtx AuditContext) (DatasetLineageChangeSet, error) {
	normalized, err := normalizeDatasetLineageChangeSetInput(input)
	if err != nil {
		return DatasetLineageChangeSet{}, err
	}
	if normalized.RootResourcePublicID != "" {
		if err := s.validateLineageResource(ctx, tenantID, normalized.RootResourceType, normalized.RootResourcePublicID); err != nil {
			return DatasetLineageChangeSet{}, err
		}
	}
	var queryJobID pgtype.Int8
	if normalized.RootResourceType == DatasetLineageResourceQueryJob && normalized.RootResourcePublicID != "" {
		parsed, _ := uuid.Parse(normalized.RootResourcePublicID)
		query, err := s.queries.GetDatasetQueryJobForTenant(ctx, db.GetDatasetQueryJobForTenantParams{PublicID: parsed, TenantID: tenantID})
		if err == nil {
			queryJobID = pgtype.Int8{Int64: query.ID, Valid: true}
		}
	}
	row, err := s.queries.CreateDatasetLineageChangeSet(ctx, db.CreateDatasetLineageChangeSetParams{
		TenantID:             tenantID,
		QueryJobID:           queryJobID,
		RootResourceType:     normalized.RootResourceType,
		RootResourcePublicID: pgUUID(normalized.RootResourcePublicID),
		SourceKind:           normalized.SourceKind,
		Title:                normalized.Title,
		Description:          normalized.Description,
		CreatedByUserID:      pgtype.Int8{Int64: userID, Valid: userID > 0},
	})
	if err != nil {
		return DatasetLineageChangeSet{}, fmt.Errorf("create lineage change set: %w", err)
	}
	out := datasetLineageChangeSetFromDB(row, "")
	s.recordDatasetLineageAudit(ctx, auditCtx, "dataset_lineage.change_set.create", "dataset_lineage_change_set", out.PublicID, map[string]any{"sourceKind": out.SourceKind})
	return out, nil
}

func (s *DatasetService) ListLineageChangeSets(ctx context.Context, tenantID int64, status string, limit int32) ([]DatasetLineageChangeSet, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" {
		switch status {
		case "draft", "published", "rejected", "archived":
		default:
			return nil, fmt.Errorf("%w: unsupported lineage change set status", ErrInvalidDatasetInput)
		}
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.queries.ListDatasetLineageChangeSets(ctx, db.ListDatasetLineageChangeSetsParams{TenantID: tenantID, Column2: status, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("list lineage change sets: %w", err)
	}
	items := make([]DatasetLineageChangeSet, 0, len(rows))
	for _, row := range rows {
		items = append(items, datasetLineageChangeSetFromDB(row, ""))
	}
	return items, nil
}

func (s *DatasetService) GetLineageChangeSet(ctx context.Context, tenantID int64, publicID string) (DatasetLineageChangeSetWithGraph, error) {
	row, err := s.getLineageChangeSetRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetLineageChangeSetWithGraph{}, err
	}
	nodes, err := s.queries.ListDatasetLineageNodesForChangeSet(ctx, db.ListDatasetLineageNodesForChangeSetParams{TenantID: tenantID, ChangeSetID: row.ID})
	if err != nil {
		return DatasetLineageChangeSetWithGraph{}, fmt.Errorf("list lineage change set nodes: %w", err)
	}
	edges, err := s.queries.ListDatasetLineageEdgesForChangeSet(ctx, db.ListDatasetLineageEdgesForChangeSetParams{TenantID: tenantID, ChangeSetID: row.ID})
	if err != nil {
		return DatasetLineageChangeSetWithGraph{}, fmt.Errorf("list lineage change set edges: %w", err)
	}
	return DatasetLineageChangeSetWithGraph{
		ChangeSet: datasetLineageChangeSetFromDB(row, ""),
		Nodes:     datasetLineageNodesFromDB(nodes),
		Edges:     datasetLineageEdgesFromDB(edges),
	}, nil
}

func (s *DatasetService) SaveLineageChangeSetGraph(ctx context.Context, tenantID int64, changeSetPublicID string, input DatasetLineageGraphInput, auditCtx AuditContext) (DatasetLineageChangeSetWithGraph, error) {
	changeSet, err := s.getLineageChangeSetRow(ctx, tenantID, changeSetPublicID)
	if err != nil {
		return DatasetLineageChangeSetWithGraph{}, err
	}
	if changeSet.Status != "draft" {
		return DatasetLineageChangeSetWithGraph{}, fmt.Errorf("%w: only draft lineage change sets can be edited", ErrInvalidDatasetInput)
	}
	nodes, edges, err := normalizeLineageGraphInput(input, changeSet.SourceKind)
	if err != nil {
		return DatasetLineageChangeSetWithGraph{}, err
	}
	if err := s.validateLineageGraph(ctx, tenantID, nodes, edges); err != nil {
		return DatasetLineageChangeSetWithGraph{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DatasetLineageChangeSetWithGraph{}, fmt.Errorf("begin lineage graph save transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.queries.WithTx(tx)
	if err := qtx.DeleteDatasetLineageEdgesByChangeSet(ctx, changeSet.ID); err != nil {
		return DatasetLineageChangeSetWithGraph{}, fmt.Errorf("delete lineage edges: %w", err)
	}
	if err := qtx.DeleteDatasetLineageNodesByChangeSet(ctx, changeSet.ID); err != nil {
		return DatasetLineageChangeSetWithGraph{}, fmt.Errorf("delete lineage nodes: %w", err)
	}
	for _, node := range nodes {
		if _, err := qtx.CreateDatasetLineageNode(ctx, createDatasetLineageNodeParams(tenantID, changeSet.ID, node)); err != nil {
			return DatasetLineageChangeSetWithGraph{}, fmt.Errorf("create lineage node: %w", err)
		}
	}
	for _, edge := range edges {
		if _, err := qtx.CreateDatasetLineageEdge(ctx, createDatasetLineageEdgeParams(tenantID, changeSet.ID, edge)); err != nil {
			return DatasetLineageChangeSetWithGraph{}, fmt.Errorf("create lineage edge: %w", err)
		}
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_lineage.change_set.save",
			TargetType:   "dataset_lineage_change_set",
			TargetID:     changeSet.PublicID.String(),
			Metadata:     map[string]any{"nodeCount": len(nodes), "edgeCount": len(edges)},
		}); err != nil {
			return DatasetLineageChangeSetWithGraph{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetLineageChangeSetWithGraph{}, fmt.Errorf("commit lineage graph save transaction: %w", err)
	}
	return s.GetLineageChangeSet(ctx, tenantID, changeSetPublicID)
}

func (s *DatasetService) PublishLineageChangeSet(ctx context.Context, tenantID, userID int64, publicID string, auditCtx AuditContext) (DatasetLineageChangeSet, error) {
	changeSet, err := s.getLineageChangeSetRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetLineageChangeSet{}, err
	}
	nodes, err := s.queries.ListDatasetLineageNodesForChangeSet(ctx, db.ListDatasetLineageNodesForChangeSetParams{TenantID: tenantID, ChangeSetID: changeSet.ID})
	if err != nil {
		return DatasetLineageChangeSet{}, fmt.Errorf("list lineage nodes before publish: %w", err)
	}
	edges, err := s.queries.ListDatasetLineageEdgesForChangeSet(ctx, db.ListDatasetLineageEdgesForChangeSetParams{TenantID: tenantID, ChangeSetID: changeSet.ID})
	if err != nil {
		return DatasetLineageChangeSet{}, fmt.Errorf("list lineage edges before publish: %w", err)
	}
	if err := ensureLineageAcyclic(lineageNodeInputsFromNodes(datasetLineageNodesFromDB(nodes)), lineageEdgeInputsFromEdges(datasetLineageEdgesFromDB(edges))); err != nil {
		return DatasetLineageChangeSet{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DatasetLineageChangeSet{}, fmt.Errorf("begin lineage publish transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.queries.WithTx(tx)
	if err := qtx.ArchivePublishedDatasetLineageChangeSetsForRoot(ctx, db.ArchivePublishedDatasetLineageChangeSetsForRootParams{
		TenantID:             tenantID,
		SourceKind:           changeSet.SourceKind,
		RootResourceType:     changeSet.RootResourceType,
		RootResourcePublicID: changeSet.RootResourcePublicID,
	}); err != nil {
		return DatasetLineageChangeSet{}, fmt.Errorf("archive previous lineage change sets: %w", err)
	}
	published, err := qtx.PublishDatasetLineageChangeSet(ctx, db.PublishDatasetLineageChangeSetParams{
		PublicID:          changeSet.PublicID,
		TenantID:          tenantID,
		PublishedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetLineageChangeSet{}, fmt.Errorf("%w: lineage change set is not draft", ErrInvalidDatasetInput)
		}
		return DatasetLineageChangeSet{}, fmt.Errorf("publish lineage change set: %w", err)
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_lineage.change_set.publish",
			TargetType:   "dataset_lineage_change_set",
			TargetID:     published.PublicID.String(),
			Metadata:     map[string]any{"sourceKind": published.SourceKind},
		}); err != nil {
			return DatasetLineageChangeSet{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetLineageChangeSet{}, fmt.Errorf("commit lineage publish transaction: %w", err)
	}
	return datasetLineageChangeSetFromDB(published, ""), nil
}

func (s *DatasetService) RejectLineageChangeSet(ctx context.Context, tenantID, userID int64, publicID string, auditCtx AuditContext) (DatasetLineageChangeSet, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return DatasetLineageChangeSet{}, ErrInvalidDatasetInput
	}
	row, err := s.queries.RejectDatasetLineageChangeSet(ctx, db.RejectDatasetLineageChangeSetParams{
		PublicID:         parsed,
		TenantID:         tenantID,
		RejectedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetLineageChangeSet{}, ErrInvalidDatasetInput
	}
	if err != nil {
		return DatasetLineageChangeSet{}, fmt.Errorf("reject lineage change set: %w", err)
	}
	out := datasetLineageChangeSetFromDB(row, "")
	s.recordDatasetLineageAudit(ctx, auditCtx, "dataset_lineage.change_set.reject", "dataset_lineage_change_set", out.PublicID, map[string]any{"sourceKind": out.SourceKind})
	return out, nil
}

func (s *DatasetService) parseQueryJobLineage(ctx context.Context, tenantID int64, query db.DatasetQueryJob) (DatasetLineageGraphInput, error) {
	normalized, err := validateDatasetSQL(tenantID, query.Statement)
	if err != nil {
		return DatasetLineageGraphInput{}, err
	}
	stmts, err := chparser.NewParser(normalized).ParseStmts()
	if err != nil {
		return DatasetLineageGraphInput{}, fmt.Errorf("%w: SQL parser failed", ErrInvalidDatasetInput)
	}
	if len(stmts) != 1 {
		return DatasetLineageGraphInput{}, fmt.Errorf("%w: expected exactly one SQL statement", ErrInvalidDatasetInput)
	}
	selectQuery, ok := findSelectQuery(stmts[0])
	if !ok {
		return DatasetLineageGraphInput{}, fmt.Errorf("%w: only SELECT lineage parsing is supported", ErrInvalidDatasetInput)
	}

	nodes := []DatasetLineageNodeInput{queryJobLineageNodeInput(query)}
	edges := []DatasetLineageEdgeInput{}
	nodeKeys := map[string]struct{}{lineageNodeID(DatasetLineageResourceQueryJob, query.PublicID.String()): {}}
	tableRefs := extractLineageTableRefs(selectQuery)
	aliasToResource := map[string]DatasetLineageNodeInput{}
	resolvedTables := []DatasetLineageNodeInput{}
	for _, ref := range tableRefs {
		resource, resolved := s.resolveLineageTableRef(ctx, tenantID, ref)
		if !resolved {
			unresolved := DatasetLineageNodeInput{
				ID:           "custom:unresolved:" + sanitizeLineageKey(ref.Display()),
				ResourceType: DatasetLineageResourceCustom,
				DisplayName:  ref.Display(),
				NodeKind:     DatasetLineageNodeKindCustom,
				SourceKind:   DatasetLineageSourceParser,
				Description:  "Unresolved SQL table reference",
				Metadata:     map[string]any{"unresolved": true},
			}
			if _, ok := nodeKeys[unresolved.ID]; !ok {
				nodes = append(nodes, unresolved)
				nodeKeys[unresolved.ID] = struct{}{}
			}
			resource = unresolved
		} else if _, ok := nodeKeys[resource.ID]; !ok {
			nodes = append(nodes, resource)
			nodeKeys[resource.ID] = struct{}{}
		}
		resolvedTables = append(resolvedTables, resource)
		for _, alias := range ref.Aliases() {
			aliasToResource[strings.ToLower(alias)] = resource
		}
		edges = append(edges, DatasetLineageEdgeInput{
			ID:           edgeInputKey(resource.ID, lineageNodeID(DatasetLineageResourceQueryJob, query.PublicID.String()), DatasetLineageRelationQueryInput),
			SourceNodeID: resource.ID,
			TargetNodeID: lineageNodeID(DatasetLineageResourceQueryJob, query.PublicID.String()),
			RelationType: DatasetLineageRelationQueryInput,
			SourceKind:   DatasetLineageSourceParser,
			Confidence:   datasetLineageConfidenceParserPartial,
		})
	}

	outputResource := queryJobLineageNodeInput(query)
	if workTables, err := s.queries.ListLineageDatasetWorkTablesByQueryJob(ctx, db.ListLineageDatasetWorkTablesByQueryJobParams{TenantID: tenantID, CreatedFromQueryJobID: pgtype.Int8{Int64: query.ID, Valid: true}, Limit: 1}); err == nil && len(workTables) > 0 {
		workTable := datasetWorkTableFromDB(workTables[0])
		outputResource = DatasetLineageNodeInput{
			ID:           lineageNodeID(DatasetLineageResourceWorkTable, workTable.PublicID),
			ResourceType: DatasetLineageResourceWorkTable,
			PublicID:     workTable.PublicID,
			DisplayName:  firstNonEmpty(workTable.DisplayName, workTable.Table),
			NodeKind:     DatasetLineageNodeKindResource,
			SourceKind:   DatasetLineageSourceParser,
		}
		if _, ok := nodeKeys[outputResource.ID]; !ok {
			nodes = append(nodes, outputResource)
			nodeKeys[outputResource.ID] = struct{}{}
		}
	}

	outputColumns := decodeStringSlice(query.ResultColumns)
	for idx, column := range outputColumns {
		outputID := columnNodeID(outputResource.ID, column)
		outputNode := DatasetLineageNodeInput{
			ID:           outputID,
			ResourceType: outputResource.ResourceType,
			PublicID:     outputResource.PublicID,
			DisplayName:  column,
			NodeKind:     DatasetLineageNodeKindColumn,
			SourceKind:   DatasetLineageSourceParser,
			ColumnName:   column,
		}
		if _, ok := nodeKeys[outputID]; !ok {
			nodes = append(nodes, outputNode)
			nodeKeys[outputID] = struct{}{}
		}
		var refs []lineageColumnRef
		if idx < len(selectQuery.SelectItems) {
			refs = extractLineageColumnRefs(selectQuery.SelectItems[idx].Expr)
		}
		if len(refs) == 0 && idx < len(selectQuery.SelectItems) && strings.TrimSpace(chparser.Format(selectQuery.SelectItems[idx].Expr)) == "*" {
			for _, tableNode := range resolvedTables {
				inputID := columnNodeID(tableNode.ID, column)
				if _, ok := nodeKeys[inputID]; !ok {
					nodes = append(nodes, DatasetLineageNodeInput{ID: inputID, ResourceType: tableNode.ResourceType, PublicID: tableNode.PublicID, DisplayName: column, NodeKind: DatasetLineageNodeKindColumn, SourceKind: DatasetLineageSourceParser, ColumnName: column})
					nodeKeys[inputID] = struct{}{}
				}
				edges = append(edges, columnEdgeInput(inputID, outputID, column, datasetLineageConfidenceParserPartial, "*"))
			}
			continue
		}
		for _, ref := range refs {
			tableNode, exact := resolveColumnRefTable(ref, aliasToResource, resolvedTables)
			inputColumn := firstNonEmpty(ref.Column, column)
			inputID := columnNodeID(tableNode.ID, inputColumn)
			if _, ok := nodeKeys[inputID]; !ok {
				nodes = append(nodes, DatasetLineageNodeInput{ID: inputID, ResourceType: tableNode.ResourceType, PublicID: tableNode.PublicID, DisplayName: inputColumn, NodeKind: DatasetLineageNodeKindColumn, SourceKind: DatasetLineageSourceParser, ColumnName: inputColumn})
				nodeKeys[inputID] = struct{}{}
			}
			confidence := datasetLineageConfidenceParserExact
			if !exact || len(refs) != 1 {
				confidence = datasetLineageConfidenceParserPartial
			}
			expression := ""
			if idx < len(selectQuery.SelectItems) {
				expression = chparser.Format(selectQuery.SelectItems[idx].Expr)
			}
			edges = append(edges, columnEdgeInput(inputID, outputID, column, confidence, expression))
		}
	}
	return DatasetLineageGraphInput{Nodes: nodes, Edges: dedupeLineageEdgeInputs(edges)}, nil
}

func (s *DatasetService) resolveLineageTableRef(ctx context.Context, tenantID int64, ref lineageTableRef) (DatasetLineageNodeInput, bool) {
	database := ref.Database
	if database == "" {
		database = datasetWorkDatabaseName(tenantID)
	}
	if database == datasetWorkDatabaseName(tenantID) {
		row, err := s.queries.GetActiveDatasetWorkTableByRefForTenant(ctx, db.GetActiveDatasetWorkTableByRefForTenantParams{TenantID: tenantID, WorkDatabase: database, WorkTable: ref.Table})
		if err == nil {
			workTable := datasetWorkTableFromDB(row)
			return DatasetLineageNodeInput{ID: lineageNodeID(DatasetLineageResourceWorkTable, workTable.PublicID), ResourceType: DatasetLineageResourceWorkTable, PublicID: workTable.PublicID, DisplayName: firstNonEmpty(workTable.DisplayName, workTable.Table), NodeKind: DatasetLineageNodeKindResource, SourceKind: DatasetLineageSourceParser}, true
		}
	}
	if database == datasetRawDatabaseName(tenantID) {
		row, err := s.queries.GetDatasetByRawRefForTenant(ctx, db.GetDatasetByRawRefForTenantParams{TenantID: tenantID, RawDatabase: database, RawTable: ref.Table})
		if err == nil {
			dataset := datasetFromDB(row)
			return DatasetLineageNodeInput{ID: lineageNodeID(DatasetLineageResourceDataset, dataset.PublicID), ResourceType: DatasetLineageResourceDataset, PublicID: dataset.PublicID, DisplayName: dataset.Name, NodeKind: DatasetLineageNodeKindResource, SourceKind: DatasetLineageSourceParser}, true
		}
	}
	return DatasetLineageNodeInput{}, false
}

func normalizeDatasetLineageChangeSetInput(input DatasetLineageChangeSetInput) (DatasetLineageChangeSetInput, error) {
	input.RootResourceType = strings.TrimSpace(input.RootResourceType)
	if input.RootResourceType == "" {
		input.RootResourceType = DatasetLineageResourceCustom
	}
	input.RootResourcePublicID = strings.TrimSpace(input.RootResourcePublicID)
	input.SourceKind = strings.ToLower(strings.TrimSpace(input.SourceKind))
	if input.SourceKind == "" {
		input.SourceKind = DatasetLineageSourceManual
	}
	switch input.SourceKind {
	case DatasetLineageSourceParser, DatasetLineageSourceManual:
	default:
		return input, fmt.Errorf("%w: unsupported lineage change set source", ErrInvalidDatasetInput)
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		input.Title = "Lineage draft"
	}
	if len(input.Title) > 160 {
		input.Title = input.Title[:160]
	}
	input.Description = strings.TrimSpace(input.Description)
	if len(input.Description) > 2000 {
		input.Description = input.Description[:2000]
	}
	if input.RootResourcePublicID != "" {
		if _, err := uuid.Parse(input.RootResourcePublicID); err != nil {
			return input, ErrInvalidDatasetInput
		}
	}
	return input, nil
}

func normalizeLineageGraphInput(input DatasetLineageGraphInput, sourceKind string) ([]DatasetLineageNodeInput, []DatasetLineageEdgeInput, error) {
	if len(input.Nodes) > datasetLineageMaxPersistedNodes || len(input.Edges) > datasetLineageMaxPersistedEdges {
		return nil, nil, fmt.Errorf("%w: lineage graph is too large", ErrInvalidDatasetInput)
	}
	nodes := make([]DatasetLineageNodeInput, 0, len(input.Nodes))
	seenNodes := map[string]struct{}{}
	for _, node := range input.Nodes {
		node.ID = strings.TrimSpace(node.ID)
		if node.ID == "" {
			return nil, nil, fmt.Errorf("%w: lineage node id is required", ErrInvalidDatasetInput)
		}
		if _, ok := seenNodes[node.ID]; ok {
			return nil, nil, fmt.Errorf("%w: duplicate lineage node id", ErrInvalidDatasetInput)
		}
		seenNodes[node.ID] = struct{}{}
		node.NodeKind = strings.ToLower(strings.TrimSpace(node.NodeKind))
		if node.NodeKind == "" {
			node.NodeKind = DatasetLineageNodeKindCustom
		}
		switch node.NodeKind {
		case DatasetLineageNodeKindResource, DatasetLineageNodeKindColumn, DatasetLineageNodeKindCustom:
		default:
			return nil, nil, fmt.Errorf("%w: unsupported lineage node kind", ErrInvalidDatasetInput)
		}
		node.SourceKind = strings.ToLower(strings.TrimSpace(node.SourceKind))
		if node.SourceKind == "" {
			node.SourceKind = sourceKind
		}
		if node.SourceKind != DatasetLineageSourceParser && node.SourceKind != DatasetLineageSourceManual {
			return nil, nil, fmt.Errorf("%w: persisted lineage node source must be parser or manual", ErrInvalidDatasetInput)
		}
		node.ResourceType = strings.TrimSpace(node.ResourceType)
		if node.ResourceType == "" {
			node.ResourceType = DatasetLineageResourceCustom
		}
		node.PublicID = strings.TrimSpace(node.PublicID)
		node.DisplayName = strings.TrimSpace(node.DisplayName)
		if node.DisplayName == "" {
			node.DisplayName = node.ID
		}
		node.Description = strings.TrimSpace(node.Description)
		node.ColumnName = strings.TrimSpace(node.ColumnName)
		nodes = append(nodes, node)
	}
	edges := make([]DatasetLineageEdgeInput, 0, len(input.Edges))
	seenEdges := map[string]struct{}{}
	for _, edge := range input.Edges {
		edge.SourceNodeID = strings.TrimSpace(edge.SourceNodeID)
		edge.TargetNodeID = strings.TrimSpace(edge.TargetNodeID)
		if edge.SourceNodeID == "" || edge.TargetNodeID == "" || edge.SourceNodeID == edge.TargetNodeID {
			return nil, nil, fmt.Errorf("%w: invalid lineage edge endpoints", ErrInvalidDatasetInput)
		}
		if _, ok := seenNodes[edge.SourceNodeID]; !ok {
			return nil, nil, fmt.Errorf("%w: lineage edge source node is missing", ErrInvalidDatasetInput)
		}
		if _, ok := seenNodes[edge.TargetNodeID]; !ok {
			return nil, nil, fmt.Errorf("%w: lineage edge target node is missing", ErrInvalidDatasetInput)
		}
		edge.RelationType = strings.TrimSpace(edge.RelationType)
		if edge.RelationType == "" {
			edge.RelationType = DatasetLineageRelationManualDependency
		}
		edge.SourceKind = strings.ToLower(strings.TrimSpace(edge.SourceKind))
		if edge.SourceKind == "" {
			edge.SourceKind = sourceKind
		}
		edge.Confidence = strings.ToLower(strings.TrimSpace(edge.Confidence))
		if edge.Confidence == "" {
			edge.Confidence = datasetLineageConfidenceManual
		}
		if edge.SourceKind == DatasetLineageSourceParser && edge.Confidence == datasetLineageConfidenceManual {
			edge.Confidence = datasetLineageConfidenceParserPartial
		}
		if edge.ID == "" {
			edge.ID = edgeInputKey(edge.SourceNodeID, edge.TargetNodeID, edge.RelationType)
		}
		edge.ID = strings.TrimSpace(edge.ID)
		if _, ok := seenEdges[edge.ID]; ok {
			return nil, nil, fmt.Errorf("%w: duplicate lineage edge id", ErrInvalidDatasetInput)
		}
		seenEdges[edge.ID] = struct{}{}
		edge.Label = strings.TrimSpace(edge.Label)
		edge.Description = strings.TrimSpace(edge.Description)
		edge.Expression = strings.TrimSpace(edge.Expression)
		edges = append(edges, edge)
	}
	if err := ensureLineageAcyclic(nodes, edges); err != nil {
		return nil, nil, err
	}
	return nodes, edges, nil
}

func (s *DatasetService) validateLineageGraph(ctx context.Context, tenantID int64, nodes []DatasetLineageNodeInput, edges []DatasetLineageEdgeInput) error {
	for _, node := range nodes {
		if node.NodeKind == DatasetLineageNodeKindCustom {
			continue
		}
		if node.PublicID == "" {
			return fmt.Errorf("%w: resource lineage node public id is required", ErrInvalidDatasetInput)
		}
		if err := s.validateLineageResource(ctx, tenantID, node.ResourceType, node.PublicID); err != nil {
			return err
		}
	}
	return ensureLineageAcyclic(nodes, edges)
}

func (s *DatasetService) validateLineageResource(ctx context.Context, tenantID int64, resourceType, publicID string) error {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return ErrInvalidDatasetInput
	}
	switch resourceType {
	case DatasetLineageResourceDataset:
		_, err = s.queries.GetDatasetForTenant(ctx, db.GetDatasetForTenantParams{PublicID: parsed, TenantID: tenantID})
	case DatasetLineageResourceWorkTable:
		_, err = s.queries.GetDatasetWorkTableForTenant(ctx, db.GetDatasetWorkTableForTenantParams{PublicID: parsed, TenantID: tenantID})
	case DatasetLineageResourceQueryJob:
		_, err = s.queries.GetDatasetQueryJobForTenant(ctx, db.GetDatasetQueryJobForTenantParams{PublicID: parsed, TenantID: tenantID})
	default:
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidDatasetInput
	}
	if err != nil {
		return fmt.Errorf("validate lineage resource: %w", err)
	}
	return nil
}

func (s *DatasetService) getLineageChangeSetRow(ctx context.Context, tenantID int64, publicID string) (db.DatasetLineageChangeSet, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DatasetLineageChangeSet{}, ErrInvalidDatasetInput
	}
	row, err := s.queries.GetDatasetLineageChangeSetByPublicID(ctx, db.GetDatasetLineageChangeSetByPublicIDParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DatasetLineageChangeSet{}, ErrInvalidDatasetInput
	}
	if err != nil {
		return db.DatasetLineageChangeSet{}, fmt.Errorf("get lineage change set: %w", err)
	}
	return row, nil
}

func (s *DatasetService) getLineageChangeSetByID(ctx context.Context, tenantID, id int64) (db.DatasetLineageChangeSet, bool, error) {
	rows, err := s.queries.ListDatasetLineageChangeSets(ctx, db.ListDatasetLineageChangeSetsParams{TenantID: tenantID, Column2: "", Limit: 100})
	if err != nil {
		return db.DatasetLineageChangeSet{}, false, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, true, nil
		}
	}
	return db.DatasetLineageChangeSet{}, false, nil
}

func (b *datasetLineageBuilder) addPersistedEdge(edge DatasetLineageEdge) DatasetLineageEdge {
	if existing, ok := b.edges[edge.ID]; ok {
		return existing
	}
	b.edges[edge.ID] = edge
	b.edgeOrder = append(b.edgeOrder, edge.ID)
	return edge
}

func datasetLineageNodeFromPersisted(row db.DatasetLineageNode) DatasetLineageNode {
	node := DatasetLineageNode{
		ID:           row.NodeKey,
		ResourceType: row.ResourceType,
		PublicID:     uuidString(row.ResourcePublicID),
		DisplayName:  row.Label,
		NodeKind:     row.NodeKind,
		SourceKind:   row.SourceKind,
		ColumnName:   optionalText(row.ColumnName),
		Description:  row.Description,
		Editable:     true,
		CreatedAt:    pgTimePtr(row.CreatedAt),
		UpdatedAt:    pgTimePtr(row.UpdatedAt),
		Metadata:     decodeMetadata(row.Metadata),
	}
	if row.PositionX.Valid && row.PositionY.Valid {
		node.Position = &DatasetLineagePosition{X: row.PositionX.Float64, Y: row.PositionY.Float64}
	}
	return node
}

func datasetLineageEdgeFromPersisted(row db.DatasetLineageEdge) DatasetLineageEdge {
	return DatasetLineageEdge{
		ID:           row.EdgeKey,
		SourceNodeID: row.SourceNodeKey,
		TargetNodeID: row.TargetNodeKey,
		RelationType: row.RelationType,
		Confidence:   row.Confidence,
		SourceKind:   row.SourceKind,
		Label:        row.Label,
		Description:  row.Description,
		Expression:   row.Expression,
		Editable:     true,
		CreatedAt:    pgTimePtr(row.CreatedAt),
	}
}

func datasetLineageNodesFromDB(rows []db.DatasetLineageNode) []DatasetLineageNode {
	items := make([]DatasetLineageNode, 0, len(rows))
	for _, row := range rows {
		items = append(items, datasetLineageNodeFromPersisted(row))
	}
	return items
}

func datasetLineageEdgesFromDB(rows []db.DatasetLineageEdge) []DatasetLineageEdge {
	items := make([]DatasetLineageEdge, 0, len(rows))
	for _, row := range rows {
		items = append(items, datasetLineageEdgeFromPersisted(row))
	}
	return items
}

func lineageNodeInputsFromNodes(nodes []DatasetLineageNode) []DatasetLineageNodeInput {
	out := make([]DatasetLineageNodeInput, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, DatasetLineageNodeInput{ID: node.ID})
	}
	return out
}

func lineageEdgeInputsFromEdges(edges []DatasetLineageEdge) []DatasetLineageEdgeInput {
	out := make([]DatasetLineageEdgeInput, 0, len(edges))
	for _, edge := range edges {
		out = append(out, DatasetLineageEdgeInput{SourceNodeID: edge.SourceNodeID, TargetNodeID: edge.TargetNodeID})
	}
	return out
}

func datasetLineageChangeSetFromDB(row db.DatasetLineageChangeSet, queryJobPublicID string) DatasetLineageChangeSet {
	return DatasetLineageChangeSet{
		ID:                   row.ID,
		PublicID:             row.PublicID.String(),
		QueryJobPublicID:     queryJobPublicID,
		RootResourceType:     row.RootResourceType,
		RootResourcePublicID: uuidString(row.RootResourcePublicID),
		SourceKind:           row.SourceKind,
		Status:               row.Status,
		Title:                row.Title,
		Description:          row.Description,
		CreatedAt:            pgTime(row.CreatedAt),
		UpdatedAt:            pgTime(row.UpdatedAt),
		PublishedAt:          pgTimePtr(row.PublishedAt),
		RejectedAt:           pgTimePtr(row.RejectedAt),
		ArchivedAt:           pgTimePtr(row.ArchivedAt),
	}
}

func datasetLineageParseRunFromDB(row db.DatasetLineageParseRun, queryJobPublicID, changeSetPublicID string) DatasetLineageParseRun {
	return DatasetLineageParseRun{
		PublicID:          row.PublicID.String(),
		QueryJobPublicID:  queryJobPublicID,
		ChangeSetPublicID: changeSetPublicID,
		Status:            row.Status,
		TableRefCount:     row.TableRefCount,
		ColumnEdgeCount:   row.ColumnEdgeCount,
		ErrorSummary:      optionalText(row.ErrorSummary),
		CreatedAt:         pgTime(row.CreatedAt),
		CompletedAt:       pgTimePtr(row.CompletedAt),
	}
}

func createDatasetLineageNodeParams(tenantID, changeSetID int64, node DatasetLineageNodeInput) db.CreateDatasetLineageNodeParams {
	return db.CreateDatasetLineageNodeParams{
		TenantID:         tenantID,
		ChangeSetID:      changeSetID,
		NodeKey:          node.ID,
		NodeKind:         node.NodeKind,
		SourceKind:       node.SourceKind,
		ResourceType:     node.ResourceType,
		ResourcePublicID: pgUUID(node.PublicID),
		ParentNodeKey:    pgText(parentNodeKey(node.ID)),
		ColumnName:       pgText(node.ColumnName),
		Label:            node.DisplayName,
		Description:      node.Description,
		PositionX:        pgFloat8(positionX(node.Position)),
		PositionY:        pgFloat8(positionY(node.Position)),
		Metadata:         encodeMetadata(node.Metadata),
	}
}

func createDatasetLineageEdgeParams(tenantID, changeSetID int64, edge DatasetLineageEdgeInput) db.CreateDatasetLineageEdgeParams {
	return db.CreateDatasetLineageEdgeParams{
		TenantID:      tenantID,
		ChangeSetID:   changeSetID,
		EdgeKey:       edge.ID,
		SourceNodeKey: edge.SourceNodeID,
		TargetNodeKey: edge.TargetNodeID,
		RelationType:  edge.RelationType,
		SourceKind:    edge.SourceKind,
		Confidence:    edge.Confidence,
		Label:         edge.Label,
		Description:   edge.Description,
		Expression:    edge.Expression,
		Metadata:      encodeMetadata(edge.Metadata),
	}
}

type lineageTableRef struct {
	Database string
	Table    string
	Alias    string
}

func (r lineageTableRef) Display() string {
	if r.Database != "" {
		return r.Database + "." + r.Table
	}
	return r.Table
}

func (r lineageTableRef) Aliases() []string {
	out := []string{r.Table}
	if r.Alias != "" && !strings.EqualFold(r.Alias, r.Table) {
		out = append(out, r.Alias)
	}
	return out
}

type lineageColumnRef struct {
	Qualifier string
	Column    string
}

func extractLineageTableRefs(query *chparser.SelectQuery) []lineageTableRef {
	cteAliases := map[string]struct{}{}
	if query.With != nil {
		for _, cte := range query.With.CTEs {
			alias := strings.TrimSpace(chparser.Format(cte.Alias))
			if alias != "" {
				cteAliases[strings.ToLower(alias)] = struct{}{}
			}
		}
	}
	refs := []lineageTableRef{}
	seen := map[string]struct{}{}
	chparser.Walk(query, func(expr chparser.Expr) bool {
		tableExpr, ok := expr.(*chparser.TableExpr)
		if !ok {
			return true
		}
		table, alias, ok := tableIdentifierFromExpr(tableExpr.Expr)
		if !ok || table.Table == nil {
			return true
		}
		ref := lineageTableRef{Table: table.Table.Name}
		if table.Database != nil {
			ref.Database = table.Database.Name
		}
		if ref.Database == "" {
			if _, isCTE := cteAliases[strings.ToLower(ref.Table)]; isCTE {
				return true
			}
		}
		if tableExpr.Alias != nil {
			ref.Alias = strings.TrimSpace(chparser.Format(tableExpr.Alias.Alias))
		} else if alias != "" {
			ref.Alias = alias
		}
		key := strings.ToLower(ref.Display() + ":" + ref.Alias)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			refs = append(refs, ref)
		}
		return true
	})
	return refs
}

func tableIdentifierFromExpr(expr chparser.Expr) (*chparser.TableIdentifier, string, bool) {
	switch typed := expr.(type) {
	case *chparser.TableIdentifier:
		return typed, "", true
	case *chparser.AliasExpr:
		table, _, ok := tableIdentifierFromExpr(typed.Expr)
		if !ok {
			return nil, "", false
		}
		return table, strings.TrimSpace(chparser.Format(typed.Alias)), true
	default:
		return nil, "", false
	}
}

func extractLineageColumnRefs(expr chparser.Expr) []lineageColumnRef {
	refs := []lineageColumnRef{}
	seen := map[string]struct{}{}
	var add = func(ref lineageColumnRef) {
		if ref.Column == "" || ref.Column == "*" {
			return
		}
		key := strings.ToLower(ref.Qualifier + "." + ref.Column)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			refs = append(refs, ref)
		}
	}
	var walk func(chparser.Expr)
	walk = func(item chparser.Expr) {
		if item == nil {
			return
		}
		switch v := item.(type) {
		case *chparser.Path:
			if len(v.Fields) == 2 {
				add(lineageColumnRef{Qualifier: v.Fields[0].Name, Column: v.Fields[1].Name})
				return
			}
		case *chparser.Ident:
			add(lineageColumnRef{Column: v.Name})
			return
		case *chparser.FunctionExpr:
			if v.Params != nil && v.Params.Items != nil {
				for _, child := range v.Params.Items.Items {
					walk(child)
				}
			}
			return
		case *chparser.ColumnExpr:
			walk(v.Expr)
			return
		case *chparser.AliasExpr:
			walk(v.Expr)
			return
		case *chparser.BinaryOperation:
			walk(v.LeftExpr)
			walk(v.RightExpr)
			return
		case *chparser.ColumnExprList:
			for _, child := range v.Items {
				walk(child)
			}
			return
		}
		chparser.Walk(item, func(child chparser.Expr) bool {
			if child == item {
				return true
			}
			switch child.(type) {
			case *chparser.FunctionExpr:
				walk(child)
				return false
			case *chparser.Path, *chparser.Ident:
				walk(child)
				return false
			default:
				return true
			}
		})
	}
	walk(expr)
	return refs
}

func findSelectQuery(expr chparser.Expr) (*chparser.SelectQuery, bool) {
	switch typed := expr.(type) {
	case *chparser.SelectQuery:
		return typed, true
	case *chparser.SubQuery:
		return typed.Select, typed.Select != nil
	default:
		var found *chparser.SelectQuery
		chparser.WalkWithBreak(expr, func(item chparser.Expr) bool {
			if query, ok := item.(*chparser.SelectQuery); ok {
				found = query
				return false
			}
			return true
		})
		return found, found != nil
	}
}

func resolveColumnRefTable(ref lineageColumnRef, aliases map[string]DatasetLineageNodeInput, tables []DatasetLineageNodeInput) (DatasetLineageNodeInput, bool) {
	if ref.Qualifier != "" {
		if table, ok := aliases[strings.ToLower(ref.Qualifier)]; ok {
			return table, true
		}
		return DatasetLineageNodeInput{ID: "custom:unresolved_column_source", ResourceType: DatasetLineageResourceCustom, DisplayName: "Unresolved column source", NodeKind: DatasetLineageNodeKindCustom, SourceKind: DatasetLineageSourceParser}, false
	}
	if len(tables) == 1 {
		return tables[0], true
	}
	if len(tables) > 0 {
		return tables[0], false
	}
	return DatasetLineageNodeInput{ID: "custom:unresolved_column_source", ResourceType: DatasetLineageResourceCustom, DisplayName: "Unresolved column source", NodeKind: DatasetLineageNodeKindCustom, SourceKind: DatasetLineageSourceParser}, false
}

func queryJobLineageNodeInput(query db.DatasetQueryJob) DatasetLineageNodeInput {
	return DatasetLineageNodeInput{
		ID:           lineageNodeID(DatasetLineageResourceQueryJob, query.PublicID.String()),
		ResourceType: DatasetLineageResourceQueryJob,
		PublicID:     query.PublicID.String(),
		DisplayName:  queryStatementPreview(query.Statement),
		NodeKind:     DatasetLineageNodeKindResource,
		SourceKind:   DatasetLineageSourceParser,
	}
}

func columnNodeID(parentNodeID, column string) string {
	return parentNodeID + ":column:" + sanitizeLineageKey(column)
}

func columnEdgeInput(sourceID, targetID, outputColumn, confidence, expression string) DatasetLineageEdgeInput {
	return DatasetLineageEdgeInput{
		ID:           edgeInputKey(sourceID, targetID, DatasetLineageRelationColumnDerives),
		SourceNodeID: sourceID,
		TargetNodeID: targetID,
		RelationType: DatasetLineageRelationColumnDerives,
		SourceKind:   DatasetLineageSourceParser,
		Confidence:   confidence,
		Expression:   expression,
		Metadata:     map[string]any{"outputColumn": outputColumn},
	}
}

func edgeInputKey(sourceID, targetID, relationType string) string {
	return relationType + ":" + sourceID + "->" + targetID
}

func sanitizeLineageKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unnamed"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == ':' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func parsedTableNodes(nodes []DatasetLineageNodeInput) []DatasetLineageNodeInput {
	out := []DatasetLineageNodeInput{}
	for _, node := range nodes {
		if node.NodeKind == DatasetLineageNodeKindResource || (node.NodeKind == DatasetLineageNodeKindCustom && node.ColumnName == "") {
			out = append(out, node)
		}
	}
	return out
}

func countColumnEdges(edges []DatasetLineageEdgeInput) int {
	count := 0
	for _, edge := range edges {
		if edge.RelationType == DatasetLineageRelationColumnDerives {
			count++
		}
	}
	return count
}

func dedupeLineageEdgeInputs(edges []DatasetLineageEdgeInput) []DatasetLineageEdgeInput {
	out := make([]DatasetLineageEdgeInput, 0, len(edges))
	seen := map[string]struct{}{}
	for _, edge := range edges {
		if _, ok := seen[edge.ID]; ok {
			continue
		}
		seen[edge.ID] = struct{}{}
		out = append(out, edge)
	}
	return out
}

func ensureLineageAcyclic(nodes []DatasetLineageNodeInput, edges []DatasetLineageEdgeInput) error {
	known := map[string]struct{}{}
	for _, node := range nodes {
		known[node.ID] = struct{}{}
	}
	graph := map[string][]string{}
	for _, edge := range edges {
		if _, ok := known[edge.SourceNodeID]; !ok {
			continue
		}
		if _, ok := known[edge.TargetNodeID]; !ok {
			continue
		}
		graph[edge.SourceNodeID] = append(graph[edge.SourceNodeID], edge.TargetNodeID)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(node string) bool {
		if visiting[node] {
			return false
		}
		if visited[node] {
			return true
		}
		visiting[node] = true
		for _, next := range graph[node] {
			if !visit(next) {
				return false
			}
		}
		visiting[node] = false
		visited[node] = true
		return true
	}
	keys := make([]string, 0, len(known))
	for key := range known {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !visit(key) {
			return fmt.Errorf("%w: lineage graph cannot contain cycles", ErrInvalidDatasetInput)
		}
	}
	return nil
}

func filterLineageGraph(graph DatasetLineageGraph, opts DatasetLineageOptions) DatasetLineageGraph {
	if !lineageSourceAllowed(opts, DatasetLineageSourceMetadata) {
		filteredEdges := graph.Edges[:0]
		for _, edge := range graph.Edges {
			if edge.SourceKind != DatasetLineageSourceMetadata {
				filteredEdges = append(filteredEdges, edge)
			}
		}
		graph.Edges = filteredEdges
	}
	nodes := graph.Nodes[:0]
	for _, node := range graph.Nodes {
		if !lineageSourceAllowed(opts, node.SourceKind) {
			continue
		}
		if opts.Level == DatasetLineageLevelTable && node.NodeKind == DatasetLineageNodeKindColumn {
			continue
		}
		nodes = append(nodes, node)
	}
	known := map[string]struct{}{}
	for _, node := range nodes {
		known[node.ID] = struct{}{}
	}
	edges := graph.Edges[:0]
	for _, edge := range graph.Edges {
		if !lineageSourceAllowed(opts, edge.SourceKind) {
			continue
		}
		if opts.Level == DatasetLineageLevelTable && edge.RelationType == DatasetLineageRelationColumnDerives {
			continue
		}
		if _, ok := known[edge.SourceNodeID]; !ok {
			continue
		}
		if _, ok := known[edge.TargetNodeID]; !ok {
			continue
		}
		edges = append(edges, edge)
	}
	graph.Nodes = nodes
	graph.Edges = edges
	return graph
}

func lineageSourceAllowed(opts DatasetLineageOptions, source string) bool {
	for _, allowed := range opts.Sources {
		if allowed == source {
			return true
		}
	}
	return false
}

func encodeMetadata(value map[string]any) []byte {
	if value == nil {
		value = map[string]any{}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return []byte(`{}`)
	}
	return body
}

func decodeMetadata(body []byte) map[string]any {
	if len(body) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}

func decodeStringSlice(body []byte) []string {
	if len(body) == 0 {
		return nil
	}
	var out []string
	_ = json.Unmarshal(body, &out)
	return out
}

func pgUUID(value string) pgtype.UUID {
	if strings.TrimSpace(value) == "" {
		return pgtype.UUID{}
	}
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func uuidString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}

func pgFloat8(value *float64) pgtype.Float8 {
	if value == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *value, Valid: true}
}

func positionX(position *DatasetLineagePosition) *float64 {
	if position == nil {
		return nil
	}
	return &position.X
}

func positionY(position *DatasetLineagePosition) *float64 {
	if position == nil {
		return nil
	}
	return &position.Y
}

func pgTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func pgTimePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func parentNodeKey(nodeID string) string {
	idx := strings.LastIndex(nodeID, ":column:")
	if idx <= 0 {
		return ""
	}
	return nodeID[:idx]
}

func (s *DatasetService) recordDatasetLineageAudit(ctx context.Context, auditCtx AuditContext, action, targetType, targetID string, metadata map[string]any) {
	if s == nil || s.audit == nil {
		return
	}
	s.audit.RecordBestEffort(ctx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       action,
		TargetType:   targetType,
		TargetID:     targetID,
		Metadata:     metadata,
	})
}
