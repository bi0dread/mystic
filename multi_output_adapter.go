package mystic

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// MultiOutputConfig holds configuration for multi-output logging
type MultiOutputConfig struct {
	Adapters []AdapterConfig `json:"adapters"` // List of adapter configurations
	Strategy string          `json:"strategy"` // Output strategy: "all", "first_success", "round_robin"
}

// AdapterConfig represents a single adapter configuration
type AdapterConfig struct {
	Name    string                 `json:"name"`   // Adapter name for identification
	Adapter func(string) Logger    `json:"-"`      // Adapter constructor function
	Config  map[string]interface{} `json:"config"` // Adapter-specific configuration
}

// MultiOutputAdapter creates a multi-output logger adapter
func MultiOutputAdapter(named string) Logger {
	return MultiOutputAdapterWithConfig(named, MultiOutputConfig{
		Adapters: []AdapterConfig{},
		Strategy: "all",
	})
}

// MultiOutputAdapterWithConfig creates a multi-output logger adapter with custom configuration
func MultiOutputAdapterWithConfig(named string, config MultiOutputConfig) Logger {
	if len(config.Adapters) == 0 {
		return nil
	}

	// Create all adapters
	var adapters []Logger
	for _, adapterConfig := range config.Adapters {
		if adapterConfig.Adapter != nil {
			adapter := adapterConfig.Adapter(named)
			if adapter != nil {
				adapters = append(adapters, adapter)
			}
		}
	}

	if len(adapters) == 0 {
		return nil
	}

	m := &mysticMultiOutput{
		adapters: adapters,
		config:   config,
		name:     named,
		fields:   map[string]interface{}{},
		skip:     1,
	}

	return m
}

// mysticMultiOutput implements the Logger interface for multi-output logging
type mysticMultiOutput struct {
	adapters []Logger
	config   MultiOutputConfig
	name     string
	fields   map[string]interface{}
	skip     int
	ctx      context.Context
	mu       sync.Mutex
}

func (m *mysticMultiOutput) SetContext(ctx context.Context) Logger {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ctx = ctx

	// Set context on all adapters
	for _, adapter := range m.adapters {
		adapter.SetContext(ctx)
	}

	return m
}

func (m *mysticMultiOutput) SkipLevel(skip int) Logger {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.skip = skip

	// Set skip level on all adapters
	for _, adapter := range m.adapters {
		adapter.SkipLevel(skip)
	}

	return m
}

func (m *mysticMultiOutput) With(args ...interface{}) Logger {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add fields to local storage
	kv := convertKeyValues(args)
	for k, v := range kv {
		m.fields[k] = v
	}

	// Add fields to all adapters
	for _, adapter := range m.adapters {
		adapter.With(args...)
	}

	return m
}

func (m *mysticMultiOutput) Debug(msg string, keysAndValues ...interface{}) {
	m.logToAll("Debug", msg, keysAndValues...)
}

func (m *mysticMultiOutput) Info(msg string, keysAndValues ...interface{}) {
	m.logToAll("Info", msg, keysAndValues...)
}

func (m *mysticMultiOutput) Warn(msg string, keysAndValues ...interface{}) {
	m.logToAll("Warn", msg, keysAndValues...)
}

func (m *mysticMultiOutput) Error(msg string, keysAndValues ...interface{}) {
	m.logToAll("Error", msg, keysAndValues...)
}

func (m *mysticMultiOutput) ErrorDetail(err error, keysAndValues ...interface{}) {
	// Add error details to keysAndValues
	fields := keysAndValues
	if fields == nil {
		fields = make([]interface{}, 0)
	}
	fields = append(fields, "error", err.Error())

	m.logToAll("ErrorDetail", err.Error(), fields...)
}

func (m *mysticMultiOutput) Panic(msg string, keysAndValues ...interface{}) {
	m.logToAll("Panic", msg, keysAndValues...)
}

// logToAll logs to all adapters based on the configured strategy
func (m *mysticMultiOutput) logToAll(method, msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Include persistent fields
	allFields := make([]interface{}, 0, len(m.fields)*2+len(keysAndValues))
	for k, v := range m.fields {
		allFields = append(allFields, k, v)
	}
	allFields = append(allFields, keysAndValues...)

	switch m.config.Strategy {
	case "first_success":
		m.logFirstSuccess(method, msg, allFields...)
	case "round_robin":
		m.logRoundRobin(method, msg, allFields...)
	default: // "all"
		m.logToAllAdapters(method, msg, allFields...)
	}
}

// logToAllAdapters logs to all adapters
func (m *mysticMultiOutput) logToAllAdapters(method, msg string, keysAndValues ...interface{}) {
	var wg sync.WaitGroup

	for _, adapter := range m.adapters {
		wg.Add(1)
		go func(adpt Logger) {
			defer wg.Done()
			m.callAdapterMethod(adpt, method, msg, keysAndValues...)
		}(adapter)
	}

	wg.Wait()
}

// logFirstSuccess logs to adapters until one succeeds
func (m *mysticMultiOutput) logFirstSuccess(method, msg string, keysAndValues ...interface{}) {
	for _, adapter := range m.adapters {
		if m.callAdapterMethod(adapter, method, msg, keysAndValues...) {
			return // Success, stop here
		}
	}
}

// logRoundRobin logs to adapters in round-robin fashion
func (m *mysticMultiOutput) logRoundRobin(method, msg string, keysAndValues ...interface{}) {
	// Simple round-robin implementation
	// In a real implementation, you might want to use atomic operations for thread safety
	static.counter++
	index := static.counter % len(m.adapters)

	adapter := m.adapters[index]
	m.callAdapterMethod(adapter, method, msg, keysAndValues...)
}

// callAdapterMethod calls the appropriate method on the adapter
func (m *mysticMultiOutput) callAdapterMethod(adapter Logger, method, msg string, keysAndValues ...interface{}) bool {
	defer func() {
		if r := recover(); r != nil {
			// Log panic locally
			fmt.Printf("Adapter %T panicked: %v\n", adapter, r)
		}
	}()

	switch method {
	case "Debug":
		adapter.Debug(msg, keysAndValues...)
	case "Info":
		adapter.Info(msg, keysAndValues...)
	case "Warn":
		adapter.Warn(msg, keysAndValues...)
	case "Error":
		adapter.Error(msg, keysAndValues...)
	case "ErrorDetail":
		// For ErrorDetail, we need to create an error
		err := fmt.Errorf(msg)
		adapter.ErrorDetail(err, keysAndValues...)
	case "Panic":
		adapter.Panic(msg, keysAndValues...)
	default:
		return false
	}

	return true
}

// static holds static data for round-robin
type staticData struct {
	counter int
}

var static = &staticData{}

// MultiOutputWriter implements io.Writer for multi-output scenarios
type MultiOutputWriter struct {
	writers []io.Writer
	mu      sync.Mutex
}

// NewMultiOutputWriter creates a new multi-output writer
func NewMultiOutputWriter(writers ...io.Writer) *MultiOutputWriter {
	return &MultiOutputWriter{
		writers: writers,
	}
}

// Write writes to all writers
func (w *MultiOutputWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n = len(p)

	for _, writer := range w.writers {
		if _, err := writer.Write(p); err != nil {
			// Continue with other writers, but return the error
			return n, err
		}
	}

	return n, nil
}

// AddWriter adds a new writer to the multi-output
func (w *MultiOutputWriter) AddWriter(writer io.Writer) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.writers = append(w.writers, writer)
}

// RemoveWriter removes a writer from the multi-output
func (w *MultiOutputWriter) RemoveWriter(writer io.Writer) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i, wr := range w.writers {
		if wr == writer {
			w.writers = append(w.writers[:i], w.writers[i+1:]...)
			break
		}
	}
}
