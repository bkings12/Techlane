package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteBucketCollapsesIDs(t *testing.T) {
	cases := map[string]string{
		"/repairs/550e8400-e29b-41d4-a716-446655440000":       "GET /repairs/:id",
		"/repairs/550e8400-e29b-41d4-a716-446655440000/parts": "GET /repairs/:id/parts",
		"/customers/42":       "GET /customers/:id",
		"/inventory/products": "GET /inventory/products",
	}
	for path, want := range cases {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		got := routeBucket(r)
		if got != want {
			t.Errorf("routeBucket(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestMetricsMiddlewareRecoversPanicAndReportsError(t *testing.T) {
	DefaultMetrics = &Metrics{routes: map[string]*routeStats{}}
	var captured struct {
		status  int
		message string
	}
	SetErrorSink(recordFunc(func(_ context.Context, _, _ string, status int, message, _, _ string) error {
		captured.status = status
		captured.message = message
		return nil
	}))
	defer SetErrorSink(nil)

	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(errors.New("boom"))
	})
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rw := httptest.NewRecorder()

	MetricsMiddleware(panicking).ServeHTTP(rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 after recovered panic, got %d", rw.Code)
	}
	if captured.status != http.StatusInternalServerError {
		t.Fatalf("expected error sink to receive 500, got %d", captured.status)
	}
	if DefaultMetrics.Errors.Load() != 1 {
		t.Fatalf("expected 1 error recorded, got %d", DefaultMetrics.Errors.Load())
	}
}

func TestMetricsMiddlewareReportsFiveHundredResponses(t *testing.T) {
	DefaultMetrics = &Metrics{routes: map[string]*routeStats{}}
	var gotStatus int
	SetErrorSink(recordFunc(func(_ context.Context, _, _ string, status int, _, _, _ string) error {
		gotStatus = status
		return nil
	}))
	defer SetErrorSink(nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	req := httptest.NewRequest(http.MethodGet, "/flaky", nil)
	rw := httptest.NewRecorder()

	MetricsMiddleware(handler).ServeHTTP(rw, req)

	if gotStatus != http.StatusServiceUnavailable {
		t.Fatalf("expected error sink to see 503, got %d", gotStatus)
	}
}

func TestMetricsMiddlewarePreservesFlusher(t *testing.T) {
	DefaultMetrics = &Metrics{routes: map[string]*routeStats{}}
	streamed := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter lost http.Flusher through MetricsMiddleware")
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(": connected\n\n"))
		f.Flush()
		streamed = true
	})
	req := httptest.NewRequest(http.MethodGet, "/events/stream", nil)
	rw := httptest.NewRecorder()

	MetricsMiddleware(handler).ServeHTTP(rw, req)

	if !streamed {
		t.Fatal("handler could not stream through the middleware")
	}
	if rw.Code != http.StatusOK {
		t.Fatalf("expected 200 from SSE handler, got %d", rw.Code)
	}
	if !rw.Flushed {
		t.Fatal("expected the flush to reach the underlying ResponseWriter")
	}
}

type recordFunc func(ctx context.Context, method, route string, status int, message, stack, correlationID string) error

func (f recordFunc) RecordError(ctx context.Context, method, route string, status int, message, stack, correlationID string) error {
	return f(ctx, method, route, status, message, stack, correlationID)
}
