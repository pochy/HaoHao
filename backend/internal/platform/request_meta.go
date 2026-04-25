package platform

import "context"

type requestMetadataContextKey struct{}

type RequestMetadata struct {
	RequestID string
	ClientIP  string
	UserAgent string
}

func ContextWithRequestMetadata(ctx context.Context, metadata RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataContextKey{}, metadata)
}

func RequestMetadataFromContext(ctx context.Context) RequestMetadata {
	metadata, _ := ctx.Value(requestMetadataContextKey{}).(RequestMetadata)
	return metadata
}
