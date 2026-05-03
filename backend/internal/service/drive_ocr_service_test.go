package service

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	db "example.com/haohao/backend/internal/db"
)

func TestDriveOCRSupportedFileSilverImagePDFTargets(t *testing.T) {
	tests := []struct {
		name string
		file DriveFile
		want bool
	}{
		{name: "pdf content type", file: DriveFile{OriginalFilename: "catalog.bin", ContentType: "application/pdf"}, want: true},
		{name: "pdf extension fallback", file: DriveFile{OriginalFilename: "catalog.pdf", ContentType: "application/octet-stream"}, want: true},
		{name: "png", file: DriveFile{OriginalFilename: "photo.png", ContentType: "image/png"}, want: true},
		{name: "jpeg", file: DriveFile{OriginalFilename: "photo.jpeg", ContentType: "image/jpeg"}, want: true},
		{name: "tiff", file: DriveFile{OriginalFilename: "scan.tiff", ContentType: "application/octet-stream"}, want: true},
		{name: "webp", file: DriveFile{OriginalFilename: "image.webp", ContentType: "application/octet-stream"}, want: true},
		{name: "docx", file: DriveFile{OriginalFilename: "brief.docx", ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := driveOCRSupportedFile(tt.file); got != tt.want {
				t.Fatalf("driveOCRSupportedFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDriveOCRPipelineConfigHashChangesWithRuntimeConfig(t *testing.T) {
	base := defaultDriveOCRPolicy()
	base.Enabled = true
	base.StructuredExtractionEnabled = true
	base.StructuredExtractor = "rules"

	same := base
	same.OCREngine = " TESSERACT "
	changed := base
	changed.MaxPages = base.MaxPages + 1

	baseHash := driveOCRPipelineConfigHash(base)
	if baseHash == "" || baseHash == "config-unavailable" {
		t.Fatalf("expected stable non-empty hash, got %q", baseHash)
	}
	if got := driveOCRPipelineConfigHash(same); got != baseHash {
		t.Fatalf("normalized equivalent policy hash = %q, want %q", got, baseHash)
	}
	if got := driveOCRPipelineConfigHash(changed); got == baseHash {
		t.Fatalf("changed max pages hash = %q, want a different hash", got)
	}
}

func TestDriveOCRPipelineConfigHashNormalizesLanguageAliases(t *testing.T) {
	base := defaultDriveOCRPolicy()
	base.OCRLanguages = []string{"jpn"}
	alias := base
	alias.OCRLanguages = []string{"japanese"}

	if got, want := driveOCRPipelineConfigHash(alias), driveOCRPipelineConfigHash(base); got != want {
		t.Fatalf("japanese alias hash = %q, want %q", got, want)
	}
}

func TestCanReuseDriveOCRRunForPipelineUsesDriveCompletedRun(t *testing.T) {
	file := DriveFile{ID: 10, SHA256Hex: "sha", ContentType: "image/png"}
	policy := defaultDriveOCRPolicy()
	policy.OCREngine = "paddleocr"
	policy.OCRLanguages = []string{"japanese"}

	run := db.DriveOcrRun{
		FileObjectID:  file.ID,
		FileRevision:  fileOCRRevision(file),
		ContentSha256: file.SHA256Hex,
		Engine:        "paddleocr",
		Languages:     []string{"jpn", "eng"},
		Status:        "completed",
		Reason:        "manual",
		ExtractedText: "経費精算申請",
	}
	if !canReuseDriveOCRRunForPipeline(run, file, policy) {
		t.Fatal("expected completed manual OCR run to be reusable for Japanese pipeline OCR")
	}
	run.Reason = "data_pipeline_preview"
	if canReuseDriveOCRRunForPipeline(run, file, policy) {
		t.Fatal("expected data pipeline preview OCR run not to be preferred for pipeline reuse")
	}
}

func TestDriveOCRProviderFailureCodeDependencyUnavailable(t *testing.T) {
	if got := driveOCRProviderFailureCode(ErrDriveOCRDependencyUnavailable); got != "dependency_unavailable" {
		t.Fatalf("driveOCRProviderFailureCode() = %q, want dependency_unavailable", got)
	}
}

func TestSearchSnippetTruncatesByRune(t *testing.T) {
	got := searchSnippet(strings.Repeat("あ", 241), "fallback")
	if !utf8.ValidString(got) {
		t.Fatalf("searchSnippet returned invalid UTF-8")
	}
	if utf8.RuneCountInString(got) != 240 {
		t.Fatalf("searchSnippet rune count = %d, want 240", utf8.RuneCountInString(got))
	}
}

func TestSearchTextSanitizersReplaceInvalidUTF8(t *testing.T) {
	searchText := sanitizeSearchText("OK \xe3\x82")
	if !utf8.ValidString(searchText) {
		t.Fatalf("sanitizeSearchText returned invalid UTF-8")
	}
	ocrText := normalizeOCRText("OK \xe3\x82")
	if !utf8.ValidString(ocrText) {
		t.Fatalf("normalizeOCRText returned invalid UTF-8")
	}
	errMessage := trimOCRProcessError(errors.New(strings.Repeat("あ", 2001)))
	if !utf8.ValidString(errMessage) {
		t.Fatalf("trimOCRProcessError returned invalid UTF-8")
	}
	if utf8.RuneCountInString(errMessage) != 2000 {
		t.Fatalf("trimOCRProcessError rune count = %d, want 2000", utf8.RuneCountInString(errMessage))
	}
}

func TestLocalDriveOCRProviderCheckUsesSelectedPaddleEngine(t *testing.T) {
	statuses := LocalDriveOCRProvider{}.Check(t.Context(), DriveOCRPolicy{OCREngine: "paddleocr"})
	if len(statuses) != 1 {
		t.Fatalf("dependency count = %d, want 1", len(statuses))
	}
	if statuses[0].Name != "paddleocr" {
		t.Fatalf("dependency name = %q, want paddleocr", statuses[0].Name)
	}
}

func TestPaddleOCRLanguagePrefersJapanese(t *testing.T) {
	if got := paddleOCRLanguage(DriveOCRPolicy{OCRLanguages: []string{"jpn", "eng"}}); got != "japan" {
		t.Fatalf("paddleOCRLanguage() = %q, want japan", got)
	}
	if got := paddleOCRLanguage(DriveOCRPolicy{OCRLanguages: []string{"japanese"}}); got != "japan" {
		t.Fatalf("paddleOCRLanguage() = %q, want japan", got)
	}
	if got := paddleOCRLanguage(DriveOCRPolicy{OCRLanguages: []string{"eng"}}); got != "en" {
		t.Fatalf("paddleOCRLanguage() = %q, want en", got)
	}
}

func TestParsePaddleOCROutput(t *testing.T) {
	raw := "[[[1.0, 2.0]], ('Sサイズ 機内持込OK!', 0.98)]\n[[[3.0, 4.0]], ('115cm', 0.96)]"
	got := parsePaddleOCROutput(raw)
	if got != "Sサイズ 機内持込OK!\n115cm" {
		t.Fatalf("parsePaddleOCROutput() = %q", got)
	}
}

func TestPaddleOCRImageRunsHelperAndParsesJSON(t *testing.T) {
	commandPath, helperPath, capturePath := writeFakePaddleOCRCommand(t, 0, `{"text":"Sサイズ 機内持込OK!\n115cm","averageConfidence":0.97,"layout":{"engine":"paddleocr","lineCount":2},"boxes":[{"text":"Sサイズ 機内持込OK!","score":0.98},{"text":"115cm","score":0.96}]}`)
	t.Setenv(paddleOCRPythonEnv, commandPath)
	t.Setenv(paddleOCRHelperEnv, helperPath)
	imagePath := filepath.Join(t.TempDir(), "input.png")
	if err := os.WriteFile(imagePath, []byte("fake image"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := paddleOCRImage(t.Context(), imagePath, DriveOCRPolicy{OCRLanguages: []string{"jpn", "eng"}, TimeoutSecondsPerPage: 5})
	if err != nil {
		t.Fatalf("paddleOCRImage() error = %v", err)
	}
	if got.Text != "Sサイズ 機内持込OK!\n115cm" {
		t.Fatalf("paddleOCRImage().Text = %q", got.Text)
	}
	if got.AverageConfidence == nil || *got.AverageConfidence != 0.97 {
		t.Fatalf("paddleOCRImage().AverageConfidence = %v", got.AverageConfidence)
	}
	if string(got.LayoutJSON) != `{"engine":"paddleocr","lineCount":2}` {
		t.Fatalf("paddleOCRImage().LayoutJSON = %s", got.LayoutJSON)
	}
	if string(got.BoxesJSON) == "" || string(got.BoxesJSON) == "[]" {
		t.Fatalf("paddleOCRImage().BoxesJSON = %s", got.BoxesJSON)
	}
	requestBytes, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	request := string(requestBytes)
	if !strings.Contains(request, `"lang":"japan"`) || !strings.Contains(request, `"device":"cpu"`) {
		t.Fatalf("helper request = %s", request)
	}
}

func TestPaddleOCRDependencyStatusUsesHelperCheck(t *testing.T) {
	commandPath, helperPath, _ := writeFakePaddleOCRCommand(t, 0, `{"text":"unused"}`)
	t.Setenv(paddleOCRPythonEnv, commandPath)
	t.Setenv(paddleOCRHelperEnv, helperPath)

	status := paddleOCRDependencyStatus(t.Context())
	if !status.Available {
		t.Fatalf("paddleOCRDependencyStatus().Available = false")
	}
	if status.Version != "paddleocr 3.5.0 / paddle 3.2.0" {
		t.Fatalf("paddleOCRDependencyStatus().Version = %q", status.Version)
	}
}

func writeFakePaddleOCRCommand(t *testing.T, extractExit int, extractOutput string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	helperPath := filepath.Join(dir, "drive_ocr_paddleocr.py")
	if err := os.WriteFile(helperPath, []byte("# fake helper\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	capturePath := filepath.Join(dir, "request.json")
	commandPath := filepath.Join(dir, "fake-python")
	script := `#!/bin/sh
if [ "$2" = "--check" ]; then
  echo "paddleocr 3.5.0 / paddle 3.2.0"
  exit 0
fi
cat > "$CAPTURE_PATH"
if [ "$EXTRACT_EXIT" != "0" ]; then
  echo "paddleocr dependency unavailable: missing package" >&2
  exit "$EXTRACT_EXIT"
fi
printf '%s\n' "$EXTRACT_OUTPUT"
`
	if err := os.WriteFile(commandPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CAPTURE_PATH", capturePath)
	t.Setenv("EXTRACT_EXIT", strconv.Itoa(extractExit))
	t.Setenv("EXTRACT_OUTPUT", extractOutput)
	return commandPath, helperPath, capturePath
}
