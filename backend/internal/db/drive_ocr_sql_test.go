package db

import (
	"strings"
	"testing"
)

func TestCreateDriveOCRRunDoesNotTouchCompletedRunUpdatedAt(t *testing.T) {
	if !strings.Contains(createDriveOCRRun, "WHEN drive_ocr_runs.status IN ('failed', 'skipped') THEN now()") {
		t.Fatalf("CreateDriveOCRRun must only refresh updated_at when retrying failed or skipped runs")
	}
	if !strings.Contains(createDriveOCRRun, "ELSE drive_ocr_runs.updated_at") {
		t.Fatalf("CreateDriveOCRRun must preserve updated_at when reusing completed OCR runs")
	}
	if strings.Contains(createDriveOCRRun, "updated_at = now()\nRETURNING") {
		t.Fatalf("CreateDriveOCRRun must not refresh updated_at for idempotent completed-run reuse")
	}
}

func TestGetLatestDriveOCRRunForFilePrefersDriveRuns(t *testing.T) {
	if !strings.Contains(getLatestDriveOCRRunForFile, "CASE WHEN reason IN ('data_pipeline', 'data_pipeline_preview') THEN 1 ELSE 0 END") {
		t.Fatalf("GetLatestDriveOCRRunForFile must rank data-pipeline OCR runs after drive-originated OCR runs")
	}
	if !strings.Contains(getLatestDriveOCRRunForFile, "updated_at DESC") {
		t.Fatalf("GetLatestDriveOCRRunForFile must use updated_at to choose the newest drive-originated OCR run")
	}
}
