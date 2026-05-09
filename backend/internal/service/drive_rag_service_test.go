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
