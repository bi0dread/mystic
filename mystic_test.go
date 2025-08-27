package mystic

import (
	"context"
	"errors"
	"testing"
)

// setSafeTestConfig sets config values that won't cause network failures for UDP writer
func setSafeTestConfig() {
	SetConfig(Config{
		LogLevel: "debug",
		Facility: "mystic-test",
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

func Test_GraylogSender_Integration(t *testing.T) {
	setSafeTestConfig()

	// Test that zap adapter can use GraylogSender
	zapLogger := New(ZapAdapter, "test-zap-graylog")
	if zapLogger == nil {
		t.Fatalf("expected non-nil zap logger with GraylogSender")
	}

	// Test that zerolog adapter can use GraylogSender
	zeroLogger := New(ZerologAdapter, "test-zerolog-graylog")
	if zeroLogger == nil {
		t.Fatalf("expected non-nil zerolog logger with GraylogSender")
	}

	// Both should work with GraylogSender integration
	zapLogger.Info("zap with graylog", "test", "integration")
	zeroLogger.Info("zerolog with graylog", "test", "integration")
}

func Test_GraylogSender_Direct(t *testing.T) {
	setSafeTestConfig()

	// Test GraylogSender directly
	graylogConfig := GraylogSenderConfig{
		GrayLogAddr: GRAYLOG_ADDR,
		Facility:    FACILITY,
	}
	sender := NewGraylogSender(graylogConfig)
	if sender == nil {
		t.Fatalf("expected non-nil GraylogSender")
	}

	// Test sending different log levels
	err := sender.Send("DEBUG", "debug message", map[string]interface{}{
		"user_id": 123,
		"action":  "test",
	})
	if err != nil {
		t.Logf("GraylogSender.Send returned error (expected if Graylog unavailable): %v", err)
	}

	err = sender.Send("INFO", "info message", map[string]interface{}{
		"service": "test-service",
		"version": "1.0.0",
	})
	if err != nil {
		t.Logf("GraylogSender.Send returned error (expected if Graylog unavailable): %v", err)
	}

	// Test io.Writer interface
	_, err = sender.Write([]byte("test write"))
	if err != nil {
		t.Logf("GraylogSender.Write returned error (expected if Graylog unavailable): %v", err)
	}

	// Clean up
	defer sender.Close()
}

func Test_GraylogSender_Configuration(t *testing.T) {
	// Test with different configurations
	testCases := []struct {
		name        string
		config      GraylogSenderConfig
		expectedNil bool
		description string
	}{
		{
			name: "Valid Configuration",
			config: GraylogSenderConfig{
				GrayLogAddr: "localhost:12201",
				Facility:    "test-facility",
			},
			expectedNil: false,
			description: "Should create sender with valid config",
		},
		{
			name: "Empty GrayLogAddr",
			config: GraylogSenderConfig{
				GrayLogAddr: "",
				Facility:    "test-facility",
			},
			expectedNil: false,
			description: "Should create fallback sender with empty address",
		},
		{
			name: "Invalid GrayLogAddr",
			config: GraylogSenderConfig{
				GrayLogAddr: "invalid:address:format",
				Facility:    "test-facility",
			},
			expectedNil: false,
			description: "Should create fallback sender with invalid address",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sender := NewGraylogSender(tc.config)

			if tc.expectedNil && sender != nil {
				t.Errorf("expected nil sender for %s", tc.description)
			}
			if !tc.expectedNil && sender == nil {
				t.Errorf("expected non-nil sender for %s", tc.description)
			}

			if sender != nil {
				// Test that the configuration is properly stored
				if sender.config.Facility != tc.config.Facility {
					t.Errorf("facility mismatch: expected %s, got %s", tc.config.Facility, sender.config.Facility)
				}

				// Test basic functionality
				err := sender.Send("TEST", "configuration test", map[string]interface{}{
					"test_case": tc.name,
				})
				if err != nil {
					t.Logf("Send returned error (expected for invalid configs): %v", err)
				}

				defer sender.Close()
			}
		})
	}
}

func Test_GraylogSender_Config_Validation(t *testing.T) {
	// Test configuration validation and defaults
	config := GraylogSenderConfig{
		GrayLogAddr: "localhost:12201",
		Facility:    "test-validation",
	}

	sender := NewGraylogSender(config)
	if sender == nil {
		t.Fatalf("expected non-nil sender")
	}
	defer sender.Close()

	// Verify configuration is stored correctly
	if sender.config.GrayLogAddr != config.GrayLogAddr {
		t.Errorf("GrayLogAddr not stored correctly: expected %s, got %s",
			config.GrayLogAddr, sender.config.GrayLogAddr)
	}

	if sender.config.Facility != config.Facility {
		t.Errorf("Facility not stored correctly: expected %s, got %s",
			config.Facility, sender.config.Facility)
	}
}

func Test_Adapter_Chaining_With_Graylog(t *testing.T) {
	setSafeTestConfig()

	// Test chaining with zap adapter (includes GraylogSender)
	logger := New(ZapAdapter, "test-chain")
	if logger == nil {
		t.Fatalf("expected non-nil logger")
	}

	// Chain multiple operations
	logger = logger.With("service", "api", "env", "test")
	logger = logger.SkipLevel(1)
	logger = logger.SetContext(context.Background())

	// All operations should work with GraylogSender integration
	logger.Debug("chained debug", "step", 1)
	logger.Info("chained info", "step", 2)
	logger.Warn("chained warn", "step", 3)
	logger.Error("chained error", "step", 4)

	// Test error detail with context
	testErr := errors.New("test error for chaining")
	logger.ErrorDetail(testErr, "operation", "chained_test")
}

func Test_All_Adapters_With_Graylog(t *testing.T) {
	setSafeTestConfig()

	// Test all adapters work with GraylogSender integration
	adapters := []struct {
		name    string
		adapter func(string) Logger
	}{
		{"ZapAdapter", ZapAdapter},
		{"ZerologAdapter", ZerologAdapter},
	}

	for _, adapter := range adapters {
		t.Run(adapter.name, func(t *testing.T) {
			logger := New(adapter.adapter, "test-"+adapter.name)
			if logger == nil {
				t.Fatalf("expected non-nil logger for %s", adapter.name)
			}

			// Test basic logging
			logger.Debug("debug test", "adapter", adapter.name)
			logger.Info("info test", "adapter", adapter.name)
			logger.Warn("warn test", "adapter", adapter.name)
			logger.Error("error test", "adapter", adapter.name)

			// Test error detail
			testErr := errors.New("test error")
			logger.ErrorDetail(testErr, "adapter", adapter.name)

			// Test chaining
			logger = logger.With("test", "chaining")
			logger = logger.SetContext(context.Background())
			logger.Info("chained test", "adapter", adapter.name)
		})
	}
}

func Test_Configuration_Integration(t *testing.T) {
	// Test that the new configuration structure works end-to-end
	testCases := []struct {
		name        string
		logLevel    string
		facility    string
		graylogAddr string
		description string
	}{
		{
			name:        "Standard Configuration",
			logLevel:    "info",
			facility:    "test-standard",
			graylogAddr: "localhost:12201",
			description: "Standard working configuration",
		},
		{
			name:        "Debug Level Configuration",
			logLevel:    "debug",
			facility:    "test-debug",
			graylogAddr: "127.0.0.1:12201",
			description: "Debug level configuration",
		},
		{
			name:        "Custom Facility",
			logLevel:    "warn",
			facility:    "custom-service",
			graylogAddr: "0.0.0.0:12201",
			description: "Custom facility configuration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set configuration
			SetConfig(Config{
				LogLevel: tc.logLevel,
				Facility: tc.facility,
			})

			// Test both adapters with the configuration
			adapters := []struct {
				name    string
				adapter func(string) Logger
			}{
				{"ZapAdapter", ZapAdapter},
				{"ZerologAdapter", ZerologAdapter},
			}

			for _, adapter := range adapters {
				t.Run(adapter.name, func(t *testing.T) {
					logger := New(adapter.adapter, "test-config-"+adapter.name)
					if logger == nil {
						t.Fatalf("expected non-nil logger for %s", adapter.name)
					}

					// Test that the configuration is properly applied
					logger.Info("configuration test",
						"test_case", tc.name,
						"log_level", tc.logLevel,
						"facility", tc.facility,
						"adapter", adapter.name,
					)
				})
			}
		})
	}
}

func Test_Adapter_With_Custom_Configuration(t *testing.T) {
	// Test that adapters can be created with custom Graylog configuration
	testCases := []struct {
		name          string
		graylogConfig GraylogSenderConfig
		expectedNil   bool
		description   string
	}{
		{
			name: "Custom Graylog Address",
			graylogConfig: GraylogSenderConfig{
				GrayLogAddr: "custom-graylog:12201",
				Facility:    "custom-facility",
			},
			expectedNil: false,
			description: "Should create logger with custom Graylog configuration",
		},
		{
			name: "Different Facility",
			graylogConfig: GraylogSenderConfig{
				GrayLogAddr: "localhost:12201",
				Facility:    "different-service",
			},
			expectedNil: false,
			description: "Should create logger with different facility",
		},
		{
			name: "Empty Graylog Address",
			graylogConfig: GraylogSenderConfig{
				GrayLogAddr: "",
				Facility:    "fallback-test",
			},
			expectedNil: false,
			description: "Should create logger with fallback configuration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test ZapAdapter with custom configuration
			zapLogger := ZapAdapterWithConfig("test-custom-zap", tc.graylogConfig)
			if tc.expectedNil && zapLogger != nil {
				t.Errorf("expected nil zap logger for %s", tc.description)
			}
			if !tc.expectedNil && zapLogger == nil {
				t.Errorf("expected non-nil zap logger for %s", tc.description)
			}

			// Test ZerologAdapter with custom configuration
			zeroLogger := ZerologAdapterWithConfig("test-custom-zero", tc.graylogConfig)
			if tc.expectedNil && zeroLogger != nil {
				t.Errorf("expected nil zerolog logger for %s", tc.description)
			}
			if !tc.expectedNil && zeroLogger == nil {
				t.Errorf("expected non-nil zerolog logger for %s", tc.description)
			}

			// Test basic functionality with custom configuration
			if zapLogger != nil {
				zapLogger.Info("custom config test", "config", tc.name)
			}
			if zeroLogger != nil {
				zeroLogger.Info("custom config test", "config", tc.name)
			}
		})
	}
}

func Test_Configuration_Structure(t *testing.T) {
	// Test the new configuration structure
	t.Run("Config Struct", func(t *testing.T) {
		config := Config{
			LogLevel: "debug",
			Facility: "test-config-struct",
		}

		if config.LogLevel != "debug" {
			t.Errorf("LogLevel not set correctly: expected debug, got %s", config.LogLevel)
		}

		if config.Facility != "test-config-struct" {
			t.Errorf("Facility not set correctly: expected test-config-struct, got %s", config.Facility)
		}
	})

	t.Run("GraylogSenderConfig Struct", func(t *testing.T) {
		graylogConfig := GraylogSenderConfig{
			GrayLogAddr: "localhost:12201",
			Facility:    "test-graylog-config",
		}

		if graylogConfig.GrayLogAddr != "localhost:12201" {
			t.Errorf("GrayLogAddr not set correctly: expected localhost:12201, got %s", graylogConfig.GrayLogAddr)
		}

		if graylogConfig.Facility != "test-graylog-config" {
			t.Errorf("Facility not set correctly: expected test-graylog-config, got %s", graylogConfig.Facility)
		}
	})
}

func Test_Configuration_Error_Handling(t *testing.T) {
	// Test error handling with invalid configurations
	t.Run("Invalid GraylogSender Config", func(t *testing.T) {
		// Test with completely invalid configuration
		invalidConfig := GraylogSenderConfig{
			GrayLogAddr: "invalid:address:format:here",
			Facility:    "test-error-handling",
		}

		sender := NewGraylogSender(invalidConfig)
		if sender == nil {
			t.Fatalf("expected non-nil sender even with invalid config (should fallback)")
		}

		// Should still work with fallback
		err := sender.Send("ERROR", "test error handling", map[string]interface{}{
			"config": "invalid",
		})
		if err != nil {
			t.Logf("Send returned error (expected for invalid config): %v", err)
		}

		defer sender.Close()
	})

	t.Run("Empty GraylogSender Config", func(t *testing.T) {
		// Test with empty configuration
		emptyConfig := GraylogSenderConfig{
			GrayLogAddr: "",
			Facility:    "test-empty",
		}

		sender := NewGraylogSender(emptyConfig)
		if sender == nil {
			t.Fatalf("expected non-nil sender with empty config (should fallback)")
		}

		// Should work with fallback
		err := sender.Send("INFO", "test empty config", map[string]interface{}{
			"config": "empty",
		})
		if err != nil {
			t.Logf("Send returned error (expected for empty config): %v", err)
		}

		defer sender.Close()
	})
}

func Test_GELF_Core_Integration(t *testing.T) {
	setSafeTestConfig()

	// Test that GELF core works with GraylogSender
	zapLogger := New(ZapAdapter, "test-gelf-core")
	if zapLogger == nil {
		t.Fatalf("expected non-nil zap logger with GELF core")
	}

	// Test structured logging that goes through GELF core
	logger := zapLogger.With("component", "gelf-test", "version", "1.0.0")

	// These should trigger both console output and GraylogSender (fallback to console)
	logger.Info("GELF core test", "test_type", "integration")
	logger.Warn("GELF core warning", "severity", "medium")
	logger.Error("GELF core error", "error_code", "TEST_001")

	// Test with context
	ctx := context.Background()
	logger = logger.SetContext(ctx)
	logger.Info("GELF core with context", "context", "present")
}

func Test_Telemetry_Core_Integration(t *testing.T) {
	setSafeTestConfig()

	// Test that telemetry core works with GraylogSender
	zapLogger := New(ZapAdapter, "test-telemetry-core")
	if zapLogger == nil {
		t.Fatalf("expected non-nil zap logger with telemetry core")
	}

	// Test telemetry integration
	logger := zapLogger.With("service", "telemetry-test")

	// These should trigger telemetry core and GraylogSender
	logger.Info("telemetry test", "metric", "request_count", "value", 42)
	logger.Debug("telemetry debug", "trace_id", "abc123")
	logger.Error("telemetry error", "error_type", "validation_failed")

	// Test context propagation
	ctx := context.Background()
	logger = logger.SetContext(ctx)
	logger.Info("telemetry with context", "span_active", "true")
}
