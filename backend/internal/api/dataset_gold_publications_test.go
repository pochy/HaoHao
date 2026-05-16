package api

import (
	"encoding/json"
	"strings"
	"testing"

	"example.com/haohao/backend/internal/service"
)

func TestDatasetGoldPublicationBodyOmitsInternalFields(t *testing.T) {
	body := toDatasetGoldPublicationBody(service.DatasetGoldPublication{
		PublicID:                "018f2f05-c6c9-7a49-b32d-04f4dd84ef4a",
		SourceWorkTablePublicID: "018f2f05-c6c9-7a49-b32d-04f4dd84ef4b",
		SourceWorkTableName:     "support",
		SourceWorkTableDatabase: "hh_t_1_work",
		SourceWorkTableTable:    "support",
		DisplayName:             "Support mart",
		GoldDatabase:            "hh_t_1_gold",
		GoldTable:               "gm_support",
		Status:                  "active",
		RefreshPolicy:           "manual",
	})
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal DatasetGoldPublicationBody: %v", err)
	}
	payload := string(data)
	for _, forbidden := range []string{"sourceWorkTableDatabase", "sourceWorkTableName", "sourceWorkTableTable", "internalTable", "internalDatabase"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("gold publication response contains forbidden field %q: %s", forbidden, payload)
		}
	}
	if !strings.Contains(payload, "sourceWorkTablePublicId") {
		t.Fatalf("gold publication response missing sourceWorkTablePublicId: %s", payload)
	}
}

func TestDatasetGoldPublishRunBodyIncludesSourcePipelineRun(t *testing.T) {
	body := toDatasetGoldPublishRunBody(service.DatasetGoldPublishRun{
		PublicID:                "018f2f05-c6c9-7a49-b32d-04f4dd84ef4c",
		PublicationPublicID:     "018f2f05-c6c9-7a49-b32d-04f4dd84ef4a",
		SourceWorkTablePublicID: "018f2f05-c6c9-7a49-b32d-04f4dd84ef4b",
		SourceDataPipelineRun: &service.DatasetGoldSourceDataPipelineRun{
			PipelinePublicID: "018f2f05-c6c9-7a49-b32d-04f4dd84ef4d",
			PipelineName:     "SCD2 customer pipeline",
			RunPublicID:      "018f2f05-c6c9-7a49-b32d-04f4dd84ef4e",
			RunStatus:        "completed",
			OutputNodeID:     "output_gold",
			OutputRowCount:   42,
			OutputWriteMode:  "scd2_merge",
			SCD2MergePolicy:  "current_only",
			SCD2UniqueKeys:   []string{"customer_id"},
		},
		Status:       "completed",
		GoldDatabase: "hh_t_1_gold",
		GoldTable:    "gm_customers",
	})
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal DatasetGoldPublishRunBody: %v", err)
	}
	payload := string(data)
	for _, required := range []string{"sourceDataPipelineRun", "pipelinePublicId", "runPublicId", "outputNodeId", "scd2UniqueKeys"} {
		if !strings.Contains(payload, required) {
			t.Fatalf("gold publish run response missing %q: %s", required, payload)
		}
	}
	for _, forbidden := range []string{"RunID", "runId"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("gold publish run response contains internal field %q: %s", forbidden, payload)
		}
	}
}
