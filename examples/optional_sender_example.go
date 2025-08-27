package main

import (
	"context"
	"fmt"
	"time"

	"github.com/bi0dread/mystic"
)

func main() {
	fmt.Println("=== Mystic Optional Sender Example ===\n")

	// Example 1: Logger without sender (console only)
	fmt.Println("1. Logger without sender (console only):")
	logger1 := mystic.ZapAdapter("console-only")
	logger1.Info("This log goes to console only")
	logger1.Error("Error message to console only")
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
	logger2.Error("Error message to console AND Graylog", "error_code", "EXAMPLE_001")
	fmt.Println()

	// Example 3: Logger with custom sender (optional)
	fmt.Println("3. Logger with custom sender (optional):")

	// Create a custom sender (could be HTTP, File, or any LogSender implementation)
	customSender := &CustomLogSender{
		name: "custom-sender",
	}

	logger3 := mystic.ZapAdapterWithSender("with-custom-sender", customSender)
	logger3.Info("This log goes to console AND custom sender", "custom_field", "custom_value")
	logger3.Warn("Warning message to console AND custom sender", "level", "warning")
	fmt.Println()

	// Example 4: Logger with custom sender
	fmt.Println("4. Logger with custom sender:")
	logger4 := mystic.ZapAdapterWithSender("with-custom", customSender)
	logger4.Info("This log goes to console AND custom sender", "priority", "high")
	fmt.Println()

	// Example 5: Zerolog adapter with optional sender
	fmt.Println("5. Zerolog adapter with optional sender:")
	zeroLogger := mystic.ZerologAdapterWithSender("zerolog-with-sender", customSender)
	zeroLogger.Info("Zerolog message to console AND custom sender", "adapter", "zerolog")
	fmt.Println()

	fmt.Println("=== Example completed ===")
}

// CustomLogSender implements LogSender interface for demonstration
type CustomLogSender struct {
	name string
}

func (c *CustomLogSender) Send(entry mystic.LogEntry) error {
	fmt.Printf("🔗 [%s] Sending to custom sender: %s - %s\n",
		c.name, entry.Level, entry.Message)
	return nil
}

func (c *CustomLogSender) SendBatch(entries []mystic.LogEntry) error {
	for _, entry := range entries {
		if err := c.Send(entry); err != nil {
			return err
		}
	}
	return nil
}

func (c *CustomLogSender) SendAsync(entry mystic.LogEntry) error {
	go func() {
		c.Send(entry)
	}()
	return nil
}

func (c *CustomLogSender) SendWithContext(ctx context.Context, entry mystic.LogEntry) error {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return c.Send(entry)
}

func (c *CustomLogSender) Close() error {
	fmt.Printf("🔗 [%s] Custom sender closed\n", c.name)
	return nil
}

func (c *CustomLogSender) IsConnected() bool {
	return true
}

func (c *CustomLogSender) GetStats() mystic.SenderStats {
	return mystic.SenderStats{
		TotalSent:        0,
		TotalFailed:      0,
		TotalLatency:     0,
		LastSentAt:       time.Time{},
		LastFailedAt:     time.Time{},
		ConnectionStatus: "connected",
	}
}
