package api

import (
	"context"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DriveOCRJobBody struct {
	PublicID     string    `json:"publicId"`
	FilePublicID string    `json:"filePublicId"`
	Engine       string    `json:"engine"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
}

type DriveOCRJobOutput struct {
	Body DriveOCRJobBody
}

type CreateDriveOCRJobInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
}

type GetDriveOCRInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
}

type DriveOCRRunBody struct {
	PublicID            string     `json:"publicId"`
	FilePublicID        string     `json:"filePublicId"`
	Engine              string     `json:"engine"`
	Languages           []string   `json:"languages"`
	StructuredExtractor string     `json:"structuredExtractor"`
	Status              string     `json:"status"`
	Reason              string     `json:"reason"`
	PageCount           int        `json:"pageCount"`
	ProcessedPageCount  int        `json:"processedPageCount"`
	AverageConfidence   *float64   `json:"averageConfidence,omitempty"`
	ErrorCode           string     `json:"errorCode,omitempty"`
	ErrorMessage        string     `json:"errorMessage,omitempty"`
	CreatedAt           time.Time  `json:"createdAt" format:"date-time"`
	CompletedAt         *time.Time `json:"completedAt,omitempty" format:"date-time"`
}

type DriveOCRPageBody struct {
	PageNumber        int      `json:"pageNumber"`
	RawText           string   `json:"rawText"`
	AverageConfidence *float64 `json:"averageConfidence,omitempty"`
}

type DriveOCROutput struct {
	Body struct {
		Run   DriveOCRRunBody    `json:"run"`
		Pages []DriveOCRPageBody `json:"pages"`
	}
}

type DriveProductExtractionItemBody struct {
	PublicID     string           `json:"publicId"`
	ItemType     string           `json:"itemType"`
	Name         string           `json:"name"`
	Brand        string           `json:"brand,omitempty"`
	Manufacturer string           `json:"manufacturer,omitempty"`
	Model        string           `json:"model,omitempty"`
	SKU          string           `json:"sku,omitempty"`
	JANCode      string           `json:"janCode,omitempty"`
	Category     string           `json:"category,omitempty"`
	Description  string           `json:"description,omitempty"`
	Price        map[string]any   `json:"price"`
	Promotion    map[string]any   `json:"promotion"`
	Availability map[string]any   `json:"availability"`
	SourceText   string           `json:"sourceText"`
	Evidence     []map[string]any `json:"evidence"`
	Attributes   map[string]any   `json:"attributes"`
	Confidence   *float64         `json:"confidence,omitempty"`
	CreatedAt    time.Time        `json:"createdAt" format:"date-time"`
}

type DriveProductExtractionsOutput struct {
	Body struct {
		Items []DriveProductExtractionItemBody `json:"items"`
	}
}

type DriveProductExtractionJobBody struct {
	FilePublicID   string    `json:"filePublicId"`
	OCRRunPublicID string    `json:"ocrRunPublicId"`
	Extractor      string    `json:"extractor"`
	Status         string    `json:"status"`
	ItemCount      int       `json:"itemCount"`
	CreatedAt      time.Time `json:"createdAt" format:"date-time"`
}

type DriveProductExtractionJobOutput struct {
	Body DriveProductExtractionJobBody
}

func registerDriveOCRRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "createDriveOCRJob",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/files/{filePublicId}/ocr/jobs",
		Tags:        []string{"drive-ocr"},
		Summary:     "Drive file OCR job を作成する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveOCRJobInput) (*DriveOCRJobOutput, error) {
		if deps.DriveOCRService == nil {
			return nil, huma.Error503ServiceUnavailable("drive ocr service is not configured")
		}
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		run, err := deps.DriveOCRService.RequestJob(ctx, tenant.ID, current.User.ID, input.FilePublicID, "manual", sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveOCRJobOutput{Body: DriveOCRJobBody{
			PublicID:     run.PublicID,
			FilePublicID: run.FilePublicID,
			Engine:       run.Engine,
			Status:       run.Status,
			CreatedAt:    run.CreatedAt,
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDriveOCR",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/files/{filePublicId}/ocr",
		Tags:        []string{"drive-ocr"},
		Summary:     "Drive file OCR result を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *GetDriveOCRInput) (*DriveOCROutput, error) {
		if deps.DriveOCRService == nil {
			return nil, huma.Error503ServiceUnavailable("drive ocr service is not configured")
		}
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		result, err := deps.DriveOCRService.GetLatest(ctx, tenant.ID, current.User.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &DriveOCROutput{}
		out.Body.Run = toDriveOCRRunBody(result.Run)
		for _, page := range result.Pages {
			out.Body.Pages = append(out.Body.Pages, DriveOCRPageBody{PageNumber: page.PageNumber, RawText: page.RawText, AverageConfidence: page.AverageConfidence})
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDriveProductExtractions",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/files/{filePublicId}/product-extractions",
		Tags:        []string{"drive-ocr"},
		Summary:     "Drive file product extraction items を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *GetDriveOCRInput) (*DriveProductExtractionsOutput, error) {
		if deps.DriveOCRService == nil {
			return nil, huma.Error503ServiceUnavailable("drive ocr service is not configured")
		}
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveOCRService.ListProductExtractions(ctx, tenant.ID, current.User.ID, input.FilePublicID)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &DriveProductExtractionsOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDriveProductExtractionItemBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDriveProductExtractionJob",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/files/{filePublicId}/product-extractions/jobs",
		Tags:        []string{"drive-ocr"},
		Summary:     "Drive file product extraction job を作成する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveOCRJobInput) (*DriveProductExtractionJobOutput, error) {
		if deps.DriveOCRService == nil {
			return nil, huma.Error503ServiceUnavailable("drive ocr service is not configured")
		}
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		run, items, err := deps.DriveOCRService.RequestProductExtraction(ctx, tenant.ID, current.User.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveProductExtractionJobOutput{Body: DriveProductExtractionJobBody{
			FilePublicID:   run.FilePublicID,
			OCRRunPublicID: run.PublicID,
			Extractor:      run.StructuredExtractor,
			Status:         "completed",
			ItemCount:      len(items),
			CreatedAt:      time.Now(),
		}}, nil
	})
}

func toDriveOCRRunBody(run service.DriveOCRRun) DriveOCRRunBody {
	return DriveOCRRunBody{
		PublicID:            run.PublicID,
		FilePublicID:        run.FilePublicID,
		Engine:              run.Engine,
		Languages:           run.Languages,
		StructuredExtractor: run.StructuredExtractor,
		Status:              run.Status,
		Reason:              run.Reason,
		PageCount:           run.PageCount,
		ProcessedPageCount:  run.ProcessedPageCount,
		AverageConfidence:   run.AverageConfidence,
		ErrorCode:           run.ErrorCode,
		ErrorMessage:        run.ErrorMessage,
		CreatedAt:           run.CreatedAt,
		CompletedAt:         run.CompletedAt,
	}
}

func toDriveProductExtractionItemBody(item service.DriveProductExtractionItem) DriveProductExtractionItemBody {
	return DriveProductExtractionItemBody{
		PublicID:     item.PublicID,
		ItemType:     item.ItemType,
		Name:         item.Name,
		Brand:        item.Brand,
		Manufacturer: item.Manufacturer,
		Model:        item.Model,
		SKU:          item.SKU,
		JANCode:      item.JANCode,
		Category:     item.Category,
		Description:  item.Description,
		Price:        item.Price,
		Promotion:    item.Promotion,
		Availability: item.Availability,
		SourceText:   item.SourceText,
		Evidence:     item.Evidence,
		Attributes:   item.Attributes,
		Confidence:   item.Confidence,
		CreatedAt:    item.CreatedAt,
	}
}
