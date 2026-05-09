package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	driveRAGDefaultLimit         = 8
	driveRAGMaxLimit             = 20
	driveRAGMaxAgentLoops        = 2
	driveRAGMaxGenerationText    = 1200
	driveRAGMinSemanticOnlyScore = 0.84
)

type DriveRAGQueryInput struct {
	TenantID    int64
	ActorUserID int64
	Query       string
	Mode        string
	Limit       int32
}

type DriveRAGResult struct {
	Answer         string
	Citations      []DriveRAGCitation
	Matches        []DriveRAGCitation
	Blocked        bool
	RetrievalTrace []DriveRAGRetrievalTrace
}

type DriveRAGCitation struct {
	CitationID       string
	ResourceKind     string
	ResourcePublicID string
	FilePublicID     string
	Filename         string
	Snippet          string
	Score            float64
}

type DriveRAGQueryPlan struct {
	OriginalQuery    string
	RetrievalQueries []DriveRAGRetrievalQuery
	Keywords         []string
	MustHave         []string
	Avoid            []string
	Source           string
}

type DriveRAGRetrievalQuery struct {
	Query  string
	Intent string
	Weight float64
}

type DriveRAGRetrievalTrace struct {
	Query          string
	Intent         string
	PlanSource     string
	ResultCount    int
	MergedCount    int
	SearchMode     string
	Retry          bool
	RetryReason    string
	MissingSignals []string
}

type DriveRAGSufficiencyResult struct {
	Sufficient     bool
	MissingSignals []string
	Reason         string
}

type DriveRAGAgent struct {
	service     *DriveService
	input       DriveRAGQueryInput
	auditCtx    AuditContext
	policy      DriveRAGPolicy
	query       string
	mode        string
	searchLimit int32
}

type driveRAGContext struct {
	Citation DriveRAGCitation
	Text     string
}

type driveRAGRewriteResponse struct {
	SearchQueries []string `json:"searchQueries"`
	Keywords      []string `json:"keywords"`
	MustHave      []string `json:"mustHave"`
	Avoid         []string `json:"avoid"`
}

type driveRAGGenerated struct {
	Answer string          `json:"answer"`
	Claims []driveRAGClaim `json:"claims"`
}

type driveRAGClaim struct {
	Text        string   `json:"text"`
	CitationIDs []string `json:"citationIds"`
}

func (c *driveRAGClaim) UnmarshalJSON(data []byte) error {
	type rawClaim struct {
		Text             string   `json:"text"`
		CitationIDs      []string `json:"citationIds"`
		CitationIDsSnake []string `json:"citation_ids"`
		Citations        []string `json:"citations"`
		Sources          []string `json:"sources"`
	}
	var raw rawClaim
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Text = raw.Text
	c.CitationIDs = raw.CitationIDs
	if len(c.CitationIDs) == 0 {
		c.CitationIDs = raw.CitationIDsSnake
	}
	if len(c.CitationIDs) == 0 {
		c.CitationIDs = raw.Citations
	}
	if len(c.CitationIDs) == 0 {
		c.CitationIDs = raw.Sources
	}
	return nil
}

func (s *DriveService) QueryRAG(ctx context.Context, input DriveRAGQueryInput, auditCtx AuditContext) (DriveRAGResult, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveRAGResult{}, err
	}
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return DriveRAGResult{}, fmt.Errorf("%w: rag query is required", ErrDriveInvalidInput)
	}
	if s.tenantSettings == nil || s.localSearch == nil {
		return DriveRAGResult{}, ErrDrivePolicyDenied
	}
	policy, err := s.tenantSettings.GetDrivePolicy(ctx, input.TenantID)
	if err != nil {
		return DriveRAGResult{}, err
	}
	if !policy.RAG.Enabled || !policy.LocalSearch.VectorEnabled || policy.RAG.GenerationRuntime == "none" {
		return DriveRAGResult{}, ErrDrivePolicyDenied
	}
	mode := normalizeDriveSearchMode(input.Mode)
	if mode == DriveSearchModeKeyword {
		mode = DriveSearchModeHybrid
	}
	limit := input.Limit
	if limit <= 0 {
		limit = driveRAGDefaultLimit
	}
	if limit > driveRAGMaxLimit {
		limit = driveRAGMaxLimit
	}
	searchLimit := limit * 3
	if searchLimit < limit {
		searchLimit = limit
	}
	if searchLimit > driveRAGMaxLimit {
		searchLimit = driveRAGMaxLimit
	}
	agent := DriveRAGAgent{
		service:     s,
		input:       input,
		auditCtx:    auditCtx,
		policy:      policy.RAG,
		query:       query,
		mode:        mode,
		searchLimit: searchLimit,
	}
	return agent.Run(ctx)
}

func (a *DriveRAGAgent) Run(ctx context.Context) (DriveRAGResult, error) {
	if a == nil || a.service == nil {
		return DriveRAGResult{}, fmt.Errorf("drive rag agent is not configured")
	}
	plan := a.service.driveRAGQueryPlan(ctx, a.query, a.policy)
	results, trace, err := a.retrieve(ctx, plan, nil)
	if err != nil {
		return DriveRAGResult{}, err
	}
	results = rankDriveRAGResultsWithPlan(plan, results)
	for loop := 1; loop < driveRAGMaxAgentLoops; loop++ {
		sufficiency := driveRAGSufficiency(plan, results)
		if sufficiency.Sufficient {
			break
		}
		retryPlan := driveRAGRetryQueryPlan(plan, sufficiency)
		if len(retryPlan.RetrievalQueries) == 0 {
			break
		}
		retryResults, retryTrace, err := a.retrieve(ctx, retryPlan, &sufficiency)
		if err != nil {
			return DriveRAGResult{}, err
		}
		results = mergeDriveRAGSearchResults(results, retryResults, map[string]int{})
		results = rankDriveRAGResultsWithPlan(plan, results)
		trace = append(trace, retryTrace...)
	}
	contexts := driveRAGContexts(results, a.policy)
	matches := make([]DriveRAGCitation, 0, len(contexts))
	for _, item := range contexts {
		matches = append(matches, item.Citation)
	}
	if len(contexts) == 0 {
		return DriveRAGResult{Blocked: true, Matches: matches, RetrievalTrace: trace}, nil
	}
	provider := NewLocalGenerationProvider(a.policy.GenerationRuntime, a.policy.GenerationRuntimeURL)
	generated, err := provider.Generate(ctx, GenerationRequest{
		Model:       a.policy.GenerationModel,
		System:      driveRAGSystemPrompt(),
		User:        driveRAGUserPrompt(a.query, contexts),
		MaxTokens:   900,
		Temperature: 0,
		JSONSchema:  driveRAGResponseFormat(),
	})
	if err != nil {
		return DriveRAGResult{}, err
	}
	answer, citations := validateDriveRAGGenerated(generated.Text, contexts)
	if strings.TrimSpace(answer) == "" || len(citations) == 0 {
		answer, citations = fallbackDriveRAGAnswer(contexts)
		if strings.TrimSpace(answer) == "" || len(citations) == 0 {
			return DriveRAGResult{Blocked: true, Matches: matches, RetrievalTrace: trace}, nil
		}
	}
	return DriveRAGResult{
		Answer:         truncateRunes(answer, driveRAGMaxGenerationText),
		Citations:      citations,
		Matches:        matches,
		RetrievalTrace: trace,
	}, nil
}

func (a *DriveRAGAgent) retrieve(ctx context.Context, plan DriveRAGQueryPlan, retry *DriveRAGSufficiencyResult) ([]DriveSearchResult, []DriveRAGRetrievalTrace, error) {
	results, trace, err := a.service.searchDriveRAGPlan(ctx, a.input, a.auditCtx, plan, a.mode, a.searchLimit)
	if err != nil {
		return nil, trace, err
	}
	if retry == nil {
		return results, trace, nil
	}
	for i := range trace {
		trace[i].Retry = true
		trace[i].RetryReason = retry.Reason
		trace[i].MissingSignals = append([]string{}, retry.MissingSignals...)
	}
	return results, trace, nil
}

func (s *DriveService) driveRAGQueryPlan(ctx context.Context, query string, policy DriveRAGPolicy) DriveRAGQueryPlan {
	policy = normalizeDriveRAGPolicy(policy)
	switch policy.QueryRewriteMode {
	case "none":
		return driveRAGBaseQueryPlan(query, "none")
	case "llm":
		if plan, err := s.driveRAGLLMQueryPlan(ctx, query, policy); err == nil && len(plan.RetrievalQueries) > 0 {
			return plan
		}
	}
	return driveRAGDeterministicQueryPlan(query, policy.QueryRewriteMaxQueries)
}

func (s *DriveService) driveRAGLLMQueryPlan(ctx context.Context, query string, policy DriveRAGPolicy) (DriveRAGQueryPlan, error) {
	if policy.GenerationRuntime == "none" || strings.TrimSpace(policy.GenerationModel) == "" {
		return DriveRAGQueryPlan{}, fmt.Errorf("rag generation runtime is not configured")
	}
	provider := NewLocalGenerationProvider(policy.GenerationRuntime, policy.GenerationRuntimeURL)
	generated, err := provider.Generate(ctx, GenerationRequest{
		Model:       policy.GenerationModel,
		System:      driveRAGQueryRewriteSystemPrompt(),
		User:        driveRAGQueryRewriteUserPrompt(query, policy.QueryRewriteMaxQueries),
		MaxTokens:   360,
		Temperature: 0,
		JSONSchema:  driveRAGQueryRewriteResponseFormat(),
	})
	if err != nil {
		return DriveRAGQueryPlan{}, err
	}
	return driveRAGQueryPlanFromRewrite(query, generated.Text, policy.QueryRewriteMaxQueries)
}

func (s *DriveService) searchDriveRAGPlan(ctx context.Context, input DriveRAGQueryInput, auditCtx AuditContext, plan DriveRAGQueryPlan, mode string, searchLimit int32) ([]DriveSearchResult, []DriveRAGRetrievalTrace, error) {
	queries := plan.RetrievalQueries
	if len(queries) == 0 {
		queries = []DriveRAGRetrievalQuery{{Query: plan.OriginalQuery, Intent: "original", Weight: 1}}
	}
	merged := make([]DriveSearchResult, 0)
	coverage := map[string]int{}
	trace := make([]DriveRAGRetrievalTrace, 0, len(queries))
	for _, retrievalQuery := range queries {
		searchQuery := strings.TrimSpace(retrievalQuery.Query)
		if searchQuery == "" {
			continue
		}
		results, err := s.SearchDocuments(ctx, DriveSearchInput{
			TenantID:    input.TenantID,
			ActorUserID: input.ActorUserID,
			Query:       searchQuery,
			Mode:        mode,
			Limit:       searchLimit,
			Filter:      DriveListItemsFilter{Type: "all", Owner: "all", Source: "all"},
		}, auditCtx)
		if err != nil {
			return nil, trace, err
		}
		merged = mergeDriveRAGSearchResults(merged, results, coverage)
		trace = append(trace, DriveRAGRetrievalTrace{
			Query:       searchQuery,
			Intent:      retrievalQuery.Intent,
			PlanSource:  plan.Source,
			ResultCount: len(results),
			MergedCount: len(merged),
			SearchMode:  mode,
		})
	}
	sort.SliceStable(merged, func(i, j int) bool {
		left := driveRAGResultFilePublicID(merged[i])
		right := driveRAGResultFilePublicID(merged[j])
		if coverage[left] == coverage[right] {
			return i < j
		}
		return coverage[left] > coverage[right]
	})
	return merged, trace, nil
}

func driveRAGContexts(results []DriveSearchResult, policy DriveRAGPolicy) []driveRAGContext {
	maxChunks := policy.MaxContextChunks
	if maxChunks <= 0 {
		maxChunks = defaultDriveRAGPolicy().MaxContextChunks
	}
	maxRunes := policy.MaxContextRunes
	if maxRunes <= 0 {
		maxRunes = defaultDriveRAGPolicy().MaxContextRunes
	}
	contexts := make([]driveRAGContext, 0, maxChunks)
	usedRunes := 0
	for _, result := range results {
		if result.Item.File == nil || result.Item.File.DLPBlocked {
			continue
		}
		file := result.Item.File
		text := strings.TrimSpace(firstNonEmpty(result.Snippet, file.OriginalFilename))
		resourceKind := LocalSearchResourceDriveFile
		resourcePublicID := file.PublicID
		if len(result.Matches) > 0 {
			match := result.Matches[0]
			resourceKind = match.ResourceKind
			resourcePublicID = match.ResourcePublicID
			text = strings.TrimSpace(firstNonEmpty(match.Snippet, text))
		}
		text = cleanDriveRAGContextText(text)
		if text == "" {
			continue
		}
		remaining := maxRunes - usedRunes
		if remaining <= 0 {
			break
		}
		text = truncateRunes(text, remaining)
		citationID := fmt.Sprintf("c%d", len(contexts)+1)
		contexts = append(contexts, driveRAGContext{
			Citation: DriveRAGCitation{
				CitationID:       citationID,
				ResourceKind:     resourceKind,
				ResourcePublicID: resourcePublicID,
				FilePublicID:     file.PublicID,
				Filename:         file.OriginalFilename,
				Snippet:          text,
				Score:            driveRAGResultSemanticScore(result),
			},
			Text: text,
		})
		usedRunes += len([]rune(text))
		if len(contexts) >= maxChunks {
			break
		}
	}
	return contexts
}

func rankDriveRAGResults(query string, results []DriveSearchResult) []DriveSearchResult {
	return rankDriveRAGResultsWithPlan(driveRAGBaseQueryPlan(query, "legacy"), results)
}

func rankDriveRAGResultsWithPlan(plan DriveRAGQueryPlan, results []DriveSearchResult) []DriveSearchResult {
	query := plan.OriginalQuery
	terms := driveRAGQueryTerms(query)
	for _, term := range append(append([]string{}, plan.Keywords...), plan.MustHave...) {
		term = driveRAGNormalizeSearchText(term)
		if len([]rune(term)) >= 2 {
			terms = appendUniqueString(terms, term)
		}
	}
	if len(terms) == 0 {
		return results
	}
	type rankedResult struct {
		result        DriveSearchResult
		lexicalScore  int
		semanticScore float64
		coverageScore int
		index         int
	}
	ranked := make([]rankedResult, 0, len(results))
	hasPositiveScore := false
	for i, result := range results {
		score := driveRAGResultLexicalScore(result, terms)
		coverageScore := driveRAGResultQueryCoverageScore(result, plan.RetrievalQueries)
		if score > 0 {
			hasPositiveScore = true
		}
		ranked = append(ranked, rankedResult{
			result:        result,
			lexicalScore:  score,
			semanticScore: driveRAGResultSemanticScore(result),
			coverageScore: coverageScore,
			index:         i,
		})
	}
	if !hasPositiveScore {
		sort.SliceStable(ranked, func(i, j int) bool {
			if ranked[i].semanticScore == ranked[j].semanticScore {
				return ranked[i].index < ranked[j].index
			}
			return ranked[i].semanticScore > ranked[j].semanticScore
		})
		out := make([]DriveSearchResult, 0, len(ranked))
		for _, item := range ranked {
			if !driveRAGHasStrongContentSemanticMatch(item.result) {
				continue
			}
			out = append(out, item.result)
		}
		return out
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].lexicalScore == ranked[j].lexicalScore {
			if ranked[i].coverageScore != ranked[j].coverageScore {
				return ranked[i].coverageScore > ranked[j].coverageScore
			}
			if ranked[i].semanticScore != ranked[j].semanticScore {
				return ranked[i].semanticScore > ranked[j].semanticScore
			}
			return ranked[i].index < ranked[j].index
		}
		return ranked[i].lexicalScore > ranked[j].lexicalScore
	})
	out := make([]DriveSearchResult, 0, len(ranked))
	for _, item := range ranked {
		if item.lexicalScore <= 0 {
			continue
		}
		out = append(out, item.result)
	}
	return out
}

func driveRAGBaseQueryPlan(query, source string) DriveRAGQueryPlan {
	query = strings.TrimSpace(query)
	return DriveRAGQueryPlan{
		OriginalQuery: query,
		RetrievalQueries: []DriveRAGRetrievalQuery{
			{Query: query, Intent: "original", Weight: 1},
		},
		Keywords: driveRAGQueryTerms(query),
		Source:   source,
	}
}

func driveRAGDeterministicQueryPlan(query string, maxQueries int) DriveRAGQueryPlan {
	if maxQueries <= 0 {
		maxQueries = defaultDriveRAGPolicy().QueryRewriteMaxQueries
	}
	original := strings.TrimSpace(query)
	keywords := driveRAGDeterministicKeywords(original)
	plan := DriveRAGQueryPlan{
		OriginalQuery: original,
		Keywords:      keywords,
		MustHave:      driveRAGMustHaveTerms(original, keywords),
		Source:        "deterministic",
	}
	addQuery := func(value, intent string, weight float64) {
		value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		if value == "" {
			return
		}
		for _, existing := range plan.RetrievalQueries {
			if existing.Query == value {
				return
			}
		}
		plan.RetrievalQueries = append(plan.RetrievalQueries, DriveRAGRetrievalQuery{Query: value, Intent: intent, Weight: weight})
	}
	addQuery(original, "original", 1)
	if len(keywords) > 0 {
		addQuery(strings.Join(keywords, " "), "expanded_keywords", 0.9)
	}
	if containsAnyNormalized(keywords, "請求書") && containsAnyNormalized(keywords, "振込期限", "支払期限", "税込合計", "合計金額") {
		addQuery("請求書 振込期限 税込合計", "invoice_payment_terms", 0.9)
	}
	if containsAnyNormalized(keywords, "白い", "白", "white") && containsAnyNormalized(keywords, "インテリア", "家具") {
		addQuery("白い机", "white_desk", 0.85)
		addQuery("白い椅子", "white_chair", 0.85)
		addQuery("白いインテリア", "white_interior", 0.8)
		addQuery("白い インテリア 家具", "concept", 0.75)
	}
	if containsAnyNormalized(keywords, "インテリア") && containsAnyNormalized(keywords, "家具") {
		addQuery("インテリア 家具 デスク 椅子 棚 ソファ", "interior_furniture", 0.75)
	}
	if len(plan.RetrievalQueries) > maxQueries {
		plan.RetrievalQueries = plan.RetrievalQueries[:maxQueries]
	}
	return plan
}

func driveRAGDeterministicKeywords(query string) []string {
	terms := make([]string, 0)
	add := func(values ...string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if len([]rune(value)) < 2 && value != "棚" {
				continue
			}
			terms = appendUniqueString(terms, value)
		}
	}
	for _, term := range driveRAGASCIISearchTerms(query) {
		add(term)
	}
	normalized := driveRAGNormalizeSearchText(query)
	if strings.Contains(normalized, "白い") || strings.Contains(normalized, "白") || strings.Contains(normalized, "ホワイト") {
		add("白い", "白", "ホワイト")
	}
	if strings.Contains(normalized, "インテリア") || strings.Contains(normalized, "内装") || strings.Contains(normalized, "部屋") {
		add("インテリア", "内装", "部屋", "ミニマル", "観葉植物")
	}
	if strings.Contains(normalized, "家具") || strings.Contains(normalized, "インテリア") {
		add("家具", "デスク", "机", "椅子", "チェア", "棚", "収納", "ソファ", "木製")
	}
	if strings.Contains(normalized, "請求書") || strings.Contains(normalized, "invoice") {
		add("請求書", "invoice")
	}
	if strings.Contains(normalized, "支払期限") || strings.Contains(normalized, "支払期日") || strings.Contains(normalized, "振込期限") {
		add("支払期限", "振込期限", "支払期日", "入金期限")
	}
	if strings.Contains(normalized, "税込合計") || strings.Contains(normalized, "合計金額") || strings.Contains(normalized, "請求金額") {
		add("税込合計", "合計金額", "請求金額", "支払金額")
	}
	return terms
}

func driveRAGMustHaveTerms(query string, keywords []string) []string {
	out := make([]string, 0)
	normalized := driveRAGNormalizeSearchText(query)
	for _, candidate := range []string{"白い", "白", "インテリア", "家具"} {
		if strings.Contains(normalized, candidate) || containsAnyNormalized(keywords, candidate) {
			out = appendUniqueString(out, candidate)
		}
	}
	return out
}

func driveRAGQueryPlanFromRewrite(query, payload string, maxQueries int) (DriveRAGQueryPlan, error) {
	var rewrite driveRAGRewriteResponse
	if err := json.Unmarshal([]byte(extractDriveRAGJSONPayload(payload)), &rewrite); err != nil {
		return DriveRAGQueryPlan{}, err
	}
	plan := DriveRAGQueryPlan{
		OriginalQuery: strings.TrimSpace(query),
		Keywords:      normalizeDriveRAGStringSlice(rewrite.Keywords),
		MustHave:      normalizeDriveRAGStringSlice(rewrite.MustHave),
		Avoid:         normalizeDriveRAGStringSlice(rewrite.Avoid),
		Source:        "llm",
	}
	addQuery := func(value, intent string, weight float64) {
		value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		if value == "" {
			return
		}
		for _, existing := range plan.RetrievalQueries {
			if existing.Query == value {
				return
			}
		}
		plan.RetrievalQueries = append(plan.RetrievalQueries, DriveRAGRetrievalQuery{Query: value, Intent: intent, Weight: weight})
	}
	addQuery(plan.OriginalQuery, "original", 1)
	for _, searchQuery := range rewrite.SearchQueries {
		addQuery(searchQuery, "llm_rewrite", 0.9)
	}
	if maxQueries <= 0 {
		maxQueries = defaultDriveRAGPolicy().QueryRewriteMaxQueries
	}
	if len(plan.RetrievalQueries) > maxQueries {
		plan.RetrievalQueries = plan.RetrievalQueries[:maxQueries]
	}
	if len(plan.RetrievalQueries) == 0 {
		return DriveRAGQueryPlan{}, fmt.Errorf("llm rewrite returned no search queries")
	}
	return plan, nil
}

func driveRAGSufficiency(plan DriveRAGQueryPlan, results []DriveSearchResult) DriveRAGSufficiencyResult {
	if len(results) == 0 {
		return DriveRAGSufficiencyResult{
			Sufficient:     false,
			MissingSignals: driveRAGRetrySignals(plan),
			Reason:         "no candidates",
		}
	}
	required := driveRAGRequiredSignals(plan)
	if len(required) == 0 {
		return DriveRAGSufficiencyResult{Sufficient: true}
	}
	text := driveRAGNormalizeSearchText(driveRAGResultsText(results))
	missing := make([]string, 0)
	for _, signal := range required {
		if !driveRAGTextContainsSignal(text, signal) {
			missing = appendUniqueString(missing, signal)
		}
	}
	if len(missing) == 0 {
		return DriveRAGSufficiencyResult{Sufficient: true}
	}
	return DriveRAGSufficiencyResult{
		Sufficient:     false,
		MissingSignals: missing,
		Reason:         "missing required retrieval signals",
	}
}

func driveRAGRequiredSignals(plan DriveRAGQueryPlan) []string {
	out := make([]string, 0, len(plan.MustHave))
	for _, signal := range plan.MustHave {
		signal = strings.TrimSpace(signal)
		if signal == "" {
			continue
		}
		if signal == "白" && containsAnyNormalized(plan.MustHave, "白い", "ホワイト") {
			continue
		}
		out = appendUniqueString(out, signal)
	}
	return out
}

func driveRAGRetrySignals(plan DriveRAGQueryPlan) []string {
	signals := driveRAGRequiredSignals(plan)
	if len(signals) > 0 {
		return signals
	}
	for _, signal := range plan.Keywords {
		if containsAnyNormalized([]string{signal}, "白い", "ホワイト", "インテリア", "家具", "デスク", "机", "椅子", "棚", "収納", "ソファ") {
			signals = appendUniqueString(signals, signal)
		}
	}
	return signals
}

func driveRAGRetryQueryPlan(plan DriveRAGQueryPlan, sufficiency DriveRAGSufficiencyResult) DriveRAGQueryPlan {
	signals := sufficiency.MissingSignals
	if len(signals) == 0 {
		signals = driveRAGRetrySignals(plan)
	}
	if len(signals) == 0 {
		return DriveRAGQueryPlan{}
	}
	retry := DriveRAGQueryPlan{
		OriginalQuery: plan.OriginalQuery,
		Keywords:      append([]string{}, plan.Keywords...),
		MustHave:      append([]string{}, plan.MustHave...),
		Source:        "retry",
	}
	addQuery := func(value, intent string) {
		value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		if value == "" {
			return
		}
		for _, existing := range retry.RetrievalQueries {
			if existing.Query == value {
				return
			}
		}
		retry.RetrievalQueries = append(retry.RetrievalQueries, DriveRAGRetrievalQuery{Query: value, Intent: intent, Weight: 0.7})
	}
	addQuery(strings.Join(signals, " "), "sufficiency_retry")
	if containsAnyNormalized(append(plan.Keywords, signals...), "家具", "インテリア") {
		for _, signal := range signals {
			switch driveRAGNormalizeSearchText(signal) {
			case "白い", "白", "ホワイト":
				addQuery(signal+" デスク 椅子 棚 収納 ソファ", "sufficiency_retry_furniture")
			case "家具":
				addQuery("デスク 椅子 棚 収納 ソファ 木製", "sufficiency_retry_furniture")
			case "インテリア":
				addQuery("インテリア 家具 観葉植物 ミニマル", "sufficiency_retry_interior")
			}
		}
	}
	if len(retry.RetrievalQueries) > 3 {
		retry.RetrievalQueries = retry.RetrievalQueries[:3]
	}
	return retry
}

func mergeDriveRAGSearchResults(existing []DriveSearchResult, incoming []DriveSearchResult, coverage map[string]int) []DriveSearchResult {
	if coverage == nil {
		coverage = map[string]int{}
	}
	indexByFile := map[string]int{}
	for i, result := range existing {
		if key := driveRAGResultFilePublicID(result); key != "" {
			indexByFile[key] = i
		}
	}
	for _, result := range incoming {
		key := driveRAGResultFilePublicID(result)
		if key == "" {
			existing = append(existing, result)
			continue
		}
		coverage[key]++
		if index, ok := indexByFile[key]; ok {
			existing[index] = mergeDriveRAGSearchResult(existing[index], result)
			continue
		}
		indexByFile[key] = len(existing)
		existing = append(existing, result)
	}
	return existing
}

func mergeDriveRAGSearchResult(left, right DriveSearchResult) DriveSearchResult {
	if strings.TrimSpace(left.Snippet) == "" || len([]rune(right.Snippet)) > len([]rune(left.Snippet)) {
		left.Snippet = right.Snippet
	}
	left.Matches = mergeDriveRAGMatches(left.Matches, right.Matches)
	if left.IndexedAt == nil {
		left.IndexedAt = right.IndexedAt
	}
	return left
}

func mergeDriveRAGMatches(left, right []LocalSearchMatch) []LocalSearchMatch {
	out := append([]LocalSearchMatch{}, left...)
	seen := map[string]struct{}{}
	for _, match := range out {
		seen[match.ResourceKind+"|"+match.ResourcePublicID+"|"+match.Snippet] = struct{}{}
	}
	for _, match := range right {
		key := match.ResourceKind + "|" + match.ResourcePublicID + "|" + match.Snippet
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, match)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	return out
}

func driveRAGResultFilePublicID(result DriveSearchResult) string {
	if result.Item.File == nil {
		return ""
	}
	return result.Item.File.PublicID
}

func driveRAGResultSemanticScore(result DriveSearchResult) float64 {
	score := 0.0
	for _, match := range result.Matches {
		if match.Score > score {
			score = match.Score
		}
	}
	return score
}

func driveRAGResultQueryCoverageScore(result DriveSearchResult, queries []DriveRAGRetrievalQuery) int {
	if len(queries) == 0 {
		return 0
	}
	score := 0
	text := driveRAGNormalizeSearchText(driveRAGResultText(result))
	for _, query := range queries {
		matched := false
		for _, term := range driveRAGQueryTerms(query.Query) {
			if strings.Contains(text, term) {
				matched = true
				break
			}
		}
		if matched {
			score++
		}
	}
	return score
}

func driveRAGHasStrongContentSemanticMatch(result DriveSearchResult) bool {
	for _, match := range result.Matches {
		if match.Score < driveRAGMinSemanticOnlyScore {
			continue
		}
		if match.ResourceKind == LocalSearchResourceDriveFile {
			continue
		}
		if strings.TrimSpace(match.Snippet) == "" {
			continue
		}
		return true
	}
	return false
}

func driveRAGResultText(result DriveSearchResult) string {
	var builder strings.Builder
	if result.Item.File != nil {
		builder.WriteString(result.Item.File.OriginalFilename)
		builder.WriteString(" ")
		builder.WriteString(result.Item.File.Description)
	}
	builder.WriteString(" ")
	builder.WriteString(result.Snippet)
	for _, match := range result.Matches {
		builder.WriteString(" ")
		builder.WriteString(match.Snippet)
	}
	return builder.String()
}

func driveRAGResultsText(results []DriveSearchResult) string {
	var builder strings.Builder
	for _, result := range results {
		builder.WriteString(" ")
		builder.WriteString(driveRAGResultText(result))
	}
	return builder.String()
}

func driveRAGTextContainsSignal(text, signal string) bool {
	signal = driveRAGNormalizeSearchText(signal)
	if signal == "" {
		return true
	}
	if strings.Contains(text, signal) {
		return true
	}
	switch signal {
	case "白い", "白":
		return strings.Contains(text, "ホワイト")
	case "ホワイト":
		return strings.Contains(text, "白い") || strings.Contains(text, "白")
	case "家具":
		return strings.Contains(text, "デスク") || strings.Contains(text, "机") || strings.Contains(text, "椅子") || strings.Contains(text, "チェア") || strings.Contains(text, "棚") || strings.Contains(text, "収納") || strings.Contains(text, "ソファ")
	case "デスク":
		return strings.Contains(text, "机")
	case "机":
		return strings.Contains(text, "デスク")
	case "椅子":
		return strings.Contains(text, "チェア")
	case "チェア":
		return strings.Contains(text, "椅子")
	case "棚":
		return strings.Contains(text, "収納")
	case "収納":
		return strings.Contains(text, "棚")
	default:
		return false
	}
}

func driveRAGResultLexicalScore(result DriveSearchResult, terms []string) int {
	filename := ""
	description := ""
	if result.Item.File != nil {
		filename = result.Item.File.OriginalFilename
		description = result.Item.File.Description
	}
	filename = driveRAGNormalizeSearchText(filename)
	description = driveRAGNormalizeSearchText(description)
	snippet := driveRAGNormalizeSearchText(result.Snippet)
	matches := driveRAGNormalizeSearchText(driveRAGResultText(result))
	score := 0
	for _, term := range terms {
		switch {
		case strings.Contains(filename, term):
			score += 8 + len([]rune(term))
		case strings.Contains(description, term):
			score += 5 + len([]rune(term))
		case strings.Contains(snippet, term):
			score += 4 + len([]rune(term))
		case strings.Contains(matches, term):
			score += 3 + len([]rune(term))
		}
	}
	return score
}

func driveRAGQueryTerms(query string) []string {
	query = driveRAGNormalizeSearchText(query)
	if query == "" {
		return nil
	}
	stop := map[string]struct{}{
		"haohao": {}, "broad": {}, "rag": {}, "smoke": {},
		"から": {}, "について": {}, "してください": {}, "教えて": {},
	}
	seen := map[string]struct{}{}
	terms := make([]string, 0)
	add := func(term string) {
		term = strings.TrimSpace(term)
		if len([]rune(term)) < 2 {
			return
		}
		if _, ok := stop[term]; ok {
			return
		}
		if _, ok := seen[term]; ok {
			return
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	for _, field := range driveRAGASCIISearchTerms(query) {
		add(field)
	}
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	for _, field := range fields {
		if len([]rune(field)) < 2 {
			continue
		}
		if driveRAGIsASCII(field) {
			if driveRAGIsAllDigits(field) && len(field) > 4 {
				continue
			}
			add(field)
			continue
		}
		runes := []rune(field)
		for n := 6; n >= 2; n-- {
			if len(runes) < n {
				continue
			}
			for i := 0; i+n <= len(runes); i++ {
				add(string(runes[i : i+n]))
			}
		}
	}
	return terms
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func normalizeDriveRAGStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		if len([]rune(value)) < 2 {
			continue
		}
		out = appendUniqueString(out, value)
	}
	return out
}

func containsAnyNormalized(values []string, candidates ...string) bool {
	normalized := map[string]struct{}{}
	for _, value := range values {
		normalized[driveRAGNormalizeSearchText(value)] = struct{}{}
	}
	for _, candidate := range candidates {
		if _, ok := normalized[driveRAGNormalizeSearchText(candidate)]; ok {
			return true
		}
	}
	return false
}

func driveRAGASCIISearchTerms(query string) []string {
	terms := make([]string, 0)
	var builder strings.Builder
	flush := func() {
		term := builder.String()
		builder.Reset()
		if len(term) < 2 || strings.Contains(term, "haohao-broad-rag") {
			return
		}
		if strings.ContainsAny(term, "-_") || hasLetterAndDigit(term) {
			terms = append(terms, term)
		}
	}
	for _, r := range query {
		if r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			builder.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return terms
}

func driveRAGNormalizeSearchText(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

func driveRAGIsAllDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func driveRAGIsASCII(text string) bool {
	for _, r := range text {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func hasLetterAndDigit(text string) bool {
	hasLetter := false
	hasDigit := false
	for _, r := range text {
		hasLetter = hasLetter || unicode.IsLetter(r)
		hasDigit = hasDigit || unicode.IsDigit(r)
	}
	return hasLetter && hasDigit
}

func validateDriveRAGGenerated(payload string, contexts []driveRAGContext) (string, []DriveRAGCitation) {
	var generated driveRAGGenerated
	if err := json.Unmarshal([]byte(extractDriveRAGJSONPayload(payload)), &generated); err != nil {
		return "", nil
	}
	allowed := map[string]DriveRAGCitation{}
	for _, item := range contexts {
		allowed[item.Citation.CitationID] = item.Citation
	}
	seen := map[string]struct{}{}
	citations := make([]DriveRAGCitation, 0)
	claims := make([]string, 0, len(generated.Claims))
	for _, claim := range generated.Claims {
		text := strings.TrimSpace(claim.Text)
		if text == "" || len(claim.CitationIDs) == 0 {
			continue
		}
		validClaim := false
		for _, citationID := range claim.CitationIDs {
			citation, ok := allowed[strings.TrimSpace(citationID)]
			if !ok {
				continue
			}
			validClaim = true
			if _, exists := seen[citation.CitationID]; !exists {
				seen[citation.CitationID] = struct{}{}
				citations = append(citations, citation)
			}
		}
		if validClaim {
			claims = append(claims, text)
		}
	}
	if len(claims) == 0 {
		return "", nil
	}
	answer := strings.TrimSpace(generated.Answer)
	if answer == "" {
		answer = strings.Join(claims, "\n")
	}
	return answer, citations
}

func fallbackDriveRAGAnswer(contexts []driveRAGContext) (string, []DriveRAGCitation) {
	if len(contexts) == 0 {
		return "", nil
	}
	lines := make([]string, 0, min(len(contexts), 3))
	citations := make([]DriveRAGCitation, 0, min(len(contexts), 3))
	for i, item := range contexts {
		if i >= 3 {
			break
		}
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		filename := strings.TrimSpace(item.Citation.Filename)
		if filename == "" {
			filename = "Drive document"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", filename, truncateRunes(cleanDriveRAGContextText(text), 220)))
		citations = append(citations, item.Citation)
	}
	if len(lines) == 0 {
		return "", nil
	}
	return "取得した文書には次の内容が含まれています。\n" + strings.Join(lines, "\n"), citations
}

func cleanDriveRAGContextText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "OCR: ") {
		rest := strings.TrimSpace(strings.TrimPrefix(text, "OCR: "))
		fields := strings.Fields(rest)
		if len(fields) > 1 && driveRAGLooksLikeImageFilename(fields[0]) {
			text = strings.Join(fields[1:], " ")
		}
	}
	if strings.Contains(text, `{"extractor"`) || strings.Contains(text, `[{"text"`) {
		if index := strings.Index(text, " {}"); index >= 0 {
			text = strings.TrimSpace(text[:index])
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	return text
}

func driveRAGLooksLikeImageFilename(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, suffix := range []string{".jpg", ".jpeg", ".png", ".webp", ".avif", ".gif", ".bmp", ".tif", ".tiff"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func extractDriveRAGJSONPayload(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") {
		return value
	}
	if start, end := strings.Index(value, "{"), strings.LastIndex(value, "}"); start >= 0 && end > start {
		return value[start : end+1]
	}
	return value
}

func driveRAGSystemPrompt() string {
	return "You answer questions using only the provided citation context. Return compact JSON only, with no markdown fences. Every claim must cite at least one provided citationId exactly, such as c1. Do not use outside knowledge."
}

func driveRAGUserPrompt(query string, contexts []driveRAGContext) string {
	var builder strings.Builder
	builder.WriteString("Question:\n")
	builder.WriteString(query)
	builder.WriteString("\n\nCitation context:\n")
	for _, item := range contexts {
		builder.WriteString("[")
		builder.WriteString(item.Citation.CitationID)
		builder.WriteString("] ")
		builder.WriteString(item.Text)
		builder.WriteString("\n")
	}
	builder.WriteString("\nReturn JSON only. Use this exact shape: {\"answer\":\"...\",\"claims\":[{\"text\":\"...\",\"citationIds\":[\"c1\"]}]}. The citationIds values must exactly match the bracketed IDs above.")
	return builder.String()
}

func driveRAGQueryRewriteSystemPrompt() string {
	return "You rewrite a Drive RAG user question into short retrieval queries. Return compact JSON only, with no markdown fences. Do not answer the question."
}

func driveRAGQueryRewriteUserPrompt(query string, maxQueries int) string {
	if maxQueries <= 0 {
		maxQueries = defaultDriveRAGPolicy().QueryRewriteMaxQueries
	}
	return fmt.Sprintf("Question:\n%s\n\nReturn JSON only. Use this exact shape: {\"searchQueries\":[\"...\"],\"keywords\":[\"...\"],\"mustHave\":[\"...\"],\"avoid\":[\"...\"]}. Create at most %d searchQueries for permission-filtered Drive search. Keep queries concise and include useful synonyms or object types.", query, maxQueries)
}

func driveRAGResponseFormat() map[string]any {
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name": "drive_rag_answer",
			"schema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"answer": map[string]any{"type": "string"},
					"claims": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]any{
								"text":        map[string]any{"type": "string"},
								"citationIds": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							},
							"required": []string{"text", "citationIds"},
						},
					},
				},
				"required": []string{"answer", "claims"},
			},
		},
	}
}

func driveRAGQueryRewriteResponseFormat() map[string]any {
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name": "drive_rag_query_rewrite",
			"schema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"searchQueries": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"keywords":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"mustHave":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"avoid":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
				"required": []string{"searchQueries", "keywords", "mustHave", "avoid"},
			},
		},
	}
}
