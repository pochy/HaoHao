package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"example.com/haohao/backend/internal/db"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDataPipelineNotFound           = errors.New("data pipeline not found")
	ErrDataPipelineVersionNotFound    = errors.New("data pipeline version not found")
	ErrDataPipelineRunNotFound        = errors.New("data pipeline run not found")
	ErrDataPipelineScheduleNotFound   = errors.New("data pipeline schedule not found")
	ErrDataPipelineReviewItemNotFound = errors.New("data pipeline review item not found")
	ErrInvalidDataPipelineInput       = errors.New("invalid data pipeline input")
	ErrInvalidDataPipelineGraph       = errors.New("invalid data pipeline graph")
	ErrDataPipelineVersionUnpublished = errors.New("data pipeline version is not published")
)

type DataPipeline struct {
	ID                    int64
	PublicID              string
	TenantID              int64
	CreatedByUserID       *int64
	UpdatedByUserID       *int64
	Name                  string
	Description           string
	Status                string
	PublishedVersionID    *int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	ArchivedAt            *time.Time
	LatestRunStatus       string
	LatestRunAt           *time.Time
	LatestRunPublicID     string
	ScheduleState         string
	EnabledScheduleCount  int64
	DisabledScheduleCount int64
	NextRunAt             *time.Time
}

type DataPipelineVersion struct {
	ID                int64
	PublicID          string
	TenantID          int64
	PipelineID        int64
	VersionNumber     int32
	Status            string
	Graph             DataPipelineGraph
	ValidationSummary DataPipelineValidationSummary
	CreatedByUserID   *int64
	PublishedByUserID *int64
	CreatedAt         time.Time
	PublishedAt       *time.Time
}

type DataPipelineRun struct {
	ID                int64
	PublicID          string
	TenantID          int64
	PipelineID        int64
	VersionID         int64
	ScheduleID        *int64
	RequestedByUserID *int64
	TriggerKind       string
	Status            string
	OutputWorkTableID *int64
	OutboxEventID     *int64
	RowCount          int64
	ErrorSummary      string
	StartedAt         *time.Time
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Steps             []DataPipelineRunStep
	Outputs           []DataPipelineRunOutput
}

type DataPipelineRunOutput struct {
	ID                    int64
	TenantID              int64
	RunID                 int64
	NodeID                string
	Status                string
	OutputWorkTableID     *int64
	LatestGoldPublication *DataPipelineRunOutputGoldPublication
	RowCount              int64
	ErrorSummary          string
	Metadata              map[string]any
	StartedAt             *time.Time
	CompletedAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type DataPipelineRunOutputGoldPublication struct {
	PublicID     string
	DisplayName  string
	Status       string
	GoldDatabase string
	GoldTable    string
}

type DataPipelineRunStep struct {
	ID           int64
	TenantID     int64
	RunID        int64
	NodeID       string
	StepType     string
	Status       string
	RowCount     int64
	ErrorSummary string
	ErrorSample  []map[string]any
	Metadata     map[string]any
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type DataPipelineSchedule struct {
	ID               int64
	PublicID         string
	TenantID         int64
	PipelineID       int64
	VersionID        int64
	CreatedByUserID  *int64
	Frequency        string
	Timezone         string
	RunTime          string
	Weekday          *int32
	MonthDay         *int32
	Enabled          bool
	NextRunAt        time.Time
	LastRunAt        *time.Time
	LastStatus       string
	LastErrorSummary string
	LastRunID        *int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type DataPipelineDetail struct {
	Pipeline         DataPipeline
	PublishedVersion *DataPipelineVersion
	Versions         []DataPipelineVersion
	Runs             []DataPipelineRun
	Schedules        []DataPipelineSchedule
}

type DataPipelineInput struct {
	Name        string
	Description string
}

type DataPipelineListInput struct {
	Query         string
	Status        string
	Publication   string
	RunStatus     string
	ScheduleState string
	Sort          string
	Cursor        string
	Limit         int32
}

type DataPipelineListResult struct {
	Items      []DataPipeline
	NextCursor string
}

type DataPipelineScheduleInput struct {
	Frequency string
	Timezone  string
	RunTime   string
	Weekday   *int32
	MonthDay  *int32
	Enabled   *bool
}

type DataPipelinePreview struct {
	NodeID        string
	StepType      string
	Columns       []string
	PreviewRows   []map[string]any
	OutputSchemas []DataPipelineNodeOutputSchema
}

type DataPipelineGraphValidation struct {
	ValidationSummary DataPipelineValidationSummary
	OutputSchemas     []DataPipelineNodeOutputSchema
	NodeWarnings      []DataPipelineNodeWarning
}

type DataPipelineNodeWarning struct {
	NodeID     string
	StepType   string
	Code       string
	Severity   string
	Message    string
	Columns    []string
	ConfigKeys []string
}

type DataPipelineReviewItem struct {
	ID                int64
	PublicID          string
	TenantID          int64
	PipelineID        int64
	PipelinePublicID  string
	PipelineName      string
	VersionID         int64
	RunID             int64
	RunPublicID       string
	NodeID            string
	Queue             string
	Status            string
	Reason            []map[string]any
	SourceSnapshot    map[string]any
	SourceFingerprint string
	CreatedByUserID   *int64
	UpdatedByUserID   *int64
	AssignedToUserID  *int64
	DecisionComment   string
	DecidedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Comments          []DataPipelineReviewItemComment
}

type DataPipelineReviewItemComment struct {
	ID           int64
	PublicID     string
	TenantID     int64
	ReviewItemID int64
	AuthorUserID *int64
	Body         string
	CreatedAt    time.Time
}

type DataPipelineReviewItemListInput struct {
	Status string
	Limit  int32
}

type DataPipelineReviewItemTransitionInput struct {
	Status  string
	Comment string
}

type dataPipelineReviewItemDraft struct {
	NodeID            string
	Queue             string
	Reason            []map[string]any
	SourceSnapshot    map[string]any
	SourceFingerprint string
}

type DataPipelineSchemaMappingCandidateInput struct {
	PipelinePublicID string
	VersionPublicID  string
	Domain           string
	SchemaType       string
	Columns          []DataPipelineSchemaMappingSourceColumn
	Limit            int32
}

type DataPipelineSchemaMappingExampleInput struct {
	PipelinePublicID     string
	VersionPublicID      string
	SchemaColumnPublicID string
	SourceColumn         string
	SheetName            string
	SampleValues         []string
	NeighborColumns      []string
	Decision             string
}

type DataPipelineSchemaMappingExample struct {
	PublicID             string
	SchemaColumnPublicID string
	SourceColumn         string
	TargetColumn         string
	Decision             string
	SharedScope          string
}

type DataPipelineSchemaMappingExampleListInput struct {
	Query       string
	SharedScope string
	Decision    string
	Limit       int32
}

type DataPipelineSchemaMappingExampleListItem struct {
	PublicID                   string
	PipelinePublicID           string
	PipelineName               string
	SchemaColumnPublicID       string
	Domain                     string
	SchemaType                 string
	SourceColumn               string
	SheetName                  string
	SampleValues               []string
	NeighborColumns            []string
	TargetColumn               string
	Decision                   string
	SharedScope                string
	SearchDocumentMaterialized bool
	DecidedAt                  time.Time
	SharedAt                   *time.Time
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

type DataPipelineSchemaMappingSearchRebuildResult struct {
	Indexed                int
	SchemaColumnsIndexed   int
	MappingExamplesIndexed int
}

type DataPipelineSchemaMappingExampleSharingInput struct {
	ExamplePublicID string
	SharedScope     string
}

type DataPipelineSchemaMappingSourceColumn struct {
	SourceColumn    string
	SheetName       string
	SampleValues    []string
	NeighborColumns []string
}

type DataPipelineSchemaMappingCandidateResult struct {
	Items []DataPipelineSchemaMappingCandidateItem
}

type DataPipelineSchemaMappingCandidateItem struct {
	SourceColumn string
	Candidates   []DataPipelineSchemaMappingCandidate
}

type DataPipelineSchemaMappingCandidate struct {
	SchemaColumnID       int64
	SchemaColumnPublicID string
	TargetColumn         string
	Score                float64
	MatchMethod          string
	Reason               string
	Snippet              string
	AcceptedEvidence     int64
	RejectedEvidence     int64
}

type schemaMappingCandidateDraft struct {
	SchemaColumnID       int64
	SchemaColumnPublicID string
	TargetColumn         string
	KeywordScore         float64
	VectorScore          float64
	HasKeyword           bool
	HasVector            bool
	Snippet              string
}

type schemaMappingExampleIndexInput struct {
	ID              int64
	PublicID        string
	TenantID        int64
	SourceColumn    string
	SheetName       string
	SampleValues    []byte
	NeighborColumns []byte
	TargetColumn    string
	Decision        string
	UpdatedAt       *time.Time
}

type DataPipelineScheduleRunSummary struct {
	Claimed  int
	Created  int
	Skipped  int
	Failed   int
	Disabled int
}

type DataPipelineService struct {
	pool        *pgxpool.Pool
	queries     *db.Queries
	outbox      *OutboxService
	datasets    *DatasetService
	driveOCR    *DriveOCRService
	medallion   *MedallionCatalogService
	authz       *DatasetAuthorizationService
	localSearch *LocalSearchService
	audit       AuditRecorder
}

func NewDataPipelineService(pool *pgxpool.Pool, queries *db.Queries, outbox *OutboxService, datasets *DatasetService, medallion *MedallionCatalogService, audit AuditRecorder) *DataPipelineService {
	return &DataPipelineService{
		pool:      pool,
		queries:   queries,
		outbox:    outbox,
		datasets:  datasets,
		medallion: medallion,
		audit:     audit,
	}
}

func (s *DataPipelineService) SetDriveOCRService(driveOCR *DriveOCRService) {
	if s != nil {
		s.driveOCR = driveOCR
	}
}

func (s *DataPipelineService) SetDatasetAuthorizationService(authz *DatasetAuthorizationService) {
	if s != nil {
		s.authz = authz
	}
}

func (s *DataPipelineService) SetLocalSearchService(localSearch *LocalSearchService) {
	if s != nil {
		s.localSearch = localSearch
	}
}

func (s *DataPipelineService) List(ctx context.Context, tenantID, actorUserID int64, input DataPipelineListInput) (DataPipelineListResult, error) {
	if s == nil || s.queries == nil {
		return DataPipelineListResult{}, fmt.Errorf("data pipeline service is not configured")
	}
	normalized, cursor, err := normalizeDataPipelineListInput(input)
	if err != nil {
		return DataPipelineListResult{}, err
	}
	if s.authz == nil {
		return DataPipelineListResult{}, ErrDataAuthzUnavailable
	}
	limit := normalized.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	result := DataPipelineListResult{Items: make([]DataPipeline, 0, limit)}
	for {
		rows, err := s.queries.ListDataPipelines(ctx, db.ListDataPipelinesParams{
			TenantID:            tenantID,
			Q:                   nullableText(normalized.Query),
			Status:              nullableText(normalized.Status),
			Publication:         normalized.Publication,
			RunStatus:           nullableText(normalized.RunStatus),
			ScheduleStateFilter: normalized.ScheduleState,
			SortKey:             normalized.Sort,
			CursorID:            nullableInt8(cursor.ID),
			CursorTime:          nullableTime(cursor.Time),
			CursorText:          nullableText(cursor.Text),
			ResultLimit:         listDataPipelineCandidateLimit(limit),
		})
		if err != nil {
			return DataPipelineListResult{}, fmt.Errorf("list data pipelines: %w", err)
		}
		if len(rows) == 0 {
			return result, nil
		}
		publicIDs := make([]string, 0, len(rows))
		candidates := make([]DataPipeline, 0, len(rows))
		for _, row := range rows {
			item := dataPipelineFromListRow(row)
			publicIDs = append(publicIDs, item.PublicID)
			candidates = append(candidates, item)
		}
		allowed, err := s.authz.FilterResourcePublicIDs(ctx, actorUserID, DataResourceDataPipeline, DataActionView, publicIDs)
		if err != nil {
			return DataPipelineListResult{}, err
		}
		for _, item := range candidates {
			if !allowed[item.PublicID] {
				continue
			}
			result.Items = append(result.Items, item)
			if int32(len(result.Items)) > limit {
				break
			}
		}
		if int32(len(result.Items)) > limit {
			visible := result.Items[:limit]
			next, err := encodeDataPipelineListCursor(normalized.Sort, visible[len(visible)-1])
			if err != nil {
				return DataPipelineListResult{}, err
			}
			result.Items = visible
			result.NextCursor = next
			return result, nil
		}
		last := candidates[len(candidates)-1]
		cursor = dataPipelineListCursorFromItem(normalized.Sort, last)
		if len(rows) < int(listDataPipelineCandidateLimit(limit)) {
			return result, nil
		}
	}
}

func (s *DataPipelineService) SchemaMappingCandidates(ctx context.Context, tenantID, actorUserID int64, input DataPipelineSchemaMappingCandidateInput) (DataPipelineSchemaMappingCandidateResult, error) {
	if s == nil || s.queries == nil {
		return DataPipelineSchemaMappingCandidateResult{}, fmt.Errorf("data pipeline service is not configured")
	}
	limit := input.Limit
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	var pipelineID pgtype.Int8
	if strings.TrimSpace(input.PipelinePublicID) != "" {
		pipeline, err := s.getPipelineRow(ctx, tenantID, input.PipelinePublicID)
		if err != nil {
			return DataPipelineSchemaMappingCandidateResult{}, err
		}
		if err := s.checkPipelineByID(ctx, tenantID, actorUserID, pipeline.ID, DataActionView); err != nil {
			return DataPipelineSchemaMappingCandidateResult{}, err
		}
		pipelineID = pgtype.Int8{Int64: pipeline.ID, Valid: true}
	}
	if strings.TrimSpace(input.VersionPublicID) != "" {
		version, err := s.getVersionRow(ctx, tenantID, input.VersionPublicID)
		if err != nil {
			return DataPipelineSchemaMappingCandidateResult{}, err
		}
		if pipelineID.Valid && version.PipelineID != pipelineID.Int64 {
			return DataPipelineSchemaMappingCandidateResult{}, fmt.Errorf("%w: version does not belong to pipeline", ErrInvalidDataPipelineInput)
		}
		if err := s.checkPipelineByID(ctx, tenantID, actorUserID, version.PipelineID, DataActionView); err != nil {
			return DataPipelineSchemaMappingCandidateResult{}, err
		}
		if !pipelineID.Valid {
			pipelineID = pgtype.Int8{Int64: version.PipelineID, Valid: true}
		}
	}
	if len(input.Columns) == 0 || len(input.Columns) > 100 {
		return DataPipelineSchemaMappingCandidateResult{}, fmt.Errorf("%w: columns must contain 1 to 100 items", ErrInvalidDataPipelineInput)
	}
	domain := nullableText(strings.TrimSpace(input.Domain))
	schemaType := nullableText(strings.TrimSpace(input.SchemaType))
	result := DataPipelineSchemaMappingCandidateResult{Items: make([]DataPipelineSchemaMappingCandidateItem, 0, len(input.Columns))}
	for _, column := range input.Columns {
		sourceColumn := strings.TrimSpace(column.SourceColumn)
		if sourceColumn == "" {
			return DataPipelineSchemaMappingCandidateResult{}, fmt.Errorf("%w: sourceColumn is required", ErrInvalidDataPipelineInput)
		}
		queryText := sourceColumn
		rows, err := s.queries.SearchDataPipelineSchemaMappingCandidates(ctx, db.SearchDataPipelineSchemaMappingCandidatesParams{
			TenantID:   tenantID,
			Domain:     domain,
			SchemaType: schemaType,
			Query:      queryText,
			LimitCount: limit,
		})
		if err != nil {
			return DataPipelineSchemaMappingCandidateResult{}, fmt.Errorf("search schema mapping candidates: %w", err)
		}
		drafts := map[int64]schemaMappingCandidateDraft{}
		for _, row := range rows {
			drafts[row.ID] = schemaMappingCandidateDraft{
				SchemaColumnID:       row.ID,
				SchemaColumnPublicID: row.PublicID.String(),
				TargetColumn:         row.TargetColumn,
				KeywordScore:         row.KeywordScore,
				HasKeyword:           true,
				Snippet:              row.Snippet,
			}
		}
		if s.localSearch != nil {
			hits, err := s.localSearch.SearchSemantic(ctx, tenantID, LocalSearchResourceSchemaColumn, schemaMappingCandidateSemanticText(column), limit*5)
			if err == nil {
				for _, hit := range hits {
					schemaColumn, ok, err := s.schemaColumnForSemanticHit(ctx, tenantID, hit, domain, schemaType)
					if err != nil {
						return DataPipelineSchemaMappingCandidateResult{}, err
					}
					if !ok {
						continue
					}
					draft := drafts[schemaColumn.ID]
					draft.SchemaColumnID = schemaColumn.ID
					draft.SchemaColumnPublicID = schemaColumn.PublicID.String()
					draft.TargetColumn = schemaColumn.TargetColumn
					draft.VectorScore = hit.Score
					draft.HasVector = true
					if strings.TrimSpace(draft.Snippet) == "" {
						draft.Snippet = searchSnippet(hit.SourceText, sourceColumn)
					}
					drafts[schemaColumn.ID] = draft
				}
			}
		}
		ids := make([]int64, 0, len(drafts))
		for id := range drafts {
			ids = append(ids, id)
		}
		evidence := map[int64]map[string]int64{}
		if len(ids) > 0 {
			counts, err := s.queries.CountDataPipelineMappingEvidence(ctx, db.CountDataPipelineMappingEvidenceParams{
				TenantID:        tenantID,
				SchemaColumnIds: ids,
				PipelineID:      pipelineID,
			})
			if err != nil {
				return DataPipelineSchemaMappingCandidateResult{}, fmt.Errorf("count schema mapping evidence: %w", err)
			}
			for _, count := range counts {
				if evidence[count.SchemaColumnID] == nil {
					evidence[count.SchemaColumnID] = map[string]int64{}
				}
				evidence[count.SchemaColumnID][count.Decision] = count.EvidenceCount
			}
		}
		orderedDrafts := make([]schemaMappingCandidateDraft, 0, len(drafts))
		for _, draft := range drafts {
			orderedDrafts = append(orderedDrafts, draft)
		}
		sort.SliceStable(orderedDrafts, func(i, j int) bool {
			left := schemaMappingCandidateScore(orderedDrafts[i].KeywordScore, orderedDrafts[i].VectorScore, orderedDrafts[i].HasVector, evidence[orderedDrafts[i].SchemaColumnID]["accepted"], evidence[orderedDrafts[i].SchemaColumnID]["rejected"], sourceColumn, orderedDrafts[i].TargetColumn)
			right := schemaMappingCandidateScore(orderedDrafts[j].KeywordScore, orderedDrafts[j].VectorScore, orderedDrafts[j].HasVector, evidence[orderedDrafts[j].SchemaColumnID]["accepted"], evidence[orderedDrafts[j].SchemaColumnID]["rejected"], sourceColumn, orderedDrafts[j].TargetColumn)
			if left == right {
				return orderedDrafts[i].TargetColumn < orderedDrafts[j].TargetColumn
			}
			return left > right
		})
		if int32(len(orderedDrafts)) > limit {
			orderedDrafts = orderedDrafts[:limit]
		}
		item := DataPipelineSchemaMappingCandidateItem{SourceColumn: sourceColumn, Candidates: make([]DataPipelineSchemaMappingCandidate, 0, len(orderedDrafts))}
		for _, draft := range orderedDrafts {
			accepted := evidence[draft.SchemaColumnID]["accepted"]
			rejected := evidence[draft.SchemaColumnID]["rejected"]
			score := schemaMappingCandidateScore(draft.KeywordScore, draft.VectorScore, draft.HasVector, accepted, rejected, sourceColumn, draft.TargetColumn)
			item.Candidates = append(item.Candidates, DataPipelineSchemaMappingCandidate{
				SchemaColumnID:       draft.SchemaColumnID,
				SchemaColumnPublicID: draft.SchemaColumnPublicID,
				TargetColumn:         draft.TargetColumn,
				Score:                score,
				MatchMethod:          schemaMappingMatchMethod(draft.HasKeyword, draft.HasVector),
				Reason:               schemaMappingCandidateReason(sourceColumn, draft.TargetColumn, draft.HasVector, accepted, rejected),
				Snippet:              draft.Snippet,
				AcceptedEvidence:     accepted,
				RejectedEvidence:     rejected,
			})
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *DataPipelineService) RebuildSchemaMappingSearchDocuments(ctx context.Context, tenantID int64, limit int32) (DataPipelineSchemaMappingSearchRebuildResult, error) {
	if s == nil || s.queries == nil || s.localSearch == nil {
		return DataPipelineSchemaMappingSearchRebuildResult{}, fmt.Errorf("data pipeline schema mapping index is not configured")
	}
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	rows, err := s.queries.ListDataPipelineSchemaColumnsForIndex(ctx, db.ListDataPipelineSchemaColumnsForIndexParams{TenantID: tenantID, LimitCount: limit})
	if err != nil {
		return DataPipelineSchemaMappingSearchRebuildResult{}, fmt.Errorf("list schema columns for index: %w", err)
	}
	result := DataPipelineSchemaMappingSearchRebuildResult{}
	for _, row := range rows {
		body := schemaColumnIndexText(row.TargetColumn, row.Description, row.Aliases, row.Examples, row.Domain, row.SchemaType)
		if err := s.localSearch.UpsertDocument(ctx, LocalSearchDocumentInput{
			TenantID:         row.TenantID,
			ResourceKind:     LocalSearchResourceSchemaColumn,
			ResourceID:       row.ID,
			ResourcePublicID: row.PublicID.String(),
			Title:            row.TargetColumn,
			BodyText:         body,
			Snippet:          searchSnippet(body, row.TargetColumn),
			ContentHash:      localSearchHash(row.PublicID.String(), body),
			SourceUpdatedAt:  &row.UpdatedAt.Time,
		}); err != nil {
			return result, err
		}
		result.SchemaColumnsIndexed++
		result.Indexed++
	}
	examples, err := s.queries.ListDataPipelineMappingExamplesForIndex(ctx, db.ListDataPipelineMappingExamplesForIndexParams{TenantID: tenantID, LimitCount: limit})
	if err != nil {
		return result, fmt.Errorf("list mapping examples for index: %w", err)
	}
	for _, row := range examples {
		if err := s.indexSchemaMappingExample(ctx, schemaMappingExampleIndexInput{
			ID:              row.ID,
			PublicID:        row.PublicID.String(),
			TenantID:        row.TenantID,
			SourceColumn:    row.SourceColumn,
			SheetName:       row.SheetName,
			SampleValues:    row.SampleValues,
			NeighborColumns: row.NeighborColumns,
			TargetColumn:    row.TargetColumn,
			Decision:        row.Decision,
			UpdatedAt:       &row.UpdatedAt.Time,
		}); err != nil {
			return result, err
		}
		result.MappingExamplesIndexed++
		result.Indexed++
	}
	return result, nil
}

func (s *DataPipelineService) ListSchemaMappingExamplesForAdmin(ctx context.Context, tenantID int64, input DataPipelineSchemaMappingExampleListInput) ([]DataPipelineSchemaMappingExampleListItem, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("data pipeline service is not configured")
	}
	limit := input.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	sharedScope := strings.ToLower(strings.TrimSpace(input.SharedScope))
	if sharedScope != "" && sharedScope != "private" && sharedScope != "tenant" {
		return nil, fmt.Errorf("%w: sharedScope must be private or tenant", ErrInvalidDataPipelineInput)
	}
	decision := strings.ToLower(strings.TrimSpace(input.Decision))
	if decision != "" && decision != "accepted" && decision != "rejected" {
		return nil, fmt.Errorf("%w: decision must be accepted or rejected", ErrInvalidDataPipelineInput)
	}
	rows, err := s.queries.ListTenantAdminDataPipelineMappingExamples(ctx, db.ListTenantAdminDataPipelineMappingExamplesParams{
		TenantID:    tenantID,
		SharedScope: nullableText(sharedScope),
		Decision:    nullableText(decision),
		Query:       nullableText(strings.TrimSpace(input.Query)),
		LimitCount:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list schema mapping examples: %w", err)
	}
	items := make([]DataPipelineSchemaMappingExampleListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, DataPipelineSchemaMappingExampleListItem{
			PublicID:                   row.PublicID.String(),
			PipelinePublicID:           row.PipelinePublicID.String(),
			PipelineName:               row.PipelineName,
			SchemaColumnPublicID:       row.SchemaColumnPublicID.String(),
			Domain:                     row.Domain,
			SchemaType:                 row.SchemaType,
			SourceColumn:               row.SourceColumn,
			SheetName:                  row.SheetName,
			SampleValues:               decodeDataPipelineJSONStringList(row.SampleValues),
			NeighborColumns:            decodeDataPipelineJSONStringList(row.NeighborColumns),
			TargetColumn:               row.TargetColumn,
			Decision:                   row.Decision,
			SharedScope:                row.SharedScope,
			SearchDocumentMaterialized: row.SearchDocumentMaterialized,
			DecidedAt:                  row.DecidedAt.Time,
			SharedAt:                   optionalPgTime(row.SharedAt),
			CreatedAt:                  row.CreatedAt.Time,
			UpdatedAt:                  row.UpdatedAt.Time,
		})
	}
	return items, nil
}

func (s *DataPipelineService) RecordSchemaMappingExample(ctx context.Context, tenantID, actorUserID int64, input DataPipelineSchemaMappingExampleInput, auditCtx AuditContext) (DataPipelineSchemaMappingExample, error) {
	if s == nil || s.queries == nil {
		return DataPipelineSchemaMappingExample{}, fmt.Errorf("data pipeline service is not configured")
	}
	pipeline, err := s.getPipelineRow(ctx, tenantID, input.PipelinePublicID)
	if err != nil {
		return DataPipelineSchemaMappingExample{}, err
	}
	if err := s.checkPipelineByID(ctx, tenantID, actorUserID, pipeline.ID, DataActionUpdate); err != nil {
		return DataPipelineSchemaMappingExample{}, err
	}
	var versionID pgtype.Int8
	if strings.TrimSpace(input.VersionPublicID) != "" {
		version, err := s.getVersionRow(ctx, tenantID, input.VersionPublicID)
		if err != nil {
			return DataPipelineSchemaMappingExample{}, err
		}
		if version.PipelineID != pipeline.ID {
			return DataPipelineSchemaMappingExample{}, fmt.Errorf("%w: version does not belong to pipeline", ErrInvalidDataPipelineInput)
		}
		versionID = pgtype.Int8{Int64: version.ID, Valid: true}
	}
	sourceColumn := strings.TrimSpace(input.SourceColumn)
	if sourceColumn == "" {
		return DataPipelineSchemaMappingExample{}, fmt.Errorf("%w: sourceColumn is required", ErrInvalidDataPipelineInput)
	}
	decision := strings.ToLower(strings.TrimSpace(input.Decision))
	if decision != "accepted" && decision != "rejected" {
		return DataPipelineSchemaMappingExample{}, fmt.Errorf("%w: decision must be accepted or rejected", ErrInvalidDataPipelineInput)
	}
	parsedSchemaColumnID, err := uuid.Parse(strings.TrimSpace(input.SchemaColumnPublicID))
	if err != nil {
		return DataPipelineSchemaMappingExample{}, fmt.Errorf("%w: invalid schemaColumnPublicId", ErrInvalidDataPipelineInput)
	}
	schemaColumn, err := s.queries.GetDataPipelineSchemaColumnByPublicIDForTenant(ctx, db.GetDataPipelineSchemaColumnByPublicIDForTenantParams{
		TenantID: tenantID,
		PublicID: parsedSchemaColumnID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DataPipelineSchemaMappingExample{}, ErrDataPipelineNotFound
	}
	if err != nil {
		return DataPipelineSchemaMappingExample{}, fmt.Errorf("get schema column for mapping example: %w", err)
	}
	sampleValues, err := encodeDataPipelineJSON(trimStringList(input.SampleValues, 20, 160))
	if err != nil {
		return DataPipelineSchemaMappingExample{}, err
	}
	neighborColumns, err := encodeDataPipelineJSON(trimStringList(input.NeighborColumns, 40, 240))
	if err != nil {
		return DataPipelineSchemaMappingExample{}, err
	}
	params := db.UpsertDataPipelineMappingExampleWithoutVersionParams{
		TenantID:        tenantID,
		PipelineID:      pipeline.ID,
		SchemaColumnID:  schemaColumn.ID,
		SourceColumn:    sourceColumn,
		SheetName:       strings.TrimSpace(input.SheetName),
		SampleValues:    sampleValues,
		NeighborColumns: neighborColumns,
		Decision:        decision,
		DecidedByUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
	}
	var example db.DataPipelineMappingExample
	if versionID.Valid {
		example, err = s.queries.UpsertDataPipelineMappingExample(ctx, db.UpsertDataPipelineMappingExampleParams{
			TenantID:        params.TenantID,
			PipelineID:      params.PipelineID,
			VersionID:       versionID,
			SchemaColumnID:  params.SchemaColumnID,
			SourceColumn:    params.SourceColumn,
			SheetName:       params.SheetName,
			SampleValues:    params.SampleValues,
			NeighborColumns: params.NeighborColumns,
			Decision:        params.Decision,
			DecidedByUserID: params.DecidedByUserID,
		})
	} else {
		example, err = s.queries.UpsertDataPipelineMappingExampleWithoutVersion(ctx, params)
	}
	if err != nil {
		return DataPipelineSchemaMappingExample{}, fmt.Errorf("upsert schema mapping example: %w", err)
	}
	if s.localSearch != nil {
		if err := s.indexSchemaMappingExample(ctx, schemaMappingExampleIndexInput{
			ID:              example.ID,
			PublicID:        example.PublicID.String(),
			TenantID:        tenantID,
			SourceColumn:    sourceColumn,
			SheetName:       params.SheetName,
			SampleValues:    sampleValues,
			NeighborColumns: neighborColumns,
			TargetColumn:    schemaColumn.TargetColumn,
			Decision:        decision,
			UpdatedAt:       &example.UpdatedAt.Time,
		}); err != nil {
			return DataPipelineSchemaMappingExample{}, err
		}
	}
	out := DataPipelineSchemaMappingExample{
		PublicID:             example.PublicID.String(),
		SchemaColumnPublicID: schemaColumn.PublicID.String(),
		SourceColumn:         sourceColumn,
		TargetColumn:         schemaColumn.TargetColumn,
		Decision:             decision,
		SharedScope:          example.SharedScope,
	}
	s.recordAudit(ctx, auditCtx, "data_pipeline.schema_mapping_example.record", "data_pipeline_mapping_example", out.PublicID, nil)
	return out, nil
}

func (s *DataPipelineService) UpdateSchemaMappingExampleSharing(ctx context.Context, tenantID, actorUserID int64, input DataPipelineSchemaMappingExampleSharingInput, auditCtx AuditContext) (DataPipelineSchemaMappingExample, error) {
	if s == nil || s.queries == nil {
		return DataPipelineSchemaMappingExample{}, fmt.Errorf("data pipeline service is not configured")
	}
	parsedExampleID, err := uuid.Parse(strings.TrimSpace(input.ExamplePublicID))
	if err != nil {
		return DataPipelineSchemaMappingExample{}, ErrDataPipelineNotFound
	}
	sharedScope := strings.ToLower(strings.TrimSpace(input.SharedScope))
	if sharedScope != "private" && sharedScope != "tenant" {
		return DataPipelineSchemaMappingExample{}, fmt.Errorf("%w: sharedScope must be private or tenant", ErrInvalidDataPipelineInput)
	}
	existing, err := s.queries.GetDataPipelineMappingExampleByPublicIDForTenant(ctx, db.GetDataPipelineMappingExampleByPublicIDForTenantParams{
		TenantID: tenantID,
		PublicID: parsedExampleID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DataPipelineSchemaMappingExample{}, ErrDataPipelineNotFound
	}
	if err != nil {
		return DataPipelineSchemaMappingExample{}, fmt.Errorf("get schema mapping example: %w", err)
	}
	updated, err := s.queries.UpdateDataPipelineMappingExampleSharing(ctx, db.UpdateDataPipelineMappingExampleSharingParams{
		TenantID:       tenantID,
		PublicID:       parsedExampleID,
		SharedScope:    sharedScope,
		SharedByUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0 && sharedScope == "tenant"},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DataPipelineSchemaMappingExample{}, ErrDataPipelineNotFound
	}
	if err != nil {
		return DataPipelineSchemaMappingExample{}, fmt.Errorf("update schema mapping example sharing: %w", err)
	}
	out := DataPipelineSchemaMappingExample{
		PublicID:             updated.PublicID.String(),
		SchemaColumnPublicID: existing.SchemaColumnPublicID.String(),
		SourceColumn:         updated.SourceColumn,
		TargetColumn:         existing.TargetColumn,
		Decision:             updated.Decision,
		SharedScope:          updated.SharedScope,
	}
	s.recordAudit(ctx, auditCtx, "data_pipeline.schema_mapping_example.sharing.update", "data_pipeline_mapping_example", out.PublicID, map[string]any{"sharedScope": out.SharedScope})
	return out, nil
}

func (s *DataPipelineService) indexSchemaMappingExample(ctx context.Context, input schemaMappingExampleIndexInput) error {
	if s == nil || s.localSearch == nil {
		return nil
	}
	body := mappingExampleIndexText(input.SourceColumn, input.SheetName, input.SampleValues, input.NeighborColumns, input.TargetColumn, input.Decision)
	return s.localSearch.UpsertDocument(ctx, LocalSearchDocumentInput{
		TenantID:         input.TenantID,
		ResourceKind:     LocalSearchResourceMappingExample,
		ResourceID:       input.ID,
		ResourcePublicID: input.PublicID,
		Title:            input.SourceColumn + " -> " + input.TargetColumn,
		BodyText:         body,
		Snippet:          searchSnippet(body, input.SourceColumn),
		ContentHash:      localSearchHash(input.PublicID, body),
		SourceUpdatedAt:  input.UpdatedAt,
	})
}

func (s *DataPipelineService) Create(ctx context.Context, tenantID, userID int64, input DataPipelineInput, auditCtx AuditContext) (DataPipeline, error) {
	if s == nil || s.queries == nil {
		return DataPipeline{}, fmt.Errorf("data pipeline service is not configured")
	}
	normalized, err := normalizeDataPipelineInput(input)
	if err != nil {
		return DataPipeline{}, err
	}
	if err := s.authzCheckScope(ctx, tenantID, userID, DataActionCreatePipeline); err != nil {
		return DataPipeline{}, err
	}
	row, err := s.queries.CreateDataPipeline(ctx, db.CreateDataPipelineParams{
		TenantID:        tenantID,
		CreatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		UpdatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		Name:            normalized.Name,
		Description:     normalized.Description,
	})
	if err != nil {
		return DataPipeline{}, fmt.Errorf("create data pipeline: %w", err)
	}
	item := dataPipelineFromDB(row)
	if s.authz != nil {
		if err := s.authz.EnsureResourceOwnerTuples(ctx, tenantID, userID, DataResourceDataPipeline, item.PublicID); err != nil {
			return DataPipeline{}, err
		}
	}
	s.recordAudit(ctx, auditCtx, "data_pipeline.create", "data_pipeline", item.PublicID, nil)
	return item, nil
}

func (s *DataPipelineService) schemaColumnForSemanticHit(ctx context.Context, tenantID int64, hit LocalSearchSemanticHit, domain, schemaType pgtype.Text) (db.DataPipelineSchemaColumn, bool, error) {
	if hit.ResourceKind != LocalSearchResourceSchemaColumn || hit.ResourceID <= 0 {
		return db.DataPipelineSchemaColumn{}, false, nil
	}
	row, err := s.queries.GetDataPipelineSchemaColumnByIDForTenant(ctx, db.GetDataPipelineSchemaColumnByIDForTenantParams{
		TenantID: tenantID,
		ID:       hit.ResourceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DataPipelineSchemaColumn{}, false, nil
	}
	if err != nil {
		return db.DataPipelineSchemaColumn{}, false, fmt.Errorf("get semantic schema column: %w", err)
	}
	if domain.Valid && row.Domain != domain.String {
		return db.DataPipelineSchemaColumn{}, false, nil
	}
	if schemaType.Valid && row.SchemaType != schemaType.String {
		return db.DataPipelineSchemaColumn{}, false, nil
	}
	return row, true, nil
}

func schemaMappingCandidateScore(keywordScore, vectorScore float64, hasVector bool, accepted, rejected int64, sourceColumn, targetColumn string) float64 {
	score := keywordScore
	if hasVector {
		score += vectorScore * 0.7
	}
	score += float64(accepted) * 0.08
	score -= float64(rejected) * 0.12
	if score < 0 {
		return 0
	}
	return score
}

func schemaMappingMatchMethod(hasKeyword, hasVector bool) string {
	if hasKeyword && hasVector {
		return "hybrid"
	}
	if hasVector {
		return "vector"
	}
	return "keyword"
}

func schemaMappingCandidateReason(sourceColumn, targetColumn string, hasVector bool, accepted, rejected int64) string {
	if hasVector && accepted > 0 {
		return "hybrid match with accepted mapping history"
	}
	if hasVector && rejected > 0 {
		return "semantic match with rejected mapping evidence"
	}
	if hasVector {
		return "semantic match against schema column definition"
	}
	if accepted > 0 {
		return "keyword match with accepted mapping history"
	}
	if rejected > 0 {
		return "keyword match with rejected mapping evidence"
	}
	return "keyword match against schema column definition"
}

func schemaMappingCandidateSemanticText(column DataPipelineSchemaMappingSourceColumn) string {
	parts := []string{
		column.SourceColumn,
		column.SheetName,
		strings.Join(column.SampleValues, " "),
		strings.Join(column.NeighborColumns, " "),
	}
	text := sanitizeSearchText(strings.Join(parts, " "))
	if text == "" {
		return strings.TrimSpace(column.SourceColumn)
	}
	return text
}

func schemaColumnIndexText(targetColumn, description string, aliases, examples []byte, domain, schemaType string) string {
	parts := []string{targetColumn, description, jsonArrayText(aliases), jsonArrayText(examples), domain, schemaType}
	return sanitizeSearchText(strings.Join(parts, " "))
}

func mappingExampleIndexText(sourceColumn, sheetName string, sampleValues, neighborColumns []byte, targetColumn, decision string) string {
	parts := []string{
		sourceColumn,
		sheetName,
		jsonArrayText(sampleValues),
		jsonArrayText(neighborColumns),
		targetColumn,
		decision,
	}
	return sanitizeSearchText(strings.Join(parts, " "))
}

func trimStringList(values []string, maxItems int, maxRunes int) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		runes := []rune(trimmed)
		if maxRunes > 0 && len(runes) > maxRunes {
			trimmed = string(runes[:maxRunes])
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
		if maxItems > 0 && len(out) >= maxItems {
			break
		}
	}
	return out
}

func jsonArrayText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return string(raw)
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			parts = append(parts, typed)
		default:
			encoded, err := json.Marshal(typed)
			if err == nil {
				parts = append(parts, string(encoded))
			}
		}
	}
	return strings.Join(parts, " ")
}

func (s *DataPipelineService) Get(ctx context.Context, tenantID int64, publicID string) (DataPipelineDetail, error) {
	if s == nil || s.queries == nil {
		return DataPipelineDetail{}, fmt.Errorf("data pipeline service is not configured")
	}
	row, err := s.getPipelineRow(ctx, tenantID, publicID)
	if err != nil {
		return DataPipelineDetail{}, err
	}
	pipeline := dataPipelineFromDB(row)
	versions, err := s.listVersionsForPipeline(ctx, tenantID, row.ID, 20)
	if err != nil {
		return DataPipelineDetail{}, err
	}
	runs, err := s.ListRuns(ctx, tenantID, publicID, 25)
	if err != nil {
		return DataPipelineDetail{}, err
	}
	schedules, err := s.ListSchedules(ctx, tenantID, publicID)
	if err != nil {
		return DataPipelineDetail{}, err
	}
	var published *DataPipelineVersion
	if row.PublishedVersionID.Valid {
		versionRow, err := s.queries.GetDataPipelineVersionByIDForTenant(ctx, db.GetDataPipelineVersionByIDForTenantParams{TenantID: tenantID, ID: row.PublishedVersionID.Int64})
		if err == nil {
			item, err := dataPipelineVersionFromDB(versionRow)
			if err != nil {
				return DataPipelineDetail{}, err
			}
			published = &item
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return DataPipelineDetail{}, fmt.Errorf("get published data pipeline version: %w", err)
		}
	}
	return DataPipelineDetail{
		Pipeline:         pipeline,
		PublishedVersion: published,
		Versions:         versions,
		Runs:             runs,
		Schedules:        schedules,
	}, nil
}

func (s *DataPipelineService) Update(ctx context.Context, tenantID, userID int64, publicID string, input DataPipelineInput, auditCtx AuditContext) (DataPipeline, error) {
	normalized, err := normalizeDataPipelineInput(input)
	if err != nil {
		return DataPipeline{}, err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return DataPipeline{}, ErrDataPipelineNotFound
	}
	row, err := s.queries.UpdateDataPipeline(ctx, db.UpdateDataPipelineParams{
		Name:            normalized.Name,
		Description:     normalized.Description,
		UpdatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		TenantID:        tenantID,
		PublicID:        parsed,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DataPipeline{}, ErrDataPipelineNotFound
	}
	if err != nil {
		return DataPipeline{}, fmt.Errorf("update data pipeline: %w", err)
	}
	item := dataPipelineFromDB(row)
	s.recordAudit(ctx, auditCtx, "data_pipeline.update", "data_pipeline", item.PublicID, nil)
	return item, nil
}

func (s *DataPipelineService) SaveDraftVersion(ctx context.Context, tenantID, userID int64, pipelinePublicID string, graph DataPipelineGraph, auditCtx AuditContext) (DataPipelineVersion, error) {
	if s == nil || s.queries == nil {
		return DataPipelineVersion{}, fmt.Errorf("data pipeline service is not configured")
	}
	pipeline, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	summary := validateDataPipelineDraftGraph(graph)
	if !summary.Valid {
		return DataPipelineVersion{}, fmt.Errorf("%w: %s", ErrInvalidDataPipelineGraph, strings.Join(summary.Errors, "; "))
	}
	graphJSON, err := encodeDataPipelineJSON(graph)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	summaryJSON, err := encodeDataPipelineJSON(summary)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	row, err := s.queries.CreateDataPipelineVersion(ctx, db.CreateDataPipelineVersionParams{
		TenantID:          tenantID,
		PipelineID:        pipeline.ID,
		Graph:             graphJSON,
		ValidationSummary: summaryJSON,
		CreatedByUserID:   pgtype.Int8{Int64: userID, Valid: userID > 0},
	})
	if err != nil {
		return DataPipelineVersion{}, fmt.Errorf("create data pipeline version: %w", err)
	}
	item, err := dataPipelineVersionFromDB(row)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	s.recordAudit(ctx, auditCtx, "data_pipeline.version.save", "data_pipeline_version", item.PublicID, nil)
	return item, nil
}

func (s *DataPipelineService) PublishVersion(ctx context.Context, tenantID, userID int64, versionPublicID string, auditCtx AuditContext) (DataPipelineVersion, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return DataPipelineVersion{}, fmt.Errorf("data pipeline service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(versionPublicID))
	if err != nil {
		return DataPipelineVersion{}, ErrDataPipelineVersionNotFound
	}
	version, err := s.queries.GetDataPipelineVersionForTenant(ctx, db.GetDataPipelineVersionForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return DataPipelineVersion{}, ErrDataPipelineVersionNotFound
	}
	if err != nil {
		return DataPipelineVersion{}, fmt.Errorf("get data pipeline version: %w", err)
	}
	if err := s.checkPipelineByID(ctx, tenantID, userID, version.PipelineID, DataActionPublishVersion); err != nil {
		return DataPipelineVersion{}, err
	}
	graph, err := decodeDataPipelineGraph(version.Graph)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	summary := validateDataPipelineGraph(graph)
	if !summary.Valid {
		return DataPipelineVersion{}, fmt.Errorf("%w: %s", ErrInvalidDataPipelineGraph, strings.Join(summary.Errors, "; "))
	}
	summaryJSON, err := encodeDataPipelineJSON(summary)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DataPipelineVersion{}, fmt.Errorf("begin data pipeline publish transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	published, err := qtx.PublishDataPipelineVersion(ctx, db.PublishDataPipelineVersionParams{
		ValidationSummary: summaryJSON,
		PublishedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		TenantID:          tenantID,
		PublicID:          parsed,
	})
	if err != nil {
		return DataPipelineVersion{}, fmt.Errorf("publish data pipeline version: %w", err)
	}
	if err := qtx.ArchivePublishedDataPipelineVersionsExcept(ctx, db.ArchivePublishedDataPipelineVersionsExceptParams{TenantID: tenantID, PipelineID: version.PipelineID, VersionID: version.ID}); err != nil {
		return DataPipelineVersion{}, fmt.Errorf("archive previous data pipeline versions: %w", err)
	}
	if _, err := qtx.SetDataPipelinePublishedVersion(ctx, db.SetDataPipelinePublishedVersionParams{
		VersionID:       pgtype.Int8{Int64: version.ID, Valid: true},
		UpdatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		TenantID:        tenantID,
		PipelineID:      version.PipelineID,
	}); err != nil {
		return DataPipelineVersion{}, fmt.Errorf("set pipeline published version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return DataPipelineVersion{}, fmt.Errorf("commit data pipeline publish transaction: %w", err)
	}
	item, err := dataPipelineVersionFromDB(published)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	s.recordAudit(ctx, auditCtx, "data_pipeline.version.publish", "data_pipeline_version", item.PublicID, nil)
	return item, nil
}

func (s *DataPipelineService) Preview(ctx context.Context, tenantID, actorUserID int64, versionPublicID, nodeID string, limit int32) (DataPipelinePreview, error) {
	version, err := s.getVersionRow(ctx, tenantID, versionPublicID)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	if err := s.checkPipelineByID(ctx, tenantID, actorUserID, version.PipelineID, DataActionPreview); err != nil {
		return DataPipelinePreview{}, err
	}
	graph, err := decodeDataPipelineGraph(version.Graph)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	return s.previewGraph(ctx, tenantID, actorUserID, graph, nodeID, limit)
}

func (s *DataPipelineService) PreviewDraft(ctx context.Context, tenantID, actorUserID int64, pipelinePublicID string, graph DataPipelineGraph, nodeID string, limit int32) (DataPipelinePreview, error) {
	if _, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID); err != nil {
		return DataPipelinePreview{}, err
	}
	if s.authz != nil {
		if err := s.authz.CheckResourceAction(ctx, tenantID, actorUserID, DataResourceDataPipeline, pipelinePublicID, DataActionPreview); err != nil {
			return DataPipelinePreview{}, err
		}
	}
	return s.previewGraph(ctx, tenantID, actorUserID, graph, nodeID, limit)
}

func (s *DataPipelineService) previewGraph(ctx context.Context, tenantID, actorUserID int64, graph DataPipelineGraph, nodeID string, limit int32) (DataPipelinePreview, error) {
	previewGraph, selectedNodeID, err := dataPipelinePreviewSubgraph(graph, nodeID)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	summary := validateDataPipelinePreviewGraph(previewGraph)
	if !summary.Valid {
		return DataPipelinePreview{}, fmt.Errorf("%w: %s", ErrInvalidDataPipelineGraph, strings.Join(summary.Errors, "; "))
	}
	if err := s.checkGraphInputPermissions(ctx, tenantID, actorUserID, previewGraph); err != nil {
		return DataPipelinePreview{}, err
	}
	outputSchemas, err := s.inferOutputSchemas(ctx, tenantID, previewGraph)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	if dataPipelineGraphNeedsHybrid(previewGraph) {
		preview, err := s.previewHybridGraph(ctx, tenantID, actorUserID, previewGraph, selectedNodeID, limit)
		if err != nil {
			return DataPipelinePreview{}, err
		}
		preview.OutputSchemas = outputSchemas
		return preview, nil
	}
	compiled, err := s.compilePreviewSelect(ctx, tenantID, previewGraph, selectedNodeID, limit)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	if err := s.datasets.ensureTenantSandbox(ctx, tenantID); err != nil {
		return DataPipelinePreview{}, err
	}
	conn, err := s.datasets.openTenantConn(ctx, tenantID)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	defer conn.Close()
	rows, err := conn.Query(clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.querySettings())), compiled.SQL)
	if err != nil {
		return DataPipelinePreview{}, fmt.Errorf("preview data pipeline: %w", err)
	}
	defer rows.Close()
	columns, previewRows, err := scanDatasetRows(rows, int(limit))
	if err != nil {
		return DataPipelinePreview{}, err
	}
	return DataPipelinePreview{
		NodeID:        compiled.NodeID,
		StepType:      compiled.StepType,
		Columns:       columns,
		PreviewRows:   previewRows,
		OutputSchemas: outputSchemas,
	}, nil
}

func (s *DataPipelineService) RequestRun(ctx context.Context, tenantID int64, userID *int64, versionPublicID string, triggerKind string, scheduleID *int64, auditCtx AuditContext) (DataPipelineRun, error) {
	if s == nil || s.pool == nil || s.queries == nil || s.outbox == nil {
		return DataPipelineRun{}, fmt.Errorf("data pipeline service is not configured")
	}
	version, err := s.getVersionRow(ctx, tenantID, versionPublicID)
	if err != nil {
		return DataPipelineRun{}, err
	}
	if version.Status != "published" {
		return DataPipelineRun{}, ErrDataPipelineVersionUnpublished
	}
	pipeline, err := s.queries.GetDataPipelineByIDForTenant(ctx, db.GetDataPipelineByIDForTenantParams{TenantID: tenantID, ID: version.PipelineID})
	if err != nil {
		return DataPipelineRun{}, fmt.Errorf("get data pipeline for run: %w", err)
	}
	if !pipeline.PublishedVersionID.Valid || pipeline.PublishedVersionID.Int64 != version.ID {
		return DataPipelineRun{}, ErrDataPipelineVersionUnpublished
	}
	if userID != nil {
		if err := s.authzCheckResource(ctx, tenantID, *userID, DataResourceDataPipeline, pipeline.PublicID.String(), DataActionRun); err != nil {
			return DataPipelineRun{}, err
		}
	}
	graph, err := decodeDataPipelineGraph(version.Graph)
	if err != nil {
		return DataPipelineRun{}, err
	}
	summary := validateDataPipelineGraph(graph)
	if !summary.Valid {
		return DataPipelineRun{}, fmt.Errorf("%w: %s", ErrInvalidDataPipelineGraph, strings.Join(summary.Errors, "; "))
	}
	if userID != nil {
		if err := s.checkGraphInputPermissions(ctx, tenantID, *userID, graph); err != nil {
			return DataPipelineRun{}, err
		}
		if dataPipelineGraphHasOutput(graph) {
			if err := s.authzCheckScope(ctx, tenantID, *userID, DataActionCreateWorkTable); err != nil {
				return DataPipelineRun{}, err
			}
		}
	}
	if triggerKind == "" {
		triggerKind = "manual"
	}
	if triggerKind != "manual" && triggerKind != "scheduled" {
		return DataPipelineRun{}, fmt.Errorf("%w: unsupported trigger kind", ErrInvalidDataPipelineInput)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DataPipelineRun{}, fmt.Errorf("begin data pipeline run transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	run, err := s.createRunWithQueries(ctx, qtx, tenantID, version, userID, triggerKind, scheduleID)
	if err != nil {
		return DataPipelineRun{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DataPipelineRun{}, fmt.Errorf("commit data pipeline run transaction: %w", err)
	}
	item := dataPipelineRunFromDB(run)
	s.recordAudit(ctx, auditCtx, "data_pipeline.run.request", "data_pipeline_run", item.PublicID, map[string]any{"triggerKind": triggerKind})
	return item, nil
}

func (s *DataPipelineService) ListRuns(ctx context.Context, tenantID int64, pipelinePublicID string, limit int32) ([]DataPipelineRun, error) {
	pipeline, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.queries.ListDataPipelineRuns(ctx, db.ListDataPipelineRunsParams{TenantID: tenantID, PipelineID: pipeline.ID, LimitCount: limit})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline runs: %w", err)
	}
	items := make([]DataPipelineRun, 0, len(rows))
	for _, row := range rows {
		item := dataPipelineRunFromDB(row)
		steps, err := s.listRunSteps(ctx, tenantID, row.ID)
		if err != nil {
			return nil, err
		}
		item.Steps = steps
		outputs, err := s.listRunOutputs(ctx, tenantID, row.ID)
		if err != nil {
			return nil, err
		}
		item.Outputs = outputs
		items = append(items, item)
	}
	return items, nil
}

func (s *DataPipelineService) ListReviewItems(ctx context.Context, tenantID int64, pipelinePublicID string, input DataPipelineReviewItemListInput) ([]DataPipelineReviewItem, error) {
	pipeline, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID)
	if err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var status pgtype.Text
	if input.Status != "" {
		status = pgtype.Text{String: input.Status, Valid: true}
	}
	rows, err := s.queries.ListDataPipelineReviewItems(ctx, db.ListDataPipelineReviewItemsParams{
		TenantID:         tenantID,
		PipelinePublicID: pipeline.PublicID,
		Status:           status,
		ResultLimit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline review items: %w", err)
	}
	items := make([]DataPipelineReviewItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dataPipelineReviewItemFromDB(row))
	}
	return items, nil
}

func (s *DataPipelineService) ListReviewItemsByDriveFile(ctx context.Context, tenantID int64, filePublicID string, input DataPipelineReviewItemListInput) ([]DataPipelineReviewItem, error) {
	filePublicID = strings.TrimSpace(filePublicID)
	if filePublicID == "" {
		return nil, fmt.Errorf("%w: drive file public id is required", ErrInvalidDataPipelineInput)
	}
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var status pgtype.Text
	if input.Status != "" {
		status = pgtype.Text{String: input.Status, Valid: true}
	}
	rows, err := s.queries.ListDataPipelineReviewItemsByDriveFile(ctx, db.ListDataPipelineReviewItemsByDriveFileParams{
		TenantID:     tenantID,
		FilePublicID: filePublicID,
		Status:       status,
		ResultLimit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline review items by drive file: %w", err)
	}
	items := make([]DataPipelineReviewItem, 0, len(rows))
	for _, row := range rows {
		item := dataPipelineReviewItemFromDB(row.DataPipelineReviewItem)
		item.PipelinePublicID = row.PipelinePublicID.String()
		item.PipelineName = row.PipelineName
		item.RunPublicID = row.RunPublicID.String()
		items = append(items, item)
	}
	return items, nil
}

func (s *DataPipelineService) GetReviewItem(ctx context.Context, tenantID, actorUserID int64, publicID string) (DataPipelineReviewItem, error) {
	row, err := s.getReviewItemRow(ctx, tenantID, publicID)
	if err != nil {
		return DataPipelineReviewItem{}, err
	}
	pipeline, err := s.checkReviewItemPipelineAction(ctx, tenantID, actorUserID, row.PipelineID, DataActionView)
	if err != nil {
		return DataPipelineReviewItem{}, err
	}
	item := dataPipelineReviewItemFromDB(row)
	item.PipelinePublicID = pipeline.PublicID.String()
	item.PipelineName = pipeline.Name
	comments, err := s.listReviewItemComments(ctx, tenantID, row.ID)
	if err != nil {
		return DataPipelineReviewItem{}, err
	}
	item.Comments = comments
	return item, nil
}

func (s *DataPipelineService) TransitionReviewItem(ctx context.Context, tenantID, actorUserID int64, publicID string, input DataPipelineReviewItemTransitionInput, auditCtx AuditContext) (DataPipelineReviewItem, error) {
	status := strings.TrimSpace(input.Status)
	if !dataPipelineReviewItemStatusAllowed(status) {
		return DataPipelineReviewItem{}, fmt.Errorf("%w: unsupported review item status", ErrInvalidDataPipelineInput)
	}
	comment := strings.TrimSpace(input.Comment)
	rowID, err := uuid.Parse(publicID)
	if err != nil {
		return DataPipelineReviewItem{}, ErrDataPipelineReviewItemNotFound
	}
	existing, err := s.getReviewItemRow(ctx, tenantID, publicID)
	if err != nil {
		return DataPipelineReviewItem{}, err
	}
	pipeline, err := s.checkReviewItemPipelineAction(ctx, tenantID, actorUserID, existing.PipelineID, DataActionUpdate)
	if err != nil {
		return DataPipelineReviewItem{}, err
	}
	row, err := s.queries.TransitionDataPipelineReviewItem(ctx, db.TransitionDataPipelineReviewItemParams{
		TenantID:        tenantID,
		PublicID:        rowID,
		Status:          status,
		DecisionComment: comment,
		UpdatedByUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DataPipelineReviewItem{}, ErrDataPipelineReviewItemNotFound
	}
	if err != nil {
		return DataPipelineReviewItem{}, fmt.Errorf("transition data pipeline review item: %w", err)
	}
	if comment != "" {
		if _, err := s.queries.CreateDataPipelineReviewItemComment(ctx, db.CreateDataPipelineReviewItemCommentParams{
			TenantID:     tenantID,
			ReviewItemID: row.ID,
			AuthorUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
			Body:         comment,
		}); err != nil {
			return DataPipelineReviewItem{}, fmt.Errorf("comment data pipeline review item: %w", err)
		}
	}
	item := dataPipelineReviewItemFromDB(row)
	item.PipelinePublicID = pipeline.PublicID.String()
	item.PipelineName = pipeline.Name
	item.Comments, _ = s.listReviewItemComments(ctx, tenantID, row.ID)
	s.recordAudit(ctx, auditCtx, "data_pipeline.review_item.transition", "data_pipeline_review_item", item.PublicID, map[string]any{"status": status})
	return item, nil
}

func (s *DataPipelineService) CreateReviewItemComment(ctx context.Context, tenantID, actorUserID int64, publicID, body string, auditCtx AuditContext) (DataPipelineReviewItemComment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return DataPipelineReviewItemComment{}, fmt.Errorf("%w: review item comment is required", ErrInvalidDataPipelineInput)
	}
	item, err := s.getReviewItemRow(ctx, tenantID, publicID)
	if err != nil {
		return DataPipelineReviewItemComment{}, err
	}
	if _, err := s.checkReviewItemPipelineAction(ctx, tenantID, actorUserID, item.PipelineID, DataActionUpdate); err != nil {
		return DataPipelineReviewItemComment{}, err
	}
	row, err := s.queries.CreateDataPipelineReviewItemComment(ctx, db.CreateDataPipelineReviewItemCommentParams{
		TenantID:     tenantID,
		ReviewItemID: item.ID,
		AuthorUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
		Body:         body,
	})
	if err != nil {
		return DataPipelineReviewItemComment{}, fmt.Errorf("create data pipeline review item comment: %w", err)
	}
	comment := dataPipelineReviewItemCommentFromDB(row)
	s.recordAudit(ctx, auditCtx, "data_pipeline.review_item.comment", "data_pipeline_review_item", publicID, map[string]any{"commentPublicID": comment.PublicID})
	return comment, nil
}

func (s *DataPipelineService) CreateSchedule(ctx context.Context, tenantID, userID int64, pipelinePublicID string, input DataPipelineScheduleInput, auditCtx AuditContext) (DataPipelineSchedule, error) {
	pipeline, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID)
	if err != nil {
		return DataPipelineSchedule{}, err
	}
	if err := s.authzCheckResource(ctx, tenantID, userID, DataResourceDataPipeline, pipeline.PublicID.String(), DataActionManageSchedule); err != nil {
		return DataPipelineSchedule{}, err
	}
	if !pipeline.PublishedVersionID.Valid {
		return DataPipelineSchedule{}, ErrDataPipelineVersionUnpublished
	}
	normalized, nextRun, err := normalizeDataPipelineScheduleInput(input, time.Now())
	if err != nil {
		return DataPipelineSchedule{}, err
	}
	enabled := true
	if normalized.Enabled != nil {
		enabled = *normalized.Enabled
	}
	row, err := s.queries.CreateDataPipelineSchedule(ctx, db.CreateDataPipelineScheduleParams{
		TenantID:        tenantID,
		PipelineID:      pipeline.ID,
		VersionID:       pipeline.PublishedVersionID.Int64,
		CreatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		Frequency:       normalized.Frequency,
		Timezone:        normalized.Timezone,
		RunTime:         normalized.RunTime,
		Weekday:         pgInt2(normalized.Weekday),
		MonthDay:        pgInt2(normalized.MonthDay),
		Enabled:         enabled,
		NextRunAt:       pgTimestamp(nextRun),
	})
	if err != nil {
		return DataPipelineSchedule{}, fmt.Errorf("create data pipeline schedule: %w", err)
	}
	item := dataPipelineScheduleFromDB(row)
	s.recordAudit(ctx, auditCtx, "data_pipeline.schedule.create", "data_pipeline_schedule", item.PublicID, nil)
	return item, nil
}

func (s *DataPipelineService) ListSchedules(ctx context.Context, tenantID int64, pipelinePublicID string) ([]DataPipelineSchedule, error) {
	pipeline, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDataPipelineSchedules(ctx, db.ListDataPipelineSchedulesParams{TenantID: tenantID, PipelineID: pipeline.ID})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline schedules: %w", err)
	}
	items := make([]DataPipelineSchedule, 0, len(rows))
	for _, row := range rows {
		items = append(items, dataPipelineScheduleFromDB(row))
	}
	return items, nil
}

func (s *DataPipelineService) UpdateSchedule(ctx context.Context, tenantID, userID int64, schedulePublicID string, input DataPipelineScheduleInput, auditCtx AuditContext) (DataPipelineSchedule, error) {
	existing, err := s.getScheduleRow(ctx, tenantID, schedulePublicID)
	if err != nil {
		return DataPipelineSchedule{}, err
	}
	if err := s.checkPipelineByID(ctx, tenantID, userID, existing.PipelineID, DataActionManageSchedule); err != nil {
		return DataPipelineSchedule{}, err
	}
	normalized, nextRun, err := normalizeDataPipelineScheduleInput(input, time.Now())
	if err != nil {
		return DataPipelineSchedule{}, err
	}
	enabled := existing.Enabled
	if normalized.Enabled != nil {
		enabled = *normalized.Enabled
	}
	parsed, _ := uuid.Parse(strings.TrimSpace(schedulePublicID))
	row, err := s.queries.UpdateDataPipelineSchedule(ctx, db.UpdateDataPipelineScheduleParams{
		Frequency: normalized.Frequency,
		Timezone:  normalized.Timezone,
		RunTime:   normalized.RunTime,
		Weekday:   pgInt2(normalized.Weekday),
		MonthDay:  pgInt2(normalized.MonthDay),
		Enabled:   enabled,
		NextRunAt: pgTimestamp(nextRun),
		TenantID:  tenantID,
		PublicID:  parsed,
	})
	if err != nil {
		return DataPipelineSchedule{}, fmt.Errorf("update data pipeline schedule: %w", err)
	}
	item := dataPipelineScheduleFromDB(row)
	s.recordAudit(ctx, auditCtx, "data_pipeline.schedule.update", "data_pipeline_schedule", item.PublicID, map[string]any{"actorUserID": userID})
	return item, nil
}

func (s *DataPipelineService) DisableSchedule(ctx context.Context, tenantID, userID int64, schedulePublicID string, auditCtx AuditContext) (DataPipelineSchedule, error) {
	existing, err := s.getScheduleRow(ctx, tenantID, schedulePublicID)
	if err != nil {
		return DataPipelineSchedule{}, err
	}
	if err := s.checkPipelineByID(ctx, tenantID, userID, existing.PipelineID, DataActionManageSchedule); err != nil {
		return DataPipelineSchedule{}, err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(schedulePublicID))
	if err != nil {
		return DataPipelineSchedule{}, ErrDataPipelineScheduleNotFound
	}
	row, err := s.queries.DisableDataPipelineSchedule(ctx, db.DisableDataPipelineScheduleParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return DataPipelineSchedule{}, ErrDataPipelineScheduleNotFound
	}
	if err != nil {
		return DataPipelineSchedule{}, fmt.Errorf("disable data pipeline schedule: %w", err)
	}
	item := dataPipelineScheduleFromDB(row)
	s.recordAudit(ctx, auditCtx, "data_pipeline.schedule.disable", "data_pipeline_schedule", item.PublicID, map[string]any{"actorUserID": userID})
	return item, nil
}

func (s *DataPipelineService) authzCheckResource(ctx context.Context, tenantID, actorUserID int64, resourceType, resourcePublicID, action string) error {
	if s == nil || s.authz == nil {
		return nil
	}
	return s.authz.CheckResourceAction(ctx, tenantID, actorUserID, resourceType, resourcePublicID, action)
}

func (s *DataPipelineService) authzCheckScope(ctx context.Context, tenantID, actorUserID int64, action string) error {
	if s == nil || s.authz == nil {
		return nil
	}
	return s.authz.CheckScopeAction(ctx, tenantID, actorUserID, action)
}

func (s *DataPipelineService) checkPipelineByID(ctx context.Context, tenantID, actorUserID, pipelineID int64, action string) error {
	if s == nil || s.authz == nil {
		return nil
	}
	pipeline, err := s.queries.GetDataPipelineByIDForTenant(ctx, db.GetDataPipelineByIDForTenantParams{TenantID: tenantID, ID: pipelineID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDataPipelineNotFound
	}
	if err != nil {
		return fmt.Errorf("get data pipeline for authorization: %w", err)
	}
	return s.authz.CheckResourceAction(ctx, tenantID, actorUserID, DataResourceDataPipeline, pipeline.PublicID.String(), action)
}

func (s *DataPipelineService) checkReviewItemPipelineAction(ctx context.Context, tenantID, actorUserID, pipelineID int64, action string) (db.DataPipeline, error) {
	pipeline, err := s.queries.GetDataPipelineByIDForTenant(ctx, db.GetDataPipelineByIDForTenantParams{TenantID: tenantID, ID: pipelineID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DataPipeline{}, ErrDataPipelineNotFound
	}
	if err != nil {
		return db.DataPipeline{}, fmt.Errorf("get data pipeline for review item authorization: %w", err)
	}
	if s == nil || s.authz == nil {
		return pipeline, nil
	}
	if err := s.authz.CheckResourceAction(ctx, tenantID, actorUserID, DataResourceDataPipeline, pipeline.PublicID.String(), action); err != nil {
		return db.DataPipeline{}, err
	}
	return pipeline, nil
}

func (s *DataPipelineService) checkGraphInputPermissions(ctx context.Context, tenantID, actorUserID int64, graph DataPipelineGraph) error {
	if s == nil || s.authz == nil || actorUserID <= 0 {
		return nil
	}
	for _, node := range graph.Nodes {
		if node.Data.StepType != DataPipelineStepInput {
			continue
		}
		config := node.Data.Config
		switch dataPipelineString(config, "sourceKind") {
		case "dataset":
			publicID := dataPipelineString(config, "datasetPublicId")
			if err := s.authz.CheckResourceAction(ctx, tenantID, actorUserID, DataResourceDataset, publicID, DataActionView); err != nil {
				return err
			}
			if err := s.authz.CheckResourceAction(ctx, tenantID, actorUserID, DataResourceDataset, publicID, DataActionQuery); err != nil {
				return err
			}
		case "work_table":
			publicID := dataPipelineString(config, "workTablePublicId")
			if err := s.authz.CheckResourceAction(ctx, tenantID, actorUserID, DataResourceWorkTable, publicID, DataActionView); err != nil {
				return err
			}
			if err := s.authz.CheckResourceAction(ctx, tenantID, actorUserID, DataResourceWorkTable, publicID, DataActionQuery); err != nil {
				return err
			}
		}
	}
	return nil
}

func dataPipelineGraphHasOutput(graph DataPipelineGraph) bool {
	for _, node := range graph.Nodes {
		if node.Data.StepType == DataPipelineStepOutput {
			return true
		}
	}
	return false
}

func (s *DataPipelineService) resolveDataPipelineRuntimeWatermarks(ctx context.Context, tenantID int64, run db.DataPipelineRun, graph DataPipelineGraph) (DataPipelineGraph, error) {
	if s == nil || s.pool == nil {
		return graph, nil
	}
	for i := range graph.Nodes {
		node := graph.Nodes[i]
		if node.Data.StepType != DataPipelineStepWatermarkFilter {
			continue
		}
		source := firstNonEmpty(dataPipelineString(node.Data.Config, "watermarkSource"), "fixed")
		if source != "previous_success" {
			continue
		}
		config := cloneDataPipelineConfig(node.Data.Config)
		initialValue := firstNonEmpty(dataPipelineString(config, "initialWatermarkValue"), dataPipelineString(config, "watermarkValue"), dataPipelineString(config, "value"))
		value, previousRunPublicID, err := s.previousDataPipelineWatermarkValue(ctx, tenantID, run, node.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return graph, fmt.Errorf("resolve previous watermark for node %s: %w", node.ID, err)
		}
		if value != "" {
			config["watermarkValue"] = value
			config["resolvedWatermarkSource"] = "previous_success"
			config["previousRunPublicId"] = previousRunPublicID
		} else {
			config["watermarkValue"] = initialValue
			config["resolvedWatermarkSource"] = "initial"
			config["previousRunPublicId"] = ""
		}
		graph.Nodes[i].Data.Config = config
	}
	return graph, nil
}

func (s *DataPipelineService) previousDataPipelineWatermarkValue(ctx context.Context, tenantID int64, run db.DataPipelineRun, nodeID string) (string, string, error) {
	const query = `
SELECT r.public_id::text, rs.metadata #>> '{watermarkFilter,nextWatermarkValue}'
FROM data_pipeline_runs r
JOIN data_pipeline_run_steps rs ON rs.run_id = r.id AND rs.tenant_id = r.tenant_id
WHERE r.tenant_id = $1
  AND r.pipeline_id = $2
  AND r.id <> $3
  AND r.status = 'completed'
  AND rs.node_id = $4
  AND rs.status = 'completed'
  AND COALESCE(rs.metadata #>> '{watermarkFilter,nextWatermarkValue}', '') <> ''
ORDER BY r.completed_at DESC NULLS LAST, r.id DESC
LIMIT 1`
	var previousRunPublicID string
	var value string
	if err := s.pool.QueryRow(ctx, query, tenantID, run.PipelineID, run.ID, nodeID).Scan(&previousRunPublicID, &value); err != nil {
		return "", "", err
	}
	return value, previousRunPublicID, nil
}

func (s *DataPipelineService) HandleRunRequested(ctx context.Context, tenantID, runID, outboxEventID int64) error {
	run, err := s.queries.MarkDataPipelineRunProcessing(ctx, db.MarkDataPipelineRunProcessingParams{TenantID: tenantID, ID: runID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDataPipelineRunNotFound
	}
	if err != nil {
		return fmt.Errorf("mark data pipeline run processing: %w", err)
	}
	version, err := s.queries.GetDataPipelineVersionByIDForTenant(ctx, db.GetDataPipelineVersionByIDForTenantParams{TenantID: tenantID, ID: run.VersionID})
	if err != nil {
		s.failRunBestEffort(ctx, tenantID, runID, err.Error())
		return fmt.Errorf("get data pipeline run version: %w", err)
	}
	graph, err := decodeDataPipelineGraph(version.Graph)
	if err != nil {
		s.failRunBestEffort(ctx, tenantID, runID, err.Error())
		return err
	}
	for _, node := range graph.Nodes {
		if _, err := s.queries.CreateDataPipelineRunStep(ctx, db.CreateDataPipelineRunStepParams{TenantID: tenantID, RunID: runID, NodeID: node.ID, StepType: node.Data.StepType}); err != nil {
			s.failRunBestEffort(ctx, tenantID, runID, err.Error())
			return fmt.Errorf("create data pipeline run step: %w", err)
		}
		_, _ = s.queries.MarkDataPipelineRunStepProcessing(ctx, db.MarkDataPipelineRunStepProcessingParams{TenantID: tenantID, RunID: runID, NodeID: node.ID})
	}
	outputNodes := dataPipelineOutputNodes(graph)
	for _, node := range outputNodes {
		if _, err := s.queries.CreateDataPipelineRunOutput(ctx, db.CreateDataPipelineRunOutputParams{TenantID: tenantID, RunID: runID, NodeID: node.ID}); err != nil {
			s.failRunBestEffort(ctx, tenantID, runID, err.Error())
			return fmt.Errorf("create data pipeline run output: %w", err)
		}
	}

	results, err := s.executeRun(ctx, tenantID, run, version)
	var firstSuccessWorkTable *DatasetWorkTable
	var totalRows int64
	var failures []string
	if err != nil {
		errorSample, _ := encodeDataPipelineJSON([]map[string]any{{"error": err.Error()}})
		for _, node := range graph.Nodes {
			_, _ = s.queries.FailDataPipelineRunStep(ctx, db.FailDataPipelineRunStepParams{TenantID: tenantID, RunID: runID, NodeID: node.ID, ErrorSummary: err.Error(), ErrorSample: errorSample})
		}
		s.failRunBestEffort(ctx, tenantID, runID, err.Error())
		return err
	}
	for _, result := range results {
		if result.Err != nil {
			failures = append(failures, result.Node.ID+": "+result.Err.Error())
			meta, _ := encodeDataPipelineJSON(map[string]any{"nodeId": result.Node.ID})
			_, _ = s.queries.FailDataPipelineRunOutput(ctx, db.FailDataPipelineRunOutputParams{TenantID: tenantID, RunID: runID, NodeID: result.Node.ID, ErrorSummary: result.Err.Error(), Metadata: meta})
			s.recordMedallionRun(ctx, tenantID, run, version, result.Compiled, nil, MedallionPipelineStatusFailed, result.Err.Error(), result.Node.ID)
			continue
		}
		totalRows += result.WorkTable.TotalRows
		if firstSuccessWorkTable == nil {
			wt := result.WorkTable
			firstSuccessWorkTable = &wt
		}
		outputMetadata := map[string]any{
			"nodeId":            result.Node.ID,
			"workTablePublicId": result.WorkTable.PublicID,
			"displayName":       result.WorkTable.DisplayName,
			"database":          result.WorkTable.Database,
			"tableName":         result.WorkTable.Table,
			"writeMode":         dataPipelineOutputWriteMode(result.Node),
		}
		if dataPipelineOutputWriteMode(result.Node) == "scd2_merge" {
			uniqueKeys := dataPipelineUniqueStrings(dataPipelineStringSlice(result.Node.Data.Config, "uniqueKeys"))
			if len(uniqueKeys) == 0 {
				uniqueKeys = dataPipelineUniqueStrings(dataPipelineStringSlice(result.Node.Data.Config, "scd2UniqueKeys"))
			}
			if len(uniqueKeys) > 0 {
				outputMetadata["scd2UniqueKeys"] = uniqueKeys
			}
			outputMetadata["scd2MergePolicy"] = firstNonEmpty(dataPipelineString(result.Node.Data.Config, "scd2MergePolicy"), dataPipelineString(result.Node.Data.Config, "mergePolicy"), "current_only")
			outputMetadata["validFromColumn"] = firstNonEmpty(dataPipelineString(result.Node.Data.Config, "validFromColumn"), "valid_from")
			outputMetadata["validToColumn"] = firstNonEmpty(dataPipelineString(result.Node.Data.Config, "validToColumn"), "valid_to")
			outputMetadata["isCurrentColumn"] = firstNonEmpty(dataPipelineString(result.Node.Data.Config, "isCurrentColumn"), "is_current")
			outputMetadata["changeHashColumn"] = firstNonEmpty(dataPipelineString(result.Node.Data.Config, "changeHashColumn"), "change_hash")
		}
		meta, _ := encodeDataPipelineJSON(outputMetadata)
		_, _ = s.queries.CompleteDataPipelineRunOutput(ctx, db.CompleteDataPipelineRunOutputParams{TenantID: tenantID, RunID: runID, NodeID: result.Node.ID, OutputWorkTableID: pgtype.Int8{Int64: result.WorkTable.ID, Valid: true}, RowCount: result.WorkTable.TotalRows, Metadata: meta})
		s.recordMedallionRun(ctx, tenantID, run, version, result.Compiled, &result.WorkTable, MedallionPipelineStatusCompleted, "", result.Node.ID)
	}
	nodeResults := make(map[string]dataPipelineRunNodeResult)
	for _, result := range results {
		for nodeID, nodeResult := range result.NodeResults {
			nodeResults[nodeID] = nodeResult
		}
	}
	if len(failures) == 0 {
		if err := s.persistDataPipelineReviewItems(ctx, tenantID, run, version, nodeResults); err != nil {
			s.failRunBestEffort(ctx, tenantID, runID, err.Error())
			return fmt.Errorf("persist data pipeline review items: %w", err)
		}
	}
	for _, node := range graph.Nodes {
		if len(failures) > 0 {
			errorSample, _ := encodeDataPipelineJSON([]map[string]any{{"error": strings.Join(failures, "; ")}})
			_, _ = s.queries.FailDataPipelineRunStep(ctx, db.FailDataPipelineRunStepParams{TenantID: tenantID, RunID: runID, NodeID: node.ID, ErrorSummary: strings.Join(failures, "; "), ErrorSample: errorSample})
			continue
		}
		nodeResult, ok := nodeResults[node.ID]
		if !ok {
			nodeResult = dataPipelineRunNodeResult{
				NodeID:   node.ID,
				StepType: node.Data.StepType,
				RowCount: totalRows,
				Metadata: map[string]any{"outputRows": totalRows, "warnings": []string{}},
			}
		}
		meta, _ := encodeDataPipelineJSON(nodeResult.Metadata)
		_, _ = s.queries.CompleteDataPipelineRunStep(ctx, db.CompleteDataPipelineRunStepParams{TenantID: tenantID, RunID: runID, NodeID: node.ID, RowCount: nodeResult.RowCount, Metadata: meta})
	}
	status := "completed"
	errorSummary := ""
	if len(failures) > 0 {
		status = "failed"
		errorSummary = strings.Join(failures, "; ")
	}
	var outputWorkTableID pgtype.Int8
	if firstSuccessWorkTable != nil {
		outputWorkTableID = pgtype.Int8{Int64: firstSuccessWorkTable.ID, Valid: true}
	}
	completed, err := s.queries.FinishDataPipelineRun(ctx, db.FinishDataPipelineRunParams{TenantID: tenantID, ID: runID, Status: status, OutputWorkTableID: outputWorkTableID, RowCount: totalRows, ErrorSummary: errorSummary})
	if err != nil {
		return fmt.Errorf("complete data pipeline run: %w", err)
	}
	s.recordAudit(ctx, AuditContext{ActorType: "system", TenantID: &tenantID}, "data_pipeline.run.complete", "data_pipeline_run", completed.PublicID.String(), map[string]any{"outboxEventID": outboxEventID})
	return nil
}

func (s *DataPipelineService) RunDueSchedules(ctx context.Context, now time.Time, batchSize int32) (DataPipelineScheduleRunSummary, error) {
	var summary DataPipelineScheduleRunSummary
	if s == nil || s.pool == nil || s.queries == nil || s.outbox == nil {
		return summary, fmt.Errorf("data pipeline service is not configured")
	}
	if now.IsZero() {
		now = time.Now()
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return summary, fmt.Errorf("begin data pipeline schedule transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	schedules, err := qtx.ClaimDueDataPipelineSchedules(ctx, db.ClaimDueDataPipelineSchedulesParams{Now: pgTimestamp(now), BatchLimit: batchSize})
	if err != nil {
		return summary, fmt.Errorf("claim data pipeline schedules: %w", err)
	}
	summary.Claimed = len(schedules)
	for _, schedule := range schedules {
		nextRun, err := nextDataPipelineScheduleRunAfter(schedule.Frequency, schedule.Timezone, schedule.RunTime, optionalPgInt2(schedule.Weekday), optionalPgInt2(schedule.MonthDay), now)
		if err != nil {
			_, markErr := qtx.MarkDataPipelineScheduleFailed(ctx, db.MarkDataPipelineScheduleFailedParams{Enabled: false, LastRunAt: pgTimestamp(now), LastStatus: pgText("disabled"), ErrorSummary: err.Error(), NextRunAt: schedule.NextRunAt, TenantID: schedule.TenantID, ID: schedule.ID})
			if markErr != nil {
				return summary, markErr
			}
			summary.Failed++
			summary.Disabled++
			continue
		}
		pipeline, err := qtx.GetDataPipelineByIDForTenant(ctx, db.GetDataPipelineByIDForTenantParams{TenantID: schedule.TenantID, ID: schedule.PipelineID})
		if errors.Is(err, pgx.ErrNoRows) {
			_, markErr := qtx.MarkDataPipelineScheduleFailed(ctx, db.MarkDataPipelineScheduleFailedParams{Enabled: false, LastRunAt: pgTimestamp(now), LastStatus: pgText("disabled"), ErrorSummary: ErrDataPipelineVersionUnpublished.Error(), NextRunAt: pgTimestamp(nextRun), TenantID: schedule.TenantID, ID: schedule.ID})
			if markErr != nil {
				return summary, markErr
			}
			summary.Disabled++
			continue
		}
		if err != nil {
			return summary, fmt.Errorf("get scheduled data pipeline: %w", err)
		}
		if !pipeline.PublishedVersionID.Valid || pipeline.PublishedVersionID.Int64 != schedule.VersionID {
			_, markErr := qtx.MarkDataPipelineScheduleFailed(ctx, db.MarkDataPipelineScheduleFailedParams{Enabled: false, LastRunAt: pgTimestamp(now), LastStatus: pgText("disabled"), ErrorSummary: ErrDataPipelineVersionUnpublished.Error(), NextRunAt: pgTimestamp(nextRun), TenantID: schedule.TenantID, ID: schedule.ID})
			if markErr != nil {
				return summary, markErr
			}
			summary.Disabled++
			continue
		}
		active, err := qtx.CountActiveDataPipelineRunsForSchedule(ctx, db.CountActiveDataPipelineRunsForScheduleParams{
			TenantID:   schedule.TenantID,
			ScheduleID: pgtype.Int8{Int64: schedule.ID, Valid: true},
		})
		if err != nil {
			return summary, fmt.Errorf("count active data pipeline runs: %w", err)
		}
		if active > 0 {
			if _, err := qtx.MarkDataPipelineScheduleSkipped(ctx, db.MarkDataPipelineScheduleSkippedParams{LastRunAt: pgTimestamp(now), ErrorSummary: "previous scheduled run is still pending or processing", NextRunAt: pgTimestamp(nextRun), TenantID: schedule.TenantID, ID: schedule.ID}); err != nil {
				return summary, err
			}
			summary.Skipped++
			continue
		}
		version, err := qtx.GetDataPipelineVersionByIDForTenant(ctx, db.GetDataPipelineVersionByIDForTenantParams{TenantID: schedule.TenantID, ID: schedule.VersionID})
		if err != nil {
			return summary, fmt.Errorf("get scheduled data pipeline version: %w", err)
		}
		scheduleID := schedule.ID
		run, err := s.createRunWithQueries(ctx, qtx, schedule.TenantID, version, optionalPgInt8(schedule.CreatedByUserID), "scheduled", &scheduleID)
		if err != nil {
			return summary, err
		}
		if _, err := qtx.MarkDataPipelineScheduleCreated(ctx, db.MarkDataPipelineScheduleCreatedParams{LastRunAt: pgTimestamp(now), LastRunID: pgtype.Int8{Int64: run.ID, Valid: true}, NextRunAt: pgTimestamp(nextRun), TenantID: schedule.TenantID, ID: schedule.ID}); err != nil {
			return summary, err
		}
		summary.Created++
	}
	if err := tx.Commit(ctx); err != nil {
		return summary, fmt.Errorf("commit data pipeline schedule transaction: %w", err)
	}
	return summary, nil
}

func (s *DataPipelineService) createRunWithQueries(ctx context.Context, qtx *db.Queries, tenantID int64, version db.DataPipelineVersion, userID *int64, triggerKind string, scheduleID *int64) (db.DataPipelineRun, error) {
	run, err := qtx.CreateDataPipelineRun(ctx, db.CreateDataPipelineRunParams{
		TenantID:          tenantID,
		PipelineID:        version.PipelineID,
		VersionID:         version.ID,
		ScheduleID:        pgInt8(scheduleID),
		RequestedByUserID: pgInt8(userID),
		TriggerKind:       triggerKind,
	})
	if err != nil {
		return db.DataPipelineRun{}, fmt.Errorf("create data pipeline run: %w", err)
	}
	pipelinePublicID := ""
	pipeline, err := qtx.GetDataPipelineByIDForTenant(ctx, db.GetDataPipelineByIDForTenantParams{TenantID: tenantID, ID: version.PipelineID})
	if err != nil {
		return db.DataPipelineRun{}, fmt.Errorf("get data pipeline for run event: %w", err)
	}
	pipelinePublicID = pipeline.PublicID.String()
	event, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "data_pipeline_run",
		AggregateID:   run.PublicID.String(),
		EventType:     "data_pipeline.run_requested",
		Payload: map[string]any{
			"tenantId":         tenantID,
			"runId":            run.ID,
			"pipelinePublicId": pipelinePublicID,
			"versionPublicId":  version.PublicID.String(),
		},
	})
	if err != nil {
		return db.DataPipelineRun{}, err
	}
	run, err = qtx.SetDataPipelineRunOutboxEvent(ctx, db.SetDataPipelineRunOutboxEventParams{TenantID: tenantID, ID: run.ID, OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true}})
	if err != nil {
		return db.DataPipelineRun{}, fmt.Errorf("set data pipeline run outbox event: %w", err)
	}
	return run, nil
}

func (s *DataPipelineService) listVersionsForPipeline(ctx context.Context, tenantID, pipelineID int64, limit int32) ([]DataPipelineVersion, error) {
	rows, err := s.queries.ListDataPipelineVersions(ctx, db.ListDataPipelineVersionsParams{TenantID: tenantID, PipelineID: pipelineID, LimitCount: limit})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline versions: %w", err)
	}
	items := make([]DataPipelineVersion, 0, len(rows))
	for _, row := range rows {
		item, err := dataPipelineVersionFromDB(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *DataPipelineService) listRunSteps(ctx context.Context, tenantID, runID int64) ([]DataPipelineRunStep, error) {
	rows, err := s.queries.ListDataPipelineRunSteps(ctx, db.ListDataPipelineRunStepsParams{TenantID: tenantID, RunID: runID})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline run steps: %w", err)
	}
	items := make([]DataPipelineRunStep, 0, len(rows))
	for _, row := range rows {
		items = append(items, dataPipelineRunStepFromDB(row))
	}
	return items, nil
}

func (s *DataPipelineService) listRunOutputs(ctx context.Context, tenantID, runID int64) ([]DataPipelineRunOutput, error) {
	rows, err := s.queries.ListDataPipelineRunOutputs(ctx, db.ListDataPipelineRunOutputsParams{TenantID: tenantID, RunID: runID})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline run outputs: %w", err)
	}
	items := make([]DataPipelineRunOutput, 0, len(rows))
	for _, row := range rows {
		item := dataPipelineRunOutputFromDB(row)
		if item.OutputWorkTableID != nil {
			item.LatestGoldPublication = s.latestGoldPublicationForWorkTable(ctx, tenantID, *item.OutputWorkTableID)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *DataPipelineService) latestGoldPublicationForWorkTable(ctx context.Context, tenantID, workTableID int64) *DataPipelineRunOutputGoldPublication {
	if s == nil || s.queries == nil || workTableID <= 0 {
		return nil
	}
	rows, err := s.queries.ListDatasetGoldPublicationsForWorkTable(ctx, db.ListDatasetGoldPublicationsForWorkTableParams{
		TenantID:          tenantID,
		SourceWorkTableID: workTableID,
		LimitCount:        1,
	})
	if err != nil || len(rows) == 0 {
		return nil
	}
	row := rows[0]
	return &DataPipelineRunOutputGoldPublication{
		PublicID:     row.PublicID.String(),
		DisplayName:  row.DisplayName,
		Status:       row.Status,
		GoldDatabase: row.GoldDatabase,
		GoldTable:    row.GoldTable,
	}
}

func (s *DataPipelineService) getPipelineRow(ctx context.Context, tenantID int64, publicID string) (db.DataPipeline, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DataPipeline{}, ErrDataPipelineNotFound
	}
	row, err := s.queries.GetDataPipelineForTenant(ctx, db.GetDataPipelineForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DataPipeline{}, ErrDataPipelineNotFound
	}
	if err != nil {
		return db.DataPipeline{}, fmt.Errorf("get data pipeline: %w", err)
	}
	return row, nil
}

func (s *DataPipelineService) getVersionRow(ctx context.Context, tenantID int64, publicID string) (db.DataPipelineVersion, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DataPipelineVersion{}, ErrDataPipelineVersionNotFound
	}
	row, err := s.queries.GetDataPipelineVersionForTenant(ctx, db.GetDataPipelineVersionForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DataPipelineVersion{}, ErrDataPipelineVersionNotFound
	}
	if err != nil {
		return db.DataPipelineVersion{}, fmt.Errorf("get data pipeline version: %w", err)
	}
	return row, nil
}

func (s *DataPipelineService) getScheduleRow(ctx context.Context, tenantID int64, publicID string) (db.DataPipelineSchedule, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DataPipelineSchedule{}, ErrDataPipelineScheduleNotFound
	}
	row, err := s.queries.GetDataPipelineScheduleForTenant(ctx, db.GetDataPipelineScheduleForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DataPipelineSchedule{}, ErrDataPipelineScheduleNotFound
	}
	if err != nil {
		return db.DataPipelineSchedule{}, fmt.Errorf("get data pipeline schedule: %w", err)
	}
	return row, nil
}

func (s *DataPipelineService) getReviewItemRow(ctx context.Context, tenantID int64, publicID string) (db.DataPipelineReviewItem, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DataPipelineReviewItem{}, ErrDataPipelineReviewItemNotFound
	}
	row, err := s.queries.GetDataPipelineReviewItemForTenant(ctx, db.GetDataPipelineReviewItemForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DataPipelineReviewItem{}, ErrDataPipelineReviewItemNotFound
	}
	if err != nil {
		return db.DataPipelineReviewItem{}, fmt.Errorf("get data pipeline review item: %w", err)
	}
	return row, nil
}

func (s *DataPipelineService) listReviewItemComments(ctx context.Context, tenantID, reviewItemID int64) ([]DataPipelineReviewItemComment, error) {
	rows, err := s.queries.ListDataPipelineReviewItemComments(ctx, db.ListDataPipelineReviewItemCommentsParams{TenantID: tenantID, ReviewItemID: reviewItemID})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline review item comments: %w", err)
	}
	items := make([]DataPipelineReviewItemComment, 0, len(rows))
	for _, row := range rows {
		items = append(items, dataPipelineReviewItemCommentFromDB(row))
	}
	return items, nil
}

func (s *DataPipelineService) persistDataPipelineReviewItems(ctx context.Context, tenantID int64, run db.DataPipelineRun, version db.DataPipelineVersion, nodeResults map[string]dataPipelineRunNodeResult) error {
	for _, nodeResult := range nodeResults {
		for _, draft := range nodeResult.ReviewItems {
			reason, err := encodeDataPipelineJSON(draft.Reason)
			if err != nil {
				return err
			}
			snapshot, err := encodeDataPipelineJSON(draft.SourceSnapshot)
			if err != nil {
				return err
			}
			nodeID := firstNonEmpty(draft.NodeID, nodeResult.NodeID)
			fingerprint := strings.TrimSpace(draft.SourceFingerprint)
			if fingerprint == "" {
				fingerprint = shortHash(nodeID + ":" + string(snapshot))
			}
			_, err = s.queries.UpsertDataPipelineReviewItem(ctx, db.UpsertDataPipelineReviewItemParams{
				TenantID:          tenantID,
				PipelineID:        version.PipelineID,
				VersionID:         version.ID,
				RunID:             run.ID,
				NodeID:            nodeID,
				Queue:             firstNonEmpty(draft.Queue, "default"),
				Reason:            reason,
				SourceSnapshot:    snapshot,
				SourceFingerprint: fingerprint,
				CreatedByUserID:   run.RequestedByUserID,
				UpdatedByUserID:   run.RequestedByUserID,
			})
			if err != nil {
				return fmt.Errorf("upsert data pipeline review item %s: %w", nodeID, err)
			}
		}
	}
	return nil
}

func (s *DataPipelineService) failRunBestEffort(ctx context.Context, tenantID, runID int64, message string) {
	if s == nil || s.queries == nil {
		return
	}
	_, _ = s.queries.FailDataPipelineRun(ctx, db.FailDataPipelineRunParams{TenantID: tenantID, ID: runID, ErrorSummary: message})
}

func (s *DataPipelineService) recordAudit(ctx context.Context, auditCtx AuditContext, action, targetType, targetID string, metadata map[string]any) {
	if s == nil || s.audit == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	s.audit.RecordBestEffort(ctx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       action,
		TargetType:   targetType,
		TargetID:     targetID,
		Metadata:     metadata,
	})
}

func (s *DataPipelineService) recordMedallionRun(ctx context.Context, tenantID int64, run db.DataPipelineRun, version db.DataPipelineVersion, compiled dataPipelineCompiledSelect, workTable *DatasetWorkTable, status, errorSummary, outputNodeID string) {
	if s == nil || s.medallion == nil || compiled.Source == nil {
		return
	}
	var targetKind string
	var targetID int64
	var targetPublicID string
	var targetAssets []MedallionAsset
	if workTable != nil && workTable.ID > 0 {
		targetKind = MedallionResourceWorkTable
		targetID = workTable.ID
		targetPublicID = workTable.PublicID
		if asset, err := s.medallion.EnsureWorkTableAsset(ctx, *workTable, optionalPgInt8(run.RequestedByUserID)); err == nil {
			targetAssets = append(targetAssets, asset)
		}
	}
	sourceKind := MedallionResourceDataset
	if compiled.Source.Kind == "work_table" {
		sourceKind = MedallionResourceWorkTable
	}
	if status == "" {
		status = MedallionPipelineStatusPending
	}
	var completedAt *time.Time
	if status == MedallionPipelineStatusCompleted || status == MedallionPipelineStatusFailed || status == MedallionPipelineStatusSkipped {
		now := time.Now()
		completedAt = &now
	}
	_, _ = s.medallion.RecordPipelineRun(ctx, medallionPipelineRunInput{
		TenantID:               tenantID,
		PipelineType:           MedallionPipelineDataPipeline,
		RunKey:                 dataPipelineMedallionRunKey(run.PublicID.String(), outputNodeID),
		SourceResourceKind:     sourceKind,
		SourceResourceID:       compiled.Source.ID,
		SourceResourcePublicID: compiled.Source.PublicID,
		TargetResourceKind:     targetKind,
		TargetResourceID:       targetID,
		TargetResourcePublicID: targetPublicID,
		Status:                 status,
		Runtime:                "clickhouse",
		TriggerKind:            run.TriggerKind,
		Retryable:              status == MedallionPipelineStatusFailed,
		ErrorSummary:           errorSummary,
		Metadata: map[string]any{
			"versionPublicId": version.PublicID.String(),
			"outputNodeId":    outputNodeID,
		},
		RequestedByUserID: optionalPgInt8(run.RequestedByUserID),
		CompletedAt:       completedAt,
		TargetAssets:      targetAssets,
	})
}

func dataPipelineMedallionRunKey(runPublicID, outputNodeID string) string {
	if strings.TrimSpace(outputNodeID) == "" {
		return runPublicID
	}
	return runPublicID + ":" + outputNodeID
}

func normalizeDataPipelineInput(input DataPipelineInput) (DataPipelineInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" {
		return DataPipelineInput{}, fmt.Errorf("%w: name is required", ErrInvalidDataPipelineInput)
	}
	if len([]rune(input.Name)) > 160 {
		return DataPipelineInput{}, fmt.Errorf("%w: name is too long", ErrInvalidDataPipelineInput)
	}
	if len([]rune(input.Description)) > 2000 {
		return DataPipelineInput{}, fmt.Errorf("%w: description is too long", ErrInvalidDataPipelineInput)
	}
	return input, nil
}

func normalizeDataPipelineScheduleInput(input DataPipelineScheduleInput, after time.Time) (DataPipelineScheduleInput, time.Time, error) {
	input.Frequency = strings.TrimSpace(input.Frequency)
	if input.Frequency == "" {
		input.Frequency = "daily"
	}
	input.Timezone = strings.TrimSpace(input.Timezone)
	if input.Timezone == "" {
		input.Timezone = "Asia/Tokyo"
	}
	input.RunTime = strings.TrimSpace(input.RunTime)
	if input.RunTime == "" {
		input.RunTime = "03:00"
	}
	nextRun, err := nextDataPipelineScheduleRunAfter(input.Frequency, input.Timezone, input.RunTime, input.Weekday, input.MonthDay, after)
	if err != nil {
		return DataPipelineScheduleInput{}, time.Time{}, err
	}
	return input, nextRun, nil
}

func dataPipelineFromDB(row db.DataPipeline) DataPipeline {
	return DataPipeline{
		ID:                 row.ID,
		PublicID:           row.PublicID.String(),
		TenantID:           row.TenantID,
		CreatedByUserID:    optionalPgInt8(row.CreatedByUserID),
		UpdatedByUserID:    optionalPgInt8(row.UpdatedByUserID),
		Name:               row.Name,
		Description:        row.Description,
		Status:             row.Status,
		PublishedVersionID: optionalPgInt8(row.PublishedVersionID),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
		ArchivedAt:         optionalPgTime(row.ArchivedAt),
	}
}

type dataPipelineListCursor struct {
	Sort string    `json:"sort"`
	ID   int64     `json:"id"`
	Time time.Time `json:"time,omitempty"`
	Text string    `json:"text,omitempty"`
}

func normalizeDataPipelineListInput(input DataPipelineListInput) (DataPipelineListInput, dataPipelineListCursor, error) {
	out := input
	out.Query = strings.TrimSpace(out.Query)
	out.Status = strings.TrimSpace(out.Status)
	out.Publication = strings.TrimSpace(out.Publication)
	out.RunStatus = strings.TrimSpace(out.RunStatus)
	out.ScheduleState = strings.TrimSpace(out.ScheduleState)
	out.Sort = strings.TrimSpace(out.Sort)
	if out.Publication == "" {
		out.Publication = "all"
	}
	if out.ScheduleState == "" {
		out.ScheduleState = "all"
	}
	if out.Sort == "" {
		out.Sort = "updated_desc"
	}
	if out.Limit <= 0 {
		out.Limit = 25
	}
	if out.Limit > 100 {
		out.Limit = 100
	}
	if out.Status != "" && !dataPipelineListStatuses[out.Status] {
		return DataPipelineListInput{}, dataPipelineListCursor{}, ErrInvalidDataPipelineInput
	}
	if out.Publication != "all" && out.Publication != "published" && out.Publication != "unpublished" {
		return DataPipelineListInput{}, dataPipelineListCursor{}, ErrInvalidDataPipelineInput
	}
	if out.RunStatus != "" && !dataPipelineRunStatuses[out.RunStatus] {
		return DataPipelineListInput{}, dataPipelineListCursor{}, ErrInvalidDataPipelineInput
	}
	if out.ScheduleState != "all" && out.ScheduleState != "enabled" && out.ScheduleState != "disabled" && out.ScheduleState != "none" {
		return DataPipelineListInput{}, dataPipelineListCursor{}, ErrInvalidDataPipelineInput
	}
	if !dataPipelineListSorts[out.Sort] {
		return DataPipelineListInput{}, dataPipelineListCursor{}, ErrInvalidDataPipelineInput
	}
	cursor, err := decodeDataPipelineListCursor(out.Cursor)
	if err != nil {
		return DataPipelineListInput{}, dataPipelineListCursor{}, err
	}
	if cursor.ID > 0 && cursor.Sort != out.Sort {
		return DataPipelineListInput{}, dataPipelineListCursor{}, ErrInvalidCursor
	}
	return out, cursor, nil
}

var dataPipelineListStatuses = map[string]bool{"draft": true, "published": true}
var dataPipelineRunStatuses = map[string]bool{"pending": true, "processing": true, "completed": true, "failed": true, "skipped": true}
var dataPipelineListSorts = map[string]bool{
	"updated_desc":    true,
	"updated_asc":     true,
	"created_desc":    true,
	"created_asc":     true,
	"name_asc":        true,
	"name_desc":       true,
	"latest_run_desc": true,
}

func listDataPipelineCandidateLimit(limit int32) int32 {
	if limit <= 0 {
		return 26
	}
	candidateLimit := limit * 3
	if candidateLimit < limit+1 {
		candidateLimit = limit + 1
	}
	if candidateLimit > 100 {
		return 100
	}
	return candidateLimit
}

func decodeDataPipelineListCursor(value string) (dataPipelineListCursor, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return dataPipelineListCursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return dataPipelineListCursor{}, ErrInvalidCursor
	}
	var cursor dataPipelineListCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return dataPipelineListCursor{}, ErrInvalidCursor
	}
	if cursor.ID <= 0 || cursor.Sort == "" || !dataPipelineListSorts[cursor.Sort] {
		return dataPipelineListCursor{}, ErrInvalidCursor
	}
	if strings.HasPrefix(cursor.Sort, "name_") && cursor.Text == "" {
		return dataPipelineListCursor{}, ErrInvalidCursor
	}
	if !strings.HasPrefix(cursor.Sort, "name_") && cursor.Time.IsZero() {
		return dataPipelineListCursor{}, ErrInvalidCursor
	}
	return cursor, nil
}

func encodeDataPipelineListCursor(sort string, item DataPipeline) (string, error) {
	cursor := dataPipelineListCursorFromItem(sort, item)
	if cursor.ID <= 0 {
		return "", ErrInvalidCursor
	}
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func dataPipelineListCursorFromItem(sort string, item DataPipeline) dataPipelineListCursor {
	cursor := dataPipelineListCursor{Sort: sort, ID: item.ID}
	switch sort {
	case "created_desc", "created_asc":
		cursor.Time = item.CreatedAt
	case "name_asc", "name_desc":
		cursor.Text = strings.ToLower(item.Name)
	case "latest_run_desc":
		if item.LatestRunAt != nil {
			cursor.Time = *item.LatestRunAt
		} else {
			cursor.Time = time.Date(1, 1, 2, 0, 0, 0, 0, time.UTC)
		}
	default:
		cursor.Time = item.UpdatedAt
	}
	return cursor
}

func dataPipelineFromListRow(row db.ListDataPipelinesRow) DataPipeline {
	item := DataPipeline{
		ID:                    row.ID,
		PublicID:              row.PublicID.String(),
		TenantID:              row.TenantID,
		CreatedByUserID:       optionalPgInt8(row.CreatedByUserID),
		UpdatedByUserID:       optionalPgInt8(row.UpdatedByUserID),
		Name:                  row.Name,
		Description:           row.Description,
		Status:                row.Status,
		PublishedVersionID:    optionalPgInt8(row.PublishedVersionID),
		CreatedAt:             row.CreatedAt.Time,
		UpdatedAt:             row.UpdatedAt.Time,
		ArchivedAt:            optionalPgTime(row.ArchivedAt),
		LatestRunStatus:       row.LatestRunStatus,
		LatestRunAt:           optionalPgTime(row.LatestRunAt),
		ScheduleState:         row.ScheduleState,
		EnabledScheduleCount:  row.EnabledScheduleCount,
		DisabledScheduleCount: row.DisabledScheduleCount,
		NextRunAt:             optionalPgTime(row.NextRunAt),
	}
	switch value := row.LatestRunPublicID.(type) {
	case string:
		item.LatestRunPublicID = value
	case []byte:
		item.LatestRunPublicID = string(value)
	}
	return item
}

func nullableText(value string) pgtype.Text {
	return pgText(strings.TrimSpace(value))
}

func nullableInt8(value int64) pgtype.Int8 {
	if value <= 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: value, Valid: true}
}

func nullableTime(value time.Time) pgtype.Timestamptz {
	return pgTimestamp(value)
}

func dataPipelineVersionFromDB(row db.DataPipelineVersion) (DataPipelineVersion, error) {
	graph, err := decodeDataPipelineGraph(row.Graph)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	var summary DataPipelineValidationSummary
	if len(row.ValidationSummary) > 0 {
		_ = json.Unmarshal(row.ValidationSummary, &summary)
	}
	return DataPipelineVersion{
		ID:                row.ID,
		PublicID:          row.PublicID.String(),
		TenantID:          row.TenantID,
		PipelineID:        row.PipelineID,
		VersionNumber:     row.VersionNumber,
		Status:            row.Status,
		Graph:             graph,
		ValidationSummary: summary,
		CreatedByUserID:   optionalPgInt8(row.CreatedByUserID),
		PublishedByUserID: optionalPgInt8(row.PublishedByUserID),
		CreatedAt:         row.CreatedAt.Time,
		PublishedAt:       optionalPgTime(row.PublishedAt),
	}, nil
}

func dataPipelineRunFromDB(row db.DataPipelineRun) DataPipelineRun {
	return DataPipelineRun{
		ID:                row.ID,
		PublicID:          row.PublicID.String(),
		TenantID:          row.TenantID,
		PipelineID:        row.PipelineID,
		VersionID:         row.VersionID,
		ScheduleID:        optionalPgInt8(row.ScheduleID),
		RequestedByUserID: optionalPgInt8(row.RequestedByUserID),
		TriggerKind:       row.TriggerKind,
		Status:            row.Status,
		OutputWorkTableID: optionalPgInt8(row.OutputWorkTableID),
		OutboxEventID:     optionalPgInt8(row.OutboxEventID),
		RowCount:          row.RowCount,
		ErrorSummary:      optionalText(row.ErrorSummary),
		StartedAt:         optionalPgTime(row.StartedAt),
		CompletedAt:       optionalPgTime(row.CompletedAt),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func dataPipelineRunOutputFromDB(row db.DataPipelineRunOutput) DataPipelineRunOutput {
	return DataPipelineRunOutput{
		ID:                row.ID,
		TenantID:          row.TenantID,
		RunID:             row.RunID,
		NodeID:            row.NodeID,
		Status:            row.Status,
		OutputWorkTableID: optionalPgInt8(row.OutputWorkTableID),
		RowCount:          row.RowCount,
		ErrorSummary:      optionalText(row.ErrorSummary),
		Metadata:          decodeDataPipelineJSONMap(row.Metadata),
		StartedAt:         optionalPgTime(row.StartedAt),
		CompletedAt:       optionalPgTime(row.CompletedAt),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func dataPipelineRunStepFromDB(row db.DataPipelineRunStep) DataPipelineRunStep {
	var sample []map[string]any
	_ = json.Unmarshal(row.ErrorSample, &sample)
	if sample == nil {
		sample = []map[string]any{}
	}
	return DataPipelineRunStep{
		ID:           row.ID,
		TenantID:     row.TenantID,
		RunID:        row.RunID,
		NodeID:       row.NodeID,
		StepType:     row.StepType,
		Status:       row.Status,
		RowCount:     row.RowCount,
		ErrorSummary: optionalText(row.ErrorSummary),
		ErrorSample:  sample,
		Metadata:     decodeDataPipelineJSONMap(row.Metadata),
		StartedAt:    optionalPgTime(row.StartedAt),
		CompletedAt:  optionalPgTime(row.CompletedAt),
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func dataPipelineReviewItemFromDB(row db.DataPipelineReviewItem) DataPipelineReviewItem {
	var reason []map[string]any
	_ = json.Unmarshal(row.Reason, &reason)
	if reason == nil {
		reason = []map[string]any{}
	}
	return DataPipelineReviewItem{
		ID:                row.ID,
		PublicID:          row.PublicID.String(),
		TenantID:          row.TenantID,
		PipelineID:        row.PipelineID,
		VersionID:         row.VersionID,
		RunID:             row.RunID,
		NodeID:            row.NodeID,
		Queue:             row.Queue,
		Status:            row.Status,
		Reason:            reason,
		SourceSnapshot:    decodeDataPipelineJSONMap(row.SourceSnapshot),
		SourceFingerprint: row.SourceFingerprint,
		CreatedByUserID:   optionalPgInt8(row.CreatedByUserID),
		UpdatedByUserID:   optionalPgInt8(row.UpdatedByUserID),
		AssignedToUserID:  optionalPgInt8(row.AssignedToUserID),
		DecisionComment:   row.DecisionComment,
		DecidedAt:         optionalPgTime(row.DecidedAt),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func dataPipelineReviewItemCommentFromDB(row db.DataPipelineReviewItemComment) DataPipelineReviewItemComment {
	return DataPipelineReviewItemComment{
		ID:           row.ID,
		PublicID:     row.PublicID.String(),
		TenantID:     row.TenantID,
		ReviewItemID: row.ReviewItemID,
		AuthorUserID: optionalPgInt8(row.AuthorUserID),
		Body:         row.Body,
		CreatedAt:    row.CreatedAt.Time,
	}
}

func dataPipelineReviewItemStatusAllowed(status string) bool {
	switch status {
	case "open", "approved", "rejected", "needs_changes", "closed":
		return true
	default:
		return false
	}
}

func dataPipelineScheduleFromDB(row db.DataPipelineSchedule) DataPipelineSchedule {
	return DataPipelineSchedule{
		ID:               row.ID,
		PublicID:         row.PublicID.String(),
		TenantID:         row.TenantID,
		PipelineID:       row.PipelineID,
		VersionID:        row.VersionID,
		CreatedByUserID:  optionalPgInt8(row.CreatedByUserID),
		Frequency:        row.Frequency,
		Timezone:         row.Timezone,
		RunTime:          row.RunTime,
		Weekday:          optionalPgInt2(row.Weekday),
		MonthDay:         optionalPgInt2(row.MonthDay),
		Enabled:          row.Enabled,
		NextRunAt:        row.NextRunAt.Time,
		LastRunAt:        optionalPgTime(row.LastRunAt),
		LastStatus:       optionalText(row.LastStatus),
		LastErrorSummary: optionalText(row.LastErrorSummary),
		LastRunID:        optionalPgInt8(row.LastRunID),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}
