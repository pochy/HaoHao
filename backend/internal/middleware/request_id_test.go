package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestRequestIDGeneratesHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/", func(c *gin.Context) {
		if RequestIDFromContext(c) == "" {
			t.Fatal("request id is empty")
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Header().Get(RequestIDHeader) == "" {
		t.Fatal("response request id header is empty")
	}
}

func TestRequestIDPreservesIncomingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(RequestIDHeader, "incoming-id")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if got := recorder.Header().Get(RequestIDHeader); got != "incoming-id" {
		t.Fatalf("request id = %q", got)
	}
}

func TestRequestLoggerWritesStructuredLog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := gin.New()
	router.Use(RequestID(), RequestLogger(logger))
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	logLine := output.String()
	for _, want := range []string{`"msg":"http request"`, `"status":204`, `"request_id":`} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log line %q does not contain %q", logLine, want)
		}
	}
}

func TestRequestLoggerWritesTraceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	traceID, err := oteltrace.TraceIDFromHex("11111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := oteltrace.SpanIDFromHex("2222222222222222")
	if err != nil {
		t.Fatal(err)
	}
	spanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
	})

	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := gin.New()
	router.Use(
		RequestID(),
		func(c *gin.Context) {
			c.Request = c.Request.WithContext(oteltrace.ContextWithSpanContext(c.Request.Context(), spanContext))
			c.Next()
		},
		RequestLogger(logger),
	)
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	logLine := output.String()
	for _, want := range []string{
		`"trace_id":"11111111111111111111111111111111"`,
		`"span_id":"2222222222222222"`,
	} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log line %q does not contain %q", logLine, want)
		}
	}
}
