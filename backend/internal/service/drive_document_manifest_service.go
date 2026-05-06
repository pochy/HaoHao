package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/extrame/xls"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
)

const (
	driveDocumentManifestMetadataKey = "documentManifest"
	driveDocumentManifestParser      = "haohao-document-manifest-v1"
	driveManifestPreviewRows         = 20
)

type DriveDocumentManifest struct {
	File          DriveDocumentManifestFile `json:"file"`
	DocumentType  string                    `json:"documentType"`
	Manifest      map[string]any            `json:"manifest"`
	GeneratedAt   time.Time                 `json:"generatedAt"`
	ParserVersion string                    `json:"parserVersion"`
	Stale         bool                      `json:"stale"`
	Reason        string                    `json:"reason,omitempty"`
}

type DriveDocumentManifestFile struct {
	PublicID         string `json:"publicId"`
	OriginalFilename string `json:"originalFilename"`
	ContentType      string `json:"contentType"`
	ByteSize         int64  `json:"byteSize"`
	SHA256Hex        string `json:"sha256Hex"`
}

type driveDocumentSheetManifest struct {
	Name            string   `json:"name"`
	Index           int      `json:"index"`
	Visible         bool     `json:"visible"`
	UsedRange       string   `json:"usedRange,omitempty"`
	RowCountHint    int      `json:"rowCountHint,omitempty"`
	ColumnCountHint int      `json:"columnCountHint,omitempty"`
	HeaderPreview   []string `json:"headerPreview,omitempty"`
}

func (s *DriveService) GetDocumentManifest(ctx context.Context, tenantID, actorUserID int64, filePublicID string, refresh bool, auditCtx AuditContext) (DriveDocumentManifest, error) {
	if err := s.ensureConfigured(true); err != nil {
		return DriveDocumentManifest{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveDocumentManifest{}, err
	}
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return DriveDocumentManifest{}, err
	}
	file := driveFileFromDB(row)
	if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.manifest_view", "drive_file", file.PublicID, err, auditCtx)
		return DriveDocumentManifest{}, err
	}
	if err := s.authz.CanDownloadFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.manifest_view", "drive_file", file.PublicID, err, auditCtx)
		return DriveDocumentManifest{}, err
	}
	if err := s.ensureFileDownloadAllowed(ctx, actor, file, auditCtx, "drive.file.manifest_view"); err != nil {
		return DriveDocumentManifest{}, err
	}
	if err := s.ensureDriveEncryptionAvailable(ctx, file.TenantID, file.ID); err != nil {
		s.auditDenied(ctx, actor, "drive.file.manifest_view", "drive_file", file.PublicID, err, auditCtx)
		return DriveDocumentManifest{}, err
	}

	if !refresh {
		if cached, ok := driveDocumentManifestFromMetadata(file.Metadata); ok && cached.File.SHA256Hex == file.SHA256Hex {
			cached.Stale = false
			return cached, nil
		}
	}

	body, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return DriveDocumentManifest{}, err
	}
	data, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		return DriveDocumentManifest{}, fmt.Errorf("read drive file for manifest: %w", err)
	}
	manifest := buildDriveDocumentManifest(file, data, s.now())
	updatedMetadata := cloneMetadata(file.Metadata)
	updatedMetadata[driveDocumentManifestMetadataKey] = manifest
	metadataJSON, err := json.Marshal(updatedMetadata)
	if err != nil {
		return DriveDocumentManifest{}, fmt.Errorf("encode drive file metadata: %w", err)
	}
	updated, err := s.queries.UpdateDriveFileMetadata(ctx, db.UpdateDriveFileMetadataParams{
		Metadata: metadataJSON,
		ID:       file.ID,
		TenantID: tenantID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return DriveDocumentManifest{}, ErrDriveNotFound
		}
		return DriveDocumentManifest{}, fmt.Errorf("update drive file manifest metadata: %w", err)
	}
	result := driveFileFromDB(updated)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.manifest_refresh", "drive_file", result.PublicID, map[string]any{
		"documentType": manifest.DocumentType,
		"sha256Hex":    result.SHA256Hex,
	})
	return manifest, nil
}

func driveDocumentManifestFromMetadata(metadata map[string]any) (DriveDocumentManifest, bool) {
	raw, ok := metadata[driveDocumentManifestMetadataKey]
	if !ok {
		return DriveDocumentManifest{}, false
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return DriveDocumentManifest{}, false
	}
	var manifest DriveDocumentManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return DriveDocumentManifest{}, false
	}
	return manifest, manifest.File.PublicID != ""
}

func buildDriveDocumentManifest(file DriveFile, body []byte, now time.Time) DriveDocumentManifest {
	out := DriveDocumentManifest{
		File: DriveDocumentManifestFile{
			PublicID:         file.PublicID,
			OriginalFilename: file.OriginalFilename,
			ContentType:      file.ContentType,
			ByteSize:         file.ByteSize,
			SHA256Hex:        file.SHA256Hex,
		},
		DocumentType:  "unsupported",
		Manifest:      map[string]any{},
		GeneratedAt:   now.UTC(),
		ParserVersion: driveDocumentManifestParser,
	}
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(file.OriginalFilename)))
	contentType := normalizeContentType(file.ContentType)
	switch {
	case ext == ".xlsx" || strings.Contains(contentType, "spreadsheetml"):
		sheets, err := parseXLSXDocumentManifest(body)
		if err != nil {
			out.Reason = err.Error()
			return out
		}
		out.DocumentType = "excel"
		out.Manifest = map[string]any{"sheets": sheets}
	case ext == ".xls" || contentType == "application/vnd.ms-excel":
		sheets, err := parseXLSDocumentManifest(body)
		if err != nil {
			out.Reason = err.Error()
			return out
		}
		out.DocumentType = "excel"
		out.Manifest = map[string]any{"sheets": sheets}
	case ext == ".pdf" || contentType == "application/pdf":
		out.DocumentType = "pdf"
		pageCount := estimatePDFPageCount(body)
		out.Manifest = map[string]any{
			"pageCount": pageCount,
			"pages":     buildPDFPageManifest(pageCount),
		}
	default:
		out.Reason = "unsupported document type"
	}
	return out
}

func parseXLSXDocumentManifest(body []byte) ([]driveDocumentSheetManifest, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("read xlsx: %v", err)
	}
	defer func() { _ = workbook.Close() }()
	names := workbook.GetSheetList()
	sheets := make([]driveDocumentSheetManifest, 0, len(names))
	for index, name := range names {
		visible, err := workbook.GetSheetVisible(name)
		if err != nil {
			visible = true
		}
		usedRange, _ := workbook.GetSheetDimension(name)
		rowCount, colCount := spreadsheetRangeDimensions(usedRange)
		headerPreview := previewXLSXHeader(workbook, name)
		sheets = append(sheets, driveDocumentSheetManifest{
			Name:            name,
			Index:           index,
			Visible:         visible,
			UsedRange:       firstNonEmpty(usedRange, spreadsheetUsedRange(rowCount, colCount)),
			RowCountHint:    rowCount,
			ColumnCountHint: colCount,
			HeaderPreview:   headerPreview,
		})
	}
	return sheets, nil
}

func previewXLSXHeader(workbook *excelize.File, sheetName string) []string {
	iterator, err := workbook.Rows(sheetName)
	if err != nil {
		return nil
	}
	defer func() { _ = iterator.Close() }()
	for iterator.Next() {
		values, err := iterator.Columns()
		if err != nil {
			return nil
		}
		row := trimTrailingEmptyStrings(values)
		if len(row) > 0 {
			if len(row) > dataPipelineSpreadsheetMaxColumns {
				return row[:dataPipelineSpreadsheetMaxColumns]
			}
			return row
		}
	}
	return nil
}

func parseXLSDocumentManifest(body []byte) ([]driveDocumentSheetManifest, error) {
	workbook, err := xls.OpenReader(bytes.NewReader(body), "utf-8")
	if err != nil {
		return nil, fmt.Errorf("read xls: %v", err)
	}
	sheets := make([]driveDocumentSheetManifest, 0, workbook.NumSheets())
	for index := 0; index < workbook.NumSheets(); index++ {
		sheet := workbook.GetSheet(index)
		if sheet == nil {
			continue
		}
		rows := make([][]string, 0, min(int(sheet.MaxRow)+1, driveManifestPreviewRows))
		rowCount := 0
		colCount := 0
		for rowIndex := 0; rowIndex <= int(sheet.MaxRow); rowIndex++ {
			row := sheet.Row(rowIndex)
			if row == nil {
				continue
			}
			rowCount = rowIndex + 1
			lastCol := row.LastCol()
			if lastCol > colCount {
				colCount = lastCol
			}
			if len(rows) < driveManifestPreviewRows {
				values := make([]string, 0, min(lastCol, dataPipelineSpreadsheetMaxColumns))
				for col := 0; col < lastCol && col < dataPipelineSpreadsheetMaxColumns; col++ {
					values = append(values, strings.TrimSpace(row.Col(col)))
				}
				rows = append(rows, trimTrailingEmptyStrings(values))
			}
		}
		_, _, headerPreview := summarizeSpreadsheetRows(rows)
		sheets = append(sheets, driveDocumentSheetManifest{
			Name:            sheet.Name,
			Index:           index,
			Visible:         true,
			UsedRange:       spreadsheetUsedRange(rowCount, colCount),
			RowCountHint:    rowCount,
			ColumnCountHint: colCount,
			HeaderPreview:   headerPreview,
		})
	}
	return sheets, nil
}

func summarizeSpreadsheetRows(rows [][]string) (int, int, []string) {
	rowCount := len(rows)
	colCount := 0
	var header []string
	for index, raw := range rows {
		row := trimTrailingEmptyStrings(raw)
		if len(row) > 0 {
			rowCount = max(rowCount, index+1)
			if header == nil {
				header = row
			}
		}
		if len(row) > colCount {
			colCount = len(row)
		}
	}
	if len(header) > dataPipelineSpreadsheetMaxColumns {
		header = header[:dataPipelineSpreadsheetMaxColumns]
	}
	return rowCount, colCount, header
}

func spreadsheetUsedRange(rowCount, colCount int) string {
	if rowCount <= 0 || colCount <= 0 {
		return ""
	}
	return "A1:" + spreadsheetColumnName(colCount) + strconv.Itoa(rowCount)
}

func spreadsheetRangeDimensions(usedRange string) (int, int) {
	parts := strings.Split(strings.TrimSpace(usedRange), ":")
	if len(parts) == 0 {
		return 0, 0
	}
	lastCell := parts[len(parts)-1]
	col, row, err := excelize.CellNameToCoordinates(lastCell)
	if err != nil {
		return 0, 0
	}
	return row, col
}

func spreadsheetColumnName(index int) string {
	if index <= 0 {
		return ""
	}
	name := ""
	for index > 0 {
		index--
		name = string(rune('A'+(index%26))) + name
		index /= 26
	}
	return name
}

var pdfPagePattern = regexp.MustCompile(`/Type\s*/Page\b`)

func estimatePDFPageCount(body []byte) int {
	matches := pdfPagePattern.FindAll(body, -1)
	return len(matches)
}

func buildPDFPageManifest(pageCount int) []map[string]any {
	pages := make([]map[string]any, 0, pageCount)
	for index := 1; index <= pageCount; index++ {
		pages = append(pages, map[string]any{
			"pageNumber":     index,
			"detectedTables": []any{},
		})
	}
	return pages
}

func cloneMetadata(metadata map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range metadata {
		out[key] = value
	}
	return out
}
