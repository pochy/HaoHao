package service

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type RulesDriveProductExtractor struct{}

func NewRulesDriveProductExtractor() RulesDriveProductExtractor {
	return RulesDriveProductExtractor{}
}

type DriveProductExtractorRouter struct {
	rules         DriveProductExtractor
	ollama        DriveProductExtractor
	lmStudio      DriveProductExtractor
	localCommands map[string]DriveProductExtractor
}

func NewDriveProductExtractorRouter(rules, ollama, lmStudio DriveProductExtractor, localCommands ...DriveProductExtractor) DriveProductExtractorRouter {
	router := DriveProductExtractorRouter{
		rules:         rules,
		ollama:        ollama,
		lmStudio:      lmStudio,
		localCommands: map[string]DriveProductExtractor{},
	}
	for _, extractor := range localCommands {
		if extractor == nil {
			continue
		}
		name := strings.TrimSpace(extractor.Name())
		if name != "" {
			router.localCommands[name] = extractor
		}
	}
	return router
}

func (DriveProductExtractorRouter) Name() string {
	return "router"
}

func (r DriveProductExtractorRouter) ExtractProducts(ctx context.Context, input DriveProductExtractionInput) (DriveProductExtractionResult, error) {
	switch input.Policy.StructuredExtractor {
	case "rules":
		if r.rules == nil {
			return DriveProductExtractionResult{}, nil
		}
		return r.rules.ExtractProducts(ctx, input)
	case "ollama":
		if r.ollama == nil {
			return DriveProductExtractionResult{}, ErrDriveOCRStructuredUnsupported
		}
		return r.ollama.ExtractProducts(ctx, input)
	case "lmstudio":
		if r.lmStudio == nil {
			return DriveProductExtractionResult{}, ErrDriveOCRStructuredUnsupported
		}
		return r.lmStudio.ExtractProducts(ctx, input)
	case "gemini", "codex", "claude":
		extractor := r.localCommands[input.Policy.StructuredExtractor]
		if extractor == nil {
			return DriveProductExtractionResult{}, ErrDriveOCRStructuredUnsupported
		}
		return extractor.ExtractProducts(ctx, input)
	default:
		return DriveProductExtractionResult{}, nil
	}
}

func (RulesDriveProductExtractor) Name() string {
	return "rules"
}

var (
	productJANPattern   = regexp.MustCompile(`\b(?:\d{8}|\d{13})\b`)
	productPricePattern = regexp.MustCompile(`(?:¥\s*([0-9,]+)|([0-9,]+)\s*円)`)
)

func (RulesDriveProductExtractor) ExtractProducts(_ context.Context, input DriveProductExtractionInput) (DriveProductExtractionResult, error) {
	lines := ocrCandidateLines(input.FullText)
	items := make([]DriveProductExtractionItem, 0)
	seen := map[string]struct{}{}
	for i, line := range lines {
		price := productPricePattern.FindStringSubmatch(line)
		jan := productJANPattern.FindString(line)
		if len(price) == 0 && jan == "" {
			continue
		}
		name := productNameNear(lines, i, line)
		if name == "" {
			name = "unknown product"
		}
		key := name + "|" + jan + "|" + productPriceValue(price)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		confidence := 0.62
		if jan != "" && len(price) > 0 {
			confidence = 0.76
		}
		item := DriveProductExtractionItem{
			PublicID:     "",
			TenantID:     input.TenantID,
			FilePublicID: input.File.PublicID,
			ItemType:     "product",
			Name:         name,
			JANCode:      jan,
			SourceText:   sourceWindow(lines, i),
			Price:        productPriceJSON(price),
			Promotion:    productPromotionJSON(line),
			Availability: map[string]any{},
			Evidence: []map[string]any{{
				"pageNumber": 1,
				"text":       line,
			}},
			Attributes: map[string]any{
				"schemaVersion": 1,
				"extractor":     "rules",
			},
			Confidence: &confidence,
			CreatedAt:  time.Now(),
		}
		items = append(items, item)
	}
	return DriveProductExtractionResult{Items: items}, nil
}

func ocrCandidateLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func productNameNear(lines []string, index int, current string) string {
	clean := func(value string) string {
		value = productPricePattern.ReplaceAllString(value, "")
		value = productJANPattern.ReplaceAllString(value, "")
		value = strings.Trim(value, " -:：|/　\t")
		value = strings.Join(strings.Fields(value), " ")
		if len([]rune(value)) > 80 {
			return ""
		}
		return value
	}
	if name := clean(current); name != "" {
		return name
	}
	for i := index - 1; i >= 0 && i >= index-2; i-- {
		if name := clean(lines[i]); name != "" {
			return name
		}
	}
	return ""
}

func sourceWindow(lines []string, index int) string {
	start := max(0, index-1)
	end := min(len(lines), index+2)
	return strings.Join(lines[start:end], "\n")
}

func productPriceValue(match []string) string {
	if len(match) == 0 {
		return ""
	}
	for _, part := range match[1:] {
		if strings.TrimSpace(part) != "" {
			return strings.ReplaceAll(part, ",", "")
		}
	}
	return ""
}

func productPriceJSON(match []string) map[string]any {
	value := productPriceValue(match)
	if value == "" {
		return map[string]any{}
	}
	amount, err := strconv.Atoi(value)
	if err != nil {
		return map[string]any{}
	}
	return map[string]any{
		"amount":      amount,
		"currency":    "JPY",
		"taxIncluded": strings.Contains(strings.Join(match, ""), "税込"),
	}
}

func productPromotionJSON(line string) map[string]any {
	for _, label := range []string{"特価", "セール", "割引", "ポイント"} {
		if strings.Contains(line, label) {
			return map[string]any{"label": label}
		}
	}
	return map[string]any{}
}

func jsonBytesOrEmptyObject(value any) []byte {
	if value == nil {
		return []byte("{}")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func jsonBytesOrEmptyArray(value any) []byte {
	if value == nil {
		return []byte("[]")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return []byte("[]")
	}
	return data
}
