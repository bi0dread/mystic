package mystic

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/bi0dread/morgana"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
)

type mysticZero struct {
	logger zerolog.Logger
	ctx    context.Context
	name   string
	skip   int
	fields map[string]interface{}
}

// ZerologAdapter creates a zerolog-backed adapter
func ZerologAdapter(named string) Logger {
	return ZerologAdapterWithConfig(named, GraylogSenderConfig{
		GrayLogAddr: GRAYLOG_ADDR,
		Facility:    FACILITY,
	})
}

// ZerologAdapterWithConfig creates a zerolog-backed adapter with custom Graylog configuration
func ZerologAdapterWithConfig(named string, graylogConfig GraylogSenderConfig) Logger {
	// Configure zerolog time format similar to ISO8601 used in zap config
	zerolog.TimeFieldFormat = time.RFC3339

	// Set global level from LOG_LEVEL if possible
	switch int8(LOG_LEVEL) {
	case -1: // default, keep
	case int8(zerolog.DebugLevel):
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case int8(zerolog.InfoLevel):
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case int8(zerolog.WarnLevel):
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case int8(zerolog.ErrorLevel):
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// Console writer
	console := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}

	// Create Graylog sender for transport using provided configuration
	graylogSender := NewGraylogSender(graylogConfig)

	// Combine console and Graylog outputs
	var out io.Writer
	if graylogSender != nil {
		out = io.MultiWriter(console, graylogSender)
	} else {
		out = console
	}

	base := zerolog.New(out).With().Timestamp().Str("logger", named).Logger()
	m := &mysticZero{logger: base, name: named, fields: map[string]interface{}{}, skip: 1}
	return m
}

func (m *mysticZero) SetContext(ctx context.Context) Logger {
	if ctx != nil {
		m.ctx = ctx
	}
	return m
}

func (m *mysticZero) SkipLevel(skip int) Logger {
	// store skip to adjust caller depth
	m.skip = skip
	return m
}

func (m *mysticZero) With(args ...interface{}) Logger {
	kv := convertKeyValues(args)
	for k, v := range kv {
		m.fields[k] = v
	}
	return m
}

func (m *mysticZero) Debug(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.DebugLevel, msg, keysAndValues...)
}

func (m *mysticZero) Info(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.InfoLevel, msg, keysAndValues...)
}

func (m *mysticZero) Warn(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.WarnLevel, msg, keysAndValues...)
}

func (m *mysticZero) Error(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.ErrorLevel, msg, keysAndValues...)
}

func (m *mysticZero) ErrorDetail(err error, keysAndValues ...interface{}) {
	fields := keysAndValues
	if fields == nil {
		fields = make([]interface{}, 0)
	}

	morgErr := morgana.GetMorgana(err)
	if morgErr != nil {
		fields = append(fields, "detail", morgErr.String())
	}
	m.logWithLevel(zerolog.ErrorLevel, err.Error(), fields...)
}

func (m *mysticZero) Panic(msg string, keysAndValues ...interface{}) {
	m.logWithLevel(zerolog.PanicLevel, msg, keysAndValues...)
}

func (m *mysticZero) logWithLevel(level zerolog.Level, msg string, keysAndValues ...interface{}) {
	// add caller
	caller := m.withStackTrace(3 + m.skip)
	kv := convertKeyValues(keysAndValues)
	kv["caller"] = caller

	// include persistent fields
	for k, v := range m.fields {
		if _, exists := kv[k]; !exists {
			kv[k] = v
		}
	}

	e := m.logger.With().Fields(kv).Logger()
	switch level {
	case zerolog.DebugLevel:
		e.Debug().Msg(msg)
	case zerolog.InfoLevel:
		e.Info().Msg(msg)
	case zerolog.WarnLevel:
		e.Warn().Msg(msg)
	case zerolog.ErrorLevel:
		e.Error().Msg(msg)
	case zerolog.PanicLevel:
		e.Panic().Msg(msg)
	default:
		e.Info().Msg(msg)
	}

	// OpenTelemetry event
	if m.ctx != nil {
		span := trace.SpanFromContext(m.ctx)
		if span != nil {
			attrs := make([]attribute.KeyValue, 0, len(kv)+3)
			for k, v := range kv {
				attrs = append(attrs, attribute.String(k, fmt.Sprintf("%v", v)))
			}
			attrs = append(attrs, attribute.String("LoggerName", m.name))
			attrs = append(attrs, attribute.String("LogLevel", level.String()))
			if c, ok := kv["caller"].(string); ok && c != "" {
				attrs = append(attrs, semconv.ExceptionType(c))
			}
			span.AddEvent(msg, trace.WithAttributes(attrs...))
		}
	}
}

func (m *mysticZero) withStackTrace(skip int) string {
	pc := make([]uintptr, 15)
	n := runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function)
}

func convertKeyValues(args []interface{}) map[string]interface{} {
	kv := make(map[string]interface{})
	if len(args)%2 != 0 {
		return kv
	}
	for i := 0; i < len(args); i += 2 {
		k, ok := args[i].(string)
		if !ok {
			continue
		}
		kv[k] = args[i+1]
	}
	return kv
}
