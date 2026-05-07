package service

import "testing"

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

func TestValidateDriveRAGGeneratedBlocksNoCitationAnswer(t *testing.T) {
	answer, citations := validateDriveRAGGenerated(`{"answer":"支払期限は2026-06-30です。","claims":[{"text":"支払期限は2026-06-30です。","citationIds":[]}]}`, []driveRAGContext{
		{Citation: DriveRAGCitation{CitationID: "c1", FilePublicID: "file-1"}},
	})
	if answer != "" || len(citations) != 0 {
		t.Fatalf("answer=%q citations=%#v, want blocked", answer, citations)
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
