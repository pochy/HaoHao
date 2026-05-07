package service

import (
	"strings"
	"testing"

	db "example.com/haohao/backend/internal/db"
)

func TestLocalSearchChunksShortDocumentUsesOrdinalZero(t *testing.T) {
	chunks := localSearchChunks(db.LocalSearchDocument{
		Title:       "Customer schema",
		BodyText:    "email address aliases",
		ContentHash: "doc-hash",
	})

	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(chunks))
	}
	if chunks[0].Ordinal != 0 {
		t.Fatalf("Ordinal = %d, want 0", chunks[0].Ordinal)
	}
	if chunks[0].ContentHash == "" {
		t.Fatal("ContentHash is empty")
	}
}

func TestLocalSearchChunksLongDocumentHasOverlapAndLimit(t *testing.T) {
	body := strings.Repeat("あ", localSearchMaxChunkRunes*localSearchMaxChunks*2)
	chunks := localSearchChunks(db.LocalSearchDocument{
		Title:       "Long document",
		BodyText:    body,
		ContentHash: "doc-hash",
	})

	if len(chunks) != localSearchMaxChunks {
		t.Fatalf("len(chunks) = %d, want %d", len(chunks), localSearchMaxChunks)
	}
	for i, chunk := range chunks {
		if chunk.Ordinal != int32(i) {
			t.Fatalf("chunks[%d].Ordinal = %d", i, chunk.Ordinal)
		}
		if len([]rune(chunk.Text)) > localSearchMaxChunkRunes {
			t.Fatalf("chunks[%d] exceeded max chunk runes", i)
		}
	}
}

func TestNormalizeDriveSearchMode(t *testing.T) {
	cases := []struct {
		name string
		mode string
		want string
	}{
		{name: "default", mode: "", want: DriveSearchModeKeyword},
		{name: "keyword", mode: "keyword", want: DriveSearchModeKeyword},
		{name: "semantic", mode: " semantic ", want: DriveSearchModeSemantic},
		{name: "hybrid", mode: "HYBRID", want: DriveSearchModeHybrid},
		{name: "invalid", mode: "vector", want: DriveSearchModeKeyword},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeDriveSearchMode(tc.mode); got != tc.want {
				t.Fatalf("normalizeDriveSearchMode(%q) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

func TestPrependLocalSearchMatchDedupesSameResource(t *testing.T) {
	existing := []LocalSearchMatch{{ResourceKind: LocalSearchResourceDriveFile, ResourcePublicID: "file-1", Snippet: "keyword"}}

	unchanged := prependLocalSearchMatch(existing, LocalSearchMatch{ResourceKind: LocalSearchResourceDriveFile, ResourcePublicID: "file-1", Snippet: "semantic"})
	if len(unchanged) != 1 || unchanged[0].Snippet != "keyword" {
		t.Fatalf("deduped matches = %#v, want existing keyword match", unchanged)
	}

	prepended := prependLocalSearchMatch(existing, LocalSearchMatch{ResourceKind: LocalSearchResourceOCRRun, ResourcePublicID: "ocr-1", Snippet: "semantic"})
	if len(prepended) != 2 || prepended[0].ResourceKind != LocalSearchResourceOCRRun {
		t.Fatalf("prepended matches = %#v, want semantic match first", prepended)
	}
}

func TestLocalSearchSemanticScoreThresholdIsResourceSpecific(t *testing.T) {
	cases := []struct {
		resourceKind string
		want         float64
	}{
		{resourceKind: LocalSearchResourceDriveFile, want: localSearchDriveMinSemanticScore},
		{resourceKind: LocalSearchResourceSchemaColumn, want: localSearchSchemaColumnMinSemanticScore},
		{resourceKind: LocalSearchResourceOCRRun, want: localSearchMinSemanticScore},
	}

	for _, tc := range cases {
		t.Run(tc.resourceKind, func(t *testing.T) {
			if got := localSearchSemanticScoreThreshold(tc.resourceKind); got != tc.want {
				t.Fatalf("localSearchSemanticScoreThreshold(%q) = %v, want %v", tc.resourceKind, got, tc.want)
			}
		})
	}
}

func TestLocalSearchSemanticQueryTextExpandsDriveBusinessTerms(t *testing.T) {
	got := localSearchSemanticQueryText(LocalSearchResourceDriveFile, "支払期限")
	for _, want := range []string{"支払期限", "振込期限", "支払期日", "入金期限"} {
		if !strings.Contains(got, want) {
			t.Fatalf("localSearchSemanticQueryText missing %q in %q", want, got)
		}
	}

	unchanged := localSearchSemanticQueryText(LocalSearchResourceSchemaColumn, "支払期限")
	if unchanged != "支払期限" {
		t.Fatalf("schema semantic query = %q, want original query", unchanged)
	}
}
