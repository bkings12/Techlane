package httpx

import (
	"net/http"
	"sync/atomic"
	"time"
)

// Metrics holds process-local HTTP counters for /metrics.
type Metrics struct {
	Requests atomic.Uint64
	Errors   atomic.Uint64
}

var DefaultMetrics Metrics

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// MetricsMiddleware increments request/error counters.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		DefaultMetrics.Requests.Add(1)
		if rec.status >= 500 {
			DefaultMetrics.Errors.Add(1)
		}
		_ = start
	})
}

// MetricsHandler exposes Prometheus-ish text metrics without extra deps.
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(
		"# HELP techlane_http_requests_total Total HTTP requests\n" +
			"# TYPE techlane_http_requests_total counter\n",
	))
	_, _ = w.Write([]byte("techlane_http_requests_total " + itoa(DefaultMetrics.Requests.Load()) + "\n"))
	_, _ = w.Write([]byte(
		"# HELP techlane_http_errors_total Total HTTP 5xx responses\n" +
			"# TYPE techlane_http_errors_total counter\n",
	))
	_, _ = w.Write([]byte("techlane_http_errors_total " + itoa(DefaultMetrics.Errors.Load()) + "\n"))
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
