package api

import "testing"

func TestRawDriveLooksLikeCSVDoesNotTreatExcelWorkbookAsCSV(t *testing.T) {
	if rawDriveLooksLikeCSV("taxonomy-with-ids.ja-JP.xls", "application/vnd.ms-excel") {
		t.Fatal("xls workbook should not be treated as csv")
	}
	if rawDriveLooksLikeCSV("taxonomy.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") {
		t.Fatal("xlsx workbook should not be treated as csv")
	}
	if !rawDriveLooksLikeCSV("customers.csv", "application/octet-stream") {
		t.Fatal("csv extension should be treated as csv")
	}
}
