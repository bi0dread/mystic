package mystic

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"runtime"

	"github.com/bi0dread/morgana"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

type mysticZap struct {
	logger        *zap.Logger
	sugaredLogger *zap.SugaredLogger
	ctx           context.Context
}

// ZapAdapter creates a zap-backed logger adapter
func ZapAdapter(named string) Logger {
	return ZapAdapterWithConfig(named, nil)
}

// ZapAdapterWithSender creates a zap-backed logger adapter with optional sender
func ZapAdapterWithSender(named string, sender LogSender) Logger {
	return ZapAdapterWithConfig(named, sender)
}

// ZapAdapterWithConfig creates a zap-backed logger adapter with optional sender
func ZapAdapterWithConfig(named string, sender LogSender) Logger {
	atomLevel := zap.NewAtomicLevelAt(zapcore.Level(LOG_LEVEL))

	config := zap.NewDevelopmentConfig()
	config.Encoding = "console"
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.Level = atomLevel

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	telemetryCore := NewTelemetryCore(atomLevel, zapcore.NewConsoleEncoder(config.EncoderConfig))

	var cores []zapcore.Core
	cores = append(cores, telemetryCore)

	// Add GELF core only if sender is provided
	if sender != nil {
		// Create GELF core for sender transport using a writer adapter
		senderWriter := &LogSenderWriter{sender: sender}
		gelfCore := NewGelfCore(zapcore.AddSync(senderWriter))
		cores = append(cores, gelfCore)
	}

	// Combine cores
	var zapTee zapcore.Core
	if len(cores) == 1 {
		zapTee = cores[0]
	} else {
		zapTee = zapcore.NewTee(cores...)
	}
	logger = zap.New(zapTee)

	logger = logger.WithOptions(zap.AddCallerSkip(2)) // Skip 2 frames to show test file instead of wrapper
	logger = logger.WithOptions(zap.AddCaller())      // Add this to show caller information
	logger = logger.Named(named)
	sugar := logger.Sugar()

	m := mysticZap{
		logger:        logger,
		sugaredLogger: sugar,
	}

	return &m
}

func setLogLevel(level string) {
	switch level {
	case "debug":
		LOG_LEVEL = int8(zap.DebugLevel)
	case "info":
		LOG_LEVEL = int8(zap.InfoLevel)
	case "warn":
		LOG_LEVEL = int8(zap.WarnLevel)
	case "error":
		LOG_LEVEL = int8(zap.ErrorLevel)
	default:
		LOG_LEVEL = int8(zap.DebugLevel)
	}
}

func (m *mysticZap) SetContext(ctx context.Context) Logger {
	if ctx != nil {
		m.ctx = ctx
		m.sugaredLogger = m.sugaredLogger.With(zap.Reflect(mysticSpanContext, ctx))
	}
	return m
}

func (m *mysticZap) SkipLevel(skip int) Logger {
	m.logger = m.logger.WithOptions(zap.AddCallerSkip(skip))
	m.sugaredLogger = m.logger.Sugar()
	return m
}

func (m *mysticZap) Debug(msg string, keysAndValues ...interface{}) {
	if keysAndValues == nil {
		keysAndValues = make([]interface{}, 0)
	}
	// Let Zap handle caller information with AddCallerSkip
	m.sugaredLogger.Debugw(msg, keysAndValues...)
}

func (m *mysticZap) Info(msg string, keysAndValues ...interface{}) {
	if keysAndValues == nil {
		keysAndValues = make([]interface{}, 0)
	}
	// Let Zap handle caller information with AddCallerSkip
	m.sugaredLogger.Infow(msg, keysAndValues...)
}

func (m *mysticZap) Warn(msg string, keysAndValues ...interface{}) {
	if keysAndValues == nil {
		keysAndValues = make([]interface{}, 0)
	}
	// Let Zap handle caller information with AddCallerSkip
	m.sugaredLogger.Warnw(msg, keysAndValues...)
}

func (m *mysticZap) Error(msg string, keysAndValues ...interface{}) {
	if keysAndValues == nil {
		keysAndValues = make([]interface{}, 0)
	}
	// Let Zap handle caller information with AddCallerSkip
	m.sugaredLogger.Errorw(msg, keysAndValues...)
}

func (m *mysticZap) ErrorDetail(err error, keysAndValues ...interface{}) {
	if keysAndValues == nil {
		keysAndValues = make([]interface{}, 0)
	}
	// Let Zap handle caller information with AddCallerSkip

	shortMsg := ""
	fullMsg := ""

	morgErr := morgana.GetMorgana(err)
	if morgErr != nil {
		fullMsg = morgErr.String()
		shortMsg = err.Error()
		keysAndValues = append(keysAndValues, zap.String("detail", fullMsg))
	} else {
		shortMsg = err.Error()
	}

	m.sugaredLogger.Errorw(shortMsg, keysAndValues...)
}

func (m *mysticZap) Panic(msg string, keysAndValues ...interface{}) {
	if keysAndValues == nil {
		keysAndValues = make([]interface{}, 0)
	}
	// Let Zap handle caller information with AddCallerSkip
	m.sugaredLogger.Panicw(msg, keysAndValues...)
}

func (m *mysticZap) With(args ...interface{}) Logger {
	m.sugaredLogger = m.sugaredLogger.With(args...)
	return m
}

func (m *mysticZap) withStackTrace(skip int) string {
	pc := make([]uintptr, 15)
	n := runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	ss := fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function)
	return ss
}

// ===== zap-specific GELF core and encoder =====

type GELFCore struct {
	encoder zapcore.Encoder
	out     zapcore.WriteSyncer
	fields  []zapcore.Field
}

func (*GELFCore) Enabled(zapcore.Level) bool {
	return true
}

func (c *GELFCore) With(fields []zapcore.Field) zapcore.Core {

	clone := &GELFEncoder{}

	return &GELFCore{
		fields:  fields,
		encoder: clone,
		out:     c.out,
	}
}

func (c *GELFCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {

	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *GELFCore) Sync() error { return nil }

func (c *GELFCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	buf, err := c.encoder.EncodeEntry(entry, fields)
	if err != nil {
		return err
	}
	_, err = c.out.Write(buf.Bytes())
	return err
}

func NewGelfCore(out zapcore.WriteSyncer) zapcore.Core {

	return &GELFCore{out: out, encoder: &GELFEncoder{}}
}

type GELFEncoder struct {
	zapcore.Encoder
}

func (e *GELFEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	gelfMessage := map[string]interface{}{
		"short_message": entry.Message,
		"level_string":  entry.Level.String(),
	}

	for _, field := range fields {

		si := ""
		if field.Interface != nil {
			si = fmt.Sprintf("%+#v\n", field.Interface)
		}

		gelfMessage[field.Key] = field.String + fmt.Sprint(field.Integer) + si
	}

	buf, err := json.Marshal(gelfMessage)
	if err != nil {
		return nil, err
	}

	bufferValue := buffer.NewPool().Get()
	_, bufferWriteErr := bufferValue.Write(buf)
	if bufferWriteErr != nil {
		return nil, bufferWriteErr
	}
	return bufferValue, nil
}

// ===== zap-specific telemetry core =====

type TelemetryCore struct {
	fields  []zapcore.Field
	level   zap.AtomicLevel
	ctx     context.Context
	encoder zapcore.Encoder
}

func (*TelemetryCore) Enabled(zapcore.Level) bool {
	return true
}

func (c *TelemetryCore) With(f []zapcore.Field) zapcore.Core {

	clone := c.encoder.Clone()

	fields := c.fields

	for _, s := range f {
		if s.Key == mysticSpanContext {
			if ctxValue, ok := s.Interface.(context.Context); ok {
				c.ctx = ctxValue
			}
		} else {
			s.AddTo(clone)
			fields = append(fields, s)
		}
	}

	return &TelemetryCore{
		fields:  fields,
		level:   c.level,
		encoder: clone,
		ctx:     c.ctx,
	}
}

func (c *TelemetryCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *TelemetryCore) Sync() error {
	return nil
}

func (c *TelemetryCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {

	if ent.Level >= c.level.Level() {
		var attributes []attribute.KeyValue

		serialized, err := c.encoder.EncodeEntry(ent, fields)
		if err != nil {
			return err
		}

		fmt.Print(serialized)

		for _, s := range c.fields {
			attributes = append(attributes, convertZapFieldToKeyValue(s)...)

		}

		attributes = append(attributes, attribute.KeyValue{
			Key:   "LogLevel",
			Value: attribute.StringValue(convertZapLogLevelToString(ent.Level)),
		})

		attributes = append(attributes, attribute.KeyValue{
			Key:   "LoggerName",
			Value: attribute.StringValue(ent.LoggerName),
		})

		for _, s := range fields {

			attributes = append(attributes, convertZapFieldToKeyValue(s)...)
		}

		callerString := ent.Caller.String()

		if len(callerString) > 0 {
			attributes = append(attributes, semconv.ExceptionType(callerString))
		}

		if len(ent.Stack) > 0 {
			attributes = append(attributes, semconv.ExceptionStacktrace(ent.Stack))
		}

		if c.ctx != nil {

			span := trace.SpanFromContext(c.ctx)
			if span != nil {
				span.AddEvent(ent.Message, trace.WithAttributes(attributes...))
			}
		}
	}

	return nil
}

func convertZapFieldToKeyValue(f zapcore.Field) []attribute.KeyValue {
	switch f.Type {
	case zapcore.UnknownType:
		return []attribute.KeyValue{attribute.String(f.Key, f.String)}
	case zapcore.BoolType:
		return []attribute.KeyValue{attribute.Bool(f.Key, f.Integer == 1)}
	case zapcore.Float64Type:
		return []attribute.KeyValue{attribute.Float64(f.Key, math.Float64frombits(uint64(f.Integer)))}
	case zapcore.Float32Type:
		return []attribute.KeyValue{attribute.Float64(f.Key, math.Float64frombits(uint64(f.Integer)))}
	case zapcore.Int64Type:
		return []attribute.KeyValue{attribute.Int64(f.Key, f.Integer)}
	case zapcore.Int32Type:
		return []attribute.KeyValue{attribute.Int64(f.Key, f.Integer)}
	case zapcore.Int16Type:
		return []attribute.KeyValue{attribute.Int64(f.Key, f.Integer)}
	case zapcore.Int8Type:
		return []attribute.KeyValue{attribute.Int64(f.Key, f.Integer)}
	case zapcore.StringType:
		return []attribute.KeyValue{attribute.String(f.Key, f.String)}
	case zapcore.Uint64Type:
		return []attribute.KeyValue{attribute.Int64(f.Key, int64(uint64(f.Integer)))}
	case zapcore.Uint32Type:
		return []attribute.KeyValue{attribute.Int64(f.Key, int64(uint64(f.Integer)))}
	case zapcore.Uint16Type:
		return []attribute.KeyValue{attribute.Int64(f.Key, int64(uint64(f.Integer)))}
	case zapcore.Uint8Type:
		return []attribute.KeyValue{attribute.Int64(f.Key, int64(uint64(f.Integer)))}
	case zapcore.ErrorType:
		err := f.Interface.(error)
		if err != nil {
			return []attribute.KeyValue{semconv.ExceptionMessage(err.Error())}
		}
		return []attribute.KeyValue{}
	case zapcore.SkipType:
		return []attribute.KeyValue{}
	}
	return []attribute.KeyValue{attribute.String(f.Key, f.String)}
}

func convertZapLogLevelToString(level zapcore.Level) string {
	switch level {
	case zapcore.DebugLevel:
		return "Debug"
	case zapcore.InfoLevel:
		return "Info"
	case zapcore.WarnLevel:
		return "Warn"
	case zapcore.ErrorLevel:
		return "Error"
	case zapcore.DPanicLevel:
		return "DPanic"
	case zapcore.PanicLevel:
		return "Panic"
	case zapcore.FatalLevel:
		return "Fetal"
	default:
		return "Trace"
	}
}

func NewTelemetryCore(level zap.AtomicLevel, encoder zapcore.Encoder) zapcore.Core {

	return &TelemetryCore{level: level, encoder: encoder}
}
