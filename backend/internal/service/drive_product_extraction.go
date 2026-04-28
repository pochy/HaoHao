package service

import (
	"context"
	"encoding/json"
	"strings"
)

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
