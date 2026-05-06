package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
)

const (
	dataPipelineDriveInputModeSpreadsheet = "spreadsheet"
	dataPipelineDriveInputModeJSON        = "json"
	dataPipelineSpreadsheetMaxFiles       = 20
	dataPipelineSpreadsheetMaxRows        = 100000
	dataPipelineSpreadsheetMaxColumns     = 256
)

type dataPipelineSpreadsheetRows struct {
	SheetName  string
	SheetIndex int
	Header     []string
	Rows       [][]string
}

func dataPipelineDriveInputMode(config map[string]any) string {
	mode := strings.ToLower(strings.TrimSpace(dataPipelineString(config, "inputMode")))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(dataPipelineString(config, "format")))
	}
	switch mode {
	case "spreadsheet", "excel", "xls", "xlsx":
		return dataPipelineDriveInputModeSpreadsheet
	case "json", "application/json":
		return dataPipelineDriveInputModeJSON
	default:
		return mode
	}
}

func (s *DataPipelineService) materializeDriveSpreadsheetInput(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, tenantID, actorUserID int64) (dataPipelineMaterializedRelation, error) {
	publicIDs := dataPipelineStringSlice(node.Data.Config, "filePublicIds")
	if len(publicIDs) == 0 {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: spreadsheet input requires filePublicIds", ErrInvalidDataPipelineGraph)
	}
	if len(publicIDs) > dataPipelineSpreadsheetMaxFiles {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: spreadsheet input cannot contain more than %d files", ErrInvalidDataPipelineGraph, dataPipelineSpreadsheetMaxFiles)
	}
	if s == nil || s.driveOCR == nil || s.driveOCR.drive == nil {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("Drive service is not configured")
	}

	sheetName := dataPipelineString(node.Data.Config, "sheetName")
	sheetIndex := int(dataPipelineFloat(node.Data.Config, "sheetIndex", -1))
	cellRange := dataPipelineString(node.Data.Config, "range")
	headerRow := int(dataPipelineFloat(node.Data.Config, "headerRow", 1))
	configColumns := dataPipelineStringSlice(node.Data.Config, "columns")
	includeSourceMetadataColumns := dataPipelineBool(node.Data.Config, "includeSourceMetadataColumns", true)
	maxRows := int(dataPipelineFloat(node.Data.Config, "maxRows", dataPipelineSpreadsheetMaxRows))
	if maxRows <= 0 || maxRows > dataPipelineSpreadsheetMaxRows {
		maxRows = dataPipelineSpreadsheetMaxRows
	}

	sourceMetadataColumns := []string{"file_public_id", "file_name", "mime_type", "file_revision", "sheet_name", "sheet_index", "row_number"}
	columns := []string{}
	if includeSourceMetadataColumns {
		columns = append(columns, sourceMetadataColumns...)
	}
	rows := make([]map[string]any, 0)
	dataColumns := []string{}
	for _, publicID := range publicIDs {
		download, err := s.driveOCR.drive.DownloadFile(ctx, tenantID, actorUserID, publicID, AuditContext{})
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		body, err := io.ReadAll(download.Body)
		_ = download.Body.Close()
		if err != nil {
			return dataPipelineMaterializedRelation{}, fmt.Errorf("read spreadsheet %s: %w", publicID, err)
		}
		parsed, err := readDataPipelineSpreadsheet(download.File, body, sheetName, sheetIndex, cellRange, headerRow, maxRows, configColumns)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		if len(dataColumns) == 0 {
			dataColumns = sanitizeDatasetColumns(parsed.Header)
			if len(dataColumns) == 0 {
				return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: spreadsheet header is required", ErrInvalidDataPipelineInput)
			}
			columns = append(columns, dataColumns...)
		}
		for index, row := range parsed.Rows {
			next := map[string]any{}
			if includeSourceMetadataColumns {
				next["file_public_id"] = publicID
				next["file_name"] = download.File.OriginalFilename
				next["mime_type"] = download.File.ContentType
				next["file_revision"] = driveFileContentRevision(download.File)
				next["sheet_name"] = parsed.SheetName
				next["sheet_index"] = strconv.Itoa(parsed.SheetIndex)
				next["row_number"] = strconv.Itoa(index + spreadsheetDataStartRow(headerRow))
			}
			for i, column := range dataColumns {
				if i < len(row) {
					next[column] = row[i]
				} else {
					next[column] = ""
				}
			}
			rows = append(rows, next)
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, rows); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func (s *DataPipelineService) materializeExcelExtract(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation, tenantID, actorUserID int64) (dataPipelineMaterializedRelation, error) {
	if s == nil || s.driveOCR == nil || s.driveOCR.drive == nil {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("Drive service is not configured")
	}
	sourceFileColumn := dataPipelineString(node.Data.Config, "sourceFileColumn")
	if sourceFileColumn == "" {
		sourceFileColumn = "file_public_id"
	}
	if err := dataPipelineValidateIdentifier(sourceFileColumn); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	if !containsString(upstream.Columns, sourceFileColumn) {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: excel_extract sourceFileColumn not found: %s", ErrInvalidDataPipelineGraph, sourceFileColumn)
	}

	rows, err := readHybridRows(ctx, conn, upstream, 5000)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	sheetName := dataPipelineString(node.Data.Config, "sheetName")
	sheetIndex := int(dataPipelineFloat(node.Data.Config, "sheetIndex", -1))
	cellRange := dataPipelineString(node.Data.Config, "range")
	headerRow := int(dataPipelineFloat(node.Data.Config, "headerRow", 1))
	configColumns := dataPipelineStringSlice(node.Data.Config, "columns")
	includeSourceColumns := dataPipelineBool(node.Data.Config, "includeSourceColumns", true)
	includeSourceMetadataColumns := dataPipelineBool(node.Data.Config, "includeSourceMetadataColumns", true)
	maxRows := int(dataPipelineFloat(node.Data.Config, "maxRows", dataPipelineSpreadsheetMaxRows))
	if maxRows <= 0 || maxRows > dataPipelineSpreadsheetMaxRows {
		maxRows = dataPipelineSpreadsheetMaxRows
	}

	sourceMetadataColumns := []string{"file_public_id", "file_name", "mime_type", "file_revision", "sheet_name", "sheet_index", "row_number"}
	columns := []string{}
	if includeSourceColumns {
		columns = append(columns, upstream.Columns...)
	}
	if includeSourceMetadataColumns {
		columns = append(columns, sourceMetadataColumns...)
	}
	out := make([]map[string]any, 0, min(len(rows), maxRows))
	dataColumns := []string{}
	seenFiles := map[string]struct{}{}
	for _, sourceRow := range rows {
		if len(out) >= maxRows {
			break
		}
		publicID := strings.TrimSpace(fmt.Sprint(sourceRow[sourceFileColumn]))
		if publicID == "" {
			continue
		}
		if _, ok := seenFiles[publicID]; ok {
			continue
		}
		seenFiles[publicID] = struct{}{}
		if len(seenFiles) > dataPipelineSpreadsheetMaxFiles {
			return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: excel_extract cannot contain more than %d files", ErrInvalidDataPipelineGraph, dataPipelineSpreadsheetMaxFiles)
		}
		download, err := s.driveOCR.drive.DownloadFile(ctx, tenantID, actorUserID, publicID, AuditContext{})
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		body, err := io.ReadAll(download.Body)
		_ = download.Body.Close()
		if err != nil {
			return dataPipelineMaterializedRelation{}, fmt.Errorf("read spreadsheet %s: %w", publicID, err)
		}
		parsed, err := readDataPipelineSpreadsheet(download.File, body, sheetName, sheetIndex, cellRange, headerRow, maxRows-len(out), configColumns)
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		if len(dataColumns) == 0 {
			dataColumns = sanitizeDatasetColumns(parsed.Header)
			if len(dataColumns) == 0 {
				return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: spreadsheet header is required", ErrInvalidDataPipelineInput)
			}
			columns = uniqueStringList(append(columns, dataColumns...))
		}
		for index, row := range parsed.Rows {
			next := map[string]any{}
			if includeSourceColumns {
				next = cloneRow(sourceRow)
			}
			if includeSourceMetadataColumns {
				next["file_public_id"] = publicID
				next["file_name"] = download.File.OriginalFilename
				next["mime_type"] = download.File.ContentType
				next["file_revision"] = driveFileContentRevision(download.File)
				next["sheet_name"] = parsed.SheetName
				next["sheet_index"] = strconv.Itoa(parsed.SheetIndex)
				next["row_number"] = strconv.Itoa(index + spreadsheetDataStartRow(headerRow))
			}
			for i, column := range dataColumns {
				if i < len(row) {
					next[column] = row[i]
				} else {
					next[column] = ""
				}
			}
			out = append(out, next)
			if len(out) >= maxRows {
				break
			}
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, out); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func readDataPipelineSpreadsheet(file DriveFile, body []byte, sheetName string, sheetIndex int, cellRange string, headerRow, maxRows int, configColumns []string) (dataPipelineSpreadsheetRows, error) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(file.OriginalFilename)))
	contentType := normalizeContentType(file.ContentType)
	switch {
	case ext == ".xlsx" || strings.Contains(contentType, "spreadsheetml"):
		return readDataPipelineXLSX(body, sheetName, sheetIndex, cellRange, headerRow, maxRows, configColumns)
	case ext == ".xls" || contentType == "application/vnd.ms-excel":
		return readDataPipelineXLS(body, sheetName, sheetIndex, cellRange, headerRow, maxRows, configColumns)
	default:
		return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: unsupported spreadsheet type %s", ErrInvalidDataPipelineInput, firstNonEmpty(file.ContentType, file.OriginalFilename))
	}
}

func readDataPipelineXLS(body []byte, sheetName string, sheetIndex int, cellRange string, headerRow, maxRows int, configColumns []string) (dataPipelineSpreadsheetRows, error) {
	workbook, err := xls.OpenReader(bytes.NewReader(body), "utf-8")
	if err != nil {
		return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: read xls: %v", ErrInvalidDataPipelineInput, err)
	}
	resolvedIndex := 0
	sheet := workbook.GetSheet(resolvedIndex)
	if sheetName != "" {
		found := false
		for i := 0; i < workbook.NumSheets(); i++ {
			candidate := workbook.GetSheet(i)
			if candidate != nil && strings.EqualFold(strings.TrimSpace(candidate.Name), strings.TrimSpace(sheetName)) {
				sheet = candidate
				resolvedIndex = i
				found = true
				break
			}
		}
		if !found && sheetIndex >= 0 && sheetIndex < workbook.NumSheets() {
			resolvedIndex = sheetIndex
			sheet = workbook.GetSheet(sheetIndex)
		}
	} else if sheetIndex >= 0 && sheetIndex < workbook.NumSheets() {
		resolvedIndex = sheetIndex
		sheet = workbook.GetSheet(sheetIndex)
	}
	if sheet == nil {
		return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: spreadsheet sheet not found", ErrInvalidDataPipelineInput)
	}
	allRows := make([][]string, 0)
	for i := 0; i <= int(sheet.MaxRow) && len(allRows) < headerRow+maxRows; i++ {
		row := sheet.Row(i)
		if row == nil {
			allRows = append(allRows, nil)
			continue
		}
		values := make([]string, 0, min(row.LastCol(), dataPipelineSpreadsheetMaxColumns))
		for col := 0; col < row.LastCol() && col < dataPipelineSpreadsheetMaxColumns; col++ {
			values = append(values, strings.TrimSpace(row.Col(col)))
		}
		allRows = append(allRows, trimTrailingEmptyStrings(values))
	}
	allRows = applySpreadsheetRange(allRows, cellRange)
	return spreadsheetRowsFromRaw(sheet.Name, resolvedIndex, allRows, headerRow, maxRows, configColumns)
}

func readDataPipelineXLSX(body []byte, sheetName string, sheetIndex int, cellRange string, headerRow, maxRows int, configColumns []string) (dataPipelineSpreadsheetRows, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: read xlsx: %v", ErrInvalidDataPipelineInput, err)
	}
	defer func() { _ = workbook.Close() }()
	sheets := workbook.GetSheetList()
	resolvedIndex := 0
	if sheetName != "" {
		found := false
		for index, name := range sheets {
			if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(sheetName)) {
				sheetName = name
				resolvedIndex = index
				found = true
				break
			}
		}
		if !found && sheetIndex >= 0 && sheetIndex < len(sheets) {
			resolvedIndex = sheetIndex
			sheetName = sheets[sheetIndex]
		}
	} else if sheetIndex >= 0 && sheetIndex < len(sheets) {
		resolvedIndex = sheetIndex
		sheetName = sheets[sheetIndex]
	} else if len(sheets) > 0 {
		sheetName = sheets[0]
	}
	if sheetName == "" {
		return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: spreadsheet sheet not found", ErrInvalidDataPipelineInput)
	}
	iterator, err := workbook.Rows(sheetName)
	if err != nil {
		return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: read xlsx sheet: %v", ErrInvalidDataPipelineInput, err)
	}
	defer func() { _ = iterator.Close() }()
	allRows := make([][]string, 0)
	for iterator.Next() && len(allRows) < headerRow+maxRows {
		values, err := iterator.Columns()
		if err != nil {
			return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: read xlsx row: %v", ErrInvalidDataPipelineInput, err)
		}
		if len(values) > dataPipelineSpreadsheetMaxColumns {
			values = values[:dataPipelineSpreadsheetMaxColumns]
		}
		allRows = append(allRows, trimTrailingEmptyStrings(values))
	}
	if err := iterator.Error(); err != nil {
		return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: read xlsx rows: %v", ErrInvalidDataPipelineInput, err)
	}
	allRows = applySpreadsheetRange(allRows, cellRange)
	return spreadsheetRowsFromRaw(sheetName, resolvedIndex, allRows, headerRow, maxRows, configColumns)
}

func applySpreadsheetRange(rows [][]string, cellRange string) [][]string {
	startCol, startRow, endCol, endRow := parseSpreadsheetRange(cellRange)
	if startCol <= 0 && startRow <= 0 && endCol <= 0 && endRow <= 0 {
		return rows
	}
	if startCol <= 0 {
		startCol = 1
	}
	if startRow <= 0 {
		startRow = 1
	}
	out := make([][]string, 0)
	for index, row := range rows {
		rowNumber := index + 1
		if rowNumber < startRow {
			continue
		}
		if endRow > 0 && rowNumber > endRow {
			break
		}
		start := min(startCol-1, len(row))
		end := len(row)
		if endCol > 0 {
			end = min(endCol, len(row))
		}
		out = append(out, trimTrailingEmptyStrings(row[start:end]))
	}
	return out
}

func parseSpreadsheetRange(cellRange string) (int, int, int, int) {
	cellRange = strings.TrimSpace(cellRange)
	if cellRange == "" {
		return 0, 0, 0, 0
	}
	parts := strings.Split(cellRange, ":")
	startCol, startRow, err := excelize.CellNameToCoordinates(parts[0])
	if err != nil {
		return 0, 0, 0, 0
	}
	endCol, endRow := 0, 0
	if len(parts) > 1 {
		endCol, endRow, _ = excelize.CellNameToCoordinates(parts[1])
	}
	return startCol, startRow, endCol, endRow
}

func spreadsheetRowsFromRaw(sheetName string, sheetIndex int, allRows [][]string, headerRow, maxRows int, configColumns []string) (dataPipelineSpreadsheetRows, error) {
	if headerRow <= 0 {
		header := trimTrailingEmptyStrings(configColumns)
		if len(header) == 0 {
			return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: spreadsheet columns are required when headerRow is 0", ErrInvalidDataPipelineInput)
		}
		rows := make([][]string, 0, min(maxRows, len(allRows)))
		for _, raw := range allRows {
			row := trimTrailingEmptyStrings(raw)
			if len(row) == 0 {
				continue
			}
			if len(row) > len(header) {
				row = row[:len(header)]
			}
			rows = append(rows, row)
			if len(rows) >= maxRows {
				break
			}
		}
		return dataPipelineSpreadsheetRows{SheetName: sheetName, SheetIndex: sheetIndex, Header: header, Rows: rows}, nil
	}
	headerIndex := headerRow - 1
	if headerIndex < 0 || headerIndex >= len(allRows) {
		return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: spreadsheet header row not found", ErrInvalidDataPipelineInput)
	}
	header := trimTrailingEmptyStrings(configColumns)
	if len(header) == 0 {
		header = trimTrailingEmptyStrings(allRows[headerIndex])
	}
	if len(header) == 0 {
		return dataPipelineSpreadsheetRows{}, fmt.Errorf("%w: spreadsheet header is required", ErrInvalidDataPipelineInput)
	}
	rows := make([][]string, 0, min(maxRows, len(allRows)-headerRow))
	for _, raw := range allRows[headerRow:] {
		row := trimTrailingEmptyStrings(raw)
		if len(row) == 0 {
			continue
		}
		if len(row) > len(header) {
			row = row[:len(header)]
		}
		rows = append(rows, row)
		if len(rows) >= maxRows {
			break
		}
	}
	return dataPipelineSpreadsheetRows{SheetName: sheetName, SheetIndex: sheetIndex, Header: header, Rows: rows}, nil
}

func spreadsheetDataStartRow(headerRow int) int {
	if headerRow <= 0 {
		return 1
	}
	return headerRow + 1
}

func trimTrailingEmptyStrings(values []string) []string {
	end := len(values)
	for end > 0 && strings.TrimSpace(values[end-1]) == "" {
		end--
	}
	if end == 0 {
		return nil
	}
	out := make([]string, end)
	for i := 0; i < end; i++ {
		out[i] = strings.TrimSpace(values[i])
	}
	return out
}
