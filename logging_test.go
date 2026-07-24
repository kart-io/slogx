package slogx

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	collectlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/protobuf/proto"
)

func TestNewJSONRedactsSecretsAndAddsService(t *testing.T) {
	var out bytes.Buffer
	logger, err := New(&out, Config{Level: "debug", Format: "json", Service: "api"})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("login", "access_token", "top-secret", "user_id", 7)
	got := out.String()
	for _, want := range []string{`"service":"api"`, `"access_token":"[REDACTED]"`, `"user_id":7`} {
		if !strings.Contains(got, want) {
			t.Fatalf("log %q does not contain %q", got, want)
		}
	}
	if strings.Contains(got, "top-secret") {
		t.Fatalf("secret leaked in %q", got)
	}
}

func TestNewHonorsLevel(t *testing.T) {
	var out bytes.Buffer
	logger, err := New(&out, Config{Level: "warn", Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("hidden")
	logger.Log(nil, slog.LevelWarn, "shown")
	if strings.Contains(out.String(), "hidden") || !strings.Contains(out.String(), "shown") {
		t.Fatalf("unexpected level output %q", out.String())
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	if _, err := New(&bytes.Buffer{}, Config{Level: "verbose"}); err == nil {
		t.Fatal("expected invalid level error")
	}
	if _, err := New(&bytes.Buffer{}, Config{Format: "xml"}); err == nil {
		t.Fatal("expected invalid format error")
	}
}

func TestNewWithOTelEnabled(t *testing.T) {
	var out bytes.Buffer
	requests := make(chan *collectlogs.ExportLogsServiceRequest, 1)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/logs" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		request := new(collectlogs.ExportLogsServiceRequest)
		if err := proto.Unmarshal(body, request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- request
		w.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()

	cfg := Config{
		Level:        "info",
		Format:       "json",
		Service:      "test-service",
		Version:      "v2.3.4",
		OTelEnabled:  true,
		OTelEndpoint: strings.TrimPrefix(collector.URL, "http://"),
		OTelInsecure: true,
	}
	logger, err := New(&out, cfg)
	if err != nil {
		t.Fatalf("unexpected error when creating otel logger: %v", err)
	}
	logger.Info("hello otel", "access_token", "top-secret", "key", "value")

	got := out.String()
	if !strings.Contains(got, "hello otel") {
		t.Fatalf("expected log output in stdout, got %q", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := logger.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown OTEL provider: %v", err)
	}

	select {
	case request := <-requests:
		if len(request.ResourceLogs) != 1 {
			t.Fatalf("resource logs = %d, want 1", len(request.ResourceLogs))
		}
		resourceLog := request.ResourceLogs[0]
		if got := otelAttributeString(resourceLog.Resource.Attributes, "service.name"); got != "test-service" {
			t.Fatalf("service.name = %q", got)
		}
		if got := otelAttributeString(resourceLog.Resource.Attributes, "service.version"); got != "v2.3.4" {
			t.Fatalf("service.version = %q", got)
		}
		scopeLog := resourceLog.ScopeLogs[0]
		if scopeLog.Scope.Name != "test-service" || scopeLog.Scope.Version != "v2.3.4" {
			t.Fatalf("unexpected scope: %+v", scopeLog.Scope)
		}
		record := scopeLog.LogRecords[0]
		if got := otelAttributeString(record.Attributes, "access_token"); got != "[REDACTED]" {
			t.Fatalf("remote access_token = %q, want redacted", got)
		}
		if got := otelAttributeString(record.Attributes, "key"); got != "value" {
			t.Fatalf("remote key = %q, want value", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("collector did not receive OTLP logs")
	}
}

func otelAttributeString(attrs []*commonpb.KeyValue, key string) string {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Value.GetStringValue()
		}
	}
	return ""
}

func TestMultiHandler(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewTextHandler(&buf1, nil)
	h2 := slog.NewTextHandler(&buf2, nil)

	mh := NewMultiHandler(h1, h2)
	l := slog.New(mh)

	l.Info("multi log test", "foo", "bar")

	if !strings.Contains(buf1.String(), "multi log test") || !strings.Contains(buf2.String(), "multi log test") {
		t.Fatalf("multiHandler failed to forward log record to all handlers. buf1=%q buf2=%q", buf1.String(), buf2.String())
	}
}

func TestErrorLogLevelIncludesCallerAndStacktrace(t *testing.T) {
	var out bytes.Buffer
	logger, err := New(&out, Config{Level: "info", Format: "json", AddSource: false})
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("normal log")
	if strings.Contains(out.String(), `"caller":`) || strings.Contains(out.String(), `"stacktrace":`) {
		t.Fatalf("expected no caller/stacktrace for Info log when AddSource=false, got %q", out.String())
	}

	out.Reset()
	logger.Error("something failed")
	got := out.String()
	if !strings.Contains(got, `"caller":`) || !strings.Contains(got, "logging_test.go:") {
		t.Fatalf("expected caller field in Error log, got %q", got)
	}
	if !strings.Contains(got, `"stacktrace":`) || !strings.Contains(got, "TestErrorLogLevelIncludesCallerAndStacktrace") {
		t.Fatalf("expected stacktrace field containing test function in Error log, got %q", got)
	}
}

func TestLoggerEnvField(t *testing.T) {
	var out bytes.Buffer
	logger, err := New(&out, Config{Level: "info", Format: "json", Env: "staging"})
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("environment check")
	got := out.String()
	if !strings.Contains(got, `"env":"staging"`) {
		t.Fatalf("expected env:staging in output, got %q", got)
	}
}

func TestTraceLogCorrelation(t *testing.T) {
	var out bytes.Buffer
	logger, err := New(&out, Config{Level: "info", Format: "json"})
	if err != nil {
		t.Fatal(err)
	}

	logger.InfoContext(context.Background(), "context log entry")
	got := out.String()
	if !strings.Contains(got, `"context log entry"`) {
		t.Fatalf("expected log entry, got %q", got)
	}
}

func TestGORMLoggerAndRedisHook(t *testing.T) {
	gormLog := NewGORMLogger(100 * time.Millisecond)
	if gormLog == nil {
		t.Fatal("expected non-nil GORMLogger")
	}

	redisHook := NewRedisHook()
	if redisHook == nil {
		t.Fatal("expected non-nil RedisHook")
	}
}
