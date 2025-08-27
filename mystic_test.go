package mystic

import (
	"context"
	"errors"
	"testing"
)

// setSafeTestConfig sets config values that won't cause network failures for UDP writer
func setSafeTestConfig() {
	SetConfig(Config{
		LogLevel:    "debug",
		GrayLogAddr: "127.0.0.1:12201",
		Facility:    "mystic-test",
	})
}

func Test_New_WithZapAdapter_Basic(t *testing.T) {
	setSafeTestConfig()

	logger := New(ZapAdapter, "test-basic")
	if logger == nil {
		t.Fatalf("expected non-nil logger")
	}

	// Basic method calls should not panic
	logger.Debug("debug msg")
	logger.Info("info msg", "k1", "v1")
	logger.Warn("warn msg", "answer", 42)
	logger.Error("error msg")

	// ErrorDetail with a simple error should not panic
	logger.ErrorDetail(errors.New("boom"))

	// Panic is not called in tests
}

func Test_With_SkipLevel_SetContext(t *testing.T) {
	setSafeTestConfig()

	logger := New(ZapAdapter, "test-ctx")
	if logger == nil {
		t.Fatalf("expected non-nil logger")
	}

	// With should be chainable
	logger = logger.With("foo", "bar")
	if logger == nil {
		t.Fatalf("expected non-nil after With")
	}

	logger.Debug("debug msg", "ff", 4)

	// SkipLevel should be chainable
	logger = logger.SkipLevel(1)
	if logger == nil {
		t.Fatalf("expected non-nil after SkipLevel")
	}

	logger.Debug("debug msg", "gg", 4)

	// SetContext should be chainable and support context propagation
	ctx := context.Background()
	logger = logger.SetContext(ctx)
	if logger == nil {
		t.Fatalf("expected non-nil after SetContext")
	}

	logger.Info("context attached")
}

func Test_SkipLevel_Chain_NoPanic(t *testing.T) {
	setSafeTestConfig()

	l := New(ZapAdapter, "test-skip")
	if l == nil {
		t.Fatalf("expected non-nil logger")
	}

	// Chain multiple SkipLevel calls; should be safe and chainable
	l = l.SkipLevel(1).SkipLevel(2).SkipLevel(3)
	if l == nil {
		t.Fatalf("expected non-nil after chained SkipLevel")
	}

	// Ensure subsequent logging still works without panic
	l.Debug("after skip level chain")
	l.Info("after skip level chain info")
}

func Test_New_WithZerologAdapter_Basic(t *testing.T) {
	setSafeTestConfig()

	logger := New(ZerologAdapter, "test-zero-basic")
	if logger == nil {
		t.Fatalf("expected non-nil zerolog adapter")
	}

	logger = logger.With("a", 1).SkipLevel(2).SetContext(context.Background())
	logger.Debug("zero debug")
	logger.Info("zero info")
	logger.Warn("zero warn")
	logger.Error("zero error")
}
