package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalCommandDriveProductExtractorReadsPromptFromStdin(t *testing.T) {
	tempDir := t.TempDir()
	commandPath := filepath.Join(tempDir, "fake-llm")
	script := `#!/bin/sh
input=$(cat)
case "$input" in
  *4B-C40GT3*) ;;
  *) echo "prompt did not contain OCR text" >&2; exit 2 ;;
esac
printf '{"items":[{"itemType":"product","name":"Sharp BD Recorder","model":"4B-C40GT3","confidence":0.91}]}'
`
	if err := os.WriteFile(commandPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake command: %v", err)
	}
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	extractor := NewLocalCommandDriveProductExtractor(LocalCommandProductExtractorProfile{
		Name:    "gemini",
		Command: "fake-llm",
		Args:    []string{"--json"},
	})
	result, err := extractor.ExtractProducts(t.Context(), DriveProductExtractionInput{
		TenantID: 1,
		File: DriveFile{
			PublicID: "file-public-id",
		},
		FullText: "形名 4B-C40GT3 ブルーレイディスクレコーダー",
		Policy: DriveOCRPolicy{
			StructuredExtractor:   "gemini",
			TimeoutSecondsPerPage: 15,
		},
	})
	if err != nil {
		t.Fatalf("ExtractProducts() error = %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	if result.Items[0].Model != "4B-C40GT3" {
		t.Fatalf("item model = %q, want 4B-C40GT3", result.Items[0].Model)
	}
	if result.Items[0].Attributes["localCommand"] != "gemini" {
		t.Fatalf("localCommand attribute = %v, want gemini", result.Items[0].Attributes["localCommand"])
	}
}

func TestCheckDriveOCRLocalCommandReportsAvailability(t *testing.T) {
	tempDir := t.TempDir()
	commandPath := filepath.Join(tempDir, "fake-llm")
	script := `#!/bin/sh
printf 'fake-llm 1.2.3\n'
`
	if err := os.WriteFile(commandPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake command: %v", err)
	}
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	status := checkDriveOCRLocalCommand(t.Context(), DriveOCRPolicy{StructuredExtractor: "codex"}, LocalCommandProductExtractorProfile{
		Name:        "codex",
		Command:     "fake-llm",
		VersionArgs: []string{"--version"},
	})
	if !status.Configured {
		t.Fatal("Configured = false, want true")
	}
	if !status.Available {
		t.Fatal("Available = false, want true")
	}
	if status.Version != "fake-llm 1.2.3" {
		t.Fatalf("Version = %q, want fake-llm 1.2.3", status.Version)
	}
}
