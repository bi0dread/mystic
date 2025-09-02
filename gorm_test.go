package mystic

import (
	"context"
	"testing"
	"time"

	gormlogger "gorm.io/gorm/logger"
)

func Test_GormLogger_Adapters(t *testing.T) {
	t.Run("Zap GormLogger", func(t *testing.T) {
		zapLogger := ZapAdapter("test-zap-gorm")
		gormLogger := NewZapGormLoggerAdapter(zapLogger)

		// Test basic functionality
		if gormLogger.ZapLogger == nil {
			t.Error("expected non-nil ZapLogger")
		}

		// Test default values
		if gormLogger.SlowThreshold != 100*time.Millisecond {
			t.Errorf("expected SlowThreshold to be 100ms, got %v", gormLogger.SlowThreshold)
		}

		// Test LogMode
		newLogger := gormLogger.LogMode(gormlogger.Info)
		if newLogger == nil {
			t.Error("expected non-nil logger from LogMode")
		}

		// Test logging methods
		ctx := context.Background()
		gormLogger.Info(ctx, "test info message")
		gormLogger.Warn(ctx, "test warn message")
		gormLogger.Error(ctx, "test error message")

		// Test Trace method
		gormLogger.Trace(ctx, time.Now(), func() (string, int64) {
			return "SELECT * FROM users", 10
		}, nil)
	})

	t.Run("Zerolog GormLogger", func(t *testing.T) {
		zeroLogger := ZerologAdapter("test-zerolog-gorm")
		gormLogger := NewZerologGormLoggerAdapter(zeroLogger)

		// Test basic functionality - zerolog.Logger is a value type, not a pointer
		// So we just verify the adapter was created successfully
		if gormLogger.SlowThreshold != 100*time.Millisecond {
			t.Errorf("expected SlowThreshold to be 100ms, got %v", gormLogger.SlowThreshold)
		}

		// Test LogMode
		newLogger := gormLogger.LogMode(gormlogger.Info)
		if newLogger == nil {
			t.Error("expected non-nil logger from LogMode")
		}

		// Test logging methods
		ctx := context.Background()
		gormLogger.Info(ctx, "test info message")
		gormLogger.Warn(ctx, "test warn message")
		gormLogger.Error(ctx, "test error message")

		// Test Trace method
		gormLogger.Trace(ctx, time.Now(), func() (string, int64) {
			return "SELECT * FROM users", 10
		}, nil)
	})

	t.Run("File GormLogger", func(t *testing.T) {
		fileLogger := New(FileAdapter("test-gorm-file"))
		if fileLogger == nil {
			t.Fatalf("expected non-nil file logger")
		}

		gormLogger := NewFileGormLoggerAdapter(fileLogger)

		// Test basic functionality
		if gormLogger.Logger == nil {
			t.Error("expected non-nil Logger")
		}

		// Test default values
		if gormLogger.SlowThreshold != 100*time.Millisecond {
			t.Errorf("expected SlowThreshold to be 100ms, got %v", gormLogger.SlowThreshold)
		}

		// Test LogMode
		newLogger := gormLogger.LogMode(gormlogger.Info)
		if newLogger == nil {
			t.Error("expected non-nil logger from LogMode")
		}

		// Test logging methods
		ctx := context.Background()
		gormLogger.Info(ctx, "test info message")
		gormLogger.Warn(ctx, "test warn message")
		gormLogger.Error(ctx, "test error message")

		// Test Trace method
		gormLogger.Trace(ctx, time.Now(), func() (string, int64) {
			return "SELECT * FROM users", 10
		}, nil)
	})

	t.Run("HTTP GormLogger", func(t *testing.T) {
		httpLogger := New(HTTPAdapter("test-gorm-http"))
		if httpLogger == nil {
			t.Fatalf("expected non-nil HTTP logger")
		}

		gormLogger := NewHTTPGormLoggerAdapter(httpLogger)

		// Test basic functionality
		if gormLogger.Logger == nil {
			t.Error("expected non-nil Logger")
		}

		// Test default values
		if gormLogger.SlowThreshold != 100*time.Millisecond {
			t.Errorf("expected SlowThreshold to be 100ms, got %v", gormLogger.SlowThreshold)
		}

		// Test LogMode
		newLogger := gormLogger.LogMode(gormlogger.Info)
		if newLogger == nil {
			t.Error("expected non-nil logger from LogMode")
		}

		// Test logging methods
		ctx := context.Background()
		gormLogger.Info(ctx, "test info message")
		gormLogger.Warn(ctx, "test warn message")
		gormLogger.Error(ctx, "test error message")

		// Test Trace method
		gormLogger.Trace(ctx, time.Now(), func() (string, int64) {
			return "SELECT * FROM users", 10
		}, nil)
	})

	t.Run("Convenience Functions", func(t *testing.T) {
		// Test convenience functions
		fileLogger := New(FileAdapter("test-convenience"))
		if fileLogger == nil {
			t.Fatalf("expected non-nil file logger")
		}

		// Test NewGormLoggerFromFile
		fileGormLogger := NewGormLoggerFromFile(fileLogger)
		if fileGormLogger.Logger == nil {
			t.Error("expected non-nil Logger from NewGormLoggerFromFile")
		}

		// Test NewGormLoggerFromHTTP
		httpLogger := New(HTTPAdapter("test-convenience-http"))
		if httpLogger == nil {
			t.Fatalf("expected non-nil HTTP logger")
		}

		httpGormLogger := NewGormLoggerFromHTTP(httpLogger)
		if httpGormLogger.Logger == nil {
			t.Error("expected non-nil Logger from NewGormLoggerFromHTTP")
		}

		// Test NewGormLoggerFromMystic
		mysticGormLogger := NewGormLoggerFromMystic(fileLogger)
		if mysticGormLogger == nil {
			t.Error("expected non-nil logger from NewGormLoggerFromMystic")
		}
	})
}
