package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type LocalDriveOCRProvider struct{}

func NewLocalDriveOCRProvider() LocalDriveOCRProvider {
	return LocalDriveOCRProvider{}
}

func (LocalDriveOCRProvider) Name() string {
	return "tesseract"
}

func (LocalDriveOCRProvider) Check(ctx context.Context, policy DriveOCRPolicy) []DriveOCRDependencyStatus {
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
	contentType := strings.ToLower(strings.TrimSpace(input.File.ContentType))
	ext := strings.ToLower(filepath.Ext(input.File.OriginalFilename))
	switch {
	case strings.HasPrefix(contentType, "text/") || strings.Contains(contentType, "json") || strings.Contains(contentType, "xml") || ext == ".txt" || ext == ".md" || ext == ".csv" || ext == ".json" || ext == ".xml":
		return p.extractText(input.Body)
	case contentType == "application/pdf" || ext == ".pdf":
		return p.extractPDF(ctx, input)
	case strings.HasPrefix(contentType, "image/") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".tif" || ext == ".tiff" || ext == ".webp":
		return p.extractImage(ctx, input)
	default:
		return DriveOCRProviderResult{}, ErrDriveOCRUnsupported
	}
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

func trimOCRProcessError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 2000 {
		message = message[:2000]
	}
	return message
}
