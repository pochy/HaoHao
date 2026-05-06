package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const (
	dataPipelineJSONMaxFiles = 20
	dataPipelineJSONMaxRows  = 100000
)

type dataPipelineJSONField struct {
	Column   string
	Segments []string
	Default  string
	Join     string
}

func (s *DataPipelineService) materializeJSONExtract(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, upstream dataPipelineMaterializedRelation) (dataPipelineMaterializedRelation, error) {
	rows, err := readHybridRows(ctx, conn, upstream, int32(dataPipelineFloat(node.Data.Config, "maxInputRows", dataPipelineJSONMaxRows)))
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	sourceColumn := dataPipelineString(node.Data.Config, "sourceColumn")
	if sourceColumn == "" {
		sourceColumn = "raw_record_json"
		if !containsString(upstream.Columns, sourceColumn) && containsString(upstream.Columns, "json") {
			sourceColumn = "json"
		}
	}
	if err := dataPipelineValidateIdentifier(sourceColumn); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	if !containsString(upstream.Columns, sourceColumn) {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: json_extract sourceColumn not found: %s", ErrInvalidDataPipelineGraph, sourceColumn)
	}
	fields, err := dataPipelineJSONFields(node.Data.Config)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	if len(fields) == 0 {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: json_extract requires fields", ErrInvalidDataPipelineGraph)
	}

	includeSourceColumns := dataPipelineBool(node.Data.Config, "includeSourceColumns", true)
	columns := make([]string, 0, len(upstream.Columns)+len(fields)+3)
	if includeSourceColumns {
		columns = append(columns, upstream.Columns...)
	}
	columns = append(columns, "json_row_number", "json_record_path")
	for _, field := range fields {
		columns = append(columns, field.Column)
	}
	if dataPipelineBool(node.Data.Config, "includeRawRecord", false) {
		columns = append(columns, "raw_record_json")
	}
	columns = uniqueStringList(columns)

	maxRows := int(dataPipelineFloat(node.Data.Config, "maxRows", dataPipelineJSONMaxRows))
	if maxRows <= 0 || maxRows > dataPipelineJSONMaxRows {
		maxRows = dataPipelineJSONMaxRows
	}
	out := make([]map[string]any, 0, min(len(rows), maxRows))
	for sourceIndex, sourceRow := range rows {
		if len(out) >= maxRows {
			break
		}
		sourceJSON := strings.TrimSpace(fmt.Sprint(sourceRow[sourceColumn]))
		if sourceJSON == "" {
			continue
		}
		var root any
		if err := json.Unmarshal([]byte(sourceJSON), &root); err != nil {
			return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: json_extract row %d sourceColumn %s: %v", ErrInvalidDataPipelineInput, sourceIndex+1, sourceColumn, err)
		}
		extractedRows, err := extractDataPipelineJSONRecords(root, node.Data.Config, fields, maxRows-len(out), "json_row_number", "json_record_path")
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		for _, extracted := range extractedRows {
			next := extracted
			if includeSourceColumns {
				next = cloneRow(sourceRow)
				for key, value := range extracted {
					next[key] = value
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

func (s *DataPipelineService) materializeDriveJSONInput(ctx context.Context, conn driver.Conn, database, table string, node DataPipelineNode, tenantID, actorUserID int64) (dataPipelineMaterializedRelation, error) {
	publicIDs := dataPipelineStringSlice(node.Data.Config, "filePublicIds")
	if len(publicIDs) == 0 {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: json input requires filePublicIds", ErrInvalidDataPipelineGraph)
	}
	if len(publicIDs) > dataPipelineJSONMaxFiles {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("%w: json input cannot contain more than %d files", ErrInvalidDataPipelineGraph, dataPipelineJSONMaxFiles)
	}
	if s == nil || s.driveOCR == nil || s.driveOCR.drive == nil {
		return dataPipelineMaterializedRelation{}, fmt.Errorf("Drive service is not configured")
	}

	includeSourceMetadataColumns := dataPipelineBool(node.Data.Config, "includeSourceMetadataColumns", true)
	sourceMetadataColumns := []string{"file_public_id", "file_name", "mime_type", "file_revision", "row_number", "record_path"}
	fields, err := dataPipelineJSONFields(node.Data.Config)
	if err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	columns := make([]string, 0, len(sourceMetadataColumns)+len(fields)+1)
	if includeSourceMetadataColumns {
		columns = append(columns, sourceMetadataColumns...)
	}
	for _, field := range fields {
		columns = append(columns, field.Column)
	}
	if dataPipelineBool(node.Data.Config, "includeRawRecord", false) {
		columns = append(columns, "raw_record_json")
	}
	columns = uniqueStringList(columns)

	rows := make([]map[string]any, 0)
	maxRows := int(dataPipelineFloat(node.Data.Config, "maxRows", dataPipelineJSONMaxRows))
	if maxRows <= 0 || maxRows > dataPipelineJSONMaxRows {
		maxRows = dataPipelineJSONMaxRows
	}
	for _, publicID := range publicIDs {
		download, err := s.driveOCR.drive.DownloadFile(ctx, tenantID, actorUserID, publicID, AuditContext{})
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		body, err := io.ReadAll(download.Body)
		_ = download.Body.Close()
		if err != nil {
			return dataPipelineMaterializedRelation{}, fmt.Errorf("read json %s: %w", publicID, err)
		}
		parsedRows, err := readDataPipelineJSON(body, node.Data.Config, fields, maxRows-len(rows))
		if err != nil {
			return dataPipelineMaterializedRelation{}, err
		}
		for _, parsed := range parsedRows {
			if includeSourceMetadataColumns {
				parsed["file_public_id"] = publicID
				parsed["file_name"] = download.File.OriginalFilename
				parsed["mime_type"] = download.File.ContentType
				parsed["file_revision"] = driveFileContentRevision(download.File)
			}
			rows = append(rows, parsed)
			if len(rows) >= maxRows {
				break
			}
		}
		if len(rows) >= maxRows {
			break
		}
	}
	if err := createHybridStringTable(ctx, conn, database, table, columns, rows); err != nil {
		return dataPipelineMaterializedRelation{}, err
	}
	return dataPipelineMaterializedRelation{Database: database, Table: table, Columns: columns}, nil
}

func dataPipelineJSONFields(config map[string]any) ([]dataPipelineJSONField, error) {
	rawFields := dataPipelineConfigObjects(config, "fields")
	fields := make([]dataPipelineJSONField, 0, len(rawFields))
	seen := map[string]struct{}{}
	for _, raw := range rawFields {
		column := dataPipelineString(raw, "column")
		if column == "" {
			column = dataPipelineString(raw, "targetColumn")
		}
		if err := dataPipelineValidateIdentifier(column); err != nil {
			return nil, err
		}
		if _, ok := seen[column]; ok {
			return nil, fmt.Errorf("%w: duplicate json field column %s", ErrInvalidDataPipelineGraph, column)
		}
		segments := dataPipelineStringSlice(raw, "pathSegments")
		if len(segments) == 0 {
			segments = parseDataPipelineJSONPath(dataPipelineString(raw, "path"))
		}
		if len(segments) == 0 {
			segments = []string{column}
		}
		join := dataPipelineString(raw, "join")
		if join == "" {
			join = dataPipelineString(raw, "delimiter")
		}
		fields = append(fields, dataPipelineJSONField{
			Column:   column,
			Segments: segments,
			Default:  dataPipelineString(raw, "default"),
			Join:     join,
		})
		seen[column] = struct{}{}
	}
	return fields, nil
}

func readDataPipelineJSON(body []byte, config map[string]any, fields []dataPipelineJSONField, maxRows int) ([]map[string]any, error) {
	if maxRows <= 0 {
		return nil, nil
	}
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("%w: read json: %v", ErrInvalidDataPipelineInput, err)
	}
	return extractDataPipelineJSONRecords(root, config, fields, maxRows, "row_number", "record_path")
}

func extractDataPipelineJSONRecords(root any, config map[string]any, fields []dataPipelineJSONField, maxRows int, rowNumberColumn, recordPathColumn string) ([]map[string]any, error) {
	if maxRows <= 0 {
		return nil, nil
	}
	recordSegments := parseDataPipelineJSONPath(dataPipelineString(config, "recordPath"))
	recordsValue, ok := dataPipelineJSONValue(root, recordSegments)
	if !ok {
		return nil, fmt.Errorf("%w: json recordPath did not match any value", ErrInvalidDataPipelineInput)
	}
	records, ok := recordsValue.([]any)
	if !ok {
		records = []any{recordsValue}
	}

	rows := make([]map[string]any, 0, min(len(records), maxRows))
	for index, record := range records {
		if len(rows) >= maxRows {
			break
		}
		row := map[string]any{}
		if rowNumberColumn != "" {
			row[rowNumberColumn] = strconv.Itoa(index + 1)
		}
		if recordPathColumn != "" {
			row[recordPathColumn] = dataPipelineJSONRecordPath(recordSegments, index)
		}
		for _, field := range fields {
			value, found := dataPipelineJSONValue(record, field.Segments)
			if !found || value == nil {
				row[field.Column] = field.Default
				continue
			}
			row[field.Column] = stringifyDataPipelineJSONField(value, field.Join)
		}
		if dataPipelineBool(config, "includeRawRecord", false) {
			row["raw_record_json"] = jsonString(record)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseDataPipelineJSONPath(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" || path == "$" {
		return nil
	}
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	segments := make([]string, 0)
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func dataPipelineJSONValue(value any, segments []string) (any, bool) {
	current := value
	for _, segment := range segments {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func stringifyDataPipelineJSONField(value any, join string) string {
	if join != "" {
		if items, ok := value.([]any); ok {
			parts := make([]string, 0, len(items))
			for _, item := range items {
				parts = append(parts, stringifyDataPipelineJSONField(item, ""))
			}
			return strings.Join(parts, join)
		}
	}
	return stringifyHybridValue(value)
}

func dataPipelineJSONRecordPath(recordSegments []string, index int) string {
	base := "$"
	if len(recordSegments) > 0 {
		base += "." + strings.Join(recordSegments, ".")
	}
	return fmt.Sprintf("%s[%d]", base, index)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
