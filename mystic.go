package mystic

import (
	"context"
)

var LOG_LEVEL = int8(-1)
var GRAYLOG_ADDR = "0.0.0.0:12201"
var FACILITY = "not set the facility!!!"

const mysticSpanContext = "mysticSpanContext"

// Logger is the abstract logging interface implemented by adapters
// like zap, zerolog, etc.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	ErrorDetail(err error, keysAndValues ...interface{})
	Panic(msg string, keysAndValues ...interface{})

	SkipLevel(skip int) Logger
	With(args ...interface{}) Logger
	SetContext(ctx context.Context) Logger
}

// mysticLogger wraps an underlying Logger and implements the Logger interface
type mysticLogger struct {
	adapter Logger
	name    string
}

// New constructs a mystic Logger that wraps the provided logger instance.
func New(logger Logger) Logger {
	if logger == nil {
		return nil
	}
	return &mysticLogger{
		adapter: logger,
		name:    "mystic",
	}
}

// Debug delegates to the underlying adapter
func (m *mysticLogger) Debug(msg string, keysAndValues ...interface{}) {
	m.adapter.Debug(msg, keysAndValues...)
}

// Info delegates to the underlying adapter
func (m *mysticLogger) Info(msg string, keysAndValues ...interface{}) {
	m.adapter.Info(msg, keysAndValues...)
}

// Warn delegates to the underlying adapter
func (m *mysticLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.adapter.Warn(msg, keysAndValues...)
}

// Error delegates to the underlying adapter
func (m *mysticLogger) Error(msg string, keysAndValues ...interface{}) {
	m.adapter.Error(msg, keysAndValues...)
}

// ErrorDetail delegates to the underlying adapter
func (m *mysticLogger) ErrorDetail(err error, keysAndValues ...interface{}) {
	m.adapter.ErrorDetail(err, keysAndValues...)
}

// Panic delegates to the underlying adapter
func (m *mysticLogger) Panic(msg string, keysAndValues ...interface{}) {
	m.adapter.Panic(msg, keysAndValues...)
}

// SkipLevel delegates to the underlying adapter
func (m *mysticLogger) SkipLevel(skip int) Logger {
	return &mysticLogger{
		adapter: m.adapter.SkipLevel(skip),
		name:    m.name,
	}
}

// With delegates to the underlying adapter
func (m *mysticLogger) With(args ...interface{}) Logger {
	return &mysticLogger{
		adapter: m.adapter.With(args...),
		name:    m.name,
	}
}

// SetContext delegates to the underlying adapter
func (m *mysticLogger) SetContext(ctx context.Context) Logger {
	return &mysticLogger{
		adapter: m.adapter.SetContext(ctx),
		name:    m.name,
	}
}

func SetConfig(config Config) {
	FACILITY = config.Facility
	setLogLevel(config.LogLevel)
}
