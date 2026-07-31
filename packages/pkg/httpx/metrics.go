package httpx

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds process-local HTTP counters for /metrics.
type Metrics struct {
	Requests atomic.Uint64
	Errors   atomic.Uint64

	mu     sync.Mutex
	routes map[string]*routeStats
}

type routeStats struct {
	count      uint64
	errorCount uint64
	totalMs    float64
}

var DefaultMetrics = &Metrics{routes: map[string]*routeStats{}}

// ErrorSink persists unexpected errors (panics, 5xx responses) somewhere a
// human can review them later — see internal/audit.Service.RecordError.
type ErrorSink interface {
	RecordError(ctx context.Context, method, route string, status int, message, stack, correlationID string) error
}

var (
	sinkMu sync.RWMutex
	sink   ErrorSink
)

// SetErrorSink wires the process-wide error sink. Call once at startup.
func SetErrorSink(s ErrorSink) {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	sink = s
}

func getErrorSink() ErrorSink {
	sinkMu.RLock()
	defer sinkMu.RUnlock()
	return sink
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.wroteHeader {
		return
	}
	s.wroteHeader = true
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(b)
}

// Flush keeps streaming responses (SSE at /events/stream) working through the
// wrapper — without it the handler can't type-assert to http.Flusher.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		if !s.wroteHeader {
			s.WriteHeader(http.StatusOK)
		}
		f.Flush()
	}
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("httpx: ResponseWriter does not support hijacking")
	}
	return h.Hijack()
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// routeBucket collapses high-cardinality paths (IDs, codes) into a stable
// label so /metrics doesn't grow one series per repair/customer/etc.
func routeBucket(r *http.Request) string {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	kept := make([]string, 0, 3)
	for _, p := range parts {
		if p == "" {
			continue
		}
		if looksLikeID(p) {
			kept = append(kept, ":id")
			continue
		}
		kept = append(kept, p)
		if len(kept) >= 4 {
			break
		}
	}
	return r.Method + " /" + strings.Join(kept, "/")
}

func looksLikeID(s string) bool {
	if len(s) >= 20 && strings.Count(s, "-") >= 3 {
		return true // uuid-shaped
	}
	if len(s) > 0 {
		digits := true
		for _, c := range s {
			if c < '0' || c > '9' {
				digits = false
				break
			}
		}
		if digits {
			return true
		}
	}
	return false
}

// MetricsMiddleware records per-route counts/latency, recovers panics into a
// 500 instead of crashing the process, and forwards panics + 5xx responses to
// the configured ErrorSink (if any) so they're visible somewhere besides logs.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		route := routeBucket(r)
		corrID := CorrelationID(r.Context())

		defer func() {
			if rerr := recover(); rerr != nil {
				stack := string(debug.Stack())
				slog.Error("panic recovered", "route", route, "correlation_id", corrID, "panic", rerr)
				if !rec.wroteHeader {
					rec.WriteHeader(http.StatusInternalServerError)
				}
				DefaultMetrics.record(route, time.Since(start), true)
				if s := getErrorSink(); s != nil {
					_ = s.RecordError(context.Background(), r.Method, route, http.StatusInternalServerError,
						fmt.Sprintf("panic: %v", rerr), stack, corrID)
				}
			}
		}()

		next.ServeHTTP(rec, r)

		elapsed := time.Since(start)
		isErr := rec.status >= 500
		DefaultMetrics.record(route, elapsed, isErr)
		if isErr {
			if s := getErrorSink(); s != nil {
				_ = s.RecordError(context.Background(), r.Method, route, rec.status,
					fmt.Sprintf("%s responded %d", route, rec.status), "", corrID)
			}
		}
	})
}

func (m *Metrics) record(route string, elapsed time.Duration, isErr bool) {
	m.Requests.Add(1)
	if isErr {
		m.Errors.Add(1)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rs, ok := m.routes[route]
	if !ok {
		rs = &routeStats{}
		m.routes[route] = rs
	}
	rs.count++
	rs.totalMs += float64(elapsed.Microseconds()) / 1000.0
	if isErr {
		rs.errorCount++
	}
}

// MetricsHandler exposes Prometheus text-format metrics without extra deps.
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	var b strings.Builder
	b.WriteString("# HELP techlane_http_requests_total Total HTTP requests\n")
	b.WriteString("# TYPE techlane_http_requests_total counter\n")
	fmt.Fprintf(&b, "techlane_http_requests_total %d\n", DefaultMetrics.Requests.Load())
	b.WriteString("# HELP techlane_http_errors_total Total HTTP 5xx responses\n")
	b.WriteString("# TYPE techlane_http_errors_total counter\n")
	fmt.Fprintf(&b, "techlane_http_errors_total %d\n", DefaultMetrics.Errors.Load())

	DefaultMetrics.mu.Lock()
	defer DefaultMetrics.mu.Unlock()
	b.WriteString("# HELP techlane_http_route_requests_total Requests per route\n")
	b.WriteString("# TYPE techlane_http_route_requests_total counter\n")
	b.WriteString("# HELP techlane_http_route_errors_total 5xx responses per route\n")
	b.WriteString("# TYPE techlane_http_route_errors_total counter\n")
	b.WriteString("# HELP techlane_http_route_latency_ms_avg Average latency per route in milliseconds\n")
	b.WriteString("# TYPE techlane_http_route_latency_ms_avg gauge\n")
	for route, rs := range DefaultMetrics.routes {
		label := promLabel(route)
		fmt.Fprintf(&b, "techlane_http_route_requests_total{route=%q} %d\n", label, rs.count)
		fmt.Fprintf(&b, "techlane_http_route_errors_total{route=%q} %d\n", label, rs.errorCount)
		avg := 0.0
		if rs.count > 0 {
			avg = rs.totalMs / float64(rs.count)
		}
		fmt.Fprintf(&b, "techlane_http_route_latency_ms_avg{route=%q} %.2f\n", label, avg)
	}
	_, _ = w.Write([]byte(b.String()))
}

func promLabel(route string) string {
	return strings.ReplaceAll(route, `"`, `'`)
}
