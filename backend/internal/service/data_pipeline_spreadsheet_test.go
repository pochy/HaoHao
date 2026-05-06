package service

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestReadDataPipelineSpreadsheetXLSXWithoutHeader(t *testing.T) {
	workbook := excelize.NewFile()
	defer func() { _ = workbook.Close() }()
	sheet := workbook.GetSheetName(0)
	values := [][]any{
		{"100", "DIY用品", "工具"},
		{"101", "DIY用品", "建築材料"},
		{"", "", ""},
	}
	for rowIndex, row := range values {
		cell, err := excelize.CoordinatesToCellName(1, rowIndex+1)
		if err != nil {
			t.Fatal(err)
		}
		if err := workbook.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := workbook.Write(&buf); err != nil {
		t.Fatal(err)
	}

	result, err := readDataPipelineSpreadsheet(
		DriveFile{OriginalFilename: "taxonomy.xlsx", ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		buf.Bytes(),
		"",
		-1,
		"",
		0,
		100,
		[]string{"taxonomy_id", "level_1", "level_2"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.SheetName != sheet {
		t.Fatalf("SheetName = %q, want %q", result.SheetName, sheet)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(result.Rows))
	}
	if result.Rows[0][0] != "100" || result.Rows[0][1] != "DIY用品" || result.Rows[1][2] != "建築材料" {
		t.Fatalf("unexpected rows: %#v", result.Rows)
	}
}

func TestBuildDriveDocumentManifestXLSX(t *testing.T) {
	workbook := excelize.NewFile()
	defer func() { _ = workbook.Close() }()
	sheet := workbook.GetSheetName(0)
	header := []any{"sku", "name", "price"}
	row := []any{"A-1", "Sample", "1200"}
	if err := workbook.SetSheetName(sheet, "Products"); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetSheetRow("Products", "A1", &header); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetSheetRow("Products", "A2", &row); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := workbook.Write(&buf); err != nil {
		t.Fatal(err)
	}

	manifest := buildDriveDocumentManifest(
		DriveFile{PublicID: "file_1", OriginalFilename: "products.xlsx", ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ByteSize: int64(buf.Len()), SHA256Hex: "sha"},
		buf.Bytes(),
		testTime(),
	)
	if manifest.DocumentType != "excel" {
		t.Fatalf("DocumentType = %q, want excel", manifest.DocumentType)
	}
	sheets, ok := manifest.Manifest["sheets"].([]driveDocumentSheetManifest)
	if !ok || len(sheets) != 1 {
		t.Fatalf("unexpected sheets: %#v", manifest.Manifest["sheets"])
	}
	if sheets[0].Name != "Products" || sheets[0].Index != 0 || sheets[0].HeaderPreview[0] != "sku" {
		t.Fatalf("unexpected sheet manifest: %#v", sheets[0])
	}
}

func TestParseXLSDocumentManifestSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "samples", "taxonomy-with-ids.ja-JP.xls")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("sample xls not available: %v", err)
	}
	sheets, err := parseXLSDocumentManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) == 0 {
		t.Fatal("expected at least one sheet")
	}
	if sheets[0].Name == "" || sheets[0].RowCountHint == 0 {
		t.Fatalf("unexpected xls sheet manifest: %#v", sheets[0])
	}
}

func TestDriveDocumentManifestCacheDecode(t *testing.T) {
	manifest := DriveDocumentManifest{
		File:          DriveDocumentManifestFile{PublicID: "file_1", SHA256Hex: "sha"},
		DocumentType:  "excel",
		Manifest:      map[string]any{"sheets": []any{}},
		GeneratedAt:   testTime(),
		ParserVersion: driveDocumentManifestParser,
	}
	decoded, ok := driveDocumentManifestFromMetadata(map[string]any{driveDocumentManifestMetadataKey: manifest})
	if !ok {
		t.Fatal("expected manifest cache decode")
	}
	if decoded.File.SHA256Hex != "sha" || decoded.DocumentType != "excel" {
		t.Fatalf("unexpected decoded manifest: %#v", decoded)
	}
}

func testTime() time.Time {
	return time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
}

func TestReadDataPipelineSpreadsheetRequiresColumnsWhenHeaderless(t *testing.T) {
	workbook := excelize.NewFile()
	defer func() { _ = workbook.Close() }()
	sheet := workbook.GetSheetName(0)
	row := []any{"100", "DIY用品"}
	if err := workbook.SetSheetRow(sheet, "A1", &row); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := workbook.Write(&buf); err != nil {
		t.Fatal(err)
	}

	_, err := readDataPipelineSpreadsheet(
		DriveFile{OriginalFilename: "taxonomy.xlsx", ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		buf.Bytes(),
		"",
		-1,
		"",
		0,
		100,
		nil,
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadDataPipelineSpreadsheetSheetIndexFallbackAndRange(t *testing.T) {
	workbook := excelize.NewFile()
	defer func() { _ = workbook.Close() }()
	first := workbook.GetSheetName(0)
	if err := workbook.SetSheetName(first, "Cover"); err != nil {
		t.Fatal(err)
	}
	second, err := workbook.NewSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	workbook.SetActiveSheet(second)
	header := []any{"skip", "id", "name"}
	row := []any{"x", "1", "Alpha"}
	if err := workbook.SetSheetRow("Data", "A1", &header); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetSheetRow("Data", "A2", &row); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := workbook.Write(&buf); err != nil {
		t.Fatal(err)
	}

	result, err := readDataPipelineSpreadsheet(
		DriveFile{OriginalFilename: "products.xlsx", ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		buf.Bytes(),
		"Renamed Data",
		1,
		"B1:C2",
		1,
		100,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.SheetName != "Data" || result.SheetIndex != 1 {
		t.Fatalf("resolved sheet = %q/%d, want Data/1", result.SheetName, result.SheetIndex)
	}
	if got := strings.Join(result.Header, ","); got != "id,name" {
		t.Fatalf("header = %q, want id,name", got)
	}
	if len(result.Rows) != 1 || result.Rows[0][0] != "1" || result.Rows[0][1] != "Alpha" {
		t.Fatalf("unexpected rows: %#v", result.Rows)
	}
}
