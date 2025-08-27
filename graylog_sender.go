package mystic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Graylog2/go-gelf/gelf"
)

// GraylogSender provides GELF transport to Graylog servers
type GraylogSender struct {
	*BaseSender
	writer      *gelf.Writer
	fallback    io.Writer
	endpoint    string
	timeout     int
	retryCount  int
	retryDelay  int
	batchSize   int
	batchDelay  int
	format      string
	compression bool
	tlsEnabled  bool
	rateLimit   int
	facility    string
}

// NewGraylogSender creates a new Graylog sender with fallback
func NewGraylogSender(config SenderConfig, facility string) *GraylogSender {
	gelfWriter, err := gelf.NewWriter(config.Endpoint)
	if err != nil {
		// Fallback to console if Graylog connection fails
		return &GraylogSender{
			BaseSender:  NewBaseSender(),
			writer:      nil,
			fallback:    os.Stdout,
			endpoint:    config.Endpoint,
			timeout:     config.Timeout,
			retryCount:  config.RetryCount,
			retryDelay:  config.RetryDelay,
			batchSize:   config.BatchSize,
			batchDelay:  config.BatchDelay,
			format:      config.Format,
			compression: config.Compression,
			tlsEnabled:  config.TLSEnabled,
			rateLimit:   config.RateLimit,
			facility:    facility,
		}
	}
	gelfWriter.Facility = facility

	return &GraylogSender{
		BaseSender:  NewBaseSender(),
		writer:      gelfWriter,
		fallback:    os.Stdout,
		endpoint:    config.Endpoint,
		timeout:     config.Timeout,
		retryCount:  config.RetryCount,
		retryDelay:  config.RetryDelay,
		batchSize:   config.BatchSize,
		batchDelay:  config.BatchDelay,
		format:      config.Format,
		compression: config.Compression,
		tlsEnabled:  config.TLSEnabled,
		rateLimit:   config.RateLimit,
		facility:    facility,
	}
}

// Write implements io.Writer interface for zap integration
func (g *GraylogSender) Write(p []byte) (n int, err error) {
	// Parse the log entry and send to Graylog
	// For now, we'll send as a generic log entry
	err = g.SendLegacy("INFO", string(p), map[string]interface{}{})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// SendLegacy sends a log entry to Graylog in GELF format (legacy method)
func (g *GraylogSender) SendLegacy(level string, msg string, fields map[string]interface{}) error {
	if g.writer != nil {
		// Send to Graylog
		gelfMessage := map[string]interface{}{
			"short_message": msg,
			"level_string":  level,
			"timestamp":     time.Now().Unix(),
			"facility":      g.facility,
		}

		// Add custom fields
		for k, v := range fields {
			if k != "short_message" && k != "level_string" && k != "timestamp" && k != "facility" {
				gelfMessage[k] = v
			}
		}

		data, err := json.Marshal(gelfMessage)
		if err != nil {
			return err
		}

		_, err = g.writer.Write(data)
		if err != nil {
			// If Graylog fails, fall back to console
			return g.sendToFallback(level, msg, fields)
		}
		return nil
	}

	// Fallback to console
	return g.sendToFallback(level, msg, fields)
}

// sendToFallback sends logs to fallback output (console)
func (g *GraylogSender) sendToFallback(level string, msg string, fields map[string]interface{}) error {
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     level,
		"message":   msg,
		"facility":  g.facility,
		"output":    "fallback",
	}

	// Add custom fields
	for k, v := range fields {
		if k != "timestamp" && k != "level" && k != "message" && k != "facility" && k != "output" {
			logEntry[k] = v
		}
	}

	data, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(g.fallback, string(data))
	return err
}

// Close closes the Graylog connection
func (g *GraylogSender) Close() error {
	if g.writer != nil {
		return g.writer.Close()
	}
	return g.BaseSender.Close()
}

// LogSender interface implementation

// Send implements LogSender.Send
func (g *GraylogSender) Send(entry LogEntry) error {
	start := time.Now()

	// Convert LogEntry to the format expected by existing Send method
	fields := make(map[string]interface{})
	for k, v := range entry.Fields {
		fields[k] = v
	}

	// Add logger name if present
	if entry.Logger != "" {
		fields["logger"] = entry.Logger
	}

	// Add timestamp if present
	if !entry.Timestamp.IsZero() {
		fields["timestamp"] = entry.Timestamp
	}

	err := g.SendLegacy(entry.Level, entry.Message, fields)

	// Update statistics
	latency := float64(time.Since(start).Milliseconds())
	g.UpdateStats(err == nil, latency)

	return err
}

// SendBatch implements LogSender.SendBatch
func (g *GraylogSender) SendBatch(entries []LogEntry) error {
	start := time.Now()
	var lastError error

	for _, entry := range entries {
		if err := g.Send(entry); err != nil {
			lastError = err
		}
	}

	// Update statistics for batch
	latency := float64(time.Since(start).Milliseconds())
	g.UpdateStats(lastError == nil, latency)

	return lastError
}

// SendAsync implements LogSender.SendAsync
func (g *GraylogSender) SendAsync(entry LogEntry) error {
	go func() {
		if err := g.Send(entry); err != nil {
			// Log error locally since this is async
			fmt.Printf("Async Graylog send failed: %v\n", err)
		}
	}()
	return nil
}

// SendWithContext implements LogSender.SendWithContext
func (g *GraylogSender) SendWithContext(ctx context.Context, entry LogEntry) error {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return g.Send(entry)
}

// IsConnected implements LogSender.IsConnected
func (g *GraylogSender) IsConnected() bool {
	return g.BaseSender.IsConnected() && g.writer != nil
}

// GetStats implements LogSender.GetStats
func (g *GraylogSender) GetStats() SenderStats {
	return g.BaseSender.GetStats()
}

// Configuration methods implementation
func (g *GraylogSender) GetEndpoint() string {
	return g.endpoint
}

func (g *GraylogSender) GetTimeout() int {
	return g.timeout
}

func (g *GraylogSender) GetRetryCount() int {
	return g.retryCount
}

func (g *GraylogSender) GetRetryDelay() int {
	return g.retryDelay
}

func (g *GraylogSender) GetBatchSize() int {
	return g.batchSize
}

func (g *GraylogSender) GetBatchDelay() int {
	return g.batchDelay
}

func (g *GraylogSender) GetFormat() string {
	return g.format
}

func (g *GraylogSender) IsCompressionEnabled() bool {
	return g.compression
}

func (g *GraylogSender) IsTLSEnabled() bool {
	return g.tlsEnabled
}

func (g *GraylogSender) GetRateLimit() int {
	return g.rateLimit
}

// GraylogSenderFactory implements SenderFactory for Graylog
type GraylogSenderFactory struct{}

// CreateSender creates a new Graylog sender
func (gsf *GraylogSenderFactory) CreateSender(config SenderConfig) (LogSender, error) {
	return NewGraylogSender(config, "mystic"), nil
}

// GetSupportedFormats returns the formats this factory supports
func (gsf *GraylogSenderFactory) GetSupportedFormats() []string {
	return []string{"gelf", "json"}
}

// ValidateConfig validates the configuration for Graylog sender
func (gsf *GraylogSenderFactory) ValidateConfig(config SenderConfig) error {
	if err := ValidateSenderConfig(config); err != nil {
		return err
	}

	// Validate Graylog-specific requirements
	if config.Format != "" && config.Format != "gelf" && config.Format != "json" {
		return fmt.Errorf("unsupported format for Graylog: %s", config.Format)
	}

	return nil
}
