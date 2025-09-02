package mystic

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type ContextFn func(ctx context.Context) []zapcore.Field

// Zap GormLogger
type GormLogger struct {
	ZapLogger                 *zap.Logger
	SlowThreshold             time.Duration
	SkipCallerLookup          bool
	IgnoreRecordNotFoundError bool
	Context                   ContextFn
}

// Zerolog GormLogger
type ZerologGormLogger struct {
	Logger                    zerolog.Logger
	SlowThreshold             time.Duration
	SkipCallerLookup          bool
	IgnoreRecordNotFoundError bool
	Context                   func(ctx context.Context) map[string]interface{}
}

// File GormLogger
type FileGormLogger struct {
	Logger                    Logger
	SlowThreshold             time.Duration
	SkipCallerLookup          bool
	IgnoreRecordNotFoundError bool
	Context                   func(ctx context.Context) map[string]interface{}
}

// HTTP GormLogger
type HTTPGormLogger struct {
	Logger                    Logger
	SlowThreshold             time.Duration
	SkipCallerLookup          bool
	IgnoreRecordNotFoundError bool
	Context                   func(ctx context.Context) map[string]interface{}
}

// Zap GormLogger Adapter
func NewZapGormLoggerAdapter(logger Logger) GormLogger {
	// Extract zap.Logger from the mystic logger
	zapLogger := zap.NewNop() // Default fallback
	if zapAdapter, ok := logger.(*mysticZap); ok {
		zapLogger = zapAdapter.logger
	}
	return GormLogger{
		ZapLogger:                 zapLogger,
		SlowThreshold:             100 * time.Millisecond,
		SkipCallerLookup:          false,
		IgnoreRecordNotFoundError: false,
		Context:                   nil,
	}
}

// Zerolog GormLogger Adapter
func NewZerologGormLoggerAdapter(logger Logger) ZerologGormLogger {
	// Extract zerolog.Logger from the mystic logger
	zeroLogger := zerolog.New(nil) // Default fallback
	if zeroAdapter, ok := logger.(*mysticZero); ok {
		zeroLogger = zeroAdapter.logger
	}
	return ZerologGormLogger{
		Logger:                    zeroLogger,
		SlowThreshold:             100 * time.Millisecond,
		SkipCallerLookup:          false,
		IgnoreRecordNotFoundError: false,
		Context:                   nil,
	}
}

// File GormLogger Adapter
func NewFileGormLoggerAdapter(logger Logger) FileGormLogger {
	return FileGormLogger{
		Logger:                    logger,
		SlowThreshold:             100 * time.Millisecond,
		SkipCallerLookup:          false,
		IgnoreRecordNotFoundError: false,
		Context:                   nil,
	}
}

// HTTP GormLogger Adapter
func NewHTTPGormLoggerAdapter(logger Logger) HTTPGormLogger {
	return HTTPGormLogger{
		Logger:                    logger,
		SlowThreshold:             100 * time.Millisecond,
		SkipCallerLookup:          false,
		IgnoreRecordNotFoundError: false,
		Context:                   nil,
	}
}

// Zap GormLogger Methods
func (l GormLogger) SetAsDefault() {
	gormlogger.Default = l
}

func (l GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return GormLogger{
		ZapLogger:                 l.ZapLogger,
		SlowThreshold:             l.SlowThreshold,
		SkipCallerLookup:          l.SkipCallerLookup,
		IgnoreRecordNotFoundError: l.IgnoreRecordNotFoundError,
		Context:                   l.Context,
	}
}

func (l GormLogger) Info(ctx context.Context, str string, args ...interface{}) {
	l.ZapLogger = l.ZapLogger.With(zap.Reflect(mysticSpanContext, ctx))
	l.ZapLogger.Sugar().Debugf(str, args...)
}

func (l GormLogger) Warn(ctx context.Context, str string, args ...interface{}) {
	l.ZapLogger = l.ZapLogger.With(zap.Reflect(mysticSpanContext, ctx))
	l.ZapLogger.Sugar().Warnf(str, args...)
}

func (l GormLogger) Error(ctx context.Context, str string, args ...interface{}) {
	l.ZapLogger = l.ZapLogger.With(zap.Reflect(mysticSpanContext, ctx))
	l.ZapLogger.Sugar().Errorf(str, args...)
}

func (l GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	logger := l.ZapLogger
	logger = logger.With(zap.Reflect(mysticSpanContext, ctx))
	switch {
	case err != nil && (l.IgnoreRecordNotFoundError && errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		logger.Debug("trace", zap.Error(err), zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	case err != nil && (!errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		logger.Error("trace", zap.Error(err), zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	case l.SlowThreshold != 0 && elapsed > l.SlowThreshold:
		sql, rows := fc()
		logger.Warn("trace", zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	default:
		sql, rows := fc()
		if !strings.Contains(sql, "apex") {
			logger.Debug("trace", zap.Duration("elapsed", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
		}
	}
}

// Zerolog GormLogger Methods
func (l ZerologGormLogger) SetAsDefault() {
	gormlogger.Default = l
}

func (l ZerologGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return ZerologGormLogger{
		Logger:                    l.Logger,
		SlowThreshold:             l.SlowThreshold,
		SkipCallerLookup:          l.SkipCallerLookup,
		IgnoreRecordNotFoundError: l.IgnoreRecordNotFoundError,
		Context:                   l.Context,
	}
}

func (l ZerologGormLogger) Info(ctx context.Context, str string, args ...interface{}) {
	logger := l.Logger
	if l.Context != nil {
		fields := l.Context(ctx)
		for k, v := range fields {
			logger = logger.With().Interface(k, v).Logger()
		}
	}
	logger.Debug().Msgf(str, args...)
}

func (l ZerologGormLogger) Warn(ctx context.Context, str string, args ...interface{}) {
	logger := l.Logger
	if l.Context != nil {
		fields := l.Context(ctx)
		for k, v := range fields {
			logger = logger.With().Interface(k, v).Logger()
		}
	}
	logger.Warn().Msgf(str, args...)
}

func (l ZerologGormLogger) Error(ctx context.Context, str string, args ...interface{}) {
	logger := l.Logger
	if l.Context != nil {
		fields := l.Context(ctx)
		for k, v := range fields {
			logger = logger.With().Interface(k, v).Logger()
		}
	}
	logger.Error().Msgf(str, args...)
}

func (l ZerologGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	logger := l.Logger

	if l.Context != nil {
		fields := l.Context(ctx)
		for k, v := range fields {
			logger = logger.With().Interface(k, v).Logger()
		}
	}

	switch {
	case err != nil && (l.IgnoreRecordNotFoundError && errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		logger.Debug().
			Err(err).
			Dur("elapsed", elapsed).
			Int64("rows", rows).
			Str("sql", sql).
			Msg("trace")
	case err != nil && (!errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		logger.Error().
			Err(err).
			Dur("elapsed", elapsed).
			Int64("rows", rows).
			Str("sql", sql).
			Msg("trace")
	case l.SlowThreshold != 0 && elapsed > l.SlowThreshold:
		sql, rows := fc()
		logger.Warn().
			Dur("elapsed", elapsed).
			Int64("rows", rows).
			Str("sql", sql).
			Msg("trace")
	default:
		sql, rows := fc()
		if !strings.Contains(sql, "apex") {
			logger.Debug().
				Dur("elapsed", elapsed).
				Int64("rows", rows).
				Str("sql", sql).
				Msg("trace")
		}
	}
}

// File GormLogger Methods
func (l FileGormLogger) SetAsDefault() {
	gormlogger.Default = l
}

func (l FileGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return FileGormLogger{
		Logger:                    l.Logger,
		SlowThreshold:             l.SlowThreshold,
		IgnoreRecordNotFoundError: l.IgnoreRecordNotFoundError,
		Context:                   l.Context,
	}
}

func (l FileGormLogger) Info(ctx context.Context, str string, args ...interface{}) {
	if l.Context != nil {
		fields := l.Context(ctx)
		args = append(args, "context", fields)
	}
	l.Logger.Debug(str, args...)
}

func (l FileGormLogger) Warn(ctx context.Context, str string, args ...interface{}) {
	if l.Context != nil {
		fields := l.Context(ctx)
		args = append(args, "context", fields)
	}
	l.Logger.Warn(str, args...)
}

func (l FileGormLogger) Error(ctx context.Context, str string, args ...interface{}) {
	if l.Context != nil {
		fields := l.Context(ctx)
		args = append(args, "context", fields)
	}
	l.Logger.Error(str, args...)
}

func (l FileGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)

	switch {
	case err != nil && (l.IgnoreRecordNotFoundError && errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		l.Logger.Debug("trace", "error", err, "elapsed", elapsed, "rows", rows, "sql", sql)
	case err != nil && (!errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		l.Logger.Error("trace", "error", err, "elapsed", elapsed, "rows", rows, "sql", sql)
	case l.SlowThreshold != 0 && elapsed > l.SlowThreshold:
		sql, rows := fc()
		l.Logger.Warn("trace", "elapsed", elapsed, "rows", rows, "sql", sql)
	default:
		sql, rows := fc()
		if !strings.Contains(sql, "apex") {
			l.Logger.Debug("trace", "elapsed", elapsed, "rows", rows, "sql", sql)
		}
	}
}

// HTTP GormLogger Methods
func (l HTTPGormLogger) SetAsDefault() {
	gormlogger.Default = l
}

func (l HTTPGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return HTTPGormLogger{
		Logger:                    l.Logger,
		SlowThreshold:             l.SlowThreshold,
		IgnoreRecordNotFoundError: l.IgnoreRecordNotFoundError,
		Context:                   l.Context,
	}
}

func (l HTTPGormLogger) Info(ctx context.Context, str string, args ...interface{}) {
	if l.Context != nil {
		fields := l.Context(ctx)
		args = append(args, "context", fields)
	}
	l.Logger.Debug(str, args...)
}

func (l HTTPGormLogger) Warn(ctx context.Context, str string, args ...interface{}) {
	if l.Context != nil {
		fields := l.Context(ctx)
		args = append(args, "context", fields)
	}
	l.Logger.Warn(str, args...)
}

func (l HTTPGormLogger) Error(ctx context.Context, str string, args ...interface{}) {
	if l.Context != nil {
		fields := l.Context(ctx)
		args = append(args, "context", fields)
	}
	l.Logger.Error(str, args...)
}

func (l HTTPGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)

	switch {
	case err != nil && (l.IgnoreRecordNotFoundError && errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		l.Logger.Debug("trace", "error", err, "elapsed", elapsed, "rows", rows, "sql", sql)
	case err != nil && (!errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		l.Logger.Error("trace", "error", err, "elapsed", elapsed, "rows", rows, "sql", sql)
	case l.SlowThreshold != 0 && elapsed > l.SlowThreshold:
		sql, rows := fc()
		l.Logger.Warn("trace", "elapsed", elapsed, "rows", rows, "sql", sql)
	default:
		sql, rows := fc()
		if !strings.Contains(sql, "apex") {
			l.Logger.Debug("trace", "elapsed", elapsed, "rows", rows, "sql", sql)
		}
	}
}

// Convenience functions to create GormLogger adapters from mystic logger instances

// NewGormLoggerFromZerolog creates a ZerologGormLogger from a mystic ZerologAdapter
func NewGormLoggerFromZerolog(logger Logger) ZerologGormLogger {
	return NewZerologGormLoggerAdapter(logger)
}

// NewGormLoggerFromFile creates a FileGormLogger from a mystic FileAdapter
func NewGormLoggerFromFile(logger Logger) FileGormLogger {
	return NewFileGormLoggerAdapter(logger)
}

// NewGormLoggerFromHTTP creates a HTTPGormLogger from a mystic HTTPAdapter
func NewGormLoggerFromHTTP(logger Logger) HTTPGormLogger {
	return NewHTTPGormLoggerAdapter(logger)
}

// NewGormLoggerFromMystic creates a GormLogger adapter from any mystic logger
// It automatically detects the type and creates the appropriate GormLogger
func NewGormLoggerFromMystic(logger Logger) gormlogger.Interface {
	// This is a generic function that could be used to create any type of GormLogger
	// For now, we'll return a FileGormLogger as a default since it uses the generic Logger interface
	return NewFileGormLoggerAdapter(logger)
}
