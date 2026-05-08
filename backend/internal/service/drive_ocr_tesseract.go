package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	paddleOCRPythonEnv = "HAOHAO_DRIVE_PADDLEOCR_PYTHON"
	paddleOCRHelperEnv = "HAOHAO_DRIVE_PADDLEOCR_HELPER"
	paddleOCRDeviceEnv = "HAOHAO_DRIVE_PADDLEOCR_DEVICE"
)

type LocalDriveOCRProvider struct{}

func NewLocalDriveOCRProvider() LocalDriveOCRProvider {
	return LocalDriveOCRProvider{}
}

func (LocalDriveOCRProvider) Name() string {
	return "tesseract"
}

func (LocalDriveOCRProvider) Check(ctx context.Context, policy DriveOCRPolicy) []DriveOCRDependencyStatus {
	policy = normalizeDriveOCRPolicy(policy)
	switch policy.OCREngine {
	case "paddleocr":
		return []DriveOCRDependencyStatus{paddleOCRDependencyStatus(ctx)}
	case "docling":
		return []DriveOCRDependencyStatus{commandStatus(ctx, "docling", "--version")}
	}
	items := []DriveOCRDependencyStatus{
		commandStatus(ctx, "tesseract", "--version"),
		commandStatus(ctx, "pdftotext", "-v"),
		commandStatus(ctx, "pdftoppm", "-v"),
	}
	langs := tesseractLanguages(ctx)
	for _, lang := range policy.OCRLanguages {
		items = append(items, DriveOCRDependencyStatus{
			Name:      lang + ".traineddata",
			Available: langs[lang],
		})
	}
	if _, err := exec.LookPath("magick"); err == nil {
		items = append(items, commandStatus(ctx, "magick", "-version"))
	} else {
		items = append(items, commandStatus(ctx, "convert", "-version"))
	}
	return items
}

func (p LocalDriveOCRProvider) Extract(ctx context.Context, input DriveOCRProviderInput) (DriveOCRProviderResult, error) {
	if input.Body == nil {
		return DriveOCRProviderResult{}, fmt.Errorf("%w: empty body", ErrDriveInvalidInput)
	}
	policy := normalizeDriveOCRPolicy(input.Policy)
	input.Policy = policy
	contentType := strings.ToLower(strings.TrimSpace(input.File.ContentType))
	ext := strings.ToLower(filepath.Ext(input.File.OriginalFilename))
	if driveOCRTextLikeContentType(contentType) || ext == ".txt" || ext == ".md" || ext == ".csv" || ext == ".json" || ext == ".xml" {
		return p.extractText(input.Body)
	}
	switch policy.OCREngine {
	case "tesseract":
		return p.extractTesseract(ctx, input, contentType, ext)
	case "paddleocr":
		return p.extractPaddleOCR(ctx, input, contentType, ext)
	case "docling":
		return DriveOCRProviderResult{}, fmt.Errorf("%w: docling ocr engine is not available in this runtime", ErrDriveOCRDependencyUnavailable)
	default:
		return DriveOCRProviderResult{}, fmt.Errorf("%w: %s ocr engine is not supported", ErrDriveOCRUnsupported, policy.OCREngine)
	}
}

func (p LocalDriveOCRProvider) extractTesseract(ctx context.Context, input DriveOCRProviderInput, contentType, ext string) (DriveOCRProviderResult, error) {
	switch {
	case contentType == "application/pdf" || ext == ".pdf":
		return p.extractPDF(ctx, input)
	case strings.HasPrefix(contentType, "image/") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".tif" || ext == ".tiff" || ext == ".webp":
		return p.extractImage(ctx, input)
	default:
		return DriveOCRProviderResult{}, ErrDriveOCRUnsupported
	}
}

func driveOCRTextLikeContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "json") ||
		contentType == "application/xml" ||
		contentType == "text/xml" ||
		strings.HasSuffix(contentType, "+xml")
}

func (LocalDriveOCRProvider) extractText(body io.Reader) (DriveOCRProviderResult, error) {
	data, err := io.ReadAll(io.LimitReader(body, 4*1024*1024))
	if err != nil {
		return DriveOCRProviderResult{}, err
	}
	text := normalizeOCRText(string(data))
	if text == "" {
		return DriveOCRProviderResult{}, ErrDriveOCRUnsupported
	}
	page := DriveOCRPageResult{PageNumber: 1, RawText: text, LayoutJSON: []byte("{}"), BoxesJSON: []byte("[]")}
	return DriveOCRProviderResult{Pages: []DriveOCRPageResult{page}, FullText: text}, nil
}

func (p LocalDriveOCRProvider) extractPDF(ctx context.Context, input DriveOCRProviderInput) (DriveOCRProviderResult, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return DriveOCRProviderResult{}, fmt.Errorf("%w: pdftotext", ErrDriveOCRDependencyUnavailable)
	}
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return DriveOCRProviderResult{}, err
	}
	tmpDir, err := os.MkdirTemp("", "haohao-drive-ocr-*")
	if err != nil {
		return DriveOCRProviderResult{}, err
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.pdf")
	outputPath := filepath.Join(tmpDir, "output.txt")
	if err := os.WriteFile(inputPath, data, 0600); err != nil {
		return DriveOCRProviderResult{}, err
	}
	timeout := time.Duration(max(1, input.Policy.TimeoutSecondsPerPage*max(1, input.Policy.MaxPages))) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "pdftotext", "-layout", "-f", "1", "-l", fmt.Sprintf("%d", input.Policy.MaxPages), inputPath, outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return DriveOCRProviderResult{}, fmt.Errorf("pdftotext: %w: %s", err, strings.TrimSpace(string(out)))
	}
	textBytes, _ := os.ReadFile(outputPath)
	result := pagesFromText(string(textBytes), input.Policy.MaxPages)
	if result.FullText != "" {
		return result, nil
	}
	return p.extractRasterizedPDF(ctx, input, inputPath, tmpDir)
}

func (p LocalDriveOCRProvider) extractRasterizedPDF(ctx context.Context, input DriveOCRProviderInput, inputPath, tmpDir string) (DriveOCRProviderResult, error) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return DriveOCRProviderResult{}, fmt.Errorf("%w: pdftoppm", ErrDriveOCRDependencyUnavailable)
	}
	prefix := filepath.Join(tmpDir, "page")
	timeout := time.Duration(max(1, input.Policy.TimeoutSecondsPerPage*max(1, input.Policy.MaxPages))) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "pdftoppm", "-r", "300", "-png", "-f", "1", "-l", fmt.Sprintf("%d", input.Policy.MaxPages), inputPath, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		return DriveOCRProviderResult{}, fmt.Errorf("pdftoppm: %w: %s", err, strings.TrimSpace(string(out)))
	}
	files, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return DriveOCRProviderResult{}, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return DriveOCRProviderResult{}, ErrDriveOCRUnsupported
	}
	pages := make([]DriveOCRPageResult, 0, min(len(files), input.Policy.MaxPages))
	for i, file := range files {
		if i >= input.Policy.MaxPages {
			break
		}
		text, err := tesseractImage(ctx, file, input.Policy)
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		pages = append(pages, DriveOCRPageResult{PageNumber: i + 1, RawText: text, LayoutJSON: []byte("{}"), BoxesJSON: []byte("[]")})
	}
	return resultFromPages(pages), nil
}

func (p LocalDriveOCRProvider) extractPaddleOCR(ctx context.Context, input DriveOCRProviderInput, contentType, ext string) (DriveOCRProviderResult, error) {
	if _, err := exec.LookPath(paddleOCRPythonCommand()); err != nil {
		return DriveOCRProviderResult{}, fmt.Errorf("%w: %s command is not available", ErrDriveOCRDependencyUnavailable, paddleOCRPythonCommand())
	}
	switch {
	case contentType == "application/pdf" || ext == ".pdf":
		data, err := io.ReadAll(input.Body)
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		tmpDir, err := os.MkdirTemp("", "haohao-drive-ocr-*")
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		defer os.RemoveAll(tmpDir)
		inputPath := filepath.Join(tmpDir, "input.pdf")
		if err := os.WriteFile(inputPath, data, 0600); err != nil {
			return DriveOCRProviderResult{}, err
		}
		return p.extractPaddleRasterizedPDF(ctx, input, inputPath, tmpDir)
	case strings.HasPrefix(contentType, "image/") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".tif" || ext == ".tiff" || ext == ".webp":
		data, err := io.ReadAll(input.Body)
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		tmpDir, err := os.MkdirTemp("", "haohao-drive-ocr-*")
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		defer os.RemoveAll(tmpDir)
		if ext == "" {
			ext = ".img"
		}
		inputPath := filepath.Join(tmpDir, "input"+ext)
		if err := os.WriteFile(inputPath, data, 0600); err != nil {
			return DriveOCRProviderResult{}, err
		}
		ocr, err := paddleOCRImage(ctx, inputPath, input.Policy)
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		return resultFromPages([]DriveOCRPageResult{ocr.page(1)}), nil
	default:
		return DriveOCRProviderResult{}, ErrDriveOCRUnsupported
	}
}

func (p LocalDriveOCRProvider) extractPaddleRasterizedPDF(ctx context.Context, input DriveOCRProviderInput, inputPath, tmpDir string) (DriveOCRProviderResult, error) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return DriveOCRProviderResult{}, fmt.Errorf("%w: pdftoppm", ErrDriveOCRDependencyUnavailable)
	}
	prefix := filepath.Join(tmpDir, "page")
	timeout := time.Duration(max(1, input.Policy.TimeoutSecondsPerPage*max(1, input.Policy.MaxPages))) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "pdftoppm", "-r", "300", "-png", "-f", "1", "-l", fmt.Sprintf("%d", input.Policy.MaxPages), inputPath, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		return DriveOCRProviderResult{}, fmt.Errorf("pdftoppm: %w: %s", err, strings.TrimSpace(string(out)))
	}
	files, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return DriveOCRProviderResult{}, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return DriveOCRProviderResult{}, ErrDriveOCRUnsupported
	}
	pages := make([]DriveOCRPageResult, 0, min(len(files), input.Policy.MaxPages))
	for i, file := range files {
		if i >= input.Policy.MaxPages {
			break
		}
		ocr, err := paddleOCRImage(ctx, file, input.Policy)
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		pages = append(pages, ocr.page(i+1))
	}
	return resultFromPages(pages), nil
}

func (LocalDriveOCRProvider) extractImage(ctx context.Context, input DriveOCRProviderInput) (DriveOCRProviderResult, error) {
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return DriveOCRProviderResult{}, err
	}
	tmpDir, err := os.MkdirTemp("", "haohao-drive-ocr-*")
	if err != nil {
		return DriveOCRProviderResult{}, err
	}
	defer os.RemoveAll(tmpDir)
	ext := strings.ToLower(filepath.Ext(input.File.OriginalFilename))
	if ext == "" {
		ext = ".img"
	}
	inputPath := filepath.Join(tmpDir, "input"+ext)
	if err := os.WriteFile(inputPath, data, 0600); err != nil {
		return DriveOCRProviderResult{}, err
	}
	text, err := tesseractImage(ctx, inputPath, input.Policy)
	if err != nil {
		return DriveOCRProviderResult{}, err
	}
	page := DriveOCRPageResult{PageNumber: 1, RawText: text, LayoutJSON: []byte("{}"), BoxesJSON: []byte("[]")}
	return resultFromPages([]DriveOCRPageResult{page}), nil
}

func tesseractImage(ctx context.Context, path string, policy DriveOCRPolicy) (string, error) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		return "", fmt.Errorf("%w: tesseract", ErrDriveOCRDependencyUnavailable)
	}
	timeout := time.Duration(max(1, policy.TimeoutSecondsPerPage)) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{path, "stdout", "-l", strings.Join(policy.OCRLanguages, "+"), "--psm", "6"}
	cmd := exec.CommandContext(cmdCtx, "tesseract", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tesseract: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return normalizeOCRText(string(out)), nil
}

type paddleOCRImageResult struct {
	Text              string
	AverageConfidence *float64
	LayoutJSON        []byte
	BoxesJSON         []byte
}

type paddleOCRHelperRequest struct {
	ImagePath                 string `json:"imagePath"`
	Lang                      string `json:"lang"`
	Device                    string `json:"device"`
	UseDocOrientationClassify bool   `json:"useDocOrientationClassify"`
	UseDocUnwarping           bool   `json:"useDocUnwarping"`
	UseTextlineOrientation    bool   `json:"useTextlineOrientation"`
}

type paddleOCRHelperResponse struct {
	Text              string          `json:"text"`
	AverageConfidence *float64        `json:"averageConfidence"`
	Layout            json.RawMessage `json:"layout"`
	Boxes             json.RawMessage `json:"boxes"`
	Error             string          `json:"error"`
}

func (r paddleOCRImageResult) page(pageNumber int) DriveOCRPageResult {
	layout := r.LayoutJSON
	if len(layout) == 0 || !json.Valid(layout) || string(layout) == "null" {
		layout = []byte("{}")
	}
	boxes := r.BoxesJSON
	if len(boxes) == 0 || !json.Valid(boxes) || string(boxes) == "null" {
		boxes = []byte("[]")
	}
	return DriveOCRPageResult{
		PageNumber:        pageNumber,
		RawText:           r.Text,
		AverageConfidence: r.AverageConfidence,
		LayoutJSON:        layout,
		BoxesJSON:         boxes,
	}
}

func paddleOCRImage(ctx context.Context, path string, policy DriveOCRPolicy) (paddleOCRImageResult, error) {
	command := paddleOCRPythonCommand()
	if _, err := exec.LookPath(command); err != nil {
		return paddleOCRImageResult{}, fmt.Errorf("%w: %s command is not available", ErrDriveOCRDependencyUnavailable, command)
	}
	helperPath, err := resolvePaddleOCRHelperPath()
	if err != nil {
		return paddleOCRImageResult{}, err
	}
	requestBody, err := json.Marshal(paddleOCRHelperRequest{
		ImagePath:                 path,
		Lang:                      paddleOCRLanguage(policy),
		Device:                    paddleOCRDevice(),
		UseDocOrientationClassify: false,
		UseDocUnwarping:           false,
		UseTextlineOrientation:    true,
	})
	if err != nil {
		return paddleOCRImageResult{}, err
	}
	timeout := time.Duration(max(1, policy.TimeoutSecondsPerPage)) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := runLocalCommand(cmdCtx, command, []string{helperPath, "extract"}, string(requestBody))
	if err != nil {
		if cmdCtx.Err() != nil {
			return paddleOCRImageResult{}, fmt.Errorf("paddleocr timed out: %w", cmdCtx.Err())
		}
		if strings.Contains(err.Error(), "paddleocr dependency unavailable") {
			return paddleOCRImageResult{}, fmt.Errorf("%w: %v", ErrDriveOCRDependencyUnavailable, err)
		}
		return paddleOCRImageResult{}, fmt.Errorf("paddleocr: %w", err)
	}
	var response paddleOCRHelperResponse
	if err := json.Unmarshal([]byte(out), &response); err != nil {
		text := parsePaddleOCROutput(out)
		if text != "" {
			return paddleOCRImageResult{Text: text, LayoutJSON: []byte("{}"), BoxesJSON: []byte("[]")}, nil
		}
		return paddleOCRImageResult{}, fmt.Errorf("decode paddleocr response: %w", err)
	}
	if response.Error != "" {
		return paddleOCRImageResult{}, fmt.Errorf("paddleocr: %s", response.Error)
	}
	text := normalizeOCRText(response.Text)
	if text == "" {
		return paddleOCRImageResult{}, ErrDriveOCRUnsupported
	}
	return paddleOCRImageResult{
		Text:              text,
		AverageConfidence: response.AverageConfidence,
		LayoutJSON:        response.Layout,
		BoxesJSON:         response.Boxes,
	}, nil
}

func paddleOCRLanguage(policy DriveOCRPolicy) string {
	for _, lang := range normalizeDriveOCRLanguages(policy.OCRLanguages) {
		switch lang {
		case "jpn", "ja", "japan":
			return "japan"
		case "eng", "en":
			return "en"
		case "kor", "ko", "korean":
			return "korean"
		case "chi_sim", "chi_tra", "zh", "ch":
			return "ch"
		}
	}
	return "en"
}

func paddleOCRDevice() string {
	device := strings.ToLower(strings.TrimSpace(os.Getenv(paddleOCRDeviceEnv)))
	if device == "" {
		return "cpu"
	}
	if device == "cpu" || device == "gpu" || strings.HasPrefix(device, "gpu:") {
		return device
	}
	return "cpu"
}

func parsePaddleOCROutput(value string) string {
	value = strings.ToValidUTF8(value, " ")
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[20") {
			continue
		}
		text := paddleOCRLineText(line)
		if text != "" {
			out = append(out, text)
		}
	}
	return normalizeOCRText(strings.Join(out, "\n"))
}

func paddleOCRLineText(line string) string {
	candidates := []string{"('", "[\""}
	for _, marker := range candidates {
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		start := idx + len(marker)
		end := -1
		if marker == "('" {
			end = strings.Index(line[start:], "',")
		} else {
			end = strings.Index(line[start:], "\",")
		}
		if end > 0 {
			return strings.TrimSpace(line[start : start+end])
		}
	}
	return ""
}

func pagesFromText(value string, maxPages int) DriveOCRProviderResult {
	parts := strings.Split(value, "\f")
	pages := make([]DriveOCRPageResult, 0, min(len(parts), maxPages))
	for _, part := range parts {
		if len(pages) >= maxPages {
			break
		}
		text := normalizeOCRText(part)
		if text == "" {
			continue
		}
		pages = append(pages, DriveOCRPageResult{PageNumber: len(pages) + 1, RawText: text, LayoutJSON: []byte("{}"), BoxesJSON: []byte("[]")})
	}
	return resultFromPages(pages)
}

func resultFromPages(pages []DriveOCRPageResult) DriveOCRProviderResult {
	parts := make([]string, 0, len(pages))
	for _, page := range pages {
		text := normalizeOCRText(page.RawText)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return DriveOCRProviderResult{Pages: pages, FullText: strings.Join(parts, "\n\n")}
}

func normalizeOCRText(value string) string {
	value = strings.ToValidUTF8(value, " ")
	value = strings.ReplaceAll(value, "\x00", " ")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func commandStatus(ctx context.Context, name string, versionArg string) DriveOCRDependencyStatus {
	path, err := exec.LookPath(name)
	if err != nil {
		return DriveOCRDependencyStatus{Name: name, Available: false}
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, path, versionArg).CombinedOutput()
	status := DriveOCRDependencyStatus{Name: name, Available: true}
	if err == nil {
		status.Version = firstNonEmptyLine(string(out))
	}
	return status
}

func tesseractLanguages(ctx context.Context) map[string]bool {
	langs := map[string]bool{}
	if _, err := exec.LookPath("tesseract"); err != nil {
		return langs
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "tesseract", "--list-langs").CombinedOutput()
	if err != nil {
		return langs
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "list of available") {
			continue
		}
		langs[line] = true
	}
	return langs
}

func driveOCREngineSkipReason(ctx context.Context, policy DriveOCRPolicy) (string, string) {
	policy = normalizeDriveOCRPolicy(policy)
	switch policy.OCREngine {
	case "tesseract":
		return "", ""
	case "paddleocr":
		status := paddleOCRDependencyStatus(ctx)
		if !status.Available {
			return "dependency_unavailable", "PaddleOCR engine is selected, but the PaddleOCR Python runtime dependencies are not available"
		}
		return "", ""
	case "docling":
		if _, err := exec.LookPath("docling"); err != nil {
			return "dependency_unavailable", "Docling OCR engine is selected, but the docling command is not available in this runtime"
		}
		return "", ""
	default:
		return "unsupported_engine", "OCR engine is not supported by this runtime"
	}
}

func paddleOCRPythonCommand() string {
	if command := strings.TrimSpace(os.Getenv(paddleOCRPythonEnv)); command != "" {
		return command
	}
	return "python3"
}

func paddleOCRDependencyStatus(ctx context.Context) DriveOCRDependencyStatus {
	status := DriveOCRDependencyStatus{Name: "paddleocr"}
	command := paddleOCRPythonCommand()
	if _, err := exec.LookPath(command); err != nil {
		return status
	}
	helperPath, err := resolvePaddleOCRHelperPath()
	if err != nil {
		return status
	}
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := runLocalCommand(checkCtx, command, []string{helperPath, "--check"}, "")
	if err != nil {
		return status
	}
	status.Available = true
	status.Version = firstOutputLine(output)
	return status
}

func resolvePaddleOCRHelperPath() (string, error) {
	if envPath := strings.TrimSpace(os.Getenv(paddleOCRHelperEnv)); envPath != "" {
		return existingPaddleOCRHelperPath(envPath)
	}
	candidates := []string{}
	if _, filename, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(filename), "scripts", "drive_ocr_paddleocr.py"))
	}
	candidates = append(candidates,
		filepath.Join("backend", "internal", "service", "scripts", "drive_ocr_paddleocr.py"),
		filepath.Join("internal", "service", "scripts", "drive_ocr_paddleocr.py"),
	)
	for _, candidate := range candidates {
		if path, err := existingPaddleOCRHelperPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: PaddleOCR helper is not available", ErrDriveOCRDependencyUnavailable)
}

func existingPaddleOCRHelperPath(value string) (string, error) {
	path := strings.TrimSpace(value)
	if path == "" {
		return "", fmt.Errorf("%w: PaddleOCR helper path is empty", ErrDriveOCRDependencyUnavailable)
	}
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("%w: resolve PaddleOCR helper path: %v", ErrDriveOCRDependencyUnavailable, err)
		}
		path = abs
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("%w: PaddleOCR helper is not available", ErrDriveOCRDependencyUnavailable)
	}
	return path, nil
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func isDriveOCRUnsupported(err error) bool {
	return errors.Is(err, ErrDriveOCRUnsupported) || errors.Is(err, ErrDriveOCRStructuredUnsupported)
}

func driveOCRProviderFailureCode(err error) string {
	if errors.Is(err, ErrDriveOCRDependencyUnavailable) {
		return "dependency_unavailable"
	}
	return "provider_failed"
}

func trimOCRProcessError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(strings.ToValidUTF8(err.Error(), " "))
	return truncateRunes(message, 2000)
}
