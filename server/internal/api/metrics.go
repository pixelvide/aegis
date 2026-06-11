package api

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Metrics holds OTel meter instruments for the Aegis server.
type Metrics struct {
	requestsTotal   otelmetric.Int64Counter
	requestDuration otelmetric.Float64Histogram
	activeRequests  otelmetric.Int64UpDownCounter
}

// InitMetrics initializes the OTel meter provider with a Prometheus exporter
// and returns the Metrics instruments and the Prometheus HTTP handler.
// The returned shutdown function should be called on server exit.
func InitMetrics(db *sql.DB) (*Metrics, http.Handler, func(context.Context) error) {
	exporter, err := prometheus.New(
		prometheus.WithNamespace("aegis"),
	)
	if err != nil {
		log.Printf("⚠️  Failed to create Prometheus exporter: %v — metrics disabled", err)
		return nil, http.NotFoundHandler(), func(context.Context) error { return nil }
	}

	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	otel.SetMeterProvider(provider)

	meter := provider.Meter("aegis-server")

	requestsTotal, _ := meter.Int64Counter("http_requests_total",
		otelmetric.WithDescription("Total HTTP requests"),
		otelmetric.WithUnit("{request}"),
	)

	requestDuration, _ := meter.Float64Histogram("http_request_duration_seconds",
		otelmetric.WithDescription("HTTP request duration in seconds"),
		otelmetric.WithUnit("s"),
	)

	activeRequests, _ := meter.Int64UpDownCounter("http_active_requests",
		otelmetric.WithDescription("Currently in-flight HTTP requests"),
		otelmetric.WithUnit("{request}"),
	)

	// Register async gauge for DB pool stats
	if db != nil {
		_, _ = meter.Int64ObservableGauge("db_pool_open_connections",
			otelmetric.WithDescription("Number of open database connections"),
			otelmetric.WithUnit("{connection}"),
			otelmetric.WithInt64Callback(func(_ context.Context, o otelmetric.Int64Observer) error {
				stats := db.Stats()
				o.Observe(int64(stats.OpenConnections))
				return nil
			}),
		)

		_, _ = meter.Int64ObservableGauge("db_pool_in_use",
			otelmetric.WithDescription("Number of connections in use"),
			otelmetric.WithUnit("{connection}"),
			otelmetric.WithInt64Callback(func(_ context.Context, o otelmetric.Int64Observer) error {
				stats := db.Stats()
				o.Observe(int64(stats.InUse))
				return nil
			}),
		)

		_, _ = meter.Int64ObservableGauge("db_pool_idle",
			otelmetric.WithDescription("Number of idle connections"),
			otelmetric.WithUnit("{connection}"),
			otelmetric.WithInt64Callback(func(_ context.Context, o otelmetric.Int64Observer) error {
				stats := db.Stats()
				o.Observe(int64(stats.Idle))
				return nil
			}),
		)
	}

	m := &Metrics{
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
		activeRequests:  activeRequests,
	}

	return m, promhttp.Handler(), provider.Shutdown
}

// Middleware returns an HTTP middleware that records request metrics.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	if m == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Track active requests
		m.activeRequests.Add(r.Context(), 1)
		defer m.activeRequests.Add(r.Context(), -1)

		// Wrap ResponseWriter to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		attrs := []attribute.KeyValue{
			attribute.String("method", r.Method),
			attribute.String("path", routePattern(r)),
			attribute.String("status", strconv.Itoa(rw.statusCode)),
		}

		m.requestsTotal.Add(r.Context(), 1, otelmetric.WithAttributes(attrs...))
		m.requestDuration.Record(r.Context(), duration, otelmetric.WithAttributes(attrs...))
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// routePattern normalizes the URL path for metric labels to avoid high
// cardinality from path parameters (UUIDs, etc.).
func routePattern(r *http.Request) string {
	// Go 1.22+ stores the matched pattern
	if pattern := r.Pattern; pattern != "" {
		return pattern
	}
	return r.URL.Path
}
