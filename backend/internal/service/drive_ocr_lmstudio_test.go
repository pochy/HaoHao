package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLMStudioOCRImageUsesVisionChatCompletion(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"請求書\n合計 1200円"}}]}`))
	}))
	defer server.Close()

	text, err := lmStudioOCRImage(t.Context(), []byte("fake png bytes"), "image/png", DriveOCRPolicy{
		LMStudioBaseURL:       server.URL,
		LMStudioModel:         "qwen2.5-vl-7b-instruct",
		TimeoutSecondsPerPage: 5,
	})
	if err != nil {
		t.Fatalf("lmStudioOCRImage() error = %v", err)
	}
	if text != "請求書\n合計 1200円" {
		t.Fatalf("text = %q", text)
	}
	if got["model"] != "qwen2.5-vl-7b-instruct" {
		t.Fatalf("model = %v", got["model"])
	}
	messages := got["messages"].([]any)
	user := messages[1].(map[string]any)
	content := user["content"].([]any)
	imagePart := content[1].(map[string]any)
	imageURL := imagePart["image_url"].(map[string]any)["url"].(string)
	if !strings.HasPrefix(imageURL, "data:image/png;base64,") {
		t.Fatalf("image url = %q", imageURL)
	}
}

func TestValidateDriveOCRPolicyAllowsLMStudioEngineWithModel(t *testing.T) {
	policy := defaultDriveOCRPolicy()
	policy.OCREngine = "lmstudio"
	policy.LMStudioModel = "qwen2.5-vl-7b-instruct"
	if err := validateDriveOCRPolicy(policy); err != nil {
		t.Fatalf("validateDriveOCRPolicy() error = %v", err)
	}
}

func TestValidateDriveOCRPolicyRequiresLMStudioOCRModel(t *testing.T) {
	policy := defaultDriveOCRPolicy()
	policy.OCREngine = "lmstudio"
	policy.LMStudioModel = ""
	if err := validateDriveOCRPolicy(policy); err == nil {
		t.Fatal("validateDriveOCRPolicy() error = nil, want error")
	}
}
