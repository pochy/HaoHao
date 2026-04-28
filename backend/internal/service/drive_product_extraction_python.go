package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	pythonNLPProductExtractionTextLimit = 160000
	pythonNLPProductExtractionItemLimit = 50
	pythonNLPHelperEnv                  = "HAOHAO_DRIVE_PRODUCT_NLP_HELPER"
)

type PythonNLPProductExtractorProfile struct {
	Name       string
	Command    string
	HelperPath string
}

type PythonNLPDriveProductExtractor struct {
	profile PythonNLPProductExtractorProfile
}

type pythonNLPProductExtractionRequest struct {
	Mode   string                    `json:"mode"`
	Text   string                    `json:"text"`
	Pages  []pythonNLPPageRequest    `json:"pages"`
	Rules  DriveOCRRulesPolicy       `json:"rules"`
	Limits pythonNLPExtractionLimits `json:"limits"`
}

type pythonNLPPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	RawText    string `json:"rawText"`
}

type pythonNLPExtractionLimits struct {
	MaxItems int `json:"maxItems"`
}

func NewPythonNLPDriveProductExtractor(profile PythonNLPProductExtractorProfile) PythonNLPDriveProductExtractor {
	profile.Name = strings.ToLower(strings.TrimSpace(profile.Name))
	profile.Command = strings.TrimSpace(profile.Command)
	if profile.Command == "" {
		profile.Command = "python3"
	}
	return PythonNLPDriveProductExtractor{profile: profile}
}

func DefaultPythonNLPDriveProductExtractors() []DriveProductExtractor {
	profiles := defaultPythonNLPProductExtractorProfiles()
	extractors := make([]DriveProductExtractor, 0, len(profiles))
	for _, profile := range profiles {
		extractors = append(extractors, NewPythonNLPDriveProductExtractor(profile))
	}
	return extractors
}

func (e PythonNLPDriveProductExtractor) Name() string {
	return e.profile.Name
}

func (e PythonNLPDriveProductExtractor) ExtractProducts(ctx context.Context, input DriveProductExtractionInput) (DriveProductExtractionResult, error) {
	profile := e.profile
	if profile.Name == "" || profile.Command == "" {
		return DriveProductExtractionResult{}, ErrDriveOCRStructuredUnsupported
	}
	if _, err := exec.LookPath(profile.Command); err != nil {
		return DriveProductExtractionResult{}, fmt.Errorf("%w: %s command is not available", ErrDriveOCRStructuredUnsupported, profile.Command)
	}
	helperPath, err := resolvePythonNLPHelperPath(profile.HelperPath)
	if err != nil {
		return DriveProductExtractionResult{}, err
	}
	requestBody, err := json.Marshal(buildPythonNLPProductExtractionRequest(input, profile.Name))
	if err != nil {
		return DriveProductExtractionResult{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, ollamaProductExtractionTimeout(input.Policy))
	defer cancel()
	raw, err := runLocalCommand(requestCtx, profile.Command, []string{helperPath, "extract"}, string(requestBody))
	if err != nil {
		if requestCtx.Err() != nil {
			return DriveProductExtractionResult{}, fmt.Errorf("%s extractor timed out: %w", profile.Name, requestCtx.Err())
		}
		return DriveProductExtractionResult{}, fmt.Errorf("%s extractor failed: %w", profile.Name, err)
	}
	items, err := parseOllamaProductItems(raw)
	if err != nil {
		return DriveProductExtractionResult{}, fmt.Errorf("decode %s product extraction response: %w", profile.Name, err)
	}
	result := DriveProductExtractionResult{Items: make([]DriveProductExtractionItem, 0, len(items))}
	for _, item := range items {
		converted := item.toDriveProductExtractionItem(input)
		if strings.TrimSpace(converted.Name) == "" {
			continue
		}
		result.Items = append(result.Items, converted)
		if len(result.Items) >= pythonNLPProductExtractionItemLimit {
			break
		}
	}
	return result, nil
}

func CheckDriveOCRPythonNLPExtractors(ctx context.Context, policy DriveOCRPolicy) []DriveOCRLocalCommandStatus {
	profiles := defaultPythonNLPProductExtractorProfiles()
	statuses := make([]DriveOCRLocalCommandStatus, 0, len(profiles))
	for _, profile := range profiles {
		statuses = append(statuses, checkDriveOCRPythonNLPExtractor(ctx, policy, profile))
	}
	return statuses
}

func checkDriveOCRPythonNLPExtractor(ctx context.Context, policy DriveOCRPolicy, profile PythonNLPProductExtractorProfile) DriveOCRLocalCommandStatus {
	status := DriveOCRLocalCommandStatus{
		Name:       profile.Name,
		Command:    defaultString(profile.Command, "python3"),
		Configured: policy.StructuredExtractor == profile.Name,
	}
	if _, err := exec.LookPath(status.Command); err != nil {
		return status
	}
	helperPath, err := resolvePythonNLPHelperPath(profile.HelperPath)
	if err != nil {
		return status
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := runLocalCommand(checkCtx, status.Command, []string{helperPath, "--check", profile.Name}, "")
	if err != nil {
		return status
	}
	status.Available = true
	status.Version = firstOutputLine(output)
	return status
}

func defaultPythonNLPProductExtractorProfiles() []PythonNLPProductExtractorProfile {
	return []PythonNLPProductExtractorProfile{
		{Name: "python", Command: "python3"},
		{Name: "ginza", Command: "python3"},
		{Name: "sudachipy", Command: "python3"},
	}
}

func buildPythonNLPProductExtractionRequest(input DriveProductExtractionInput, mode string) pythonNLPProductExtractionRequest {
	text := strings.TrimSpace(input.FullText)
	if text == "" {
		lines := make([]string, 0, len(input.Pages))
		for _, page := range input.Pages {
			if strings.TrimSpace(page.RawText) != "" {
				lines = append(lines, page.RawText)
			}
		}
		text = strings.Join(lines, "\n")
	}
	pages := make([]pythonNLPPageRequest, 0, len(input.Pages))
	for _, page := range input.Pages {
		pages = append(pages, pythonNLPPageRequest{
			PageNumber: page.PageNumber,
			RawText:    truncateRunes(page.RawText, pythonNLPProductExtractionTextLimit),
		})
	}
	policy := normalizeDriveOCRPolicy(input.Policy)
	return pythonNLPProductExtractionRequest{
		Mode:  mode,
		Text:  truncateRunes(text, pythonNLPProductExtractionTextLimit),
		Pages: pages,
		Rules: policy.Rules,
		Limits: pythonNLPExtractionLimits{
			MaxItems: pythonNLPProductExtractionItemLimit,
		},
	}
}

func resolvePythonNLPHelperPath(value string) (string, error) {
	if envPath := strings.TrimSpace(os.Getenv(pythonNLPHelperEnv)); envPath != "" {
		value = envPath
	}
	if strings.TrimSpace(value) != "" {
		return existingPythonNLPHelperPath(value)
	}
	candidates := []string{}
	if _, filename, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(filename), "scripts", "drive_product_extraction_nlp.py"))
	}
	candidates = append(candidates,
		filepath.Join("backend", "internal", "service", "scripts", "drive_product_extraction_nlp.py"),
		filepath.Join("internal", "service", "scripts", "drive_product_extraction_nlp.py"),
	)
	for _, candidate := range candidates {
		if path, err := existingPythonNLPHelperPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: python NLP helper is not available", ErrDriveOCRStructuredUnsupported)
}

func existingPythonNLPHelperPath(value string) (string, error) {
	path := strings.TrimSpace(value)
	if path == "" {
		return "", fmt.Errorf("%w: python NLP helper path is empty", ErrDriveOCRStructuredUnsupported)
	}
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("%w: resolve python NLP helper path: %v", ErrDriveOCRStructuredUnsupported, err)
		}
		path = abs
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("%w: python NLP helper is not available", ErrDriveOCRStructuredUnsupported)
	}
	return path, nil
}
