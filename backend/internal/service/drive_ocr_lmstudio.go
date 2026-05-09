package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const lmStudioOCRImageTokenLimit = 3000

type lmStudioMessageContentPart struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	ImageURL map[string]string `json:"image_url,omitempty"`
}

func (p LocalDriveOCRProvider) extractLMStudioOCR(ctx context.Context, input DriveOCRProviderInput, contentType, ext string) (DriveOCRProviderResult, error) {
	if strings.TrimSpace(input.Policy.LMStudioModel) == "" {
		return DriveOCRProviderResult{}, fmt.Errorf("%w: LM Studio model is required", ErrDriveOCRDependencyUnavailable)
	}
	switch {
	case contentType == "application/pdf" || ext == ".pdf":
		return p.extractLMStudioPDF(ctx, input)
	case strings.HasPrefix(contentType, "image/") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".tif" || ext == ".tiff" || ext == ".webp":
		data, err := io.ReadAll(input.Body)
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		text, err := lmStudioOCRImage(ctx, data, lmStudioImageMIMEType(contentType, ext), input.Policy)
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		return resultFromPages([]DriveOCRPageResult{{PageNumber: 1, RawText: text, LayoutJSON: []byte("{}"), BoxesJSON: []byte("[]")}}), nil
	default:
		return DriveOCRProviderResult{}, ErrDriveOCRUnsupported
	}
}

func (p LocalDriveOCRProvider) extractLMStudioPDF(ctx context.Context, input DriveOCRProviderInput) (DriveOCRProviderResult, error) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return DriveOCRProviderResult{}, fmt.Errorf("%w: pdftoppm", ErrDriveOCRDependencyUnavailable)
	}
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return DriveOCRProviderResult{}, err
	}
	tmpDir, err := os.MkdirTemp("", "haohao-drive-ocr-lmstudio-*")
	if err != nil {
		return DriveOCRProviderResult{}, err
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.pdf")
	if err := os.WriteFile(inputPath, data, 0600); err != nil {
		return DriveOCRProviderResult{}, err
	}
	prefix := filepath.Join(tmpDir, "page")
	timeout := time.Duration(max(1, input.Policy.TimeoutSecondsPerPage*max(1, input.Policy.MaxPages))) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "pdftoppm", "-r", "220", "-png", "-f", "1", "-l", fmt.Sprintf("%d", input.Policy.MaxPages), inputPath, prefix)
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
		image, err := os.ReadFile(file)
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		text, err := lmStudioOCRImage(ctx, image, "image/png", input.Policy)
		if err != nil {
			return DriveOCRProviderResult{}, err
		}
		pages = append(pages, DriveOCRPageResult{PageNumber: i + 1, RawText: text, LayoutJSON: []byte("{}"), BoxesJSON: []byte("[]")})
	}
	return resultFromPages(pages), nil
}

func lmStudioOCRImage(ctx context.Context, image []byte, mimeType string, policy DriveOCRPolicy) (string, error) {
	model := strings.TrimSpace(policy.LMStudioModel)
	if model == "" {
		return "", fmt.Errorf("%w: LM Studio model is required", ErrDriveOCRDependencyUnavailable)
	}
	baseURL, err := normalizedLMStudioBaseURL(policy.LMStudioBaseURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDriveOCRDependencyUnavailable, err)
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	requestBody, err := json.Marshal(lmStudioChatCompletionRequest{
		Model: model,
		Messages: []lmStudioChatMessage{
			{Role: "system", Content: "You extract searchable text from images. Return plain text only. Include any visible text, then add a concise Japanese visual description and search keywords for important subjects such as people, objects, products, scenes, colors, and style. Do not mention that text is absent."},
			{Role: "user", Content: []lmStudioMessageContentPart{
				{Type: "text", Text: "Extract readable text and describe the visual content for image search. Return plain Japanese text only."},
				{Type: "image_url", ImageURL: map[string]string{"url": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(image)}},
			}},
		},
		Temperature: 0,
		MaxTokens:   lmStudioOCRImageTokenLimit,
		Stream:      false,
	})
	if err != nil {
		return "", err
	}
	timeout := time.Duration(max(1, policy.TimeoutSecondsPerPage)) * time.Second
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("call LM Studio OCR: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("LM Studio OCR failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var completed lmStudioChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completed); err != nil {
		return "", fmt.Errorf("decode LM Studio OCR response: %w", err)
	}
	text := normalizeOCRText(completed.firstContent())
	if text == "" {
		return "", ErrDriveOCRUnsupported
	}
	return text, nil
}

func lmStudioImageMIMEType(contentType, ext string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(contentType, "image/") {
		return contentType
	}
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".tif", ".tiff":
		return "image/tiff"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}
