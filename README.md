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
- **Simplified configuration**: Configuration is embedded directly in senders
- **Optional sender support**: Use any LogSender implementation or console-only logging
- **Unified LogSender interface**: Consistent API for all log destinations
- **Embedded configuration**: No more redundant parameter passing
- **Clean function signatures**: Simplified adapter creation methods

## Quick Start

```go
package main

import (
    "github.com/bi0dread/mystic"
)

func main() {
    // Create logger with zap adapter (console only)
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

type SenderConfig struct {
    Endpoint     string `env:"ENDPOINT"`            // Server endpoint (Graylog, HTTP, etc.)
    Timeout      int    `env:"TIMEOUT"`             // Timeout in milliseconds
    RetryCount   int    `env:"RETRY_COUNT"`         // Number of retry attempts
    RetryDelay   int    `env:"RETRY_DELAY"`         // Delay between retries in milliseconds
    BatchSize    int    `env:"BATCH_SIZE"`          // Batch size for sending logs
    BatchDelay   int    `env:"BATCH_DELAY"`         // Delay between batches in milliseconds
    Format       string `env:"FORMAT"`              // Output format (json, gelf, console)
    Compression  bool   `env:"COMPRESSION"`         // Enable compression
    TLSEnabled   bool   `env:"TLS_ENABLED"`         // Enable TLS
    RateLimit    int    `env:"RATE_LIMIT"`          // Rate limit per second
}
```

### Environment Variables

#### Main Configuration
- `LOG_LEVEL`: Logging level (debug, info, warn, error)
- `FACILITY`: Application facility name

#### Sender Configuration
- `ENDPOINT`: Server endpoint (Graylog, HTTP, etc.)
- `TIMEOUT`: Timeout in milliseconds
- `RETRY_COUNT`: Number of retry attempts
- `RETRY_DELAY`: Delay between retries in milliseconds
- `BATCH_SIZE`: Batch size for sending logs
- `BATCH_DELAY`: Delay between batches in milliseconds
- `FORMAT`: Output format (json, gelf, console)
- `COMPRESSION`: Enable compression (true/false)
- `TLS_ENABLED`: Enable TLS (true/false)
- `RATE_LIMIT`: Rate limit per second

## Usage

### Basic Logging

```go
// Console only logging
logger := mystic.New(mystic.ZapAdapter("my-service"))

logger.Debug("Debug message")
logger.Info("Info message", "key", "value")
logger.Warn("Warning message", "count", 42)
logger.Error("Error message", "err", err)
logger.Panic("Panic message") // Use with caution
```

### With LogSender Integration

```go
// Create a Graylog sender
graylogSender := mystic.NewGraylogSender(mystic.SenderConfig{
    Endpoint: "localhost:12201",
    Format:   "gelf",
}, "my-app")

// Use with zap adapter
logger := mystic.New(mystic.ZapAdapterWithSender("my-service", graylogSender))

// Use with zerolog adapter
logger := mystic.New(mystic.ZerologAdapterWithSender("my-service", graylogSender))

// Logs will go to both console and Graylog
logger.Info("This goes to console AND Graylog")
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

The zap adapter provides high-performance structured logging with OpenTelemetry integration and optional LogSender transport.

```go
import "github.com/bi0dread/mystic"

// Console only logging
logger := mystic.New(mystic.ZapAdapter("my-service"))

// With custom LogSender
graylogSender := mystic.NewGraylogSender(mystic.SenderConfig{
    Endpoint: "localhost:12201",
    Format:   "gelf",
}, "my-app")

logger := mystic.New(mystic.ZapAdapterWithSender("my-service", graylogSender))

// Alternative syntax (same result)
logger := mystic.New(mystic.ZapAdapterWithConfig("my-service", graylogSender))
```

**Dependencies:**
- `go.uber.org/zap`
- `go.opentelemetry.io/otel/*`
- `github.com/bi0dread/jeeko/morgana`
- `github.com/Graylog2/go-gelf/gelf` (for Graylog transport)

### Zerolog Adapter

The zerolog adapter provides zero-allocation JSON logging with OpenTelemetry integration and optional LogSender transport.

```go
import "github.com/bi0dread/mystic"

// Console only logging
logger := mystic.New(mystic.ZerologAdapter("my-service"))

// With custom LogSender
graylogSender := mystic.NewGraylogSender(mystic.SenderConfig{
    Endpoint: "localhost:12201",
    Format:   "gelf",
}, "my-app")

logger := mystic.New(mystic.ZerologAdapterWithSender("my-service", graylogSender))

// Alternative syntax (same result)
logger := mystic.New(mystic.ZerologAdapterWithConfig("my-service", graylogSender))
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
    SenderConfig: mystic.SenderConfig{
        Endpoint:   "/var/log/myapp/application",
        Format:     "json",
        BatchSize:  100,
        BatchDelay: 1000,
    },
}
logger := mystic.New(mystic.FileAdapterWithConfig("my-service", fileConfig))
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
    SenderConfig: mystic.SenderConfig{
        Endpoint:   "https://api.logs.com/ingest",
        Format:     "json",
        BatchSize:  100,
        BatchDelay: 100,
    },
}
logger := mystic.New(mystic.HTTPAdapterWithConfig("my-service", httpConfig))
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
            graylogSender := mystic.NewGraylogSender(mystic.SenderConfig{
                Endpoint: "localhost:12201",
                Format:   "gelf",
            }, "myapp")
            return mystic.ZapAdapterWithSender(name, graylogSender)
        }},
    },
    Strategy: "all", // "all", "first_success", "round_robin"
}

logger := mystic.New(mystic.MultiOutputAdapterWithConfig("my-service", multiConfig))
```

**Strategies:**
- `all`: Send to all adapters
- `first_success`: Send to adapters until one succeeds
- `round_robin`: Distribute logs across adapters

## LogSender Interface

Mystic provides a unified `LogSender` interface for sending logs to various destinations:

```go
type LogSender interface {
    // Core sending methods
    Send(entry LogEntry) error
    SendBatch(entries []LogEntry) error
    SendAsync(entry LogEntry) error
    SendWithContext(ctx context.Context, entry LogEntry) error
    
    // Connection management
    Close() error
    IsConnected() bool
    
    // Statistics
    GetStats() SenderStats
    
    // Configuration methods (embedded in sender)
    GetEndpoint() string
    GetTimeout() int
    GetRetryCount() int
    GetRetryDelay() int
    GetBatchSize() int
    GetBatchDelay() int
    GetFormat() string
    IsCompressionEnabled() bool
    IsTLSEnabled() bool
    GetRateLimit() int
}
```

### Available LogSender Implementations

#### GraylogSender
```go
graylogSender := mystic.NewGraylogSender(mystic.SenderConfig{
    Endpoint:     "localhost:12201",
    Timeout:      5000,
    RetryCount:   3,
    RetryDelay:   1000,
    BatchSize:    100,
    BatchDelay:   1000,
    Format:       "gelf",
    Compression:  true,
    TLSEnabled:   false,
    RateLimit:    1000,
}, "my-app")
```

#### FileSender
```go
fileSender, err := mystic.NewFileSender(mystic.FileAdapterConfig{
    Path:       "/var/log/myapp",
    Filename:   "application",
    MaxSize:    100 * 1024 * 1024,
    MaxAge:     30 * 24 * time.Hour,
    MaxBackups: 5,
    Compress:   true,
    Format:     "json",
    Level:      "info",
    Facility:   "myapp",
    SenderConfig: mystic.SenderConfig{
        Endpoint:   "/var/log/myapp/application",
        Format:     "json",
        BatchSize:  100,
        BatchDelay: 1000,
    },
})
```

#### HTTPSender
```go
httpSender, err := mystic.NewHTTPSender(mystic.HTTPAdapterConfig{
    Method:     "POST",
    Headers:    map[string]string{"Content-Type": "application/json"},
    Timeout:    5 * time.Second,
    RetryCount: 3,
    RetryDelay: 1 * time.Second,
    BatchSize:  100,
    BatchDelay: 100 * time.Millisecond,
    Format:     "json",
    Level:      "info",
    Facility:   "myapp",
    SenderConfig: mystic.SenderConfig{
        Endpoint:   "https://api.logs.com/ingest",
        Format:     "json",
        BatchSize:  100,
        BatchDelay: 100,
    },
})
```

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

Mystic uses a layered approach with a simplified, clean architecture:

### Simplified Function Signatures

All adapter functions now use simplified signatures:

```go
// Console only logging
ZapAdapter("service-name")
ZerologAdapter("service-name")

// With optional LogSender
ZapAdapterWithSender("service-name", sender)
ZerologAdapterWithSender("service-name", sender)

// Alternative syntax (same result)
ZapAdapterWithConfig("service-name", sender)
ZerologAdapterWithConfig("service-name", sender)
```

### Configuration Embedded in Senders

Configuration is now embedded directly in senders, eliminating the need for separate config parameters:

```go
// Old way (no longer available)
// ZapAdapterWithConfig("name", sender, config, facility)

// New way (simplified)
graylogSender := NewGraylogSender(config, facility)
logger := ZapAdapterWithSender("name", graylogSender)
```

### LogSender Interface

The `LogSender` interface provides a unified way to send logs to various destinations:

```go
type LogSender interface {
    Send(entry LogEntry) error
    SendBatch(entries []LogEntry) error
    SendAsync(entry LogEntry) error
    SendWithContext(ctx context.Context, entry LogEntry) error
    Close() error
    IsConnected() bool
    GetStats() SenderStats
    
    // Configuration methods (embedded)
    GetEndpoint() string
    GetTimeout() int
    GetRetryCount() int
    GetRetryDelay() int
    GetBatchSize() int
    GetBatchDelay() int
    GetFormat() string
    IsCompressionEnabled() bool
    IsTLSEnabled() bool
    GetRateLimit() int
}
```

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

### Benefits of New Architecture

The new simplified architecture provides several key benefits:

#### **1. Cleaner API Design**
- **Simplified function signatures**: No more redundant `SenderConfig` and `facility` parameters
- **Consistent patterns**: All adapters follow the same creation pattern
- **Better readability**: Clear intent with fewer parameters

#### **2. Improved Configuration Management**
- **Embedded configuration**: Configuration is part of the sender itself
- **No parameter duplication**: Each sender manages its own configuration
- **Easier testing**: No need to pass multiple config parameters around

#### **3. Enhanced Flexibility**
- **Optional sender integration**: Use console-only logging or add any LogSender
- **Unified interface**: All senders implement the same `LogSender` interface
- **Easy customization**: Create custom LogSender implementations

#### **4. Better Performance**
- **Reduced parameter passing**: Configuration is accessed directly from sender
- **Optimized memory usage**: No unnecessary config structs
- **Efficient field access**: Direct field access instead of method calls

#### **5. Simplified Testing**
- **Mock senders**: Easy to create mock LogSender implementations
- **Isolated testing**: Test senders independently from adapters
- **Clear dependencies**: Explicit sender dependencies in adapter constructors

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

## Advanced Usage

### Creating Custom LogSenders

You can create custom LogSender implementations for any destination:

```go
type MyCustomSender struct {
    mystic.BaseSender
    endpoint string
    // ... other fields
}

func (m *MyCustomSender) Send(entry mystic.LogEntry) error {
    // Custom sending logic
    return nil
}

func (m *MyCustomSender) GetEndpoint() string {
    return m.endpoint
}

// Implement other required methods...

// Usage
customSender := &MyCustomSender{endpoint: "custom://endpoint"}
logger := mystic.New(mystic.ZapAdapterWithSender("my-service", customSender))
```

### Using LogSenderWriter

The `LogSenderWriter` adapts any `LogSender` to `io.Writer` for integration with other systems:

```go
import "github.com/bi0dread/mystic"

// Create a LogSender
graylogSender := mystic.NewGraylogSender(mystic.SenderConfig{
    Endpoint: "localhost:12201",
    Format:   "gelf",
}, "my-app")

// Adapt to io.Writer
writer := mystic.NewLogSenderWriter(graylogSender)

// Use with any system that expects io.Writer
fmt.Fprintf(writer, "Log message: %s\n", "Hello World")
```

**Use Cases:**
- **Integration with existing systems**: Use with libraries that expect `io.Writer`
- **Standard output redirection**: Redirect stdout/stderr to LogSender
- **File descriptor replacement**: Replace file writers with LogSender
- **Third-party integration**: Integrate with logging libraries that use `io.Writer`

## Performance

The simplified architecture ensures minimal performance overhead:

- **Zero external dependencies** in core package
- **Optimized adapters** for their respective backends
- **Efficient field handling** and context propagation
- **Embedded configuration** eliminates parameter passing overhead
- **Optional sender integration** allows console-only logging when needed

## Testing

Mystic provides comprehensive test coverage for all functionality:

```bash
# Run all tests
go test ./...

# Run tests with specific adapter
go test -tags zap ./...
go test -tags zerolog ./...

# Run specific test categories
go test -run Test_LogSender_Configuration_Methods
go test -run Test_Simplified_Adapter_Functions
go test -run Test_LogSender_Interface_Compliance
go test -run Test_Configuration_Merged_Into_Sender
```

### Test Coverage Areas

The test suite covers:

- **Function Signature Testing**: All simplified adapter function variants
- **Configuration Method Testing**: All LogSender configuration getter methods
- **Interface Compliance Testing**: Verification that all senders implement LogSender interface
- **Configuration Embedding Testing**: Verification that config is properly embedded in senders
- **Architecture Verification**: Testing of the new simplified architecture
- **Migration Testing**: Verification that old complex signatures are no longer available

## Examples

See the `examples/` directory for complete working examples:

- **Basic usage with zap**: Console-only logging with zap adapter
- **Basic usage with zerolog**: Console-only logging with zerolog adapter
- **Custom LogSender implementation**: Creating custom log senders
- **Graylog integration**: Using GraylogSender with adapters
- **OpenTelemetry integration**: Context propagation and span events
- **Optional sender usage**: Demonstrating the new simplified function signatures
- **LogSenderWriter usage**: Adapting LogSender to io.Writer

### Example: Optional Sender Integration

```go
package main

import (
    "fmt"
    "github.com/bi0dread/mystic"
)

func main() {
    // Example 1: Console only logging
    fmt.Println("1. Console only logging:")
    logger1 := mystic.ZapAdapter("console-only")
    logger1.Info("This log goes to console only", "service", "example")
    fmt.Println()

    // Example 2: Logger with Graylog sender
    fmt.Println("2. Logger with Graylog sender:")
    
    // Create a Graylog sender directly
    graylogSender := mystic.NewGraylogSender(mystic.SenderConfig{
        Endpoint:   "localhost:12201",
        Timeout:    5000,
        RetryCount: 3,
        Format:     "gelf",
    }, "my-application")
    
    logger2 := mystic.ZapAdapterWithSender("with-graylog", graylogSender)
    logger2.Info("This log goes to console AND Graylog", "service", "example", "version", "1.0.0")
    fmt.Println()

    // Example 3: Logger with custom sender
    fmt.Println("3. Logger with custom sender:")
    customSender := &MyCustomSender{endpoint: "custom://endpoint"}
    logger3 := mystic.ZapAdapterWithSender("with-custom", customSender)
    logger3.Info("This log goes to console AND custom sender", "priority", "high")
    fmt.Println()

    // Example 4: Logger with custom sender
    fmt.Println("4. Logger with custom sender:")
    logger4 := mystic.ZapAdapterWithSender("with-custom", customSender)
    logger4.Info("This log goes to console AND custom sender", "priority", "high")
    fmt.Println()
}
```

## Migration Guide

### From Old Complex Signatures

**Before (Old Way):**
```go
// Old complex signature (no longer available)
logger := mystic.ZapAdapterWithConfig("service", nil, mystic.SenderConfig{
    Endpoint: "localhost:12201",
    Format:   "gelf",
}, "facility")
```

**After (New Way):**
```go
// New simplified signature
graylogSender := mystic.NewGraylogSender(mystic.SenderConfig{
    Endpoint: "localhost:12201",
    Format:   "gelf",
}, "facility")

logger := mystic.ZapAdapterWithSender("service", graylogSender)
```

### Benefits of New Architecture

1. **Simplified API**: No more redundant parameters
2. **Better Separation**: Each sender manages its own configuration
3. **Easier Testing**: No need to pass multiple config parameters
4. **Cleaner Code**: Configuration is part of the sender itself
5. **More Flexible**: Easy to create custom LogSender implementations

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
