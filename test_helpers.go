package mystic

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"
)

// CapturedLogs holds captured log entries during testing
type CapturedLogs struct {
	Entries []CapturedLogEntry
	mu      sync.RWMutex
}

// CapturedLogEntry represents a single captured log entry
type CapturedLogEntry struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Fields    map[string]interface{} `json:"fields"`
	Logger    string                 `json:"logger"`
}

// CaptureLogs captures logs during the execution of the provided function
func CaptureLogs(fn func()) *CapturedLogs {
	captured := &CapturedLogs{
		Entries: make([]CapturedLogEntry, 0),
	}

	// Execute the function
	fn()

	return captured
}

// captureLogger implements Logger to capture log entries
type captureLogger struct {
	captured *CapturedLogs
	name     string
	fields   map[string]interface{}
	skip     int
	ctx      context.Context
}

func (c *captureLogger) Debug(msg string, keysAndValues ...interface{}) {
	c.capture("debug", msg, keysAndValues...)
}

func (c *captureLogger) Info(msg string, keysAndValues ...interface{}) {
	c.capture("info", msg, keysAndValues...)
}

func (c *captureLogger) Warn(msg string, keysAndValues ...interface{}) {
	c.capture("warn", msg, keysAndValues...)
}

func (c *captureLogger) Error(msg string, keysAndValues ...interface{}) {
	c.capture("error", msg, keysAndValues...)
}

func (c *captureLogger) ErrorDetail(err error, keysAndValues ...interface{}) {
	fields := keysAndValues
	if fields == nil {
		fields = make([]interface{}, 0)
	}
	fields = append(fields, "error", err.Error())
	c.capture("error", err.Error(), fields...)
}

func (c *captureLogger) Panic(msg string, keysAndValues ...interface{}) {
	c.capture("panic", msg, keysAndValues...)
}

func (c *captureLogger) SkipLevel(skip int) Logger {
	c.skip = skip
	return c
}

func (c *captureLogger) With(args ...interface{}) Logger {
	kv := convertKeyValues(args)
	for k, v := range kv {
		c.fields[k] = v
	}
	return c
}

func (c *captureLogger) SetContext(ctx context.Context) Logger {
	c.ctx = ctx
	return c
}

func (c *captureLogger) capture(level, msg string, keysAndValues ...interface{}) {
	entry := CapturedLogEntry{
		Level:     level,
		Message:   msg,
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
		Logger:    c.name,
	}

	// Copy persistent fields
	for k, v := range c.fields {
		entry.Fields[k] = v
	}

	// Add new fields
	kv := convertKeyValues(keysAndValues)
	for k, v := range kv {
		entry.Fields[k] = v
	}

	c.captured.mu.Lock()
	c.captured.Entries = append(c.captured.Entries, entry)
	c.captured.mu.Unlock()
}

// Contains checks if the captured logs contain a message
func (cl *CapturedLogs) Contains(msg string) bool {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	for _, entry := range cl.Entries {
		if entry.Message == msg {
			return true
		}
	}
	return false
}

// ContainsLevel checks if the captured logs contain a specific level
func (cl *CapturedLogs) ContainsLevel(level string) bool {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	for _, entry := range cl.Entries {
		if entry.Level == level {
			return true
		}
	}
	return false
}

// GetEntriesByLevel returns all entries for a specific level
func (cl *CapturedLogs) GetEntriesByLevel(level string) []CapturedLogEntry {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	var entries []CapturedLogEntry
	for _, entry := range cl.Entries {
		if entry.Level == level {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetEntriesByMessage returns all entries containing a specific message
func (cl *CapturedLogs) GetEntriesByMessage(msg string) []CapturedLogEntry {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	var entries []CapturedLogEntry
	for _, entry := range cl.Entries {
		if entry.Message == msg {
			entries = append(entries, entry)
		}
	}
	return entries
}

// Clear clears all captured entries
func (cl *CapturedLogs) Clear() {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	cl.Entries = cl.Entries[:0]
}

// Count returns the total number of captured entries
func (cl *CapturedLogs) Count() int {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	return len(cl.Entries)
}

// MockLogger provides a mock logger for testing with expectations
type MockLogger struct {
	expectations []LogExpectation
	entries      []CapturedLogEntry
	mu           sync.RWMutex
}

// LogExpectation represents an expected log entry
type LogExpectation struct {
	Level   string
	Message string
	Fields  map[string]interface{}
	Count   int
	Actual  int
	Matched bool
}

// NewMockLogger creates a new mock logger
func NewMockLogger() *MockLogger {
	return &MockLogger{
		expectations: make([]LogExpectation, 0),
		entries:      make([]CapturedLogEntry, 0),
	}
}

// ExpectInfo expects an info log message
func (m *MockLogger) ExpectInfo(msg string, fields ...interface{}) *MockLogger {
	expectation := LogExpectation{
		Level:   "info",
		Message: msg,
		Count:   1,
	}

	if len(fields) > 0 {
		expectation.Fields = make(map[string]interface{})
		kv := convertKeyValues(fields)
		for k, v := range kv {
			expectation.Fields[k] = v
		}
	}

	m.expectations = append(m.expectations, expectation)
	return m
}

// ExpectDebug expects a debug log message
func (m *MockLogger) ExpectDebug(msg string, fields ...interface{}) *MockLogger {
	expectation := LogExpectation{
		Level:   "debug",
		Message: msg,
		Count:   1,
	}

	if len(fields) > 0 {
		expectation.Fields = make(map[string]interface{})
		kv := convertKeyValues(fields)
		for k, v := range kv {
			expectation.Fields[k] = v
		}
	}

	m.expectations = append(m.expectations, expectation)
	return m
}

// ExpectWarn expects a warn log message
func (m *MockLogger) ExpectWarn(msg string, fields ...interface{}) *MockLogger {
	expectation := LogExpectation{
		Level:   "warn",
		Message: msg,
		Count:   1,
	}

	if len(fields) > 0 {
		expectation.Fields = make(map[string]interface{})
		kv := convertKeyValues(fields)
		for k, v := range kv {
			expectation.Fields[k] = v
		}
	}

	m.expectations = append(m.expectations, expectation)
	return m
}

// ExpectError expects an error log message
func (m *MockLogger) ExpectError(msg string, fields ...interface{}) *MockLogger {
	expectation := LogExpectation{
		Level:   "error",
		Message: msg,
		Count:   1,
	}

	if len(fields) > 0 {
		expectation.Fields = make(map[string]interface{})
		kv := convertKeyValues(fields)
		for k, v := range kv {
			expectation.Fields[k] = v
		}
	}

	m.expectations = append(m.expectations, expectation)
	return m
}

// ExpectCount sets the expected count for the last expectation
func (m *MockLogger) ExpectCount(count int) *MockLogger {
	if len(m.expectations) > 0 {
		m.expectations[len(m.expectations)-1].Count = count
	}
	return m
}

// Verify checks if all expectations were met
func (m *MockLogger) Verify() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i, expectation := range m.expectations {
		if expectation.Actual != expectation.Count {
			return fmt.Errorf("expectation %d: expected %d %s logs with message '%s', got %d",
				i+1, expectation.Count, expectation.Level, expectation.Message, expectation.Actual)
		}
	}

	return nil
}

// Logger interface implementation
func (m *MockLogger) Debug(msg string, keysAndValues ...interface{}) {
	m.record("debug", msg, keysAndValues...)
}

func (m *MockLogger) Info(msg string, keysAndValues ...interface{}) {
	m.record("info", msg, keysAndValues...)
}

func (m *MockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.record("warn", msg, keysAndValues...)
}

func (m *MockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.record("error", msg, keysAndValues...)
}

func (m *MockLogger) ErrorDetail(err error, keysAndValues ...interface{}) {
	fields := keysAndValues
	if fields == nil {
		fields = make([]interface{}, 0)
	}
	fields = append(fields, "error", err.Error())
	m.record("error", err.Error(), fields...)
}

func (m *MockLogger) Panic(msg string, keysAndValues ...interface{}) {
	m.record("panic", msg, keysAndValues...)
}

func (m *MockLogger) SkipLevel(skip int) Logger {
	return m
}

func (m *MockLogger) With(args ...interface{}) Logger {
	return m
}

func (m *MockLogger) SetContext(ctx context.Context) Logger {
	return m
}

func (m *MockLogger) record(level, msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := CapturedLogEntry{
		Level:     level,
		Message:   msg,
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
		Logger:    "mock",
	}

	// Add fields
	kv := convertKeyValues(keysAndValues)
	for k, v := range kv {
		entry.Fields[k] = v
	}

	m.entries = append(m.entries, entry)

	// Check expectations
	for i := range m.expectations {
		expectation := &m.expectations[i]
		if expectation.Level == level && expectation.Message == msg {
			expectation.Actual++
			expectation.Matched = true
			break
		}
	}
}

// TestLogger provides a simple test logger that writes to a buffer
type TestLogger struct {
	buffer *bytes.Buffer
	mu     sync.Mutex
}

// NewTestLogger creates a new test logger
func NewTestLogger() *TestLogger {
	return &TestLogger{
		buffer: &bytes.Buffer{},
	}
}

// GetOutput returns the captured output
func (t *TestLogger) GetOutput() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.buffer.String()
}

// Clear clears the output buffer
func (t *TestLogger) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buffer.Reset()
}

// Logger interface implementation
func (t *TestLogger) Debug(msg string, keysAndValues ...interface{}) {
	t.write("DEBUG", msg, keysAndValues...)
}

func (t *TestLogger) Info(msg string, keysAndValues ...interface{}) {
	t.write("INFO", msg, keysAndValues...)
}

func (t *TestLogger) Warn(msg string, keysAndValues ...interface{}) {
	t.write("WARN", msg, keysAndValues...)
}

func (t *TestLogger) Error(msg string, keysAndValues ...interface{}) {
	t.write("ERROR", msg, keysAndValues...)
}

func (t *TestLogger) ErrorDetail(err error, keysAndValues ...interface{}) {
	fields := keysAndValues
	if fields == nil {
		fields = make([]interface{}, 0)
	}
	fields = append(fields, "error", err.Error())
	t.write("ERROR", err.Error(), fields...)
}

func (t *TestLogger) Panic(msg string, keysAndValues ...interface{}) {
	t.write("PANIC", msg, keysAndValues...)
}

func (t *TestLogger) SkipLevel(skip int) Logger {
	return t
}

func (t *TestLogger) With(args ...interface{}) Logger {
	return t
}

func (t *TestLogger) SetContext(ctx context.Context) Logger {
	return t
}

func (t *TestLogger) write(level, msg string, keysAndValues ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	output := fmt.Sprintf("[%s] %s", level, msg)
	if len(keysAndValues) > 0 {
		output += fmt.Sprintf(" %v", keysAndValues)
	}
	output += "\n"

	t.buffer.WriteString(output)
}
