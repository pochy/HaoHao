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
	driveRAGDefaultLimit      = 8
	driveRAGMaxLimit          = 20
	driveRAGMaxGenerationText = 1200
)

type DriveRAGQueryInput struct {
	TenantID    int64
	ActorUserID int64
	Query       string
	Mode        string
	Limit       int32
}

type DriveRAGResult struct {
	Answer    string
	Citations []DriveRAGCitation
	Matches   []DriveRAGCitation
	Blocked   bool
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

type driveRAGContext struct {
	Citation DriveRAGCitation
	Text     string
}

type driveRAGGenerated struct {
	Answer string          `json:"answer"`
	Claims []driveRAGClaim `json:"claims"`
}

type driveRAGClaim struct {
	Text        string   `json:"text"`
	CitationIDs []string `json:"citationIds"`
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
	results, err := s.SearchDocuments(ctx, DriveSearchInput{
		TenantID:    input.TenantID,
		ActorUserID: input.ActorUserID,
		Query:       query,
		Mode:        mode,
		Limit:       searchLimit,
		Filter:      DriveListItemsFilter{Type: "all", Owner: "all", Source: "all"},
	}, auditCtx)
	if err != nil {
		return DriveRAGResult{}, err
	}
	results = rankDriveRAGResults(query, results)
	contexts := driveRAGContexts(results, policy.RAG)
	matches := make([]DriveRAGCitation, 0, len(contexts))
	for _, item := range contexts {
		matches = append(matches, item.Citation)
	}
	if len(contexts) == 0 {
		return DriveRAGResult{Blocked: true, Matches: matches}, nil
	}
	provider := NewLocalGenerationProvider(policy.RAG.GenerationRuntime, policy.RAG.GenerationRuntimeURL)
	generated, err := provider.Generate(ctx, GenerationRequest{
		Model:       policy.RAG.GenerationModel,
		System:      driveRAGSystemPrompt(),
		User:        driveRAGUserPrompt(query, contexts),
		MaxTokens:   900,
		Temperature: 0,
		JSONSchema:  driveRAGResponseFormat(),
	})
	if err != nil {
		return DriveRAGResult{}, err
	}
	answer, citations := validateDriveRAGGenerated(generated.Text, contexts)
	if strings.TrimSpace(answer) == "" || len(citations) == 0 {
		return DriveRAGResult{Blocked: true, Matches: matches}, nil
	}
	return DriveRAGResult{
		Answer:    truncateRunes(answer, driveRAGMaxGenerationText),
		Citations: citations,
		Matches:   matches,
	}, nil
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
	if len(results) < 2 {
		return results
	}
	terms := driveRAGQueryTerms(query)
	if len(terms) == 0 {
		return results
	}
	type rankedResult struct {
		result DriveSearchResult
		score  int
		index  int
	}
	ranked := make([]rankedResult, 0, len(results))
	hasPositiveScore := false
	for i, result := range results {
		score := driveRAGResultLexicalScore(result, terms)
		if score > 0 {
			hasPositiveScore = true
		}
		ranked = append(ranked, rankedResult{result: result, score: score, index: i})
	}
	if !hasPositiveScore {
		return results
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].index < ranked[j].index
		}
		return ranked[i].score > ranked[j].score
	})
	out := make([]DriveSearchResult, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, item.result)
	}
	return out
}

func driveRAGResultLexicalScore(result DriveSearchResult, terms []string) int {
	filename := ""
	if result.Item.File != nil {
		filename = result.Item.File.OriginalFilename
	}
	filename = driveRAGNormalizeSearchText(filename)
	snippet := driveRAGNormalizeSearchText(result.Snippet)
	matchText := strings.Builder{}
	for _, match := range result.Matches {
		matchText.WriteString(" ")
		matchText.WriteString(match.Snippet)
	}
	matches := driveRAGNormalizeSearchText(matchText.String())
	score := 0
	for _, term := range terms {
		switch {
		case strings.Contains(filename, term):
			score += 8 + len([]rune(term))
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
	if err := json.Unmarshal([]byte(strings.TrimSpace(payload)), &generated); err != nil {
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

func driveRAGSystemPrompt() string {
	return "You answer questions using only the provided citation context. Return compact JSON only. Every claim must cite at least one citationId. Do not use outside knowledge."
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
	builder.WriteString("\nReturn JSON: {\"answer\":\"...\",\"claims\":[{\"text\":\"...\",\"citationIds\":[\"c1\"]}]}")
	return builder.String()
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
