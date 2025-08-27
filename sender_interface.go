package mystic

import (
	"context"
)

// LogSender defines the interface for sending logs to various destinations
type LogSender interface {
	// Send sends a log entry to the destination
	Send(entry LogEntry) error

	// SendBatch sends multiple log entries in a batch
	SendBatch(entries []LogEntry) error

	// SendAsync sends a log entry asynchronously
	SendAsync(entry LogEntry) error

	// SendWithContext sends a log entry with context
	SendWithContext(ctx context.Context, entry LogEntry) error

	// Close closes the sender and releases resources
	Close() error

	// IsConnected checks if the sender is connected to the destination
	IsConnected() bool

	// GetStats returns statistics about the sender
	GetStats() SenderStats

	// Configuration methods
	GetEndpoint() string
	GetTimeout() int
	GetRetryCount() int
	GetRetryDelay() int
	GetBatchSize() int
	GetBatchDelay() int
	GetFormat() string
	IsCompressionEnabled() bool
	IsTLSEnabled() bool
	GetRateLimit() int
}

// Note: LogEntry is already defined in http_adapter.go
// This interface uses the existing LogEntry type

// SenderStats contains statistics about the sender
type SenderStats struct {
	TotalSent      int64   `json:"total_sent"`
	TotalFailed    int64   `json:"total_failed"`
	LastSent       int64   `json:"last_sent"`
	LastFailed     int64   `json:"last_failed"`
	SuccessRate    float64 `json:"success_rate"`
	AverageLatency float64 `json:"average_latency"`
}

// SenderConfig holds common configuration for all senders
type SenderConfig struct {
	// Connection settings
	Endpoint   string `json:"endpoint"`
	Timeout    int    `json:"timeout"` // in milliseconds
	RetryCount int    `json:"retry_count"`
	RetryDelay int    `json:"retry_delay"` // in milliseconds

	// Authentication
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	APIKey   string `json:"api_key,omitempty"`

	// Headers for HTTP-based senders
	Headers map[string]string `json:"headers,omitempty"`

	// Batching settings
	BatchSize  int `json:"batch_size"`
	BatchDelay int `json:"batch_delay"` // in milliseconds

	// Format settings
	Format      string `json:"format"` // json, gelf, etc.
	Compression bool   `json:"compression"`

	// TLS settings
	TLSEnabled bool   `json:"tls_enabled"`
	TLSCert    string `json:"tls_cert,omitempty"`
	TLSKey     string `json:"tls_key,omitempty"`
	TLSCA      string `json:"tls_ca,omitempty"`

	// Rate limiting
	RateLimit int `json:"rate_limit"` // logs per second
}

// SenderFactory creates new senders based on configuration
type SenderFactory interface {
	// CreateSender creates a new sender with the given configuration
	CreateSender(config SenderConfig) (LogSender, error)

	// GetSupportedFormats returns the formats this factory supports
	GetSupportedFormats() []string

	// ValidateConfig validates the configuration for this sender type
	ValidateConfig(config SenderConfig) error
}

// BaseSender provides common functionality for all senders
type BaseSender struct {
	stats  SenderStats
	closed bool
}

// NewBaseSender creates a new base sender
func NewBaseSender() *BaseSender {
	return &BaseSender{
		stats:  SenderStats{},
		closed: false,
	}
}

// GetStats returns the current statistics
func (bs *BaseSender) GetStats() SenderStats {
	return bs.stats
}

// IsConnected returns true if the sender is not closed
func (bs *BaseSender) IsConnected() bool {
	return !bs.closed
}

// Close marks the sender as closed
func (bs *BaseSender) Close() error {
	bs.closed = true
	return nil
}

// UpdateStats updates the statistics
func (bs *BaseSender) UpdateStats(success bool, latency float64) {
	if success {
		bs.stats.TotalSent++
		bs.stats.LastSent = bs.stats.TotalSent
		// Update average latency
		if bs.stats.TotalSent == 1 {
			bs.stats.AverageLatency = latency
		} else {
			bs.stats.AverageLatency = (bs.stats.AverageLatency*float64(bs.stats.TotalSent-1) + latency) / float64(bs.stats.TotalSent)
		}
	} else {
		bs.stats.TotalFailed++
		bs.stats.LastFailed = bs.stats.TotalFailed
	}

	// Calculate success rate
	total := bs.stats.TotalSent + bs.stats.TotalFailed
	if total > 0 {
		bs.stats.SuccessRate = float64(bs.stats.TotalSent) / float64(total)
	}
}

// ValidateConfig validates common configuration settings
func ValidateSenderConfig(config SenderConfig) error {
	if config.Endpoint == "" {
		return ErrInvalidEndpoint
	}

	if config.Timeout <= 0 {
		config.Timeout = 5000 // Default 5 seconds
	}

	if config.RetryCount < 0 {
		config.RetryCount = 3 // Default 3 retries
	}

	if config.RetryDelay < 0 {
		config.RetryDelay = 1000 // Default 1 second
	}

	if config.BatchSize <= 0 {
		config.BatchSize = 100 // Default batch size
	}

	if config.BatchDelay < 0 {
		config.BatchDelay = 1000 // Default 1 second
	}

	if config.RateLimit < 0 {
		config.RateLimit = 1000 // Default 1000 logs per second
	}

	return nil
}

// Common errors for senders
var (
	ErrInvalidEndpoint = &SenderError{Message: "invalid endpoint"}
	ErrNotConnected    = &SenderError{Message: "sender not connected"}
	ErrSendFailed      = &SenderError{Message: "failed to send log entry"}
	ErrInvalidFormat   = &SenderError{Message: "invalid log format"}
	ErrConfigInvalid   = &SenderError{Message: "invalid configuration"}
)

// SenderError represents an error from a sender
type SenderError struct {
	Message string
	Cause   error
}

func (e *SenderError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *SenderError) Unwrap() error {
	return e.Cause
}

// MultiSender sends logs to multiple destinations
type MultiSender struct {
	senders []LogSender
	config  SenderConfig
}

// NewMultiSender creates a new multi-sender
func NewMultiSender(senders []LogSender, config SenderConfig) *MultiSender {
	return &MultiSender{
		senders: senders,
		config:  config,
	}
}

// Send sends to all senders
func (ms *MultiSender) Send(entry LogEntry) error {
	var lastError error
	for _, sender := range ms.senders {
		if err := sender.Send(entry); err != nil {
			lastError = err
		}
	}
	return lastError
}

// SendBatch sends batch to all senders
func (ms *MultiSender) SendBatch(entries []LogEntry) error {
	var lastError error
	for _, sender := range ms.senders {
		if err := sender.SendBatch(entries); err != nil {
			lastError = err
		}
	}
	return lastError
}

// SendAsync sends asynchronously to all senders
func (ms *MultiSender) SendAsync(entry LogEntry) error {
	for _, sender := range ms.senders {
		go sender.SendAsync(entry)
	}
	return nil
}

// SendWithContext sends with context to all senders
func (ms *MultiSender) SendWithContext(ctx context.Context, entry LogEntry) error {
	var lastError error
	for _, sender := range ms.senders {
		if err := sender.SendWithContext(ctx, entry); err != nil {
			lastError = err
		}
	}
	return lastError
}

// Close closes all senders
func (ms *MultiSender) Close() error {
	var lastError error
	for _, sender := range ms.senders {
		if err := sender.Close(); err != nil {
			lastError = err
		}
	}
	return lastError
}

// IsConnected checks if any sender is connected
func (ms *MultiSender) IsConnected() bool {
	for _, sender := range ms.senders {
		if sender.IsConnected() {
			return true
		}
	}
	return false
}

// GetStats returns combined stats from all senders
func (ms *MultiSender) GetStats() SenderStats {
	var combined SenderStats
	for _, sender := range ms.senders {
		stats := sender.GetStats()
		combined.TotalSent += stats.TotalSent
		combined.TotalFailed += stats.TotalFailed
		combined.AverageLatency += stats.AverageLatency
	}

	if len(ms.senders) > 0 {
		combined.AverageLatency /= float64(len(ms.senders))
	}

	total := combined.TotalSent + combined.TotalFailed
	if total > 0 {
		combined.SuccessRate = float64(combined.TotalSent) / float64(total)
	}

	return combined
}
