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

// Adapter represents a constructor function that returns a concrete Logger.
type Adapter func(named string) Logger

// New constructs a Logger using the provided adapter constructor.
func New(adapter Adapter, named string) Logger {
	if adapter == nil {
		return nil
	}
	return adapter(named)
}

func SetConfig(config Config) {
	FACILITY = config.Facility
	GRAYLOG_ADDR = config.GrayLogAddr
	setLogLevel(config.LogLevel)
}
