package export

import "context"

// Exporter defines the interface for exporting metric records
type Exporter interface {
	// Export sends a batch of metric records to the remote backend
	Export(ctx context.Context, records []MetricRecord) error

	// Name returns the exporter name for logging/identification
	Name() string

	// Close cleans up resources
	Close() error
}

// StatsCollectorInterface defines the interface for getting stats
// This allows us to decouple from the specific implementation
type StatsCollectorInterface interface {
	GetStats() interface{}
}

// Logger defines the logging interface used by exporters
type Logger interface {
	Infow(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
	Debugw(msg string, keysAndValues ...interface{})
}
