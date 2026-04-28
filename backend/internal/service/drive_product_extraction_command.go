package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	localCommandProductExtractionTextLimit = 3000
	localCommandProductExtractionItemLimit = 8
	localCommandOutputLimit                = 1 << 20
	localCommandErrorOutputLimit           = 16 << 10
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

type LocalCommandProductExtractorProfile struct {
	Name        string
	Command     string
	Args        []string
	VersionArgs []string
}

type LocalCommandDriveProductExtractor struct {
	profile LocalCommandProductExtractorProfile
}

func NewLocalCommandDriveProductExtractor(profile LocalCommandProductExtractorProfile) LocalCommandDriveProductExtractor {
	profile.Name = strings.ToLower(strings.TrimSpace(profile.Name))
	profile.Command = strings.TrimSpace(profile.Command)
	if profile.Command == "" {
		profile.Command = profile.Name
	}
	if len(profile.VersionArgs) == 0 {
		profile.VersionArgs = []string{"--version"}
	}
	return LocalCommandDriveProductExtractor{profile: profile}
}

func DefaultLocalCommandDriveProductExtractors() []DriveProductExtractor {
	profiles := defaultLocalCommandProductExtractorProfiles()
	extractors := make([]DriveProductExtractor, 0, len(profiles))
	for _, profile := range profiles {
		extractors = append(extractors, NewLocalCommandDriveProductExtractor(profile))
	}
	return extractors
}

func (e LocalCommandDriveProductExtractor) Name() string {
	return e.profile.Name
}

func (e LocalCommandDriveProductExtractor) ExtractProducts(ctx context.Context, input DriveProductExtractionInput) (DriveProductExtractionResult, error) {
	profile := e.profile
	if profile.Name == "" || profile.Command == "" {
		return DriveProductExtractionResult{}, ErrDriveOCRStructuredUnsupported
	}
	if _, err := exec.LookPath(profile.Command); err != nil {
		return DriveProductExtractionResult{}, fmt.Errorf("%w: %s command is not available", ErrDriveOCRStructuredUnsupported, profile.Command)
	}
	prompt := buildLocalCommandProductPrompt(input, profile.Name)
	raw, err := runLocalCommandPrompt(ctx, profile, prompt, ollamaProductExtractionTimeout(input.Policy))
	if err != nil {
		return DriveProductExtractionResult{}, err
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
		if len(result.Items) >= localCommandProductExtractionItemLimit {
			break
		}
	}
	return result, nil
}

func CheckDriveOCRLocalCommands(ctx context.Context, policy DriveOCRPolicy) []DriveOCRLocalCommandStatus {
	profiles := defaultLocalCommandProductExtractorProfiles()
	statuses := make([]DriveOCRLocalCommandStatus, 0, len(profiles))
	for _, profile := range profiles {
		statuses = append(statuses, checkDriveOCRLocalCommand(ctx, policy, profile))
	}
	return statuses
}

func checkDriveOCRLocalCommand(ctx context.Context, policy DriveOCRPolicy, profile LocalCommandProductExtractorProfile) DriveOCRLocalCommandStatus {
	status := DriveOCRLocalCommandStatus{
		Name:       profile.Name,
		Command:    profile.Command,
		Configured: policy.StructuredExtractor == profile.Name,
	}
	if _, err := exec.LookPath(profile.Command); err != nil {
		return status
	}
	status.Available = true
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := runLocalCommand(checkCtx, profile.Command, profile.VersionArgs, "")
	if err != nil {
		return status
	}
	status.Version = firstOutputLine(output)
	return status
}

func defaultLocalCommandProductExtractorProfiles() []LocalCommandProductExtractorProfile {
	return []LocalCommandProductExtractorProfile{
		{
			Name:        "gemini",
			Command:     "gemini",
			Args:        []string{"--prompt", "", "--approval-mode", "plan", "--output-format", "text", "--skip-trust"},
			VersionArgs: []string{"--version"},
		},
		{
			Name:        "codex",
			Command:     "codex",
			Args:        []string{"exec", "--sandbox", "read-only", "--ask-for-approval", "never", "--skip-git-repo-check", "--ephemeral", "--ignore-rules", "--color", "never", "-"},
			VersionArgs: []string{"--version"},
		},
		{
			Name:        "claude",
			Command:     "claude",
			Args:        []string{"--print", "--output-format", "text", "--permission-mode", "plan", "--tools", "", "--no-session-persistence"},
			VersionArgs: []string{"--version"},
		},
	}
}

func buildLocalCommandProductPrompt(input DriveProductExtractionInput, extractor string) string {
	text := ollamaProductPromptText(input.FullText)
	text = truncateRunes(text, localCommandProductExtractionTextLimit)
	return `Extract structured product records from the OCR text below.
Return only a JSON object with this exact shape:
{"items":[{"itemType":"product","name":"","brand":"","manufacturer":"","model":"","sku":"","janCode":"","category":"","description":"","price":{},"promotion":{},"availability":{},"sourceText":"","evidence":[{"pageNumber":1,"text":""}],"attributes":{},"confidence":0.0}]}

Rules:
- Use only the OCR text in this prompt. Do not inspect files, run tools, browse, or modify anything.
- Include concrete catalog products, variants, and model numbers.
- For recorder catalogs, model numbers such as 4B-C40GT3 or 2B-C20GT1 are separate products even when there is no price.
- Do not invent prices, JAN codes, or availability. Use empty strings or empty objects when the text does not contain the value.
- Keep sourceText short and copy the OCR text that supports the item.
- Limit the result to the most important ` + fmt.Sprint(localCommandProductExtractionItemLimit) + ` items.
- Set attributes.extractor to "` + extractor + `".

OCR text:
` + text
}

func runLocalCommandPrompt(ctx context.Context, profile LocalCommandProductExtractorProfile, prompt string, timeout time.Duration) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, err := runLocalCommand(requestCtx, profile.Command, profile.Args, prompt)
	if err != nil {
		if requestCtx.Err() != nil {
			return "", fmt.Errorf("%s command timed out: %w", profile.Name, requestCtx.Err())
		}
		return "", fmt.Errorf("%s command failed: %w", profile.Name, err)
	}
	output = strings.TrimSpace(stripANSI(output))
	if output == "" {
		return "", fmt.Errorf("%s command returned empty output", profile.Name)
	}
	return output, nil
}

func runLocalCommand(ctx context.Context, command string, args []string, stdin string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = os.TempDir()
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr limitedBuffer
	stdout.limit = localCommandOutputLimit
	stderr.limit = localCommandErrorOutputLimit
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return strings.TrimSpace(stdout.String()), fmt.Errorf("%w: %s", err, detail)
		}
		return strings.TrimSpace(stdout.String()), err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func stripANSI(value string) string {
	return ansiEscapePattern.ReplaceAllString(value, "")
}

func firstOutputLine(value string) string {
	value = strings.TrimSpace(stripANSI(value))
	if value == "" {
		return ""
	}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateRunes(line, 160)
		}
	}
	return ""
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		return len(p), nil
	}
	_, _ = b.Buffer.Write(p)
	return len(p), nil
}

var _ io.Writer = (*limitedBuffer)(nil)
