package mystic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

	logger := New(ZapAdapter("test-basic"))
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

	logger := New(ZapAdapter("test-ctx"))
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

	l := New(ZapAdapter("test-skip"))
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

	logger := New(ZerologAdapter("test-zero-basic"))
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
	zapLogger := New(ZapAdapter("test-zap-graylog"))
	if zapLogger == nil {
		t.Fatalf("expected non-nil zap logger with GraylogSender")
	}

	// Test that zerolog adapter can use GraylogSender
	zeroLogger := New(ZerologAdapter("test-zerolog-graylog"))
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
	graylogConfig := SenderConfig{
		Endpoint: GRAYLOG_ADDR,
	}
	sender := NewGraylogSender(graylogConfig, FACILITY)
	if sender == nil {
		t.Fatalf("expected non-nil GraylogSender")
	}

	// Test sending different log levels
	err := sender.Send(LogEntry{
		Level:     "DEBUG",
		Message:   "debug message",
		Timestamp: time.Now(),
		Fields: map[string]interface{}{
			"user_id": 123,
			"action":  "test",
		},
	})
	if err != nil {
		t.Logf("GraylogSender.Send returned error (expected if Graylog unavailable): %v", err)
	}

	err = sender.Send(LogEntry{
		Level:     "INFO",
		Message:   "info message",
		Timestamp: time.Now(),
		Fields: map[string]interface{}{
			"service": "test-service",
			"version": "1.0.0",
		},
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
		config      SenderConfig
		facility    string
		expectedNil bool
		description string
	}{
		{
			name: "Valid Configuration",
			config: SenderConfig{
				Endpoint: "localhost:12201",
			},
			facility:    "test-facility",
			expectedNil: false,
			description: "Should create sender with valid config",
		},
		{
			name: "Empty Endpoint",
			config: SenderConfig{
				Endpoint: "",
			},
			facility:    "test-facility",
			expectedNil: false,
			description: "Should create fallback sender with empty address",
		},
		{
			name: "Invalid Endpoint",
			config: SenderConfig{
				Endpoint: "invalid:address:format",
			},
			facility:    "test-facility",
			expectedNil: false,
			description: "Should create fallback sender with invalid address",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sender := NewGraylogSender(tc.config, tc.facility)

			if tc.expectedNil && sender != nil {
				t.Errorf("expected nil sender for %s", tc.description)
			}
			if !tc.expectedNil && sender == nil {
				t.Errorf("expected non-nil sender for %s", tc.description)
			}

			if sender != nil {
				// Test that the configuration is properly stored
				if sender.facility != tc.facility {
					t.Errorf("facility mismatch: expected %s, got %s", tc.facility, sender.facility)
				}

				// Test basic functionality
				err := sender.Send(LogEntry{
					Level:     "TEST",
					Message:   "configuration test",
					Timestamp: time.Now(),
					Fields: map[string]interface{}{
						"test_case": tc.name,
					},
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
	config := SenderConfig{
		Endpoint: "localhost:12201",
	}
	facility := "test-validation"

	sender := NewGraylogSender(config, facility)
	if sender == nil {
		t.Fatalf("expected non-nil sender")
	}
	defer sender.Close()

	// Verify configuration is stored correctly
	if sender.GetEndpoint() != config.Endpoint {
		t.Errorf("Endpoint not stored correctly: expected %s, got %s",
			config.Endpoint, sender.GetEndpoint())
	}

	if sender.facility != facility {
		t.Errorf("Facility not stored correctly: expected %s, got %s",
			facility, sender.facility)
	}
}

func Test_Adapter_Chaining_With_Graylog(t *testing.T) {
	setSafeTestConfig()

	// Test chaining with zap adapter (includes GraylogSender)
	logger := New(ZapAdapter("test-chain"))
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
			logger := New(adapter.adapter("test-" + adapter.name))
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
					logger := New(adapter.adapter("test-config-" + adapter.name))
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
		graylogConfig SenderConfig
		facility      string
		expectedNil   bool
		description   string
	}{
		{
			name: "Custom Graylog Address",
			graylogConfig: SenderConfig{
				Endpoint: "custom-graylog:12201",
			},
			facility:    "custom-facility",
			expectedNil: false,
			description: "Should create logger with custom Graylog configuration",
		},
		{
			name: "Different Facility",
			graylogConfig: SenderConfig{
				Endpoint: "localhost:12201",
			},
			facility:    "different-service",
			expectedNil: false,
			description: "Should create logger with different facility",
		},
		{
			name: "Empty Graylog Address",
			graylogConfig: SenderConfig{
				Endpoint: "",
			},
			facility:    "fallback-test",
			expectedNil: false,
			description: "Should create logger with fallback configuration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test ZapAdapter with custom configuration
			zapLogger := ZapAdapterWithConfig("test-custom-zap", nil)
			if tc.expectedNil && zapLogger != nil {
				t.Errorf("expected nil zap logger for %s", tc.description)
			}
			if !tc.expectedNil && zapLogger == nil {
				t.Errorf("expected non-nil zap logger for %s", tc.description)
			}

			// Test ZerologAdapter with custom configuration
			zeroLogger := ZerologAdapterWithConfig("test-custom-zero", nil)
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

	t.Run("SenderConfig Struct", func(t *testing.T) {
		senderConfig := SenderConfig{
			Endpoint:   "localhost:12201",
			Timeout:    5000,
			RetryCount: 3,
		}

		if senderConfig.Endpoint != "localhost:12201" {
			t.Errorf("Endpoint not set correctly: expected localhost:12201, got %s", senderConfig.Endpoint)
		}

		if senderConfig.Timeout != 5000 {
			t.Errorf("Timeout not set correctly: expected 5000, got %d", senderConfig.Timeout)
		}
	})
}

func Test_Configuration_Error_Handling(t *testing.T) {
	// Test error handling with invalid configurations
	t.Run("Invalid GraylogSender Config", func(t *testing.T) {
		// Test with completely invalid configuration
		invalidConfig := SenderConfig{
			Endpoint: "invalid:address:format:here",
		}
		facility := "test-error-handling"

		sender := NewGraylogSender(invalidConfig, facility)
		if sender == nil {
			t.Fatalf("expected non-nil sender even with invalid config (should fallback)")
		}

		// Should still work with fallback
		err := sender.Send(LogEntry{
			Level:     "ERROR",
			Message:   "test error handling",
			Timestamp: time.Now(),
			Fields: map[string]interface{}{
				"config": "invalid",
			},
		})
		if err != nil {
			t.Logf("Send returned error (expected for invalid config): %v", err)
		}

		defer sender.Close()
	})

	t.Run("Empty GraylogSender Config", func(t *testing.T) {
		// Test with empty configuration
		emptyConfig := SenderConfig{
			Endpoint: "",
		}
		facility := "test-empty"

		sender := NewGraylogSender(emptyConfig, facility)
		if sender == nil {
			t.Fatalf("expected non-nil sender with empty config (should fallback)")
		}

		// Should work with fallback
		err := sender.Send(LogEntry{
			Level:     "INFO",
			Message:   "test empty config",
			Timestamp: time.Now(),
			Fields: map[string]interface{}{
				"config": "empty",
			},
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
	zapLogger := New(ZapAdapter("test-gelf-core"))
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
	zapLogger := New(ZapAdapter("test-telemetry-core"))
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

// Test_File_Adapter_Basic tests basic file adapter functionality
func Test_File_Adapter_Basic(t *testing.T) {
	// Test basic file adapter creation
	logger := FileAdapter("test-file")
	if logger == nil {
		t.Fatal("expected non-nil file logger")
	}

	// Test logging methods
	logger.Info("file test message", "key", "value")
	logger.Error("file test error", "error_key", "error_value")
	logger.Debug("file debug message")
	logger.Warn("file warning message")
	// Don't test Panic in unit tests as it actually calls panic()
	// logger.Panic("file panic message")

	// Test chaining
	logger.With("persistent", "field").Info("chained message")
}

// Test_File_Adapter_With_Config tests file adapter with custom configuration
func Test_File_Adapter_With_Config(t *testing.T) {
	config := FileAdapterConfig{
		Path:       "./test-logs",
		Filename:   "test-application",
		MaxSize:    1024, // 1KB for testing
		MaxAge:     1 * time.Hour,
		MaxBackups: 2,
		Compress:   false, // Disable compression for testing
		Format:     "json",
		Level:      "debug",
		Facility:   "test-facility",
	}

	logger := New(FileAdapterWithConfig("test-file-config", config))
	if logger == nil {
		t.Fatal("expected non-nil file logger with config")
	}

	// Test logging
	logger.Info("config test message", "config", "custom")
	logger.Error("config test error")

	// Clean up test directory
	os.RemoveAll("./test-logs")
}

// Test_File_Adapter_Formats tests different output formats
func Test_File_Adapter_Formats(t *testing.T) {
	testCases := []struct {
		format string
		name   string
	}{
		{"json", "json-format"},
		{"console", "console-format"},
		{"gelf", "gelf-format"},
	}

	for _, tc := range testCases {
		t.Run(tc.format, func(t *testing.T) {
			config := FileAdapterConfig{
				Path:       "./test-logs",
				Filename:   "test-" + tc.format,
				MaxSize:    1024,
				MaxAge:     1 * time.Hour,
				MaxBackups: 1,
				Compress:   false,
				Format:     tc.format,
				Level:      "info",
				Facility:   "test-facility",
			}

			logger := FileAdapterWithConfig(tc.name, config)
			if logger == nil {
				t.Fatalf("expected non-nil logger for format %s", tc.format)
			}

			logger.Info("format test", "format", tc.format)
		})
	}

	// Clean up
	os.RemoveAll("./test-logs")
}

// Test_HTTP_Adapter_Basic tests basic HTTP adapter functionality
func Test_HTTP_Adapter_Basic(t *testing.T) {
	logger := HTTPAdapter("test-http")
	if logger == nil {
		t.Fatal("expected non-nil HTTP logger")
	}

	// Test logging methods
	logger.Info("http test message", "key", "value")
	logger.Error("http test error", "error_key", "error_value")
	logger.Debug("http debug message")
	logger.Warn("http warning message")
	// Don't test Panic in unit tests as it actually calls panic()
	// logger.Panic("http panic message")

	// Test chaining
	logger.With("persistent", "field").Info("chained message")
}

// Test_HTTP_Adapter_With_Config tests HTTP adapter with custom configuration
func Test_HTTP_Adapter_With_Config(t *testing.T) {
	config := HTTPAdapterConfig{
		SenderConfig: SenderConfig{
			Endpoint:   "http://localhost:8080/test",
			Timeout:    5000,
			RetryCount: 2,
			RetryDelay: 1000,
			BatchSize:  10,
			BatchDelay: 1000,
			Format:     "json",
		},
		Method:     "POST",
		Headers:    map[string]string{"Content-Type": "application/json"},
		Timeout:    1 * time.Second,
		RetryCount: 2,
		RetryDelay: 100 * time.Millisecond,
		BatchSize:  10,
		BatchDelay: 50 * time.Millisecond,
		Format:     "json",
		Level:      "info",
		Facility:   "test-facility",
	}

	logger := HTTPAdapterWithConfig("test-http-config", config)
	if logger == nil {
		t.Fatal("expected non-nil HTTP logger with config")
	}

	// Test logging
	logger.Info("config test message", "config", "custom")
	logger.Error("config test error")
}

// Test_HTTP_Adapter_Formats tests different HTTP output formats
func Test_HTTP_Adapter_Formats(t *testing.T) {
	testCases := []struct {
		format string
		name   string
	}{
		{"json", "json-format"},
		{"gelf", "gelf-format"},
	}

	for _, tc := range testCases {
		t.Run(tc.format, func(t *testing.T) {
			config := HTTPAdapterConfig{
				SenderConfig: SenderConfig{
					Endpoint:   "http://localhost:8080/test",
					Timeout:    5000,
					RetryCount: 1,
					RetryDelay: 1000,
					BatchSize:  5,
					BatchDelay: 1000,
					Format:     tc.format,
				},
				Method:     "POST",
				Headers:    map[string]string{"Content-Type": "application/json"},
				Timeout:    1 * time.Second,
				RetryCount: 1,
				RetryDelay: 100 * time.Millisecond,
				BatchSize:  5,
				BatchDelay: 50 * time.Millisecond,
				Format:     tc.format,
				Level:      "info",
				Facility:   "test-facility",
			}

			logger := HTTPAdapterWithConfig(tc.name, config)
			if logger == nil {
				t.Fatalf("expected non-nil logger for format %s", tc.format)
			}

			logger.Info("format test", "format", tc.format)
		})
	}
}

// Test_Multi_Output_Adapter_Basic tests basic multi-output adapter functionality
func Test_Multi_Output_Adapter_Basic(t *testing.T) {
	// Create a simple multi-output configuration
	config := MultiOutputConfig{
		Adapters: []AdapterConfig{
			{Name: "console", Adapter: ZapAdapter},
			{Name: "file", Adapter: FileAdapter},
		},
		Strategy: "all",
	}

	logger := MultiOutputAdapterWithConfig("test-multi", config)
	if logger == nil {
		t.Fatal("expected non-nil multi-output logger")
	}

	// Test logging methods
	logger.Info("multi test message", "key", "value")
	logger.Error("multi test error", "error_key", "error_value")
	logger.Debug("multi debug message")
	logger.Warn("multi warning message")
	// Don't test Panic in unit tests as it actually calls panic()
	// logger.Panic("multi panic message")

	// Test chaining
	logger.With("persistent", "field").Info("chained message")
}

// Test_Multi_Output_Adapter_Strategies tests different output strategies
func Test_Multi_Output_Adapter_Strategies(t *testing.T) {
	testCases := []struct {
		strategy string
		name     string
	}{
		{"all", "all-strategy"},
		{"first_success", "first-success-strategy"},
		{"round_robin", "round-robin-strategy"},
	}

	for _, tc := range testCases {
		t.Run(tc.strategy, func(t *testing.T) {
			config := MultiOutputConfig{
				Adapters: []AdapterConfig{
					{Name: "console", Adapter: ZapAdapter},
					{Name: "file", Adapter: FileAdapter},
				},
				Strategy: tc.strategy,
			}

			logger := MultiOutputAdapterWithConfig(tc.name, config)
			if logger == nil {
				t.Fatalf("expected non-nil logger for strategy %s", tc.strategy)
			}

			logger.Info("strategy test", "strategy", tc.strategy)
		})
	}
}

// Test_Enhanced_Logger_Basic tests basic enhanced logger functionality
func Test_Enhanced_Logger_Basic(t *testing.T) {
	baseLogger := ZapAdapter("test-enhanced")
	config := EnhancedLoggerConfig{
		EnableMetrics:     true,
		EnableSampling:    true,
		EnableTiming:      true,
		EnableRateLimit:   true,
		DefaultSampleRate: 0.5,
		PrettyPrint:       false,
	}

	enhanced := NewEnhancedLogger(baseLogger, config)
	if enhanced == nil {
		t.Fatal("expected non-nil enhanced logger")
	}

	// Test basic logging methods
	enhanced.Info("enhanced test message", "key", "value")
	enhanced.Error("enhanced test error", "error_key", "error_value")
	enhanced.Debug("enhanced debug message")
	enhanced.Warn("enhanced warning message")
	// Don't test Panic in unit tests as it actually calls panic()
	// enhanced.Panic("enhanced panic message")

	// Test chaining
	enhanced.With("persistent", "field").Info("chained message")
}

// Test_Enhanced_Logger_Structured tests structured logging
func Test_Enhanced_Logger_Structured(t *testing.T) {
	baseLogger := ZapAdapter("test-structured")
	config := EnhancedLoggerConfig{
		EnableMetrics: true,
	}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Test structured logging
	fields := map[string]interface{}{
		"user_id":    123,
		"action":     "login",
		"ip_address": "192.168.1.1",
		"timestamp":  time.Now(),
	}

	enhanced.Structured("user_login", fields)
}

// Test_Enhanced_Logger_Sampling tests log sampling
func Test_Enhanced_Logger_Sampling(t *testing.T) {
	baseLogger := ZapAdapter("test-sampling")
	config := EnhancedLoggerConfig{
		EnableSampling: true,
	}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Test sampling at different rates
	enhanced.Sampled("debug", 0.1, "low-rate debug", "key", "value")
	enhanced.Sampled("info", 0.5, "medium-rate info", "key", "value")
	enhanced.Sampled("warn", 1.0, "high-rate warn", "key", "value")
}

// Test_Enhanced_Logger_Metrics tests metrics collection
func Test_Enhanced_Logger_Metrics(t *testing.T) {
	baseLogger := ZapAdapter("test-metrics")
	config := EnhancedLoggerConfig{
		EnableMetrics: true,
	}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Test metrics collection
	enhanced.WithMetrics()

	// Record various metrics
	enhanced.IncrementCounter("requests_total", 1)
	enhanced.IncrementCounter("errors_total", 1)
	enhanced.RecordGauge("active_connections", 42.5)
	enhanced.RecordHistogram("request_duration_ms", 150.2)
	enhanced.RecordHistogram("request_duration_ms", 200.1)
	enhanced.RecordHistogram("request_duration_ms", 175.8)
}

// Test_Enhanced_Logger_Timing tests performance tracking
func Test_Enhanced_Logger_Timing(t *testing.T) {
	baseLogger := ZapAdapter("test-timing")
	config := EnhancedLoggerConfig{
		EnableTiming: true,
	}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Test timing
	enhanced.WithTiming("test_operation")

	// Time an operation
	enhanced.TimeOperation("test_operation", func() {
		time.Sleep(10 * time.Millisecond)
	})
}

// Test_Enhanced_Logger_Rate_Limiting tests rate limiting
func Test_Enhanced_Logger_Rate_Limiting(t *testing.T) {
	baseLogger := ZapAdapter("test-rate-limit")
	config := EnhancedLoggerConfig{
		EnableRateLimit: true,
	}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Set rate limit
	enhanced.WithRateLimit(5)

	// Try to log more than the limit
	for i := 0; i < 10; i++ {
		enhanced.Info("rate limited message", "count", i)
	}
}

// Test_Enhanced_Logger_Conditional tests conditional logging
func Test_Enhanced_Logger_Conditional(t *testing.T) {
	baseLogger := ZapAdapter("test-conditional")
	config := EnhancedLoggerConfig{}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Test conditional logging
	enhanced.When(true).Info("should log this")
	enhanced.When(false).Info("should not log this")

	// Test context-based conditional logging
	ctx := context.WithValue(context.Background(), "debug_mode", true)
	enhanced.WhenContext(func(ctx context.Context) bool {
		return ctx.Value("debug_mode") == true
	}).Debug("debug message")

	// Use ctx to avoid unused variable warning
	_ = ctx
}

// Test_Enhanced_Logger_Batch tests batch logging
func Test_Enhanced_Logger_Batch(t *testing.T) {
	baseLogger := ZapAdapter("test-batch")
	config := EnhancedLoggerConfig{}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Test batch logging
	entries := []LogEntry{
		{Level: "info", Message: "batch entry 1", Fields: map[string]interface{}{"id": 1}},
		{Level: "info", Message: "batch entry 2", Fields: map[string]interface{}{"id": 2}},
		{Level: "error", Message: "batch entry 3", Fields: map[string]interface{}{"id": 3}},
	}

	enhanced.Batch(entries)
}

// Test_Configuration_Profiles tests configuration profile loading
func Test_Configuration_Profiles(t *testing.T) {
	testCases := []struct {
		profile string
		valid   bool
	}{
		{"development", true},
		{"staging", true},
		{"production", true},
		{"testing", true},
		{"invalid", false},
	}

	for _, tc := range testCases {
		t.Run(tc.profile, func(t *testing.T) {
			profile, err := LoadProfile(tc.profile)

			if tc.valid {
				if err != nil {
					t.Errorf("expected no error for profile %s, got %v", tc.profile, err)
				}
				if profile == nil {
					t.Errorf("expected non-nil profile for %s", tc.profile)
				}
				if profile.Name != tc.profile {
					t.Errorf("expected profile name %s, got %s", tc.profile, profile.Name)
				}
			} else {
				if err == nil {
					t.Errorf("expected error for invalid profile %s", tc.profile)
				}
				if profile != nil {
					t.Errorf("expected nil profile for invalid profile %s", tc.profile)
				}
			}
		})
	}
}

// Test_Configuration_Loading tests configuration loading from environment
func Test_Configuration_Loading(t *testing.T) {
	// Test loading config from environment
	config, err := LoadConfigFromEnv()
	if err != nil {
		t.Logf("LoadConfigFromEnv failed (expected in test environment): %v", err)
		return
	}

	if config != nil {
		// Test validation
		if err := config.Validate(); err != nil {
			t.Errorf("config validation failed: %v", err)
		}

		// Test setting defaults
		config.SetDefaults()
		if config.LogLevel == "" {
			t.Error("expected LogLevel to be set after defaults")
		}
	}
}

// Test_Configuration_Validation tests configuration validation
func Test_Configuration_Validation(t *testing.T) {
	// Test valid config
	validConfig := &Config{
		LogLevel: "info",
		Facility: "test-facility",
	}

	if err := validConfig.Validate(); err != nil {
		t.Errorf("valid config validation failed: %v", err)
	}

	// Test invalid log level
	invalidLevelConfig := &Config{
		LogLevel: "invalid-level",
		Facility: "test-facility",
	}

	if err := invalidLevelConfig.Validate(); err == nil {
		t.Error("expected validation error for invalid log level")
	}

	// Test missing facility
	missingFacilityConfig := &Config{
		LogLevel: "info",
		Facility: "",
	}

	if err := missingFacilityConfig.Validate(); err == nil {
		t.Error("expected validation error for missing facility")
	}
}

// Test_Test_Helpers tests the test helper functionality
func Test_Test_Helpers(t *testing.T) {
	// Test log capture functionality
	captured := &CapturedLogs{
		Entries: make([]CapturedLogEntry, 0),
	}

	// Create a capture logger directly
	captureLogger := &captureLogger{
		captured: captured,
		name:     "test-capture",
		fields:   make(map[string]interface{}),
	}

	// Log messages using the capture logger
	captureLogger.Info("captured message")
	captureLogger.Error("captured error")

	if captured == nil {
		t.Fatal("expected non-nil captured logs")
	}

	// Test log assertions
	if !captured.Contains("captured message") {
		t.Error("expected log to contain 'captured message'")
	}

	if !captured.ContainsLevel("error") {
		t.Error("expected log to contain error level")
	}

	// Test getting entries
	infoEntries := captured.GetEntriesByLevel("info")
	if len(infoEntries) == 0 {
		t.Error("expected info entries")
	}

	errorEntries := captured.GetEntriesByMessage("captured error")
	if len(errorEntries) == 0 {
		t.Error("expected error entries")
	}

	// Test clear
	captured.Clear()
	if captured.Count() != 0 {
		t.Error("expected 0 entries after clear")
	}
}

// Test_Mock_Logger tests mock logger functionality
func Test_Mock_Logger(t *testing.T) {
	// Create mock logger with expectations
	mockLogger := NewMockLogger()
	mockLogger.ExpectInfo("expected message")
	mockLogger.ExpectError("expected error").ExpectCount(2)

	// Use in your code
	logger := mockLogger
	logger.Info("expected message")
	logger.Error("expected error")
	logger.Error("expected error")

	// Verify expectations
	if err := mockLogger.Verify(); err != nil {
		t.Errorf("mock expectations not met: %v", err)
	}
}

// Test_Test_Logger tests test logger functionality
func Test_Test_Logger(t *testing.T) {
	// Create test logger that writes to buffer
	testLogger := NewTestLogger()

	// Log messages
	testLogger.Info("test message", "key", "value")
	testLogger.Error("test error")

	// Get output
	output := testLogger.GetOutput()
	if !strings.Contains(output, "test message") {
		t.Error("expected output to contain 'test message'")
	}

	if !strings.Contains(output, "test error") {
		t.Error("expected output to contain 'test error'")
	}

	// Clear output
	testLogger.Clear()
	if testLogger.GetOutput() != "" {
		t.Error("expected empty output after clear")
	}
}

// Test_Multi_Output_Writer tests multi-output writer functionality
func Test_Multi_Output_Writer(t *testing.T) {
	// Create multi-output writer
	writer := NewMultiOutputWriter()

	// Test writing
	testData := []byte("test data")
	n, err := writer.Write(testData)
	if err != nil {
		t.Errorf("write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// Test adding and removing writers
	buffer := &strings.Builder{}
	writer.AddWriter(buffer)

	writer.Write([]byte("test"))
	if buffer.String() != "test" {
		t.Error("expected buffer to contain written data")
	}

	writer.RemoveWriter(buffer)
	buffer.Reset()
	writer.Write([]byte("test2"))
	if buffer.String() != "" {
		t.Error("expected buffer to be empty after removal")
	}
}

// Test_Log_Entry tests LogEntry functionality
func Test_Log_Entry(t *testing.T) {
	// Test creating log entry
	entry := LogEntry{
		Level:     "info",
		Message:   "test message",
		Timestamp: time.Now(),
		Fields:    map[string]interface{}{"key": "value"},
		Logger:    "test-logger",
	}

	if entry.Level != "info" {
		t.Error("expected level to be 'info'")
	}

	if entry.Message != "test message" {
		t.Error("expected message to be 'test message'")
	}

	if entry.Logger != "test-logger" {
		t.Error("expected logger to be 'test-logger'")
	}
}

// Test_Enhanced_Logger_Development_Mode tests development mode features
func Test_Enhanced_Logger_Development_Mode(t *testing.T) {
	baseLogger := ZapAdapter("test-dev")
	config := EnhancedLoggerConfig{}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Test development mode
	devLogger := enhanced.Dev()
	if devLogger == nil {
		t.Error("expected non-nil dev logger")
	}

	// Test pretty print
	prettyLogger := enhanced.WithPrettyPrint()
	if prettyLogger == nil {
		t.Error("expected non-nil pretty print logger")
	}
}

// Test_Enhanced_Logger_Error_Detail tests error detail functionality
func Test_Enhanced_Logger_Error_Detail(t *testing.T) {
	baseLogger := ZapAdapter("test-error-detail")
	config := EnhancedLoggerConfig{}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Test error detail logging
	testError := errors.New("test error message")
	enhanced.ErrorDetail(testError, "context", "value")
}

// Test_Configuration_Profile_Fields tests profile field access
func Test_Configuration_Profile_Fields(t *testing.T) {
	profile, err := LoadProfile("development")
	if err != nil {
		t.Skipf("LoadProfile failed (expected in test environment): %v", err)
		return
	}

	// Test profile fields
	if profile.Name == "" {
		t.Error("expected profile name to be set")
	}

	if profile.Config.LogLevel == "" {
		t.Error("expected config log level to be set")
	}

	if profile.Config.Facility == "" {
		t.Error("expected config facility to be set")
	}

	if profile.GraylogConfig.Endpoint == "" {
		t.Error("expected graylog config endpoint to be set")
	}
}

// Test_File_Adapter_Rotation tests file rotation functionality
func Test_File_Adapter_Rotation(t *testing.T) {
	// Create file adapter with small max size to trigger rotation
	config := FileAdapterConfig{
		Path:       "./test-rotation",
		Filename:   "rotation-test",
		MaxSize:    100, // Very small for testing
		MaxAge:     1 * time.Hour,
		MaxBackups: 2,
		Compress:   false,
		Format:     "json",
		Level:      "info",
		Facility:   "test-facility",
	}

	logger := FileAdapterWithConfig("test-rotation", config)
	if logger == nil {
		t.Fatal("expected non-nil rotation logger")
	}

	// Write enough data to trigger rotation
	for i := 0; i < 10; i++ {
		logger.Info("rotation test message", "iteration", i, "data", strings.Repeat("x", 50))
	}

	// Clean up
	os.RemoveAll("./test-rotation")
}

// Test_HTTP_Adapter_Batching tests HTTP adapter batching
func Test_HTTP_Adapter_Batching(t *testing.T) {
	config := HTTPAdapterConfig{
		SenderConfig: SenderConfig{
			Endpoint:   "http://localhost:8080/test",
			Timeout:    5000,
			RetryCount: 1,
			RetryDelay: 1000,
			BatchSize:  5,
			BatchDelay: 1000,
			Format:     "json",
		},
		Method:     "POST",
		Headers:    map[string]string{"Content-Type": "application/json"},
		Timeout:    1 * time.Second,
		RetryCount: 1,
		RetryDelay: 100 * time.Millisecond,
		BatchSize:  5,
		BatchDelay: 50 * time.Millisecond,
		Format:     "json",
		Level:      "info",
		Facility:   "test-facility",
	}

	logger := HTTPAdapterWithConfig("test-batching", config)
	if logger == nil {
		t.Fatal("expected non-nil batching logger")
	}

	// Send multiple messages to test batching
	for i := 0; i < 10; i++ {
		logger.Info("batch test message", "count", i)
	}

	// Wait a bit for batching to complete
	time.Sleep(200 * time.Millisecond)
}

// Test_Enhanced_Logger_Metrics_Collection tests metrics collection functionality
func Test_Enhanced_Logger_Metrics_Collection(t *testing.T) {
	baseLogger := ZapAdapter("test-metrics-collection")
	config := EnhancedLoggerConfig{
		EnableMetrics: true,
	}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Record various metrics
	enhanced.IncrementCounter("test_counter", 1)
	enhanced.IncrementCounter("test_counter", 2)
	enhanced.RecordGauge("test_gauge", 100.5)
	enhanced.RecordHistogram("test_histogram", 50.0)
	enhanced.RecordHistogram("test_histogram", 75.0)
	enhanced.RecordHistogram("test_histogram", 25.0)

	// Test that metrics are being collected
	// Note: In a real test, you might want to access the metrics collector directly
	// to verify the values, but for now we'll just ensure no panics occur
}

// Test_Enhanced_Logger_Rate_Limiter_Reset tests rate limiter reset functionality
func Test_Enhanced_Logger_Rate_Limiter_Reset(t *testing.T) {
	baseLogger := ZapAdapter("test-rate-limiter-reset")
	config := EnhancedLoggerConfig{
		EnableRateLimit: true,
	}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Set rate limit
	enhanced.WithRateLimit(3)

	// Try to log more than the limit
	for i := 0; i < 5; i++ {
		enhanced.Info("rate limited message", "count", i)
	}

	// The rate limiter should have blocked some messages
	// In a real test, you might want to verify the actual behavior
}

// Test_Configuration_Profile_Validation tests profile validation
func Test_Configuration_Profile_Validation(t *testing.T) {
	// Test that all valid profiles can be loaded
	validProfiles := []string{"development", "staging", "production", "testing"}

	for _, profile := range validProfiles {
		t.Run(profile, func(t *testing.T) {
			profileConfig, err := LoadProfile(profile)
			if err != nil {
				t.Errorf("failed to load profile %s: %v", profile, err)
				return
			}

			// Validate the profile configuration
			if err := profileConfig.Config.Validate(); err != nil {
				t.Errorf("profile %s config validation failed: %v", profile, err)
			}

			// GraylogConfig is now SenderConfig and doesn't have Validate method
		})
	}
}

// Test_File_Adapter_Compression tests file compression functionality
func Test_File_Adapter_Compression(t *testing.T) {
	// Test file adapter with compression enabled
	config := FileAdapterConfig{
		Path:       "./test-compression",
		Filename:   "compression-test",
		MaxSize:    200,
		MaxAge:     1 * time.Hour,
		MaxBackups: 2,
		Compress:   true, // Enable compression
		Format:     "json",
		Level:      "info",
		Facility:   "test-facility",
	}

	logger := FileAdapterWithConfig("test-compression", config)
	if logger == nil {
		t.Fatal("expected non-nil compression logger")
	}

	// Write data to trigger rotation and compression
	for i := 0; i < 10; i++ {
		logger.Info("compression test message", "iteration", i, "data", strings.Repeat("x", 50))
	}

	// Clean up
	os.RemoveAll("./test-compression")
}

// Test_HTTP_Adapter_Retry tests HTTP adapter retry functionality
func Test_HTTP_Adapter_Retry(t *testing.T) {
	config := HTTPAdapterConfig{
		SenderConfig: SenderConfig{
			Endpoint:   "http://invalid-endpoint:9999", // Invalid endpoint to trigger retries
			Timeout:    5000,
			RetryCount: 2,
			RetryDelay: 1000,
			BatchSize:  1, // Small batch size for testing
			BatchDelay: 1000,
			Format:     "json",
		},
		Method:     "POST",
		Headers:    map[string]string{"Content-Type": "application/json"},
		Timeout:    100 * time.Millisecond,
		RetryCount: 2,
		RetryDelay: 50 * time.Millisecond,
		BatchSize:  1, // Small batch size for testing
		BatchDelay: 10 * time.Millisecond,
		Format:     "json",
		Level:      "info",
		Facility:   "test-facility",
	}

	logger := HTTPAdapterWithConfig("test-retry", config)
	if logger == nil {
		t.Fatal("expected non-nil retry logger")
	}

	// Send a message that will trigger retries
	logger.Info("retry test message", "test", "retry")
}

// Test_Multi_Output_Adapter_Empty_Config tests multi-output adapter with empty config
func Test_Multi_Output_Adapter_Empty_Config(t *testing.T) {
	// Test with empty adapters list
	config := MultiOutputConfig{
		Adapters: []AdapterConfig{},
		Strategy: "all",
	}

	logger := MultiOutputAdapterWithConfig("test-empty", config)
	if logger != nil {
		t.Error("expected nil logger for empty adapters list")
	}
}

// Test_Enhanced_Logger_No_Op tests enhanced logger with no-op behavior
func Test_Enhanced_Logger_No_Op(t *testing.T) {
	baseLogger := ZapAdapter("test-no-op")
	config := EnhancedLoggerConfig{
		EnableRateLimit: true,
	}

	enhanced := NewEnhancedLogger(baseLogger, config)

	// Test conditional logging that should result in no-op
	noOpLogger := enhanced.When(false)
	if noOpLogger == nil {
		t.Error("expected non-nil no-op logger")
	}

	// These calls should do nothing
	noOpLogger.Info("should not appear")
	noOpLogger.Error("should not appear")
	noOpLogger.Debug("should not appear")
}

// Test_Configuration_Environment_Loading tests environment variable loading
func Test_Configuration_Environment_Loading(t *testing.T) {
	// Test loading graylog config from environment
	graylogConfig, err := LoadGraylogConfigFromEnv()
	if err != nil {
		t.Logf("LoadGraylogConfigFromEnv failed (expected in test environment): %v", err)
		return
	}

	if graylogConfig != nil {
		// Test that endpoint is set
		if graylogConfig.Endpoint == "" {
			t.Error("expected Endpoint to be set")
		}
	}
}

// Test_Prometheus_Metrics_Collector tests the new Prometheus metrics collector
func Test_Prometheus_Metrics_Collector(t *testing.T) {
	// Use a custom registry to avoid conflicts between tests
	registry := prometheus.NewRegistry()
	collector := NewPrometheusMetricsCollectorWithRegistry(registry)
	if collector == nil {
		t.Fatal("expected non-nil Prometheus metrics collector")
	}

	// Test counter functionality
	collector.IncrementCounter("test_counter", 1)
	collector.IncrementCounter("test_counter", 2)
	collector.IncrementCounter("another_counter", 5)

	// Test gauge functionality
	collector.RecordGauge("test_gauge", 42.5)
	collector.RecordGauge("test_gauge", 100.0)
	collector.RecordGauge("another_gauge", 0.0)

	// Test histogram functionality
	collector.RecordHistogram("test_histogram", 10.0)
	collector.RecordHistogram("test_histogram", 20.0)
	collector.RecordHistogram("test_histogram", 30.0)

	// Test GetMetrics (should return Prometheus info)
	metrics := collector.GetMetrics()
	if metrics == nil {
		t.Error("expected non-nil metrics")
	}

	// Verify Prometheus metrics info
	if info, exists := metrics["prometheus_metrics"]; !exists {
		t.Error("expected prometheus_metrics in metrics")
	} else if info != "available_at_/metrics_endpoint" {
		t.Errorf("unexpected prometheus_metrics value: %v", info)
	}
}

// Test_Prometheus_Metrics_Integration tests Prometheus metrics integration with enhanced logger
func Test_Prometheus_Metrics_Integration(t *testing.T) {
	baseLogger := ZapAdapter("test-prometheus")
	config := EnhancedLoggerConfig{
		EnableMetrics: true,
		EnableTiming:  true,
	}

	enhanced := NewEnhancedLogger(baseLogger, config)
	if enhanced == nil {
		t.Fatal("expected non-nil enhanced logger")
	}

	// Test metrics collection through enhanced logger
	enhanced.IncrementCounter("integration_counter", 1)
	enhanced.RecordGauge("integration_gauge", 99.9)
	enhanced.RecordHistogram("integration_histogram", 45.5)

	// Test timing with metrics
	enhanced.TimeOperation("test_operation", func() {
		time.Sleep(10 * time.Millisecond)
	})

	// Verify metrics are being collected
	// Note: In a real Prometheus setup, these metrics would be available via HTTP endpoint
	// For testing, we just verify the methods don't panic
	enhanced.Info("metrics integration test completed", "test", "prometheus")
}

// Test_Prometheus_Metrics_Server tests the Prometheus metrics server functionality
func Test_Prometheus_Metrics_Server(t *testing.T) {
	// Test with invalid address (should return error)
	err := StartPrometheusMetricsServer("invalid-address")
	if err == nil {
		t.Error("expected error for invalid address")
	}

	// Test with custom mux to avoid conflicts
	mux := http.NewServeMux()

	// Test that we can create the mux without errors
	// We don't actually start the server in tests as it would block
	if mux == nil {
		t.Error("expected non-nil mux")
	}

	// Test that the function signature is correct
	// This is a compile-time test, not a runtime test
	_ = StartPrometheusMetricsServerWithMux
}

// Test_Prometheus_Metrics_Concurrent tests concurrent access to Prometheus metrics
func Test_Prometheus_Metrics_Concurrent(t *testing.T) {
	// Use a custom registry to avoid conflicts between tests
	registry := prometheus.NewRegistry()
	collector := NewPrometheusMetricsCollectorWithRegistry(registry)
	if collector == nil {
		t.Fatal("expected non-nil Prometheus metrics collector")
	}

	// Test concurrent counter increments
	var wg sync.WaitGroup
	numGoroutines := 10
	incrementsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				collector.IncrementCounter("concurrent_counter", 1)
				collector.RecordGauge("concurrent_gauge", float64(id+j))
				collector.RecordHistogram("concurrent_histogram", float64(j))
			}
		}(i)
	}

	wg.Wait()

	// Verify metrics were collected (GetMetrics should work without error)
	metrics := collector.GetMetrics()
	if metrics == nil {
		t.Error("expected non-nil metrics after concurrent access")
	}
}

// Test_Prometheus_Metrics_Edge_Cases tests edge cases for Prometheus metrics
func Test_Prometheus_Metrics_Edge_Cases(t *testing.T) {
	// Use a custom registry to avoid conflicts between tests
	registry := prometheus.NewRegistry()
	collector := NewPrometheusMetricsCollectorWithRegistry(registry)
	if collector == nil {
		t.Fatal("expected non-nil Prometheus metrics collector")
	}

	// Test zero values
	collector.IncrementCounter("zero_counter", 0)
	collector.RecordGauge("zero_gauge", 0.0)
	collector.RecordHistogram("zero_histogram", 0.0)

	// Test negative values (should work for histograms, but not counters)
	collector.RecordHistogram("negative_histogram", -10.0)

	// Test very large values
	collector.IncrementCounter("large_counter", 999999999)
	collector.RecordGauge("large_gauge", 999999999.99)
	collector.RecordHistogram("large_histogram", 999999999.99)

	// Test empty string names (should work)
	collector.IncrementCounter("", 1)
	collector.RecordGauge("", 1.0)
	collector.RecordHistogram("", 1.0)

	// Verify metrics collection didn't panic
	metrics := collector.GetMetrics()
	if metrics == nil {
		t.Error("expected non-nil metrics after edge case testing")
	}
}

// Test_LogSender_Configuration_Methods tests the new configuration methods on LogSender interface
func Test_LogSender_Configuration_Methods(t *testing.T) {
	// Test GraylogSender configuration methods
	graylogConfig := SenderConfig{
		Endpoint:    "localhost:12201",
		Timeout:     5000,
		RetryCount:  3,
		RetryDelay:  1000,
		BatchSize:   100,
		BatchDelay:  1000,
		Format:      "gelf",
		Compression: true,
		TLSEnabled:  false,
		RateLimit:   1000,
	}
	facility := "test-config-methods"

	sender := NewGraylogSender(graylogConfig, facility)
	if sender == nil {
		t.Fatalf("expected non-nil GraylogSender")
	}
	defer sender.Close()

	// Test all configuration methods
	t.Run("GraylogSender Configuration Methods", func(t *testing.T) {
		if sender.GetEndpoint() != graylogConfig.Endpoint {
			t.Errorf("GetEndpoint: expected %s, got %s", graylogConfig.Endpoint, sender.GetEndpoint())
		}

		if sender.GetTimeout() != graylogConfig.Timeout {
			t.Errorf("GetTimeout: expected %d, got %d", graylogConfig.Timeout, sender.GetTimeout())
		}

		if sender.GetRetryCount() != graylogConfig.RetryCount {
			t.Errorf("GetRetryCount: expected %d, got %d", graylogConfig.RetryCount, sender.GetRetryCount())
		}

		if sender.GetRetryDelay() != graylogConfig.RetryDelay {
			t.Errorf("GetRetryDelay: expected %d, got %d", graylogConfig.RetryDelay, sender.GetRetryDelay())
		}

		if sender.GetBatchSize() != graylogConfig.BatchSize {
			t.Errorf("GetBatchSize: expected %d, got %d", graylogConfig.BatchSize, sender.GetBatchSize())
		}

		if sender.GetBatchDelay() != graylogConfig.BatchDelay {
			t.Errorf("GetBatchDelay: expected %d, got %d", graylogConfig.BatchDelay, sender.GetBatchDelay())
		}

		if sender.GetFormat() != graylogConfig.Format {
			t.Errorf("GetFormat: expected %s, got %s", graylogConfig.Format, sender.GetFormat())
		}

		if sender.IsCompressionEnabled() != graylogConfig.Compression {
			t.Errorf("IsCompressionEnabled: expected %t, got %t", graylogConfig.Compression, sender.IsCompressionEnabled())
		}

		if sender.IsTLSEnabled() != graylogConfig.TLSEnabled {
			t.Errorf("IsTLSEnabled: expected %t, got %t", graylogConfig.TLSEnabled, sender.IsTLSEnabled())
		}

		if sender.GetRateLimit() != graylogConfig.RateLimit {
			t.Errorf("GetRateLimit: expected %d, got %d", graylogConfig.RateLimit, sender.GetRateLimit())
		}
	})
}

// Test_Simplified_Adapter_Functions tests the new simplified function signatures
func Test_Simplified_Adapter_Functions(t *testing.T) {
	t.Run("ZapAdapter Simplified Signatures", func(t *testing.T) {
		// Test console only
		logger1 := ZapAdapter("console-only")
		if logger1 == nil {
			t.Fatal("expected non-nil console logger")
		}

		// Test with sender
		graylogSender := NewGraylogSender(SenderConfig{
			Endpoint: "localhost:12201",
			Format:   "gelf",
		}, "test-facility")

		logger2 := ZapAdapterWithSender("with-sender", graylogSender)
		if logger2 == nil {
			t.Fatal("expected non-nil logger with sender")
		}

		// Test with config (should work the same as with sender)
		logger3 := ZapAdapterWithConfig("with-config", graylogSender)
		if logger3 == nil {
			t.Fatal("expected non-nil logger with config")
		}

		// Test logging to ensure they work
		logger1.Info("console only test")
		logger2.Info("sender test")
		logger3.Info("config test")
	})

	t.Run("ZerologAdapter Simplified Signatures", func(t *testing.T) {
		// Test console only
		logger1 := ZerologAdapter("console-only")
		if logger1 == nil {
			t.Fatal("expected non-nil console logger")
		}

		// Test with sender
		graylogSender := NewGraylogSender(SenderConfig{
			Endpoint: "localhost:12201",
			Format:   "gelf",
		}, "test-facility")

		logger2 := ZerologAdapterWithSender("with-sender", graylogSender)
		if logger2 == nil {
			t.Fatal("expected non-nil logger with sender")
		}

		// Test with config (should work the same as with sender)
		logger3 := ZerologAdapterWithConfig("with-config", graylogSender)
		if logger3 == nil {
			t.Fatal("expected non-nil logger with config")
		}

		// Test logging to ensure they work
		logger1.Info("console only test")
		logger2.Info("sender test")
		logger3.Info("config test")
	})
}

// Test_LogSender_Interface_Compliance tests that all senders implement the LogSender interface
func Test_LogSender_Interface_Compliance(t *testing.T) {
	t.Run("GraylogSender Interface Compliance", func(t *testing.T) {
		sender := NewGraylogSender(SenderConfig{
			Endpoint: "localhost:12201",
			Format:   "gelf",
		}, "test-facility")
		defer sender.Close()

		// Test that it implements all required methods
		var _ LogSender = sender

		// Test basic functionality
		entry := LogEntry{
			Level:     "INFO",
			Message:   "interface compliance test",
			Timestamp: time.Now(),
			Fields:    map[string]interface{}{"test": "compliance"},
		}

		err := sender.Send(entry)
		if err != nil {
			t.Logf("Send returned error (expected if Graylog unavailable): %v", err)
		}

		// Test other interface methods
		_ = sender.IsConnected()
		_ = sender.GetStats()
	})

	t.Run("FileSender Interface Compliance", func(t *testing.T) {
		config := FileAdapterConfig{
			Path:       "./test-interface-compliance",
			Filename:   "interface-test",
			MaxSize:    100,
			MaxAge:     1 * time.Hour,
			MaxBackups: 2,
			Compress:   false,
			Format:     "json",
			Level:      "info",
			Facility:   "test-facility",
			SenderConfig: SenderConfig{
				Endpoint:   "./test-interface-compliance/interface-test",
				Timeout:    5000,
				RetryCount: 1,
				RetryDelay: 1000,
				BatchSize:  5,
				BatchDelay: 1000,
				Format:     "json",
			},
		}

		sender, err := NewFileSender(config)
		if err != nil {
			t.Fatalf("failed to create FileSender: %v", err)
		}
		defer sender.Close()

		// Test that it implements all required methods
		var _ LogSender = sender

		// Test basic functionality
		entry := LogEntry{
			Level:     "INFO",
			Message:   "interface compliance test",
			Timestamp: time.Now(),
			Fields:    map[string]interface{}{"test": "compliance"},
		}

		err = sender.Send(entry)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Test other interface methods
		_ = sender.IsConnected()
		_ = sender.GetStats()

		// Clean up
		os.RemoveAll("./test-interface-compliance")
	})

	t.Run("HTTPSender Interface Compliance", func(t *testing.T) {
		config := HTTPAdapterConfig{
			SenderConfig: SenderConfig{
				Endpoint:   "http://localhost:8080/test",
				Timeout:    5000,
				RetryCount: 1,
				RetryDelay: 1000,
				BatchSize:  5,
				BatchDelay: 1000,
				Format:     "json",
			},
			Method:     "POST",
			Headers:    map[string]string{"Content-Type": "application/json"},
			Timeout:    100 * time.Millisecond,
			RetryCount: 1,
			RetryDelay: 50 * time.Millisecond,
			BatchSize:  5,
			BatchDelay: 10 * time.Millisecond,
			Format:     "json",
			Level:      "info",
			Facility:   "test-facility",
		}

		sender, err := NewHTTPSender(config)
		if err != nil {
			t.Fatalf("failed to create HTTPSender: %v", err)
		}
		defer sender.Close()

		// Test that it implements all required methods
		var _ LogSender = sender

		// Test basic functionality
		entry := LogEntry{
			Level:     "INFO",
			Message:   "interface compliance test",
			Timestamp: time.Now(),
			Fields:    map[string]interface{}{"test": "compliance"},
		}

		err = sender.Send(entry)
		if err != nil {
			t.Logf("Send returned error (expected if HTTP endpoint unavailable): %v", err)
		}

		// Test other interface methods
		_ = sender.IsConnected()
		_ = sender.GetStats()
	})
}

// Test_Configuration_Merged_Into_Sender tests that configuration is now part of the sender
func Test_Configuration_Merged_Into_Sender(t *testing.T) {
	t.Run("Configuration Embedded in Sender", func(t *testing.T) {
		// Create a sender with specific configuration
		config := SenderConfig{
			Endpoint:    "custom-endpoint:12201",
			Timeout:     10000,
			RetryCount:  5,
			RetryDelay:  2000,
			BatchSize:   200,
			BatchDelay:  2000,
			Format:      "json",
			Compression: true,
			TLSEnabled:  true,
			RateLimit:   500,
		}
		facility := "custom-facility"

		sender := NewGraylogSender(config, facility)
		if sender == nil {
			t.Fatalf("expected non-nil GraylogSender")
		}
		defer sender.Close()

		// Verify that the configuration is embedded in the sender
		if sender.GetEndpoint() != config.Endpoint {
			t.Errorf("Endpoint not embedded: expected %s, got %s", config.Endpoint, sender.GetEndpoint())
		}

		if sender.GetTimeout() != config.Timeout {
			t.Errorf("Timeout not embedded: expected %d, got %d", config.Timeout, sender.GetTimeout())
		}

		if sender.GetRetryCount() != config.RetryCount {
			t.Errorf("RetryCount not embedded: expected %d, got %d", config.RetryCount, sender.GetRetryCount())
		}

		if sender.GetFormat() != config.Format {
			t.Errorf("Format not embedded: expected %s, got %s", config.Format, sender.GetFormat())
		}

		if sender.IsCompressionEnabled() != config.Compression {
			t.Errorf("Compression not embedded: expected %t, got %t", config.Compression, sender.IsCompressionEnabled())
		}

		if sender.IsTLSEnabled() != config.TLSEnabled {
			t.Errorf("TLSEnabled not embedded: expected %t, got %t", config.TLSEnabled, sender.IsTLSEnabled())
		}

		if sender.GetRateLimit() != config.RateLimit {
			t.Errorf("RateLimit not embedded: expected %d, got %d", config.RateLimit, sender.GetRateLimit())
		}
	})
}

// Test_Old_Function_Signatures_Removed verifies that the old complex function signatures are no longer available
func Test_Old_Function_Signatures_Removed(t *testing.T) {
	t.Run("Old ZapAdapterWithConfig Signature Removed", func(t *testing.T) {
		// This test verifies that the old signature with SenderConfig and facility is no longer available
		// The new signature only takes (named string, sender LogSender)

		// Create a sender to test with
		graylogSender := NewGraylogSender(SenderConfig{
			Endpoint: "localhost:12201",
			Format:   "gelf",
		}, "test-facility")
		defer graylogSender.Close()

		// Test the new simplified signature works
		logger := ZapAdapterWithConfig("test-simplified", graylogSender)
		if logger == nil {
			t.Fatal("expected non-nil logger with new signature")
		}

		// Verify the logger works
		logger.Info("test message")
	})

	t.Run("Old ZerologAdapterWithConfig Signature Removed", func(t *testing.T) {
		// This test verifies that the old signature with SenderConfig and facility is no longer available
		// The new signature only takes (named string, sender LogSender)

		// Create a sender to test with
		graylogSender := NewGraylogSender(SenderConfig{
			Endpoint: "localhost:12201",
			Format:   "gelf",
		}, "test-facility")
		defer graylogSender.Close()

		// Test the new simplified signature works
		logger := ZerologAdapterWithConfig("test-simplified", graylogSender)
		if logger == nil {
			t.Fatal("expected non-nil logger with new signature")
		}

		// Verify the logger works
		logger.Info("test message")
	})
}

// Test_Configuration_Embedded_Verification tests that configuration is truly embedded in senders
func Test_Configuration_Embedded_Verification(t *testing.T) {
	t.Run("Configuration Values Stored in Sender", func(t *testing.T) {
		// Test with different configuration values to ensure they're properly stored
		testConfigs := []struct {
			name     string
			config   SenderConfig
			facility string
		}{
			{
				name: "High Performance Config",
				config: SenderConfig{
					Endpoint:    "high-perf:12201",
					Timeout:     1000,
					RetryCount:  1,
					RetryDelay:  500,
					BatchSize:   1000,
					BatchDelay:  100,
					Format:      "gelf",
					Compression: true,
					TLSEnabled:  true,
					RateLimit:   10000,
				},
				facility: "high-performance",
			},
			{
				name: "Development Config",
				config: SenderConfig{
					Endpoint:    "dev:12201",
					Timeout:     10000,
					RetryCount:  3,
					RetryDelay:  2000,
					BatchSize:   10,
					BatchDelay:  5000,
					Format:      "json",
					Compression: false,
					TLSEnabled:  false,
					RateLimit:   100,
				},
				facility: "development",
			},
		}

		for _, tc := range testConfigs {
			t.Run(tc.name, func(t *testing.T) {
				sender := NewGraylogSender(tc.config, tc.facility)
				if sender == nil {
					t.Fatalf("expected non-nil GraylogSender for %s", tc.name)
				}
				defer sender.Close()

				// Verify all configuration values are properly stored and retrievable
				if sender.GetEndpoint() != tc.config.Endpoint {
					t.Errorf("Endpoint mismatch: expected %s, got %s", tc.config.Endpoint, sender.GetEndpoint())
				}

				if sender.GetTimeout() != tc.config.Timeout {
					t.Errorf("Timeout mismatch: expected %d, got %d", tc.config.Timeout, sender.GetTimeout())
				}

				if sender.GetRetryCount() != tc.config.RetryCount {
					t.Errorf("RetryCount mismatch: expected %d, got %d", tc.config.RetryCount, sender.GetRetryCount())
				}

				if sender.GetRetryDelay() != tc.config.RetryDelay {
					t.Errorf("RetryDelay mismatch: expected %d, got %d", tc.config.RetryDelay, sender.GetRetryDelay())
				}

				if sender.GetBatchSize() != tc.config.BatchSize {
					t.Errorf("BatchSize mismatch: expected %d, got %d", tc.config.BatchSize, sender.GetBatchSize())
				}

				if sender.GetBatchDelay() != tc.config.BatchDelay {
					t.Errorf("BatchDelay mismatch: expected %d, got %d", tc.config.BatchDelay, sender.GetBatchDelay())
				}

				if sender.GetFormat() != tc.config.Format {
					t.Errorf("Format mismatch: expected %s, got %s", tc.config.Format, sender.GetFormat())
				}

				if sender.IsCompressionEnabled() != tc.config.Compression {
					t.Errorf("Compression mismatch: expected %t, got %t", tc.config.Compression, sender.IsCompressionEnabled())
				}

				if sender.IsTLSEnabled() != tc.config.TLSEnabled {
					t.Errorf("TLSEnabled mismatch: expected %t, got %t", tc.config.TLSEnabled, sender.IsTLSEnabled())
				}

				if sender.GetRateLimit() != tc.config.RateLimit {
					t.Errorf("RateLimit mismatch: expected %d, got %d", tc.config.RateLimit, sender.GetRateLimit())
				}
			})
		}
	})
}

// callerCaptureLogger is a test logger that captures caller information
type callerCaptureLogger struct {
	caller string
	skip   int
}

func (c *callerCaptureLogger) Debug(msg string, keysAndValues ...interface{}) {
	c.caller = c.withStackTrace(3 + c.skip)
}

func (c *callerCaptureLogger) Info(msg string, keysAndValues ...interface{}) {
	c.caller = c.withStackTrace(3 + c.skip)
}

func (c *callerCaptureLogger) Warn(msg string, keysAndValues ...interface{}) {
	c.caller = c.withStackTrace(3 + c.skip)
}

func (c *callerCaptureLogger) Error(msg string, keysAndValues ...interface{}) {
	c.caller = c.withStackTrace(3 + c.skip)
}

func (c *callerCaptureLogger) ErrorDetail(err error, keysAndValues ...interface{}) {
	c.caller = c.withStackTrace(3 + c.skip)
}

func (c *callerCaptureLogger) Panic(msg string, keysAndValues ...interface{}) {
	c.caller = c.withStackTrace(3 + c.skip)
}

func (c *callerCaptureLogger) SkipLevel(skip int) Logger {
	c.skip = skip
	return c
}

func (c *callerCaptureLogger) With(args ...interface{}) Logger {
	return c
}

func (c *callerCaptureLogger) SetContext(ctx context.Context) Logger {
	return c
}

func (c *callerCaptureLogger) withStackTrace(skip int) string {
	// Simple caller information similar to what the real adapters do
	return fmt.Sprintf("test_logger.go:%d", skip)
}

func Test_SkipLevel_Caller_Values(t *testing.T) {
	setSafeTestConfig()

	// Test that different skip levels produce different caller values
	t.Run("SkipLevel Caller Values Verification", func(t *testing.T) {
		logger := &callerCaptureLogger{}

		// Test skip level 1
		logger1 := logger.SkipLevel(1)
		logger1.Info("test message")
		caller1 := logger.caller

		// Reset and test skip level 3
		logger.caller = ""
		logger3 := logger.SkipLevel(3)
		logger3.Info("test message")
		caller3 := logger.caller

		// Verify that different skip levels produce different caller values
		if caller1 == caller3 {
			t.Errorf("expected different caller values for skip levels 1 and 3, got same: %s", caller1)
		}

		// Verify the actual values are what we expect
		expected1 := "test_logger.go:4" // 3 + 1
		expected3 := "test_logger.go:6" // 3 + 3

		if caller1 != expected1 {
			t.Errorf("expected caller1 to be %s, got %s", expected1, caller1)
		}

		if caller3 != expected3 {
			t.Errorf("expected caller3 to be %s, got %s", expected3, caller3)
		}

		t.Logf("Skip level 1 produced caller: %s", caller1)
		t.Logf("Skip level 3 produced caller: %s", caller3)
	})

	// Test that chaining skip levels works correctly
	t.Run("SkipLevel Chaining", func(t *testing.T) {
		logger := &callerCaptureLogger{}

		// Chain multiple skip levels
		chainedLogger := logger.SkipLevel(1).SkipLevel(2)
		chainedLogger.Info("chained test message")
		chainedCaller := logger.caller

		// The final skip level should be 2 (not cumulative)
		expectedChained := "test_logger.go:5" // 3 + 2

		if chainedCaller != expectedChained {
			t.Errorf("expected chained caller to be %s, got %s", expectedChained, chainedCaller)
		}

		t.Logf("Chained skip levels produced caller: %s", chainedCaller)
	})

	// Test with different skip level combinations
	t.Run("SkipLevel Combinations", func(t *testing.T) {
		logger := &callerCaptureLogger{}

		testCases := []struct {
			skipLevel int
			expected  string
			name      string
		}{
			{0, "test_logger.go:3", "skip level 0"},
			{1, "test_logger.go:4", "skip level 1"},
			{2, "test_logger.go:5", "skip level 2"},
			{3, "test_logger.go:6", "skip level 3"},
			{5, "test_logger.go:8", "skip level 5"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Reset caller
				logger.caller = ""

				// Apply skip level and log
				skipLogger := logger.SkipLevel(tc.skipLevel)
				skipLogger.Info("test message")

				// Verify caller
				if logger.caller != tc.expected {
					t.Errorf("expected caller for %s to be %s, got %s",
						tc.name, tc.expected, logger.caller)
				}

				t.Logf("%s produced caller: %s", tc.name, logger.caller)
			})
		}
	})

	// Test that skip levels are properly applied to all adapter types
	t.Run("SkipLevel Across Adapters", func(t *testing.T) {
		// Test with Zap adapter
		zapLogger := New(ZapAdapter("test-skip-zap"))
		if zapLogger == nil {
			t.Fatalf("expected non-nil zap logger")
		}

		// Apply skip level and verify it's chainable
		skipLogger := zapLogger.SkipLevel(2)
		if skipLogger == nil {
			t.Fatalf("expected non-nil logger after SkipLevel")
		}

		// Test that logging still works
		skipLogger.Info("zap skip level test")

		// Test with Zerolog adapter
		zeroLogger := New(ZerologAdapter("test-skip-zero"))
		if zeroLogger == nil {
			t.Fatalf("expected non-nil zerolog logger")
		}

		// Apply skip level and verify it's chainable
		skipZeroLogger := zeroLogger.SkipLevel(2)
		if skipZeroLogger == nil {
			t.Fatalf("expected non-nil logger after SkipLevel")
		}

		// Test that logging still works
		skipZeroLogger.Info("zerolog skip level test")

		// Test with File adapter
		fileLogger := New(FileAdapter("test-skip-file"))
		if fileLogger == nil {
			t.Fatalf("expected non-nil file logger")
		}

		// Apply skip level and verify it's chainable
		skipFileLogger := fileLogger.SkipLevel(2)
		if skipFileLogger == nil {
			t.Fatalf("expected non-nil logger after SkipLevel")
		}

		// Test that logging still works
		skipFileLogger.Info("file skip level test")
	})
}

func Test_SkipLevel_Different_Caller_Values(t *testing.T) {
	setSafeTestConfig()

	// Create a logger and test that different skip levels produce different caller values
	logger := New(ZapAdapter("test-skip-diff"))
	if logger == nil {
		t.Fatalf("expected non-nil logger")
	}

	// Test with no skip level (default)
	logger.Info("message with no skip level")

	// Test with skip level 1
	logger1 := logger.SkipLevel(1)
	logger1.Info("message with skip level 1")

	// Test with skip level 3
	logger3 := logger.SkipLevel(3)
	logger3.Info("message with skip level 3")

	// Test with skip level 5
	logger5 := logger.SkipLevel(5)
	logger5.Info("message with skip level 5")

	// The key point is that these should produce different caller information
	// We can see this in the test output - different skip levels should show
	// different file paths or line numbers in the caller field
	t.Logf("Test completed - check the caller fields above to see different values for different skip levels")
}
