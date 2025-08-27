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
    logger := mystic.New(mystic.ZapAdapter, "my-service")
    
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
logger := mystic.New(mystic.ZapAdapter, "my-service")

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
logger := mystic.New(mystic.ZapAdapter, "my-service")
    .With("service", "api")
    .SkipLevel(1)
    .SetContext(ctx)

logger.Info("Request processed")
```

### With Fields

```go
logger := mystic.New(mystic.ZapAdapter, "my-service")
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
logger := mystic.New(mystic.ZapAdapter, "my-service")

// Use custom Graylog configuration
customConfig := mystic.GraylogSenderConfig{
    GrayLogAddr: "custom-graylog:12201",
    Facility:    "my-custom-facility",
}
logger := mystic.New(func(name string) mystic.Logger {
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

logger := mystic.New(mystic.ZapAdapter, "my-service")
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
logger := mystic.New(mystic.ZerologAdapter, "my-service")

// Use custom Graylog configuration
customConfig := mystic.GraylogSenderConfig{
    GrayLogAddr: "custom-graylog:12201",
    Facility:    "my-custom-facility",
}
logger := mystic.New(func(name string) mystic.Logger {
    return mystic.ZerologAdapterWithConfig(name, customConfig)
}, "my-service")
```

**Dependencies:**
- `github.com/rs/zerolog`
- `go.opentelemetry.io/otel/*`
- `github.com/bi0dread/jeeko/morgana`
- `github.com/Graylog2/go-gelf/gelf` (for Graylog transport)

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

```go
// High-performance structured logging with OpenTelemetry + Graylog transport
zapLogger := mystic.New(mystic.ZapAdapter, "zap-service")

// Zero-allocation JSON logging with OpenTelemetry + Graylog transport  
zerologLogger := mystic.New(mystic.ZerologAdapter, "zerolog-service")
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
logger := mystic.New(MyCustomAdapter, "my-service")
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
