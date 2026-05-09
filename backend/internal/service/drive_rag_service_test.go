package service

import (
	"strings"
	"testing"
)

func TestValidateDriveRAGGeneratedRequiresKnownCitations(t *testing.T) {
	contexts := []driveRAGContext{
		{Citation: DriveRAGCitation{CitationID: "c1", FilePublicID: "file-1", Snippet: "支払期限: 2026-06-30"}},
	}

	answer, citations := validateDriveRAGGenerated(`{"answer":"支払期限は2026-06-30です。","claims":[{"text":"支払期限は2026-06-30です。","citationIds":["c1"]},{"text":"根拠なしの主張です。","citationIds":[]},{"text":"存在しない引用です。","citationIds":["c99"]}]}`, contexts)
	if answer == "" {
		t.Fatal("answer is empty")
	}
	if len(citations) != 1 || citations[0].CitationID != "c1" {
		t.Fatalf("citations = %#v, want c1 only", citations)
	}
}

func TestValidateDriveRAGGeneratedAcceptsMarkdownAndCitationAliases(t *testing.T) {
	contexts := []driveRAGContext{
		{Citation: DriveRAGCitation{CitationID: "c1", FilePublicID: "file-1", Snippet: "ゲームの企画書"}},
	}

	answer, citations := validateDriveRAGGenerated("```json\n{\"answer\":\"ゲームの企画書です。\",\"claims\":[{\"text\":\"ゲームの企画書です。\",\"citation_ids\":[\"c1\"]}]}\n```", contexts)
	if answer == "" {
		t.Fatal("answer is empty")
	}
	if len(citations) != 1 || citations[0].CitationID != "c1" {
		t.Fatalf("citations = %#v, want c1", citations)
	}
}

func TestValidateDriveRAGGeneratedBlocksNoCitationAnswer(t *testing.T) {
	answer, citations := validateDriveRAGGenerated(`{"answer":"支払期限は2026-06-30です。","claims":[{"text":"支払期限は2026-06-30です。","citationIds":[]}]}`, []driveRAGContext{
		{Citation: DriveRAGCitation{CitationID: "c1", FilePublicID: "file-1"}},
	})
	if answer != "" || len(citations) != 0 {
		t.Fatalf("answer=%q citations=%#v, want blocked", answer, citations)
	}
}

func TestFallbackDriveRAGAnswerUsesRetrievedContext(t *testing.T) {
	answer, citations := fallbackDriveRAGAnswer([]driveRAGContext{
		{
			Citation: DriveRAGCitation{CitationID: "c1", FilePublicID: "file-1", Filename: "MVP企画書作成：ダークファンタジーRPG.md"},
			Text:     "ゲームの企画書。探索、バトル、育成を中心にしたMVP。",
		},
	})
	if answer == "" || !strings.Contains(answer, "MVP企画書作成") {
		t.Fatalf("answer = %q, want fallback content", answer)
	}
	if len(citations) != 1 || citations[0].CitationID != "c1" {
		t.Fatalf("citations = %#v, want c1", citations)
	}
}

func TestFallbackDriveRAGAnswerCleansOCRAndProductMetadata(t *testing.T) {
	answer, citations := fallbackDriveRAGAnswer([]driveRAGContext{
		{
			Citation: DriveRAGCitation{CitationID: "c1", FilePublicID: "file-1", Filename: "milk.jpg"},
			Text:     `OCR: milk.jpg サントリ 烏龍茶 ミルクティー 甘み`,
		},
		{
			Citation: DriveRAGCitation{CitationID: "c2", FilePublicID: "file-2", Filename: "product.jpg"},
			Text:     `ミルクティー product ミルクティー OOLONG MILKTEA {} {} {} [{"text":"ミルクティー"}] {"extractor":"lmstudio"}`,
		},
	})
	if strings.Contains(answer, "OCR:") || strings.Contains(answer, `{"extractor"`) || strings.Contains(answer, `[{"text"`) {
		t.Fatalf("answer = %q, want cleaned fallback", answer)
	}
	if !strings.Contains(answer, "サントリ 烏龍茶 ミルクティー") || !strings.Contains(answer, "OOLONG MILKTEA") {
		t.Fatalf("answer = %q, want cleaned content", answer)
	}
	if len(citations) != 2 {
		t.Fatalf("citations = %#v, want 2", citations)
	}
}

func TestDriveRAGContextsSkipsDLPBlockedFiles(t *testing.T) {
	contexts := driveRAGContexts([]DriveSearchResult{
		{Item: DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "blocked", OriginalFilename: "blocked.txt", DLPBlocked: true}}, Snippet: "blocked"},
		{Item: DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "allowed", OriginalFilename: "allowed.txt"}}, Snippet: "allowed snippet"},
	}, defaultDriveRAGPolicy())
	if len(contexts) != 1 || contexts[0].Citation.FilePublicID != "allowed" {
		t.Fatalf("contexts = %#v, want allowed file only", contexts)
	}
}

func TestRankDriveRAGResultsPromotesQuerySpecificFile(t *testing.T) {
	results := []DriveSearchResult{
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "policy", OriginalFilename: "買掛運用メモ.txt"}},
			Snippet: "請求書の支払期限は各請求書本文を確認する。",
		},
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "invoice", OriginalFilename: "青葉商事_請求書_AB-2026-0412.txt"}},
			Snippet: "青葉商事 請求書 支払期限: 2026-06-30",
		},
	}

	ranked := rankDriveRAGResults("青葉商事の請求書から支払期限を教えてください", results)
	if ranked[0].Item.File == nil || ranked[0].Item.File.PublicID != "invoice" {
		t.Fatalf("ranked[0] = %#v, want invoice first", ranked[0])
	}
}

func TestRankDriveRAGResultsDropsZeroLexicalMatchesWhenAnyResultMatches(t *testing.T) {
	results := []DriveSearchResult{
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "game-plan", OriginalFilename: "ゲーム企画書.txt"}},
			Snippet: "ゲームの企画書。バトル、進行、報酬について。",
		},
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "invoice", OriginalFilename: "青葉商事_請求書.txt"}},
			Snippet: "支払期限: 2026-06-30",
		},
	}

	ranked := rankDriveRAGResults("ゲームの企画書", results)
	if len(ranked) != 1 {
		t.Fatalf("ranked length = %d, want 1: %#v", len(ranked), ranked)
	}
	if ranked[0].Item.File == nil || ranked[0].Item.File.PublicID != "game-plan" {
		t.Fatalf("ranked[0] = %#v, want game-plan", ranked[0])
	}
}

func TestRankDriveRAGResultsDropsSingleUnrelatedSemanticHit(t *testing.T) {
	results := []DriveSearchResult{
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "design", OriginalFilename: "高性能デザインツール技術設計.txt"}},
			Snippet: "次世代Webグラフィックツールのためのハイパフォーマンス・アーキテクチャ設計報告書",
			Matches: []LocalSearchMatch{
				{ResourceKind: LocalSearchResourceDriveFile, ResourcePublicID: "design", Snippet: "次世代Webグラフィックツールのためのハイパフォーマンス・アーキテクチャ設計報告書", Score: 0.72},
			},
		},
	}

	ranked := rankDriveRAGResults("結婚相談所", results)
	if len(ranked) != 0 {
		t.Fatalf("ranked = %#v, want no unrelated candidates", ranked)
	}
}

func TestRankDriveRAGResultsKeepsStrongSemanticHitWithoutLexicalSignal(t *testing.T) {
	results := []DriveSearchResult{
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "milk-tea", OriginalFilename: "drink.txt"}},
			Snippet: "ミルクティー 茶葉 砂糖",
			Matches: []LocalSearchMatch{
				{ResourceKind: LocalSearchResourceOCRRun, ResourcePublicID: "ocr-1", Snippet: "ミルクティー 茶葉 砂糖", Score: 0.87},
			},
		},
	}

	ranked := rankDriveRAGResults("紅茶", results)
	if len(ranked) != 1 {
		t.Fatalf("ranked length = %d, want 1: %#v", len(ranked), ranked)
	}
	if ranked[0].Item.File == nil || ranked[0].Item.File.PublicID != "milk-tea" {
		t.Fatalf("ranked[0] = %#v, want milk-tea", ranked[0])
	}
}

func TestRankDriveRAGResultsDropsFilenameOnlySemanticHitWithoutLexicalSignal(t *testing.T) {
	results := []DriveSearchResult{
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "milk-tea-image", OriginalFilename: "alt111_4901777442184_i_20251111155311.jpg"}},
			Snippet: "alt111_4901777442184_i_20251111155311.jpg",
			Matches: []LocalSearchMatch{
				{ResourceKind: LocalSearchResourceDriveFile, ResourcePublicID: "milk-tea-image", Snippet: "alt111_4901777442184_i_20251111155311.jpg", Score: 0.87},
			},
		},
	}

	ranked := rankDriveRAGResults("ガンダム", results)
	if len(ranked) != 0 {
		t.Fatalf("ranked = %#v, want no filename-only semantic candidates", ranked)
	}
}

func TestRankDriveRAGResultsKeepsOriginalOrderWithoutLexicalSignal(t *testing.T) {
	results := []DriveSearchResult{
		{Item: DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "first", OriginalFilename: "first.txt"}}},
		{Item: DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "second", OriginalFilename: "second.txt"}}},
	}

	ranked := rankDriveRAGResults("   ", results)
	if ranked[0].Item.File.PublicID != "first" || ranked[1].Item.File.PublicID != "second" {
		t.Fatalf("ranked = %#v, want original order", ranked)
	}
}

func TestDriveRAGQueryTermsKeepsHyphenatedIdentifiers(t *testing.T) {
	terms := driveRAGQueryTerms("TP-2026-0412 の支払期限と金額を教えて haohao-broad-rag-1778163058217")
	for _, term := range terms {
		if term == "tp-2026-0412" {
			return
		}
	}
	t.Fatalf("terms = %#v, want tp-2026-0412", terms)
}

func TestDriveRAGDeterministicQueryPlanExpandsWhiteInteriorFurniture(t *testing.T) {
	plan := driveRAGDeterministicQueryPlan("白いインテリアに合う家具は？", 6)
	joinedKeywords := strings.Join(plan.Keywords, " ")
	for _, want := range []string{"白い", "インテリア", "家具", "デスク", "椅子", "棚", "ソファ", "木製", "観葉植物", "ミニマル"} {
		if !strings.Contains(joinedKeywords, want) {
			t.Fatalf("keywords = %#v, want %q", plan.Keywords, want)
		}
	}
	if len(plan.RetrievalQueries) < 3 {
		t.Fatalf("retrievalQueries = %#v, want multiple expanded queries", plan.RetrievalQueries)
	}
	var hasFocusedFurniture bool
	for _, query := range plan.RetrievalQueries {
		if query.Query == "白い机" || query.Query == "白い椅子" || query.Query == "白いインテリア" {
			hasFocusedFurniture = true
		}
	}
	if !hasFocusedFurniture {
		t.Fatalf("retrievalQueries = %#v, want focused furniture expansion", plan.RetrievalQueries)
	}
}

func TestDriveRAGDeterministicQueryPlanExpandsInvoicePaymentTerms(t *testing.T) {
	plan := driveRAGDeterministicQueryPlan("この請求書の支払期限と税込合計は？", 6)
	joinedKeywords := strings.Join(plan.Keywords, " ")
	for _, want := range []string{"請求書", "支払期限", "振込期限", "税込合計"} {
		if !strings.Contains(joinedKeywords, want) {
			t.Fatalf("keywords = %#v, want %q", plan.Keywords, want)
		}
	}
	for _, query := range plan.RetrievalQueries {
		if query.Query == "請求書 振込期限 税込合計" {
			return
		}
	}
	t.Fatalf("retrievalQueries = %#v, want invoice payment expansion", plan.RetrievalQueries)
}

func TestDriveRAGDeterministicQueryPlanKeepsSingleInteriorSearch(t *testing.T) {
	plan := driveRAGDeterministicQueryPlan("インテリア", 4)
	if len(plan.RetrievalQueries) == 0 || plan.RetrievalQueries[0].Query != "インテリア" {
		t.Fatalf("retrievalQueries = %#v, want original query first", plan.RetrievalQueries)
	}
	if !containsAnyNormalized(plan.Keywords, "インテリア") {
		t.Fatalf("keywords = %#v, want インテリア", plan.Keywords)
	}
}

func TestDriveRAGQueryPlanFromRewriteParsesLLMJSON(t *testing.T) {
	plan, err := driveRAGQueryPlanFromRewrite("白いインテリアに合う家具は？", `{"searchQueries":["白い インテリア 家具","白い デスク 椅子 棚"],"keywords":["白い","家具"],"mustHave":["白い"],"avoid":["屋外"]}`, 3)
	if err != nil {
		t.Fatalf("driveRAGQueryPlanFromRewrite() error = %v", err)
	}
	if plan.Source != "llm" {
		t.Fatalf("source = %q, want llm", plan.Source)
	}
	if len(plan.RetrievalQueries) != 3 {
		t.Fatalf("retrievalQueries = %#v, want original plus two rewrites", plan.RetrievalQueries)
	}
	if !containsAnyNormalized(plan.MustHave, "白い") || !containsAnyNormalized(plan.Avoid, "屋外") {
		t.Fatalf("plan = %#v, want mustHave and avoid", plan)
	}
}

func TestMergeDriveRAGSearchResultsDeduplicatesByFilePublicID(t *testing.T) {
	coverage := map[string]int{}
	first := []DriveSearchResult{
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "file-1", OriginalFilename: "a.txt"}},
			Snippet: "白い家具",
			Matches: []LocalSearchMatch{
				{ResourceKind: LocalSearchResourceOCRRun, ResourcePublicID: "ocr-1", Snippet: "白い家具", Score: 0.7},
			},
		},
	}
	second := []DriveSearchResult{
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "file-1", OriginalFilename: "a.txt"}},
			Snippet: "白い家具と椅子と棚",
			Matches: []LocalSearchMatch{
				{ResourceKind: LocalSearchResourceProductExtraction, ResourcePublicID: "product-1", Snippet: "椅子 棚", Score: 0.9},
			},
		},
	}

	merged := mergeDriveRAGSearchResults(nil, first, coverage)
	merged = mergeDriveRAGSearchResults(merged, second, coverage)
	if len(merged) != 1 {
		t.Fatalf("merged = %#v, want one file", merged)
	}
	if merged[0].Snippet != "白い家具と椅子と棚" {
		t.Fatalf("snippet = %q, want longer snippet", merged[0].Snippet)
	}
	if len(merged[0].Matches) != 2 {
		t.Fatalf("matches = %#v, want merged matches", merged[0].Matches)
	}
	if coverage["file-1"] != 2 {
		t.Fatalf("coverage = %#v, want two query hits", coverage)
	}
}

func TestDriveRAGSufficiencyDetectsMissingSignals(t *testing.T) {
	plan := driveRAGDeterministicQueryPlan("白いインテリアに合う家具は？", 6)
	results := []DriveSearchResult{
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "white", OriginalFilename: "white.txt"}},
			Snippet: "白い壁と明るい部屋",
		},
	}

	sufficiency := driveRAGSufficiency(plan, results)
	if sufficiency.Sufficient {
		t.Fatalf("sufficiency = %#v, want missing signals", sufficiency)
	}
	if !containsAnyNormalized(sufficiency.MissingSignals, "インテリア") || !containsAnyNormalized(sufficiency.MissingSignals, "家具") {
		t.Fatalf("missingSignals = %#v, want インテリア and 家具", sufficiency.MissingSignals)
	}
}

func TestDriveRAGSufficiencyAcceptsFurnitureSynonyms(t *testing.T) {
	plan := driveRAGDeterministicQueryPlan("白いインテリアに合う家具は？", 6)
	results := []DriveSearchResult{
		{
			Item:    DriveItem{Type: DriveItemTypeFile, File: &DriveFile{PublicID: "desk", OriginalFilename: "desk.txt"}},
			Snippet: "白いデスクはインテリアに合わせやすい",
		},
	}

	sufficiency := driveRAGSufficiency(plan, results)
	if !sufficiency.Sufficient {
		t.Fatalf("sufficiency = %#v, want sufficient with furniture synonym", sufficiency)
	}
}

func TestDriveRAGRetryQueryPlanUsesMissingSignals(t *testing.T) {
	plan := driveRAGDeterministicQueryPlan("白いインテリアに合う家具は？", 6)
	retry := driveRAGRetryQueryPlan(plan, DriveRAGSufficiencyResult{
		MissingSignals: []string{"家具"},
	})
	if len(retry.RetrievalQueries) == 0 {
		t.Fatal("retry.RetrievalQueries is empty")
	}
	var hasFurnitureRetry bool
	for _, query := range retry.RetrievalQueries {
		if strings.Contains(query.Query, "デスク") && strings.Contains(query.Query, "椅子") && strings.Contains(query.Query, "棚") {
			hasFurnitureRetry = true
		}
	}
	if !hasFurnitureRetry {
		t.Fatalf("retrievalQueries = %#v, want furniture retry query", retry.RetrievalQueries)
	}
}
