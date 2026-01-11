package export

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileExporter exports metrics to a JSONL file with rotation support
type FileExporter struct {
	name     string
	config   FileExporterConfig
	logger   Logger
	writer   *lumberjack.Logger
	mu       sync.Mutex
	encoder  *json.Encoder
}

// NewFileExporter creates a new file exporter
func NewFileExporter(config FileExporterConfig, logger Logger) (*FileExporter, error) {
	// Ensure directory exists
	dir := filepath.Dir(config.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create lumberjack logger for rotation
	writer := &lumberjack.Logger{
		Filename:   config.Path,
		MaxSize:    config.MaxSizeMB, // megabytes
		MaxBackups: config.MaxBackups,
		MaxAge:     0,               // days (0 = don't delete old backups)
		Compress:   config.Compress, // compress rotated files
	}

	exporter := &FileExporter{
		name:   config.Name,
		config: config,
		logger: logger,
		writer: writer,
	}

	return exporter, nil
}

// Export writes metric records to file as JSONL (one record per line)
func (e *FileExporter) Export(ctx context.Context, records []MetricRecord) error {
	if len(records) == 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	startTime := time.Now()

	// Write each record as a single line
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			e.logger.Errorw("Failed to marshal metric record",
				"exporter", e.name,
				"counter_id", record.CounterID,
				"error", err)
			continue
		}

		// Write line to file
		if _, err := e.writer.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	duration := time.Since(startTime)
	e.logger.Debugw("Exported metrics to file",
		"exporter", e.name,
		"records", len(records),
		"duration_ms", duration.Milliseconds())

	return nil
}

// Name returns the exporter name
func (e *FileExporter) Name() string {
	return e.name
}

// Close closes the file writer
func (e *FileExporter) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.writer != nil {
		return e.writer.Close()
	}
	return nil
}
