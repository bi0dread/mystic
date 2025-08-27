# Mystic - Abstract Logging Package for Go

[![Go Report Card](https://goreportcard.com/badge/github.com/bi0dread/mystic)](https://goreportcard.com/report/github.com/bi0dread/mystic)
[![Go Version](https://img.shields.io/github/go-mod/go-version/bi0dread/mystic)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Mystic is a Go package that provides an abstract logging interface with automatic Graylog and OpenTelemetry integration. It's designed to be backend-agnostic, allowing you to plug in any logging library (zap, zerolog, etc.) while maintaining consistent behavior.

## Features

- **Adapter-based architecture**: Use any logging backend (zap, zerolog, etc.)
- **Automatic Graylog integration**: Structured logging to Graylog via GELF
- **OpenTelemetry support**: Automatic span events and context propagation
- **Zero-dependency core**: Core package has no external dependencies
- **Consistent interface**: Same API regardless of underlying logger
- **Performance**: Minimal overhead with efficient adapter implementations

## Quick Start

```go
package main

import (
    "github.com/bi0dread/mystic"
)

func main() {
    // Configure mystic
    mystic.SetConfig(mystic.Config{
        LogLevel:    "info",
        GrayLogAddr: "graylog.example.com:12201",
        Facility:    "my-app",
    })

    // Create logger with zap adapter
    logger := mystic.New(mystic.ZapAdapter("my-service"))
    
    // Use the logger
    logger.Info("Server started", "port", 8080)
    logger.Error("Connection failed", "error", "timeout")
}
```

## Installation

```bash
go get github.com/bi0dread/mystic
```

## Configuration

```go
type Config struct {
    LogLevel string `env:"LOG_LEVEL, required"`     // debug, info, warn, error
    Facility string `env:"FACILITY, required"`       // Application facility name
}

type GraylogSenderConfig struct {
    GrayLogAddr string `env:"GRAYLOG_ADDR"`            // Graylog server address
    Facility    string `env:"FACILITY, required"`       // Application facility name
}
```

### Environment Variables

#### Main Configuration
- `LOG_LEVEL`: Logging level (debug, info, warn, error)
- `FACILITY`: Application facility name

#### Graylog Configuration
- `GRAYLOG_ADDR`: Graylog server address (default: 0.0.0.0:12201)
- `FACILITY`: Application facility name (shared with main config)

## Usage

### Basic Logging

```go
logger := mystic.New(mystic.ZapAdapter("my-service"))

logger.Debug("Debug message")
logger.Info("Info message", "key", "value")
logger.Warn("Warning message", "count", 42)
logger.Error("Error message", "err", err)
logger.Panic("Panic message") // Use with caution
```

### Error Handling

```go
err := someOperation()
logger.ErrorDetail(err, "operation", "failed", "user_id", 123)
```

### Context and Chaining

```go
logger := mystic.New(mystic.ZapAdapter("my-service"))
    .With("service", "api")
    .SkipLevel(1)
    .SetContext(ctx)

logger.Info("Request processed")
```

### With Fields

```go
logger := mystic.New(mystic.ZapAdapter("my-service"))
    .With("user_id", 123, "session", "abc123")

logger.Info("User action", "action", "login")
// Output includes: user_id=123, session=abc123, action=login
```

## Available Adapters

### Zap Adapter

The zap adapter provides high-performance structured logging with OpenTelemetry integration and Graylog transport.

```go
import "github.com/bi0dread/mystic"

// Use default configuration (from environment variables)
logger := mystic.New(mystic.ZapAdapter("my-service"))

// Use custom Graylog configuration
customConfig := mystic.GraylogSenderConfig{
    GrayLogAddr: "custom-graylog:12201",
    Facility:    "my-custom-facility",
}
logger := func(name string) mystic.Logger {
    return mystic.ZapAdapterWithConfig(name, customConfig)
}, "my-service")
```

**Dependencies:**
- `go.uber.org/zap`
- `go.opentelemetry.io/otel/*`
- `github.com/bi0dread/jeeko/morgana`
- `github.com/Graylog2/go-gelf/gelf` (for Graylog transport)

```go
import "github.com/bi0dread/mystic"

logger := mystic.New(mystic.ZapAdapter("my-service"))
```

**Dependencies:**
- `go.uber.org/zap`
- `go.opentelemetry.io/otel/*`
- `github.com/bi0dread/jeeko/morgana`

### Zerolog Adapter

The zerolog adapter provides zero-allocation JSON logging with OpenTelemetry integration and Graylog transport.

```go
import "github.com/bi0dread/mystic"

// Use default configuration (from environment variables)
logger := mystic.New(mystic.ZerologAdapter("my-service"))

// Use custom Graylog configuration
customConfig := mystic.GraylogSenderConfig{
    GrayLogAddr: "custom-graylog:12201",
    Facility:    "my-custom-facility",
}
logger := func(name string) mystic.Logger {
    return mystic.ZerologAdapterWithConfig(name, customConfig)
}, "my-service")
```

**Dependencies:**
- `github.com/rs/zerolog`
- `go.opentelemetry.io/otel/*`
- `github.com/bi0dread/jeeko/morgana`
- `github.com/Graylog2/go-gelf/gelf` (for Graylog transport)

### File Adapter

The file adapter provides file-based logging with rotation, compression, and multiple output formats.

```go
import "github.com/bi0dread/mystic"

// Use default configuration
logger := mystic.New(mystic.FileAdapter("my-service"))

// Use custom configuration
fileConfig := mystic.FileAdapterConfig{
    Path:        "/var/log/myapp",
    Filename:    "application",
    MaxSize:     100 * 1024 * 1024, // 100MB
    MaxAge:      30 * 24 * time.Hour, // 30 days
    MaxBackups:  5,
    Compress:    true,
    Format:      "json", // "json", "console", "gelf"
    Level:       "info",
    Facility:    "myapp",
}
logger := func(name string) mystic.Logger {
    return mystic.FileAdapterWithConfig(name, fileConfig)
}, "my-service")
```

**Features:**
- Automatic file rotation based on size and age
- Compression of rotated files
- Multiple output formats (JSON, Console, GELF)
- Configurable log levels and facilities

### HTTP Adapter

The HTTP adapter sends logs to custom HTTP endpoints with batching, retries, and multiple formats.

```go
import "github.com/bi0dread/mystic"

// Use default configuration
logger := mystic.New(mystic.HTTPAdapter("my-service"))

// Use custom configuration
httpConfig := mystic.HTTPAdapterConfig{
    Endpoint:    "https://api.logs.com/ingest",
    Method:      "POST",
    Headers:     map[string]string{"Authorization": "Bearer token"},
    Timeout:     5 * time.Second,
    RetryCount:  3,
    RetryDelay:  1 * time.Second,
    BatchSize:   100,
    BatchDelay:  100 * time.Millisecond,
    Format:      "json", // "json", "gelf"
    Level:       "info",
    Facility:    "myapp",
}
logger := func(name string) mystic.Logger {
    return mystic.HTTPAdapterWithConfig(name, httpConfig)
}, "my-service")
```

**Features:**
- HTTP/HTTPS endpoint support
- Automatic batching for performance
- Configurable retry logic
- Multiple output formats
- Custom headers and authentication

### Multi-Output Adapter

The multi-output adapter sends logs to multiple destinations simultaneously with configurable strategies.

```go
import "github.com/bi0dread/mystic"

// Create multi-output configuration
multiConfig := mystic.MultiOutputConfig{
    Adapters: []mystic.AdapterConfig{
        {Name: "console", Adapter: mystic.ZapAdapter},
        {Name: "file", Adapter: mystic.FileAdapter},
        {Name: "graylog", Adapter: func(name string) mystic.Logger {
            return mystic.ZapAdapterWithConfig(name, mystic.GraylogSenderConfig{
                GrayLogAddr: "localhost:12201",
                Facility:    "myapp",
            })
        }},
    },
    Strategy: "all", // "all", "first_success", "round_robin"
}

logger := func(name string) mystic.Logger {
    return mystic.MultiOutputAdapterWithConfig(name, multiConfig)
}, "my-service")
```

**Strategies:**
- `all`: Send to all adapters
- `first_success`: Send to adapters until one succeeds
- `round_robin`: Distribute logs across adapters

## Enhanced Logging Features

Mystic provides advanced logging capabilities beyond basic logging:

### Structured Logging

```go
logger := mystic.New(mystic.ZapAdapter("my-service"))
enhanced := mystic.NewEnhancedLogger(logger, mystic.EnhancedLoggerConfig{
    EnableMetrics: true,
    EnableSampling: true,
    EnableTiming: true,
})

// Structured event logging
enhanced.Structured("user_login", map[string]interface{}{
    "user_id": 123,
    "ip_address": "192.168.1.1",
    "user_agent": "Mozilla/5.0...",
    "timestamp": time.Now(),
})
```

### Log Sampling

```go
// Sample debug logs at 10% rate
enhanced.Sampled("debug", 0.1, "high-volume-debug", "key", "value")

// Sample info logs at 50% rate
enhanced.Sampled("info", 0.5, "sampled-info", "metric", "value")
```

### Prometheus Metrics Collection

```go
// Enable metrics collection
enhanced.WithMetrics()

// Record counters
enhanced.IncrementCounter("requests_total", 1)
enhanced.IncrementCounter("errors_total", 1)

// Record gauges
enhanced.RecordGauge("active_connections", 42.5)

// Record histograms
enhanced.RecordHistogram("request_duration_ms", 150.2)

// Start Prometheus metrics server
go func() {
    if err := mystic.StartPrometheusMetricsServer(":8080"); err != nil {
        log.Printf("Failed to start metrics server: %v", err)
    }
}()

// Access metrics at: http://localhost:8080/metrics
```

**Available Metrics:**
- **Counters**: `{name}_total` - Incrementing counters for events
- **Gauges**: `{name}_current` - Current values that can go up and down
- **Histograms**: `{name}_duration` - Distribution of values with configurable buckets

**Prometheus Integration:**
- Metrics are automatically registered with Prometheus
- Standard Prometheus naming conventions
- Built-in HTTP endpoint for scraping
- Compatible with Grafana dashboards

### Performance Tracking

```go
// Enable timing
enhanced.WithTiming("database_query")

// Time an operation
enhanced.TimeOperation("database_query", func() {
    // Database query code here
    time.Sleep(100 * time.Millisecond)
})
```

### Rate Limiting

```go
// Limit to 100 logs per second
enhanced.WithRateLimit(100)

// Log messages (will be rate limited)
for i := 0; i < 1000; i++ {
    enhanced.Info("rate limited message", "count", i)
}
```

### Conditional Logging

```go
// Log only in development
enhanced.When(os.Getenv("ENV") == "development").Info("dev only message")

// Log based on context
enhanced.WhenContext(func(ctx context.Context) bool {
    return ctx.Value("debug_mode") == true
}).Debug("debug message")
```

### Batch Logging

```go
// Log multiple entries in batch
entries := []mystic.LogEntry{
    {Level: "info", Message: "batch entry 1", Fields: map[string]interface{}{"id": 1}},
    {Level: "info", Message: "batch entry 2", Fields: map[string]interface{}{"id": 2}},
    {Level: "error", Message: "batch entry 3", Fields: map[string]interface{}{"id": 3}},
}

enhanced.Batch(entries)
```

## Testing & Development

Mystic provides comprehensive testing utilities and development tools:

### Test Helpers

```go
import "github.com/bi0dread/mystic"

// Capture logs during testing
captured := mystic.CaptureLogs(func() {
    logger := mystic.ZapAdapter("test-service")
    logger.Info("test message")
    logger.Error("test error")
})

// Assert log contents
if !captured.Contains("test message") {
    t.Error("expected log to contain 'test message'")
}

if !captured.ContainsLevel("error") {
    t.Error("expected log to contain error level")
}

// Get specific entries
infoEntries := captured.GetEntriesByLevel("info")
errorEntries := captured.GetEntriesByMessage("test error")
```

### Mock Logger

```go
// Create mock logger with expectations
mockLogger := mystic.NewMockLogger()
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
```

### Test Logger

```go
// Create test logger that writes to buffer
testLogger := mystic.NewTestLogger()

// Log messages
testLogger.Info("test message", "key", "value")
testLogger.Error("test error")

// Get output
output := testLogger.GetOutput()
if !strings.Contains(output, "test message") {
    t.Error("expected output to contain 'test message'")
}

// Clear output
testLogger.Clear()
```

## Architecture

Mystic uses a layered approach where logging adapters can optionally use GraylogSender for transport:

### Configuration Flexibility

Mystic provides two levels of configuration:

1. **Global Configuration** - Set via environment variables and `SetConfig()`
2. **Per-Adapter Configuration** - Custom Graylog configuration for each adapter instance

This allows you to:
- Use different Graylog servers for different services
- Override facility names per logger instance
- Maintain backward compatibility with existing code
- Test with custom configurations without affecting global state

### Configuration Profiles

Mystic supports environment-specific configuration profiles:

```go
import "github.com/bi0dread/mystic"

// Load development profile
devProfile, err := mystic.LoadProfile("development")
if err != nil {
    log.Fatal(err)
}

// Apply profile configuration
mystic.SetConfig(devProfile.Config)

// Load staging profile
stagingProfile, err := mystic.LoadProfile("staging")
if err != nil {
    log.Fatal(err)
}

// Load production profile
prodProfile, err := mystic.LoadProfile("production")
if err != nil {
    log.Fatal(err)
}

// Load testing profile
testProfile, err := mystic.LoadProfile("testing")
if err != nil {
    log.Fatal(err)
}
```

**Available Profiles:**
- **development**: Debug level, localhost Graylog, mystic-dev facility
- **staging**: Info level, staging Graylog, mystic-staging facility  
- **production**: Warn level, production Graylog, mystic-prod facility
- **testing**: Debug level, no Graylog (fallback only), mystic-test facility

### Environment Variable Loading

```go
// Load configuration from environment variables
config, err := mystic.LoadConfigFromEnv()
if err != nil {
    log.Fatal(err)
}

// Validate configuration
if err := config.Validate(); err != nil {
    log.Fatal(err)
}

// Set defaults
config.SetDefaults()

// Apply configuration
mystic.SetConfig(*config)
```

```go
// High-performance structured logging with OpenTelemetry + Graylog transport
zapLogger := mystic.ZapAdapter("zap-service")

// Zero-allocation JSON logging with OpenTelemetry + Graylog transport  
zerologLogger := mystic.ZerologAdapter("zerolog-service")
```

The GraylogSender automatically handles:
- GELF protocol communication with Graylog servers
- Fallback to console output if Graylog is unavailable
- Structured field mapping and timestamp formatting

## Advanced Usage

### GraylogSender

For advanced use cases, you can use the GraylogSender directly:

```go
import "github.com/bi0dread/mystic"

// Create a custom GraylogSender
	graylogConfig := mystic.GraylogSenderConfig{
		GrayLogAddr: "localhost:12201",
		Facility:    "mystic-test",
	}
	sender := mystic.NewGraylogSender(graylogConfig)

// Send custom log entries
sender.Send("INFO", "Custom message", map[string]interface{}{
    "user_id": 123,
    "action": "login",
})

// Don't forget to close when done
defer sender.Close()
```

### Creating Custom Adapters

You can create custom adapters for any logging library by implementing the `Logger` interface:

```go
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

// Custom adapter constructor
func MyCustomAdapter(named string) Logger {
    // Implementation here
    return &myCustomLogger{}
}

// Usage
logger := MyCustomAdapter("my-service")
```

## Graylog Integration

Mystic automatically sends structured logs to Graylog using the GELF format. Each log entry includes:

- Log level
- Message
- Timestamp
- Facility name
- Custom fields
- Caller information

## OpenTelemetry Integration

When a context with an active span is set, mystic automatically:

- Adds log events to the current span
- Includes log level, logger name, and custom fields as attributes
- Preserves context for distributed tracing

## Performance

The adapter-based architecture ensures minimal performance overhead:

- Core package has zero external dependencies
- Adapters are optimized for their respective backends
- Efficient field handling and context propagation

## Testing

```bash
# Run all tests
go test ./...

# Run tests with specific adapter
go test -tags zap ./...
go test -tags zerolog ./...
```

## Examples

See the `examples/` directory for complete working examples:

- Basic usage with zap
- Basic usage with zerolog
- Custom adapter implementation
- Graylog integration
- OpenTelemetry integration

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Uber Zap](https://github.com/uber-go/zap) for the zap adapter
- [RS Zerolog](https://github.com/rs/zerolog) for the zerolog adapter
- [Graylog](https://www.graylog.org/) for GELF protocol support
- [OpenTelemetry](https://opentelemetry.io/) for observability standards

## Support

If you encounter any issues or have questions:

- Open an issue on GitHub
- Check the examples directory
- Review the test files for usage patterns

---

**Mystic** - Making logging magical across different backends! ✨
