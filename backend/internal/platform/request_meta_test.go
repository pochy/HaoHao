package platform

import (
	"context"
	"testing"
)

func TestRequestMetadataContext(t *testing.T) {
	ctx := ContextWithRequestMetadata(context.Background(), RequestMetadata{
		RequestID: "req-1",
		ClientIP:  "127.0.0.1",
		UserAgent: "test-agent",
	})

	got := RequestMetadataFromContext(ctx)
	if got.RequestID != "req-1" || got.ClientIP != "127.0.0.1" || got.UserAgent != "test-agent" {
		t.Fatalf("RequestMetadataFromContext() = %#v", got)
	}
}
