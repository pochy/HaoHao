package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPythonNLPDriveProductExtractorReadsJSONFromStdin(t *testing.T) {
	commandPath, helperPath, capturePath := writeFakePythonNLPCommand(t)
	extractor := NewPythonNLPDriveProductExtractor(PythonNLPProductExtractorProfile{
		Name:       "python",
		Command:    commandPath,
		HelperPath: helperPath,
	})
	result, err := extractor.ExtractProducts(t.Context(), DriveProductExtractionInput{
		TenantID: 1,
		File: DriveFile{
			PublicID: "file-public-id",
		},
		Pages: []DriveOCRPageResult{{
			PageNumber: 1,
			RawText:    "商品名: Python Product\n型番: PY-100",
		}},
		FullText: "商品名: Python Product\n型番: PY-100",
		Policy: DriveOCRPolicy{
			StructuredExtractor:   "python",
			TimeoutSecondsPerPage: 15,
		},
	})
	if err != nil {
		t.Fatalf("ExtractProducts() error = %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Name != "Python Product" {
		t.Fatalf("item name = %q, want Python Product", item.Name)
	}
	if item.Model != "PY-100" {
		t.Fatalf("item model = %q, want PY-100", item.Model)
	}
	if item.Attributes["pythonHelper"] != "drive_product_extraction_nlp.py" {
		t.Fatalf("pythonHelper attribute = %#v", item.Attributes["pythonHelper"])
	}
	if item.Attributes["nlpEngine"] != "python" {
		t.Fatalf("nlpEngine attribute = %#v, want python", item.Attributes["nlpEngine"])
	}

	captured, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("read captured request: %v", err)
	}
	var request pythonNLPProductExtractionRequest
	if err := json.Unmarshal(captured, &request); err != nil {
		t.Fatalf("decode captured request: %v", err)
	}
	if request.Mode != "python" {
		t.Fatalf("request mode = %q, want python", request.Mode)
	}
	if !strings.Contains(request.Text, "PY-100") {
		t.Fatalf("request text = %q, want OCR text", request.Text)
	}
	if len(request.Pages) != 1 || request.Pages[0].PageNumber != 1 {
		t.Fatalf("request pages = %#v, want one page", request.Pages)
	}
}

func TestCheckDriveOCRPythonNLPExtractorReportsAvailability(t *testing.T) {
	commandPath, helperPath, _ := writeFakePythonNLPCommand(t)
	status := checkDriveOCRPythonNLPExtractor(t.Context(), DriveOCRPolicy{StructuredExtractor: "python"}, PythonNLPProductExtractorProfile{
		Name:       "python",
		Command:    commandPath,
		HelperPath: helperPath,
	})
	if !status.Configured {
		t.Fatal("Configured = false, want true")
	}
	if !status.Available {
		t.Fatal("Available = false, want true")
	}
	if status.Version != "fake-python 0.1" {
		t.Fatalf("Version = %q, want fake-python 0.1", status.Version)
	}
}

func TestCheckDriveOCRPythonNLPExtractorReportsDependencyUnavailable(t *testing.T) {
	commandPath, helperPath, _ := writeFakePythonNLPCommand(t)
	status := checkDriveOCRPythonNLPExtractor(t.Context(), DriveOCRPolicy{StructuredExtractor: "ginza"}, PythonNLPProductExtractorProfile{
		Name:       "ginza",
		Command:    commandPath,
		HelperPath: helperPath,
	})
	if !status.Configured {
		t.Fatal("Configured = false, want true")
	}
	if status.Available {
		t.Fatal("Available = true, want false")
	}
	if status.Version != "" {
		t.Fatalf("Version = %q, want empty", status.Version)
	}
}

func TestDriveProductExtractorRouterDispatchesPythonNLP(t *testing.T) {
	extractor := &recordingDriveProductExtractor{name: "python"}
	router := NewDriveProductExtractorRouter(nil, nil, nil, extractor)

	result, err := router.ExtractProducts(t.Context(), DriveProductExtractionInput{
		Policy: DriveOCRPolicy{StructuredExtractor: "python"},
	})
	if err != nil {
		t.Fatalf("ExtractProducts() error = %v", err)
	}
	if !extractor.called {
		t.Fatal("python extractor was not called")
	}
	if len(result.Items) != 1 || result.Items[0].Name != "router product" {
		t.Fatalf("result items = %#v", result.Items)
	}
}

func TestDriveOCRPythonNLPExtractorMigrationAddsConstraintValues(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	up, err := os.ReadFile(filepath.Join(root, "db", "migrations", "0025_drive_ocr_python_nlp_extractors.up.sql"))
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	down, err := os.ReadFile(filepath.Join(root, "db", "migrations", "0025_drive_ocr_python_nlp_extractors.down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	schema, err := os.ReadFile(filepath.Join(root, "db", "schema.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	for _, value := range []string{"'python'", "'ginza'", "'sudachipy'"} {
		if !strings.Contains(string(up), value) {
			t.Fatalf("up migration missing %s", value)
		}
		if !strings.Contains(string(schema), value) {
			t.Fatalf("schema missing %s", value)
		}
	}
	if !strings.Contains(string(down), "SET structured_extractor = 'rules'") {
		t.Fatal("down migration does not map python NLP runs back to rules")
	}
}

func writeFakePythonNLPCommand(t *testing.T) (string, string, string) {
	t.Helper()
	tempDir := t.TempDir()
	commandPath := filepath.Join(tempDir, "fake-python3")
	helperPath := filepath.Join(tempDir, "drive_product_extraction_nlp.py")
	capturePath := filepath.Join(tempDir, "request.json")
	script := `#!/bin/sh
case "$2" in
  extract)
    cat > "$CAPTURE_FILE"
    printf '{"items":[{"itemType":"product","name":"Python Product","model":"PY-100","attributes":{"nlpEngine":"python"},"confidence":0.88}]}'
    ;;
  --check)
    if [ "$3" = "ginza" ]; then
      echo "ginza missing" >&2
      exit 7
    fi
    printf 'fake-%s 0.1\n' "$3"
    ;;
  *)
    echo "unexpected args: $*" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(commandPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake command: %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("# fake helper\n"), 0o644); err != nil {
		t.Fatalf("write fake helper: %v", err)
	}
	t.Setenv("CAPTURE_FILE", capturePath)
	return commandPath, helperPath, capturePath
}

type recordingDriveProductExtractor struct {
	name   string
	called bool
}

func (e *recordingDriveProductExtractor) Name() string {
	return e.name
}

func (e *recordingDriveProductExtractor) ExtractProducts(_ context.Context, _ DriveProductExtractionInput) (DriveProductExtractionResult, error) {
	e.called = true
	return DriveProductExtractionResult{Items: []DriveProductExtractionItem{{Name: "router product"}}}, nil
}
