// Package observability provides Prometheus metrics and middleware for the
// ProveNN API. HLD §5: counters/histograms per endpoint, structured logs.
package observability

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

var (
	// HTTPRequestsTotal counts HTTP requests by method, route, and status code.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "provenn",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests by method, route, and status.",
		},
		[]string{"method", "route", "status"},
	)

	// HTTPRequestDuration measures request latency in seconds.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "provenn",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "route"},
	)

	// HTTPResponseSize tracks response body size in bytes.
	HTTPResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "provenn",
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes.",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 7), // 100B → 100MB
		},
		[]string{"method", "route"},
	)

	// InvoicesIssuedTotal counts invoices issued (for billing visibility).
	InvoicesIssuedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "provenn",
			Subsystem: "business",
			Name:      "invoices_issued_total",
			Help:      "Total invoices issued across all partners/providers.",
		},
	)

	// VerificationsSubmittedTotal counts verifications submitted.
	VerificationsSubmittedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "provenn",
			Subsystem: "business",
			Name:      "verifications_submitted_total",
			Help:      "Total verification submissions by result.",
		},
		[]string{"result"},
	)

	// WorkerJobDuration measures River job processing time.
	WorkerJobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "provenn",
			Subsystem: "worker",
			Name:      "job_duration_seconds",
			Help:      "River worker job duration in seconds.",
			Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"job_type", "status"},
	)
)

// responseWriter wraps http.ResponseWriter to capture status code and size.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += n
	return n, err
}

// Middleware records Prometheus metrics for every HTTP request.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)

		next.ServeHTTP(rw, r)

		// Use the Chi route pattern (e.g. "/api/v1/invoices/{referenceCode}")
		// instead of the actual path to keep cardinality bounded.
		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = "unknown"
		}

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)

		HTTPRequestsTotal.WithLabelValues(r.Method, routePattern, status).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, routePattern).Observe(duration)
		HTTPResponseSize.WithLabelValues(r.Method, routePattern).Observe(float64(rw.written))
	})
}

// RiverMiddleware implements rivertype.WorkerMiddleware to record per-job
// duration and success/error in Prometheus.
type RiverMiddleware struct {
	river.MiddlewareDefaults
}

func (RiverMiddleware) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
	start := time.Now()
	err := doInner(ctx)
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		status = "error"
	}
	WorkerJobDuration.WithLabelValues(job.Kind, status).Observe(duration)
	return err
}

