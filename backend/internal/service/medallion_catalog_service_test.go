package service

import "testing"

func TestMedallionDriveFileEligible(t *testing.T) {
	tests := []struct {
		name string
		file DriveFile
		want bool
	}{
		{
			name: "csv",
			file: DriveFile{OriginalFilename: "customers.csv", ContentType: "text/csv"},
			want: true,
		},
		{
			name: "image",
			file: DriveFile{OriginalFilename: "photo.png", ContentType: "image/png"},
			want: true,
		},
		{
			name: "pdf extension fallback",
			file: DriveFile{OriginalFilename: "catalog.pdf", ContentType: "application/octet-stream"},
			want: true,
		},
		{
			name: "docx",
			file: DriveFile{OriginalFilename: "brief.docx", ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := medallionDriveFileEligible(tt.file); got != tt.want {
				t.Fatalf("medallionDriveFileEligible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMedallionDatasetLayer(t *testing.T) {
	tests := []struct {
		name    string
		dataset Dataset
		want    string
	}{
		{name: "file backed dataset is bronze", dataset: Dataset{SourceKind: "file"}, want: MedallionLayerBronze},
		{name: "work table backed dataset is silver", dataset: Dataset{SourceKind: "work_table"}, want: MedallionLayerSilver},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := medallionDatasetLayer(tt.dataset); got != tt.want {
				t.Fatalf("medallionDatasetLayer() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMedallionStatusMappings(t *testing.T) {
	datasetStatuses := map[string]string{
		"pending":   MedallionAssetStatusBuilding,
		"importing": MedallionAssetStatusBuilding,
		"failed":    MedallionAssetStatusFailed,
		"deleted":   MedallionAssetStatusArchived,
		"ready":     MedallionAssetStatusActive,
	}
	for input, want := range datasetStatuses {
		if got := medallionAssetStatusFromDatasetStatus(input); got != want {
			t.Fatalf("medallionAssetStatusFromDatasetStatus(%q) = %q, want %q", input, got, want)
		}
	}

	ocrStatuses := map[string]string{
		"completed": MedallionPipelineStatusCompleted,
		"failed":    MedallionPipelineStatusFailed,
		"skipped":   MedallionPipelineStatusSkipped,
		"running":   MedallionPipelineStatusProcessing,
		"pending":   MedallionPipelineStatusPending,
	}
	for input, want := range ocrStatuses {
		if got := medallionPipelineStatusFromOCRStatus(input); got != want {
			t.Fatalf("medallionPipelineStatusFromOCRStatus(%q) = %q, want %q", input, got, want)
		}
	}

	pipelineStatuses := map[string]string{
		MedallionPipelineStatusCompleted:  MedallionAssetStatusActive,
		MedallionPipelineStatusFailed:     MedallionAssetStatusFailed,
		MedallionPipelineStatusSkipped:    MedallionAssetStatusSkipped,
		MedallionPipelineStatusProcessing: MedallionAssetStatusBuilding,
	}
	for input, want := range pipelineStatuses {
		if got := medallionAssetStatusFromPipelineStatus(input); got != want {
			t.Fatalf("medallionAssetStatusFromPipelineStatus(%q) = %q, want %q", input, got, want)
		}
	}
}
