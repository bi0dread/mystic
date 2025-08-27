package mystic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// HTTPAdapterConfig holds configuration for HTTP logging
type HTTPAdapterConfig struct {
	Endpoint   string            `json:"endpoint"`    // HTTP endpoint URL
	Method     string            `json:"method"`      // HTTP method (POST, PUT, etc.)
	Headers    map[string]string `json:"headers"`     // Custom headers
	Timeout    time.Duration     `json:"timeout"`     // Request timeout
	RetryCount int               `json:"retry_count"` // Number of retries on failure
	RetryDelay time.Duration     `json:"retry_delay"` // Delay between retries
	BatchSize  int               `json:"batch_size"`  // Batch size for log entries
	BatchDelay time.Duration     `json:"batch_delay"` // Delay before sending batch
	Format     string            `json:"format"`      // Output format: "json", "gelf"
	Level      string            `json:"level"`       // Log level for HTTP output
	Facility   string            `json:"facility"`    // GELF facility name
}

// HTTPAdapter creates an HTTP-backed logger adapter
func HTTPAdapter(named string) Logger {
	return HTTPAdapterWithConfig(named, HTTPAdapterConfig{
		Endpoint:   "http://localhost:8080/logs",
		Method:     "POST",
		Headers:    map[string]string{"Content-Type": "application/json"},
		Timeout:    5 * time.Second,
		RetryCount: 3,
		RetryDelay: 1 * time.Second,
		BatchSize:  100,
		BatchDelay: 100 * time.Millisecond,
		Format:     "json",
		Level:      "info",
	})
}

// HTTPAdapterWithConfig creates an HTTP-backed logger adapter with custom configuration
func HTTPAdapterWithConfig(named string, config HTTPAdapterConfig) Logger {
	// Set up zerolog
	zerolog.TimeFieldFormat = time.RFC3339

	// Determine log level
	level := parseHTTPLogLevel(config.Level)
	zerolog.SetGlobalLevel(level)

	// Create HTTP client
	client := &http.Client{
		Timeout: config.Timeout,
	}

	// Create batch processor
	batchProcessor := &HTTPBatchProcessor{
		config: config,
		client: client,
		queue:  make(chan LogEntry, config.BatchSize*2),
		mu:     &sync.Mutex{},
	}

	// Start batch processor
	go batchProcessor.Start()

	// Create output based on format
	var out io.Writer
	switch config.Format {
	case "gelf":
		out = &GELFHTTPWriter{processor: batchProcessor, facility: config.Facility}
	default: // json
		out = &JSONHTTPWriter{processor: batchProcessor}
	}

	base := zerolog.New(out).With().Timestamp().Str("logger", named).Logger()
	m := &mysticHTTP{
		logger:         base,
		name:           named,
		fields:         map[string]interface{}{},
		skip:           1,
		config:         config,
		batchProcessor: batchProcessor,
	}

	return m
}

// LogEntry represents a single log entry for batching
type LogEntry struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Fields    map[string]interface{} `json:"fields"`
	Logger    string                 `json:"logger"`
}

// HTTPBatchProcessor handles batching and sending log entries
type HTTPBatchProcessor struct {
	config HTTPAdapterConfig
	client *http.Client
	queue  chan LogEntry
	mu     *sync.Mutex
	closed bool
}

func (p *HTTPBatchProcessor) Start() {
	ticker := time.NewTicker(p.config.BatchDelay)
	defer ticker.Stop()

	var batch []LogEntry

	for {
		select {
		case entry, ok := <-p.queue:
			if !ok {
				// Channel closed, send remaining batch
				if len(batch) > 0 {
					p.sendBatch(batch)
				}
				return
			}

			batch = append(batch, entry)

			// Send batch if it reaches the size limit
			if len(batch) >= p.config.BatchSize {
				p.sendBatch(batch)
				batch = batch[:0] // Reset batch
			}

		case <-ticker.C:
			// Send batch if there are entries and delay has passed
			if len(batch) > 0 {
				p.sendBatch(batch)
				batch = batch[:0] // Reset batch
			}
		}
	}
}

func (p *HTTPBatchProcessor) sendBatch(batch []LogEntry) {
	if len(batch) == 0 {
		return
	}

	// Prepare request body
	var body []byte
	var err error

	switch p.config.Format {
	case "gelf":
		body, err = p.prepareGELFBatch(batch)
	default:
		body, err = json.Marshal(batch)
	}

	if err != nil {
		// Log error locally
		fmt.Printf("Failed to marshal batch: %v\n", err)
		return
	}

	// Send with retries
	for attempt := 0; attempt <= p.config.RetryCount; attempt++ {
		if err := p.sendRequest(body); err == nil {
			return // Success
		}

		if attempt < p.config.RetryCount {
			time.Sleep(p.config.RetryDelay)
		}
	}

	// All retries failed, log error locally
	fmt.Printf("Failed to send batch after %d attempts\n", p.config.RetryCount+1)
}

func (p *HTTPBatchProcessor) prepareGELFBatch(batch []LogEntry) ([]byte, error) {
	var gelfMessages []map[string]interface{}

	for _, entry := range batch {
		gelfMessage := map[string]interface{}{
			"short_message": entry.Message,
			"level_string":  entry.Level,
			"timestamp":     entry.Timestamp.Unix(),
			"facility":      p.config.Facility,
			"logger":        entry.Logger,
		}

		// Add custom fields
		for k, v := range entry.Fields {
			if k != "short_message" && k != "level_string" && k != "timestamp" && k != "facility" && k != "logger" {
				gelfMessage[k] = v
			}
		}

		gelfMessages = append(gelfMessages, gelfMessage)
	}

	return json.Marshal(gelfMessages)
}

func (p *HTTPBatchProcessor) sendRequest(body []byte) error {
	req, err := http.NewRequest(p.config.Method, p.config.Endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	// Add headers
	for k, v := range p.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (p *HTTPBatchProcessor) AddEntry(entry LogEntry) {
	if !p.closed {
		p.queue <- entry
	}
}

func (p *HTTPBatchProcessor) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.closed {
		close(p.queue)
		p.closed = true
	}
}

// JSONHTTPWriter formats output as JSON for HTTP
type JSONHTTPWriter struct {
	processor *HTTPBatchProcessor
}

func (w *JSONHTTPWriter) Write(p []byte) (n int, err error) {
	entry := LogEntry{
		Level:     "INFO",
		Message:   string(p),
		Timestamp: time.Now(),
		Fields:    map[string]interface{}{},
		Logger:    "http-adapter",
	}

	w.processor.AddEntry(entry)
	return len(p), nil
}

// GELFHTTPWriter formats output as GELF for HTTP
type GELFHTTPWriter struct {
	processor *HTTPBatchProcessor
	facility  string
}

func (g *GELFHTTPWriter) Write(p []byte) (n int, err error) {
	entry := LogEntry{
		Level:     "INFO",
		Message:   string(p),
		Timestamp: time.Now(),
		Fields:    map[string]interface{}{},
		Logger:    "http-adapter",
	}

	g.processor.AddEntry(entry)
	return len(p), nil
}

// mysticHTTP implements the Logger interface for HTTP logging
type mysticHTTP struct {
	logger         zerolog.Logger
	name           string
	fields         map[string]interface{}
	skip           int
	config         HTTPAdapterConfig
	batchProcessor *HTTPBatchProcessor
	ctx            context.Context
}

func (m *mysticHTTP) SetContext(ctx context.Context) Logger {
	if ctx != nil {
		m.ctx = ctx
	}
	return m
}

func (m *mysticHTTP) SkipLevel(skip int) Logger {
	m.skip = skip
	return m
}

func (m *mysticHTTP) With(args ...interface{}) Logger {
	kv := convertKeyValues(args)
	for k, v := range kv {
		m.fields[k] = v
	}
	return m
}

func (m *mysticHTTP) Debug(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.DebugLevel, msg, keysAndValues...)
}

func (m *mysticHTTP) Info(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.InfoLevel, msg, keysAndValues...)
}

func (m *mysticHTTP) Warn(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.WarnLevel, msg, keysAndValues...)
}

func (m *mysticHTTP) Error(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.ErrorLevel, msg, keysAndValues...)
}

func (m *mysticHTTP) ErrorDetail(err error, keysAndValues ...interface{}) {
	fields := keysAndValues
	if fields == nil {
		fields = make([]interface{}, 0)
	}

	// Add error details
	fields = append(fields, "error", err.Error())
	m.logWithLevel(zerolog.ErrorLevel, err.Error(), fields...)
}

func (m *mysticHTTP) Panic(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.PanicLevel, msg, keysAndValues...)
}

func (m *mysticHTTP) logWithLevel(level zerolog.Level, msg string, keysAndValues ...interface{}) {
	// Add caller
	caller := m.withStackTrace(3 + m.skip)
	kv := convertKeyValues(keysAndValues)
	kv["caller"] = caller

	// Include persistent fields
	for k, v := range m.fields {
		if _, exists := kv[k]; !exists {
			kv[k] = v
		}
	}

	// Create event
	event := m.logger.With().Fields(kv)

	// Log at appropriate level
	logger := event.Logger()
	switch level {
	case zerolog.DebugLevel:
		logger.Debug().Msg(msg)
	case zerolog.InfoLevel:
		logger.Info().Msg(msg)
	case zerolog.WarnLevel:
		logger.Warn().Msg(msg)
	case zerolog.ErrorLevel:
		logger.Error().Msg(msg)
	case zerolog.PanicLevel:
		logger.Panic().Msg(msg)
	}
}

func (m *mysticHTTP) withStackTrace(skip int) string {
	// Simple caller information
	return fmt.Sprintf("http_adapter.go:%d", skip)
}

// Helper function to parse log level
func parseHTTPLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}
