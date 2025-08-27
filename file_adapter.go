package mystic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// FileAdapterConfig holds configuration for file logging
type FileAdapterConfig struct {
	SenderConfig               // Embed the common sender configuration
	Path         string        `json:"path"`        // Directory path for log files
	Filename     string        `json:"filename"`    // Base filename (without extension)
	MaxSize      int64         `json:"max_size"`    // Maximum file size in bytes
	MaxAge       time.Duration `json:"max_age"`     // Maximum age of log files
	MaxBackups   int           `json:"max_backups"` // Maximum number of old log files to retain
	Compress     bool          `json:"compress"`    // Whether to compress rotated files
	Format       string        `json:"format"`      // Output format: "json", "console", "gelf"
	Level        string        `json:"level"`       // Log level for file output
	Facility     string        `json:"facility"`    // GELF facility name
}

// FileAdapter creates a file-backed logger adapter
func FileAdapter(named string) Logger {
	return FileAdapterWithConfig(named, FileAdapterConfig{
		SenderConfig: SenderConfig{
			Endpoint:   "./logs",
			Timeout:    5000,
			RetryCount: 3,
			RetryDelay: 1000,
			BatchSize:  100,
			BatchDelay: 1000,
			Format:     "json",
		},
		Path:       "./logs",
		Filename:   "application",
		MaxSize:    100 * 1024 * 1024,   // 100MB
		MaxAge:     30 * 24 * time.Hour, // 30 days
		MaxBackups: 5,
		Compress:   true,
		Format:     "json",
		Level:      "info",
	})
}

// FileAdapterWithConfig creates a file-backed logger adapter with custom configuration
func FileAdapterWithConfig(named string, config FileAdapterConfig) Logger {
	// Ensure log directory exists
	if err := os.MkdirAll(config.Path, 0755); err != nil {
		log.Error().Err(err).Str("path", config.Path).Msg("failed to create log directory")
		return nil
	}

	// Create file writer with rotation
	fileWriter := &RotatingFileWriter{
		config: config,
		mu:     &sync.Mutex{},
	}

	// Set up zerolog
	zerolog.TimeFieldFormat = time.RFC3339

	// Determine log level
	level := parseLogLevel(config.Level)
	zerolog.SetGlobalLevel(level)

	// Create output based on format
	var out io.Writer
	switch config.Format {
	case "console":
		out = zerolog.ConsoleWriter{Out: fileWriter, TimeFormat: time.RFC3339}
	case "gelf":
		// For GELF format, we'll use a custom writer
		out = &GELFFileWriter{writer: fileWriter, facility: config.Facility}
	default: // json
		out = fileWriter
	}

	base := zerolog.New(out).With().Timestamp().Str("logger", named).Logger()
	m := &mysticFile{
		logger: base,
		name:   named,
		fields: map[string]interface{}{},
		skip:   1,
		config: config,
		writer: fileWriter,
	}

	return m
}

// RotatingFileWriter handles file rotation and compression
type RotatingFileWriter struct {
	config      FileAdapterConfig
	file        *os.File
	currentSize int64
	mu          *sync.Mutex
}

func (w *RotatingFileWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if we need to rotate
	if w.file != nil && w.currentSize+int64(len(p)) > w.config.MaxSize {
		w.rotate()
	}

	// Open file if not open
	if w.file == nil {
		if err := w.openFile(); err != nil {
			return 0, err
		}
	}

	n, err = w.file.Write(p)
	if err == nil {
		w.currentSize += int64(n)
	}
	return n, err
}

func (w *RotatingFileWriter) openFile() error {
	filename := filepath.Join(w.config.Path, w.config.Filename+".log")
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}

	w.file = file
	w.currentSize = info.Size()
	return nil
}

func (w *RotatingFileWriter) rotate() error {
	if w.file == nil {
		return nil
	}

	// Close current file
	w.file.Close()
	w.file = nil

	// Rotate existing files
	w.rotateFiles()

	// Open new file
	return w.openFile()
}

func (w *RotatingFileWriter) rotateFiles() {
	basePath := filepath.Join(w.config.Path, w.config.Filename)

	// Remove old files beyond MaxBackups
	for i := w.config.MaxBackups; i >= 0; i-- {
		oldPath := fmt.Sprintf("%s.%d.log", basePath, i)
		if i == 0 {
			oldPath = basePath + ".log"
		}

		if _, err := os.Stat(oldPath); err == nil {
			if i >= w.config.MaxBackups {
				os.Remove(oldPath)
			} else {
				newPath := fmt.Sprintf("%s.%d.log", basePath, i+1)
				os.Rename(oldPath, newPath)
			}
		}
	}
}

func (w *RotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// GELFFileWriter formats output as GELF
type GELFFileWriter struct {
	writer   io.Writer
	facility string
}

func (g *GELFFileWriter) Write(p []byte) (n int, err error) {
	// Convert zerolog output to GELF format
	gelfMessage := map[string]interface{}{
		"short_message": string(p),
		"timestamp":     time.Now().Unix(),
		"facility":      g.facility,
		"level":         1, // INFO level
	}

	// Write as JSON
	data := fmt.Sprintf("%s\n", gelfMessage)
	return g.writer.Write([]byte(data))
}

// mysticFile implements the Logger interface for file logging
type mysticFile struct {
	logger zerolog.Logger
	name   string
	fields map[string]interface{}
	skip   int
	config FileAdapterConfig
	writer *RotatingFileWriter
	ctx    context.Context
}

func (m *mysticFile) SetContext(ctx context.Context) Logger {
	if ctx != nil {
		m.ctx = ctx
	}
	return m
}

func (m *mysticFile) SkipLevel(skip int) Logger {
	m.skip = skip
	return m
}

func (m *mysticFile) With(args ...interface{}) Logger {
	kv := convertKeyValues(args)
	for k, v := range kv {
		m.fields[k] = v
	}
	return m
}

func (m *mysticFile) Debug(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.DebugLevel, msg, keysAndValues...)
}

func (m *mysticFile) Info(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.InfoLevel, msg, keysAndValues...)
}

func (m *mysticFile) Warn(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.WarnLevel, msg, keysAndValues...)
}

func (m *mysticFile) Error(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.ErrorLevel, msg, keysAndValues...)
}

func (m *mysticFile) ErrorDetail(err error, keysAndValues ...interface{}) {
	fields := keysAndValues
	if fields == nil {
		fields = make([]interface{}, 0)
	}

	// Add error details
	fields = append(fields, "error", err.Error())
	m.logWithLevel(zerolog.ErrorLevel, err.Error(), fields...)
}

func (m *mysticFile) Panic(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.PanicLevel, msg, keysAndValues...)
}

func (m *mysticFile) logWithLevel(level zerolog.Level, msg string, keysAndValues ...interface{}) {
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

func (m *mysticFile) withStackTrace(skip int) string {
	// Simple caller information
	return fmt.Sprintf("file_adapter.go:%d", skip)
}

// Helper function to parse log level
func parseLogLevel(level string) zerolog.Level {
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

// FileSender implements LogSender interface for file transport
type FileSender struct {
	*BaseSender
	config FileAdapterConfig
	writer *RotatingFileWriter
	mu     *sync.Mutex
	// Configuration fields extracted from SenderConfig
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
}

// NewFileSender creates a new file sender
func NewFileSender(config FileAdapterConfig) (*FileSender, error) {
	if err := ValidateSenderConfig(config.SenderConfig); err != nil {
		return nil, fmt.Errorf("invalid sender config: %w", err)
	}

	// Ensure log directory exists
	if err := os.MkdirAll(config.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create file writer with rotation
	fileWriter := &RotatingFileWriter{
		config: config,
		mu:     &sync.Mutex{},
	}

	return &FileSender{
		BaseSender: NewBaseSender(),
		config:     config,
		writer:     fileWriter,
		mu:         &sync.Mutex{},
		// Extract configuration from SenderConfig
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
	}, nil
}

// Send implements LogSender.Send
func (f *FileSender) Send(entry LogEntry) error {
	if !f.IsConnected() {
		return ErrNotConnected
	}

	start := time.Now()
	f.mu.Lock()
	defer f.mu.Unlock()

	// Convert LogEntry to appropriate format
	var data []byte
	var err error

	switch f.config.Format {
	case "json":
		data, err = json.Marshal(entry)
	case "gelf":
		data, err = f.prepareGELF(entry)
	default:
		data, err = json.Marshal(entry)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Write to file
	_, err = f.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	// Update statistics
	latency := float64(time.Since(start).Milliseconds())
	f.UpdateStats(true, latency)

	return nil
}

// SendBatch implements LogSender.SendBatch
func (f *FileSender) SendBatch(entries []LogEntry) error {
	if !f.IsConnected() {
		return ErrNotConnected
	}

	start := time.Now()
	var lastError error

	for _, entry := range entries {
		if err := f.Send(entry); err != nil {
			lastError = err
		}
	}

	// Update statistics for batch
	latency := float64(time.Since(start).Milliseconds())
	f.UpdateStats(lastError == nil, latency)

	return lastError
}

// SendAsync implements LogSender.SendAsync
func (f *FileSender) SendAsync(entry LogEntry) error {
	go func() {
		if err := f.Send(entry); err != nil {
			fmt.Printf("Async file send failed: %v\n", err)
		}
	}()
	return nil
}

// SendWithContext implements LogSender.SendWithContext
func (f *FileSender) SendWithContext(ctx context.Context, entry LogEntry) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return f.Send(entry)
}

// Close implements LogSender.Close
func (f *FileSender) Close() error {
	if f.writer != nil && f.writer.file != nil {
		return f.writer.file.Close()
	}
	return f.BaseSender.Close()
}

// IsConnected implements LogSender.IsConnected
func (f *FileSender) IsConnected() bool {
	return f.BaseSender.IsConnected() && f.writer != nil
}

// GetStats implements LogSender.GetStats
func (f *FileSender) GetStats() SenderStats {
	return f.BaseSender.GetStats()
}

// Configuration methods implementation
func (f *FileSender) GetEndpoint() string {
	return f.endpoint
}

func (f *FileSender) GetTimeout() int {
	return f.timeout
}

func (f *FileSender) GetRetryCount() int {
	return f.retryCount
}

func (f *FileSender) GetRetryDelay() int {
	return f.retryDelay
}

func (f *FileSender) GetBatchSize() int {
	return f.batchSize
}

func (f *FileSender) GetBatchDelay() int {
	return f.batchDelay
}

func (f *FileSender) GetFormat() string {
	return f.format
}

func (f *FileSender) IsCompressionEnabled() bool {
	return f.compression
}

func (f *FileSender) IsTLSEnabled() bool {
	return f.tlsEnabled
}

func (f *FileSender) GetRateLimit() int {
	return f.rateLimit
}

// prepareGELF converts LogEntry to GELF format
func (f *FileSender) prepareGELF(entry LogEntry) ([]byte, error) {
	gelfMessage := map[string]interface{}{
		"short_message": entry.Message,
		"level_string":  entry.Level,
		"timestamp":     entry.Timestamp.Unix(),
		"facility":      f.config.Facility,
		"logger":        entry.Logger,
	}

	// Add custom fields
	for k, v := range entry.Fields {
		if k != "short_message" && k != "level_string" && k != "timestamp" && k != "facility" && k != "logger" {
			gelfMessage[k] = v
		}
	}

	return json.Marshal(gelfMessage)
}

// FileSenderFactory implements SenderFactory for file transport
type FileSenderFactory struct{}

// CreateSender creates a new file sender
func (fsf *FileSenderFactory) CreateSender(config SenderConfig) (LogSender, error) {
	fileConfig := FileAdapterConfig{
		SenderConfig: config,
		Path:         config.Endpoint,
		Filename:     "application",
		MaxSize:      100 * 1024 * 1024,   // 100MB
		MaxAge:       30 * 24 * time.Hour, // 30 days
		MaxBackups:   5,
		Compress:     config.Compression,
		Format:       config.Format,
		Level:        "info",
		Facility:     "mystic",
	}

	return NewFileSender(fileConfig)
}

// GetSupportedFormats returns the formats this factory supports
func (fsf *FileSenderFactory) GetSupportedFormats() []string {
	return []string{"json", "console", "gelf"}
}

// ValidateConfig validates the configuration for file sender
func (fsf *FileSenderFactory) ValidateConfig(config SenderConfig) error {
	if err := ValidateSenderConfig(config); err != nil {
		return err
	}

	// Validate file-specific requirements
	if config.Endpoint == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	if config.Format != "" && config.Format != "json" && config.Format != "console" && config.Format != "gelf" {
		return fmt.Errorf("unsupported format for file: %s", config.Format)
	}

	return nil
}
