package export

import (
	"fmt"
	"time"
)

// CreateExporter creates an exporter based on configuration
func CreateExporter(config ExporterConfig, logger Logger) (Exporter, error) {
	switch config.Type {
	case "http":
		return createHTTPExporter(config, logger)
	case "postgres", "postgresql":
		return createPostgresExporter(config, logger)
	case "file":
		return createFileExporter(config, logger)
	default:
		return nil, fmt.Errorf("unknown exporter type: %s", config.Type)
	}
}

// createHTTPExporter creates an HTTP exporter from generic config
func createHTTPExporter(config ExporterConfig, logger Logger) (*HTTPExporter, error) {
	httpConfig := HTTPExporterConfig{
		Name: config.Name,
	}

	// Extract URL (required)
	url, ok := config.Config["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("HTTP exporter requires 'url' in config")
	}
	httpConfig.URL = url

	// Extract optional headers
	if headersInterface, ok := config.Config["headers"].(map[string]interface{}); ok {
		httpConfig.Headers = make(map[string]string)
		for k, v := range headersInterface {
			if strVal, ok := v.(string); ok {
				httpConfig.Headers[k] = strVal
			}
		}
	}

	// Extract timeout
	if timeoutStr, ok := config.Config["timeout"].(string); ok {
		if duration, err := time.ParseDuration(timeoutStr); err == nil {
			httpConfig.Timeout = duration
		}
	}

	// Extract retry attempts
	if retryAttempts, ok := config.Config["retry_attempts"].(int); ok {
		httpConfig.RetryAttempts = retryAttempts
	} else if retryAttemptsFloat, ok := config.Config["retry_attempts"].(float64); ok {
		httpConfig.RetryAttempts = int(retryAttemptsFloat)
	}

	// Extract retry delay
	if retryDelayStr, ok := config.Config["retry_delay"].(string); ok {
		if duration, err := time.ParseDuration(retryDelayStr); err == nil {
			httpConfig.RetryDelay = duration
		}
	}

	return NewHTTPExporter(httpConfig, logger)
}

// createPostgresExporter creates a PostgreSQL exporter from generic config
func createPostgresExporter(config ExporterConfig, logger Logger) (*PostgresExporter, error) {
	pgConfig := PostgresExporterConfig{
		Name: config.Name,
	}

	// Extract connection string (required)
	connStr, ok := config.Config["connection_string"].(string)
	if !ok || connStr == "" {
		return nil, fmt.Errorf("PostgreSQL exporter requires 'connection_string' in config")
	}
	pgConfig.ConnectionString = connStr

	// Extract table name
	if tableName, ok := config.Config["table_name"].(string); ok {
		pgConfig.TableName = tableName
	}

	// Extract batch size
	if batchSize, ok := config.Config["batch_size"].(int); ok {
		pgConfig.BatchSize = batchSize
	} else if batchSizeFloat, ok := config.Config["batch_size"].(float64); ok {
		pgConfig.BatchSize = int(batchSizeFloat)
	}

	// Extract max retry
	if maxRetry, ok := config.Config["max_retry"].(int); ok {
		pgConfig.MaxRetry = maxRetry
	} else if maxRetryFloat, ok := config.Config["max_retry"].(float64); ok {
		pgConfig.MaxRetry = int(maxRetryFloat)
	}

	return NewPostgresExporter(pgConfig, logger)
}

// createFileExporter creates a file exporter from generic config
func createFileExporter(config ExporterConfig, logger Logger) (*FileExporter, error) {
	fileConfig := FileExporterConfig{
		Name: config.Name,
	}

	// Extract path (required)
	path, ok := config.Config["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("File exporter requires 'path' in config")
	}
	fileConfig.Path = path

	// Extract max size MB
	if maxSizeMB, ok := config.Config["max_size_mb"].(int); ok {
		fileConfig.MaxSizeMB = maxSizeMB
	} else if maxSizeMBFloat, ok := config.Config["max_size_mb"].(float64); ok {
		fileConfig.MaxSizeMB = int(maxSizeMBFloat)
	} else {
		fileConfig.MaxSizeMB = 100 // default
	}

	// Extract max backups
	if maxBackups, ok := config.Config["max_backups"].(int); ok {
		fileConfig.MaxBackups = maxBackups
	} else if maxBackupsFloat, ok := config.Config["max_backups"].(float64); ok {
		fileConfig.MaxBackups = int(maxBackupsFloat)
	} else {
		fileConfig.MaxBackups = 5 // default
	}

	// Extract compress flag
	if compress, ok := config.Config["compress"].(bool); ok {
		fileConfig.Compress = compress
	}

	return NewFileExporter(fileConfig, logger)
}
