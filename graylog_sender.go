package mystic

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Graylog2/go-gelf/gelf"
)

// GraylogSenderConfig holds configuration for GraylogSender
type GraylogSenderConfig struct {
	GrayLogAddr string `env:"GRAYLOG_ADDR"`
	Facility    string `env:"FACILITY, required"`
}

// GraylogSender provides GELF transport to Graylog servers
type GraylogSender struct {
	writer   *gelf.Writer
	fallback io.Writer
	config   GraylogSenderConfig
}

// NewGraylogSender creates a new Graylog sender with fallback
func NewGraylogSender(config GraylogSenderConfig) *GraylogSender {
	gelfWriter, err := gelf.NewWriter(config.GrayLogAddr)
	if err != nil {
		// Fallback to console if Graylog connection fails
		return &GraylogSender{
			writer:   nil,
			fallback: os.Stdout,
			config:   config,
		}
	}
	gelfWriter.Facility = config.Facility

	return &GraylogSender{
		writer:   gelfWriter,
		fallback: os.Stdout,
		config:   config,
	}
}

// Write implements io.Writer interface for zap integration
func (g *GraylogSender) Write(p []byte) (n int, err error) {
	// Parse the log entry and send to Graylog
	// For now, we'll send as a generic log entry
	err = g.Send("INFO", string(p), map[string]interface{}{})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Send sends a log entry to Graylog in GELF format
func (g *GraylogSender) Send(level string, msg string, fields map[string]interface{}) error {
	if g.writer != nil {
		// Send to Graylog
		gelfMessage := map[string]interface{}{
			"short_message": msg,
			"level_string":  level,
			"timestamp":     time.Now().Unix(),
			"facility":      g.config.Facility,
		}

		// Add custom fields
		for k, v := range fields {
			if k != "short_message" && k != "level_string" && k != "timestamp" && k != "facility" {
				gelfMessage[k] = v
			}
		}

		data, err := json.Marshal(gelfMessage)
		if err != nil {
			return err
		}

		_, err = g.writer.Write(data)
		if err != nil {
			// If Graylog fails, fall back to console
			return g.sendToFallback(level, msg, fields)
		}
		return nil
	}

	// Fallback to console
	return g.sendToFallback(level, msg, fields)
}

// sendToFallback sends logs to fallback output (console)
func (g *GraylogSender) sendToFallback(level string, msg string, fields map[string]interface{}) error {
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     level,
		"message":   msg,
		"facility":  g.config.Facility,
		"output":    "fallback",
	}

	// Add custom fields
	for k, v := range fields {
		if k != "timestamp" && k != "level" && k != "message" && k != "facility" && k != "output" {
			logEntry[k] = v
		}
	}

	data, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(g.fallback, string(data))
	return err
}

// Close closes the Graylog connection
func (g *GraylogSender) Close() error {
	if g.writer != nil {
		return g.writer.Close()
	}
	return nil
}
