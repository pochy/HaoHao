package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLMStudioDriveProductExtractorUsesJSONSchemaResponseFormat(t *testing.T) {
	var gotRequest lmStudioChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"items\":[{\"itemType\":\"product\",\"name\":\"Sharp BD Recorder\",\"model\":\"4B-C40GT3\",\"confidence\":0.8}]}"}}]}`))
	}))
	defer server.Close()

	extractor := NewLMStudioDriveProductExtractor(server.Client())
	result, err := extractor.ExtractProducts(t.Context(), DriveProductExtractionInput{
		TenantID: 1,
		File: DriveFile{
			PublicID: "file-public-id",
		},
		FullText: "形名 4B-C40GT3 ブルーレイディスクレコーダー",
		Policy: DriveOCRPolicy{
			StructuredExtractor:   "lmstudio",
			LMStudioBaseURL:       server.URL,
			LMStudioModel:         "qwen3-4b-mlx",
			TimeoutSecondsPerPage: 15,
		},
	})
	if err != nil {
		t.Fatalf("ExtractProducts() error = %v", err)
	}
	if gotRequest.Model != "qwen3-4b-mlx" {
		t.Fatalf("model = %q, want qwen3-4b-mlx", gotRequest.Model)
	}
	format, ok := gotRequest.ResponseFormat.(map[string]any)
	if !ok {
		t.Fatalf("response_format type = %T, want map[string]any", gotRequest.ResponseFormat)
	}
	if format["type"] != "json_schema" {
		t.Fatalf("response_format.type = %v, want json_schema", format["type"])
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	if result.Items[0].Model != "4B-C40GT3" {
		t.Fatalf("item model = %q, want 4B-C40GT3", result.Items[0].Model)
	}
}
