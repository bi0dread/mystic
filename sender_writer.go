package mystic

import (
	"time"
)

// LogSenderWriter adapts LogSender to io.Writer for adapter integration
type LogSenderWriter struct {
	sender LogSender
}

// Write implements io.Writer interface
func (lsw *LogSenderWriter) Write(p []byte) (n int, err error) {
	// Convert adapter output to LogEntry and send via sender
	entry := LogEntry{
		Level:     "INFO", // Default level, could be parsed from p if needed
		Message:   string(p),
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
	}

	err = lsw.sender.Send(entry)
	if err != nil {
		return 0, err
	}

	return len(p), nil
}
