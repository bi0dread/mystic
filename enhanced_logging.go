package mystic

import (
	"context"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// EnhancedLogger extends the basic Logger with advanced features
type EnhancedLogger interface {
	Logger

	// Structured logging
	Structured(event string, fields map[string]interface{})

	// Sampling
	Sampled(level string, rate float64, msg string, keysAndValues ...interface{})

	// Metrics
	WithMetrics() EnhancedLogger
	IncrementCounter(name string, value int64)
	RecordGauge(name string, value float64)
	RecordHistogram(name string, value float64)

	// Performance tracking
	WithTiming(operation string) EnhancedLogger
	TimeOperation(operation string, fn func())

	// Rate limiting
	WithRateLimit(limit int) EnhancedLogger

	// Conditional logging
	When(condition bool) EnhancedLogger
	WhenContext(predicate func(context.Context) bool) EnhancedLogger

	// Batch logging
	Batch(entries []LogEntry)

	// Development mode
	Dev() EnhancedLogger
	WithPrettyPrint() EnhancedLogger
}

// EnhancedLoggerConfig holds configuration for enhanced logging
type EnhancedLoggerConfig struct {
	EnableMetrics     bool    `json:"enable_metrics"`      // Enable metrics collection
	EnableSampling    bool    `json:"enable_sampling"`     // Enable log sampling
	EnableTiming      bool    `json:"enable_timing"`       // Enable performance timing
	EnableRateLimit   bool    `json:"enable_rate_limit"`   // Enable rate limiting
	DefaultSampleRate float64 `json:"default_sample_rate"` // Default sampling rate
	PrettyPrint       bool    `json:"pretty_print"`        // Enable pretty printing in dev mode
}

// NewEnhancedLogger creates an enhanced logger from a base logger
func NewEnhancedLogger(base Logger, config EnhancedLoggerConfig) EnhancedLogger {
	return &mysticEnhanced{
		base:    base,
		config:  config,
		metrics: NewPrometheusMetricsCollector(),
		rateLimiter: &RateLimiter{
			limit: 1000,
			count: 0,
			mu:    &sync.Mutex{},
		},
	}
}

// mysticEnhanced implements the EnhancedLogger interface
type mysticEnhanced struct {
	base        Logger
	config      EnhancedLoggerConfig
	metrics     *PrometheusMetricsCollector
	rateLimiter *RateLimiter
	devMode     bool
	prettyPrint bool
	mu          sync.RWMutex
}

// Structured logging
func (m *mysticEnhanced) Structured(event string, fields map[string]interface{}) {
	// Add event name to fields
	allFields := make([]interface{}, 0, len(fields)*2+2)
	allFields = append(allFields, "event", event)

	// Add all fields
	for k, v := range fields {
		allFields = append(allFields, k, v)
	}

	m.base.Info("structured event", allFields...)
}

// Sampling
func (m *mysticEnhanced) Sampled(level string, rate float64, msg string, keysAndValues ...interface{}) {
	if !m.config.EnableSampling {
		// If sampling is disabled, log normally
		m.logAtLevel(level, msg, keysAndValues...)
		return
	}

	// Apply sampling
	if rand.Float64() <= rate {
		m.logAtLevel(level, msg, keysAndValues...)
	}
}

// Metrics
func (m *mysticEnhanced) WithMetrics() EnhancedLogger {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.EnableMetrics = true
	return m
}

func (m *mysticEnhanced) IncrementCounter(name string, value int64) {
	if !m.config.EnableMetrics {
		return
	}

	m.metrics.IncrementCounter(name, value)
}

func (m *mysticEnhanced) RecordGauge(name string, value float64) {
	if !m.config.EnableMetrics {
		return
	}

	m.metrics.RecordGauge(name, value)
}

func (m *mysticEnhanced) RecordHistogram(name string, value float64) {
	if !m.config.EnableMetrics {
		return
	}

	m.metrics.RecordHistogram(name, value)
}

// Performance tracking
func (m *mysticEnhanced) WithTiming(operation string) EnhancedLogger {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.EnableTiming = true
	return m
}

func (m *mysticEnhanced) TimeOperation(operation string, fn func()) {
	if !m.config.EnableTiming {
		fn()
		return
	}

	start := time.Now()
	fn()
	duration := time.Since(start)

	// Log timing information
	m.base.Info("operation completed",
		"operation", operation,
		"duration", duration.String(),
		"duration_ms", duration.Milliseconds(),
	)

	// Record as histogram if metrics are enabled
	if m.config.EnableMetrics {
		m.metrics.RecordHistogram(operation+"_duration", float64(duration.Milliseconds()))
	}
}

// Rate limiting
func (m *mysticEnhanced) WithRateLimit(limit int) EnhancedLogger {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.EnableRateLimit = true
	m.rateLimiter.SetLimit(limit)
	return m
}

// Conditional logging
func (m *mysticEnhanced) When(condition bool) EnhancedLogger {
	if !condition {
		return &noOpLogger{}
	}
	return m
}

func (m *mysticEnhanced) WhenContext(predicate func(context.Context) bool) EnhancedLogger {
	// This would need context to be passed in
	// For now, return self
	return m
}

// Batch logging
func (m *mysticEnhanced) Batch(entries []LogEntry) {
	for _, entry := range entries {
		m.base.Info(entry.Message, "batch_entry", entry)
	}
}

// Development mode
func (m *mysticEnhanced) Dev() EnhancedLogger {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.devMode = true
	return m
}

func (m *mysticEnhanced) WithPrettyPrint() EnhancedLogger {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.prettyPrint = true
	return m
}

// Standard Logger interface methods
func (m *mysticEnhanced) Debug(msg string, keysAndValues ...interface{}) {
	if m.shouldLog() {
		m.base.Debug(msg, keysAndValues...)
	}
}

func (m *mysticEnhanced) Info(msg string, keysAndValues ...interface{}) {
	if m.shouldLog() {
		m.base.Info(msg, keysAndValues...)
	}
}

func (m *mysticEnhanced) Warn(msg string, keysAndValues ...interface{}) {
	if m.shouldLog() {
		m.base.Warn(msg, keysAndValues...)
	}
}

func (m *mysticEnhanced) Error(msg string, keysAndValues ...interface{}) {
	if m.shouldLog() {
		m.base.Error(msg, keysAndValues...)
	}
}

func (m *mysticEnhanced) ErrorDetail(err error, keysAndValues ...interface{}) {
	if m.shouldLog() {
		m.base.ErrorDetail(err, keysAndValues...)
	}
}

func (m *mysticEnhanced) Panic(msg string, keysAndValues ...interface{}) {
	if m.shouldLog() {
		m.base.Panic(msg, keysAndValues...)
	}
}

func (m *mysticEnhanced) SkipLevel(skip int) Logger {
	return m.base.SkipLevel(skip)
}

func (m *mysticEnhanced) With(args ...interface{}) Logger {
	return m.base.With(args...)
}

func (m *mysticEnhanced) SetContext(ctx context.Context) Logger {
	return m.base.SetContext(ctx)
}

// Helper methods
func (m *mysticEnhanced) shouldLog() bool {
	if !m.config.EnableRateLimit {
		return true
	}

	return m.rateLimiter.Allow()
}

func (m *mysticEnhanced) logAtLevel(level, msg string, keysAndValues ...interface{}) {
	switch level {
	case "debug":
		m.base.Debug(msg, keysAndValues...)
	case "info":
		m.base.Info(msg, keysAndValues...)
	case "warn":
		m.base.Warn(msg, keysAndValues...)
	case "error":
		m.base.Error(msg, keysAndValues...)
	case "panic":
		m.base.Panic(msg, keysAndValues...)
	default:
		m.base.Info(msg, keysAndValues...)
	}
}

// PrometheusMetricsCollector collects and manages Prometheus metrics
type PrometheusMetricsCollector struct {
	counters   map[string]prometheus.Counter
	gauges     map[string]prometheus.Gauge
	histograms map[string]prometheus.Histogram
	mu         *sync.RWMutex
	registry   prometheus.Registerer
}

// NewPrometheusMetricsCollector creates a new Prometheus metrics collector
func NewPrometheusMetricsCollector() *PrometheusMetricsCollector {
	return &PrometheusMetricsCollector{
		counters:   make(map[string]prometheus.Counter),
		gauges:     make(map[string]prometheus.Gauge),
		histograms: make(map[string]prometheus.Histogram),
		mu:         &sync.RWMutex{},
		registry:   prometheus.DefaultRegisterer,
	}
}

// NewPrometheusMetricsCollectorWithRegistry creates a new Prometheus metrics collector with custom registry
func NewPrometheusMetricsCollectorWithRegistry(registry prometheus.Registerer) *PrometheusMetricsCollector {
	return &PrometheusMetricsCollector{
		counters:   make(map[string]prometheus.Counter),
		gauges:     make(map[string]prometheus.Gauge),
		histograms: make(map[string]prometheus.Histogram),
		mu:         &sync.RWMutex{},
		registry:   registry,
	}
}

func (mc *PrometheusMetricsCollector) IncrementCounter(name string, value int64) {
	mc.mu.RLock()
	counter, exists := mc.counters[name]
	mc.mu.RUnlock()

	if !exists {
		mc.mu.Lock()
		defer mc.mu.Unlock()
		// Double-check after acquiring write lock
		if counter, exists = mc.counters[name]; !exists {
			counter = prometheus.NewCounter(prometheus.CounterOpts{
				Name: name + "_total",
				Help: "Counter for " + name,
			})
			if err := mc.registry.Register(counter); err != nil {
				// If registration fails, create a no-op counter
				counter = prometheus.NewCounter(prometheus.CounterOpts{})
			}
			mc.counters[name] = counter
		}
	}

	counter.Add(float64(value))
}

func (mc *PrometheusMetricsCollector) RecordGauge(name string, value float64) {
	mc.mu.RLock()
	gauge, exists := mc.gauges[name]
	mc.mu.RUnlock()

	if !exists {
		mc.mu.Lock()
		defer mc.mu.Unlock()
		// Double-check after acquiring write lock
		if gauge, exists = mc.gauges[name]; !exists {
			gauge = prometheus.NewGauge(prometheus.GaugeOpts{
				Name: name + "_current",
				Help: "Gauge for " + name,
			})
			if err := mc.registry.Register(gauge); err != nil {
				// If registration fails, create a no-op gauge
				gauge = prometheus.NewGauge(prometheus.GaugeOpts{})
			}
			mc.gauges[name] = gauge
		}
	}

	gauge.Set(value)
}

func (mc *PrometheusMetricsCollector) RecordHistogram(name string, value float64) {
	mc.mu.RLock()
	histogram, exists := mc.histograms[name]
	mc.mu.RUnlock()

	if !exists {
		mc.mu.Lock()
		defer mc.mu.Unlock()
		// Double-check after acquiring write lock
		if histogram, exists = mc.histograms[name]; !exists {
			histogram = prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:    name + "_duration",
				Help:    "Histogram for " + name,
				Buckets: prometheus.DefBuckets,
			})
			if err := mc.registry.Register(histogram); err != nil {
				// If registration fails, create a no-op histogram
				histogram = prometheus.NewHistogram(prometheus.HistogramOpts{})
			}
			mc.histograms[name] = histogram
		}
	}

	histogram.Observe(value)
}

// GetMetrics returns a copy of all metrics (for backward compatibility)
func (mc *PrometheusMetricsCollector) GetMetrics() map[string]interface{} {
	// This method is kept for backward compatibility but Prometheus metrics
	// are automatically exposed via HTTP endpoint
	return map[string]interface{}{
		"prometheus_metrics": "available_at_/metrics_endpoint",
	}
}

// RateLimiter implements rate limiting for logging
type RateLimiter struct {
	limit int
	count int
	mu    *sync.Mutex
}

func (rl *RateLimiter) SetLimit(limit int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.limit = limit
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.count < rl.limit {
		rl.count++
		return true
	}

	return false
}

func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.count = 0
}

// noOpLogger implements EnhancedLogger but does nothing
type noOpLogger struct{}

func (n *noOpLogger) Debug(msg string, keysAndValues ...interface{})      {}
func (n *noOpLogger) Info(msg string, keysAndValues ...interface{})       {}
func (n *noOpLogger) Warn(msg string, keysAndValues ...interface{})       {}
func (n *noOpLogger) Error(msg string, keysAndValues ...interface{})      {}
func (n *noOpLogger) ErrorDetail(err error, keysAndValues ...interface{}) {}
func (n *noOpLogger) Panic(msg string, keysAndValues ...interface{})      {}
func (n *noOpLogger) SkipLevel(skip int) Logger                           { return n }
func (n *noOpLogger) With(args ...interface{}) Logger                     { return n }
func (n *noOpLogger) SetContext(ctx context.Context) Logger               { return n }

// EnhancedLogger methods
func (n *noOpLogger) Structured(event string, fields map[string]interface{})                       {}
func (n *noOpLogger) Sampled(level string, rate float64, msg string, keysAndValues ...interface{}) {}
func (n *noOpLogger) WithMetrics() EnhancedLogger                                                  { return n }
func (n *noOpLogger) IncrementCounter(name string, value int64)                                    {}
func (n *noOpLogger) RecordGauge(name string, value float64)                                       {}
func (n *noOpLogger) RecordHistogram(name string, value float64)                                   {}
func (n *noOpLogger) WithTiming(operation string) EnhancedLogger                                   { return n }
func (n *noOpLogger) TimeOperation(operation string, fn func())                                    {}
func (n *noOpLogger) WithRateLimit(limit int) EnhancedLogger                                       { return n }
func (n *noOpLogger) When(condition bool) EnhancedLogger                                           { return n }
func (n *noOpLogger) WhenContext(predicate func(context.Context) bool) EnhancedLogger              { return n }
func (n *noOpLogger) Batch(entries []LogEntry)                                                     {}
func (n *noOpLogger) Dev() EnhancedLogger                                                          { return n }
func (n *noOpLogger) WithPrettyPrint() EnhancedLogger                                              { return n }

// Helper function to compute histogram statistics
func computeHistogramStats(values []float64) map[string]float64 {
	if len(values) == 0 {
		return map[string]float64{}
	}

	var sum, min, max float64
	min = values[0]
	max = values[0]

	for _, v := range values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	mean := sum / float64(len(values))

	return map[string]float64{
		"count": float64(len(values)),
		"sum":   sum,
		"mean":  mean,
		"min":   min,
		"max":   max,
	}
}

// StartPrometheusMetricsServer starts a Prometheus metrics server
func StartPrometheusMetricsServer(addr string) error {
	http.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(addr, nil)
}

// StartPrometheusMetricsServerWithMux starts a Prometheus metrics server with a custom mux
func StartPrometheusMetricsServerWithMux(addr string, mux *http.ServeMux) error {
	if mux == nil {
		mux = http.NewServeMux()
	}
	mux.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(addr, mux)
}
