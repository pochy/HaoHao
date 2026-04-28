package service

import (
	"context"
	"errors"
	"io"
)

var (
	ErrDriveOCRUnsupported           = errors.New("drive ocr unsupported file")
	ErrDriveOCRDependencyUnavailable = errors.New("drive ocr dependency unavailable")
	ErrDriveOCRStructuredUnsupported = errors.New("drive structured extraction unsupported")
)

type DriveOCRDependencyStatus struct {
	Name      string
	Available bool
	Version   string
}

type DriveOCROllamaStatus struct {
	Configured     bool
	Reachable      bool
	ModelAvailable bool
}

type DriveOCRLMStudioStatus struct {
	Configured     bool
	Reachable      bool
	ModelAvailable bool
}

type DriveOCRLocalCommandStatus struct {
	Name       string
	Command    string
	Configured bool
	Available  bool
	Version    string
}

type DriveOCRRuntimeStatus struct {
	Enabled             bool
	OCREngine           string
	StructuredExtractor string
	Dependencies        []DriveOCRDependencyStatus
	Ollama              DriveOCROllamaStatus
	LMStudio            DriveOCRLMStudioStatus
	LocalCommands       []DriveOCRLocalCommandStatus
	StatusCounts        map[string]int64
}

type DriveOCRProviderInput struct {
	TenantID int64
	File     DriveFile
	Body     io.Reader
	Policy   DriveOCRPolicy
}

type DriveOCRPageResult struct {
	PageNumber        int
	RawText           string
	AverageConfidence *float64
	LayoutJSON        []byte
	BoxesJSON         []byte
}

type DriveOCRProviderResult struct {
	Pages             []DriveOCRPageResult
	FullText          string
	AverageConfidence *float64
	Warnings          []string
}

type DriveOCRProvider interface {
	Name() string
	Check(ctx context.Context, policy DriveOCRPolicy) []DriveOCRDependencyStatus
	Extract(ctx context.Context, input DriveOCRProviderInput) (DriveOCRProviderResult, error)
}

type DriveProductExtractionInput struct {
	TenantID int64
	File     DriveFile
	Run      DriveOCRRun
	Pages    []DriveOCRPageResult
	FullText string
	Policy   DriveOCRPolicy
}

type DriveProductExtractionResult struct {
	Items []DriveProductExtractionItem
}

type DriveProductExtractor interface {
	Name() string
	ExtractProducts(ctx context.Context, input DriveProductExtractionInput) (DriveProductExtractionResult, error)
}
