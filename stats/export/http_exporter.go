package export

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPExporter exports metrics to an HTTP endpoint
type HTTPExporter struct {
	name       string
	config     HTTPExporterConfig
	logger     Logger
	httpClient *http.Client
}

// NewHTTPExporter creates a new HTTP exporter
func NewHTTPExporter(config HTTPExporterConfig, logger Logger) (*HTTPExporter, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("HTTP exporter URL is required")
	}

	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}

	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}

	return &HTTPExporter{
		name:   config.Name,
		config: config,
		logger: logger,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// Export sends metric records to HTTP endpoint as JSON
func (e *HTTPExporter) Export(ctx context.Context, records []MetricRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Marshal records to JSON
	data, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("failed to marshal records: %w", err)
	}

	// Retry logic
	var lastErr error
	for attempt := 1; attempt <= e.config.RetryAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		startTime := time.Now()
		err := e.sendRequest(ctx, data)
		duration := time.Since(startTime)

		if err == nil {
			e.logger.Debugw("Exported metrics via HTTP",
				"exporter", e.name,
				"records", len(records),
				"attempt", attempt,
				"duration_ms", duration.Milliseconds())
			return nil
		}

		lastErr = err
		e.logger.Warnw("HTTP export attempt failed",
			"exporter", e.name,
			"attempt", attempt,
			"max_attempts", e.config.RetryAttempts,
			"error", err)

		// Don't sleep after last attempt
		if attempt < e.config.RetryAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(e.config.RetryDelay):
			}
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", e.config.RetryAttempts, lastErr)
}

// sendRequest sends a single HTTP request
func (e *HTTPExporter) sendRequest(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", e.config.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set default Content-Type
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, value := range e.config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Name returns the exporter name
func (e *HTTPExporter) Name() string {
	return e.name
}

// Close closes the HTTP client (no-op for http.Client)
func (e *HTTPExporter) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}
