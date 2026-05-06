package service

import (
	"bytes"
	"testing"

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
		0,
		100,
		nil,
	)
	if err == nil {
		t.Fatal("expected error")
	}
}
