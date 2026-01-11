package export

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresExporter exports metrics to a PostgreSQL database
type PostgresExporter struct {
	name   string
	config PostgresExporterConfig
	logger Logger
	db     *sql.DB
}

// NewPostgresExporter creates a new PostgreSQL exporter
func NewPostgresExporter(config PostgresExporterConfig, logger Logger) (*PostgresExporter, error) {
	if config.ConnectionString == "" {
		return nil, fmt.Errorf("PostgreSQL connection string is required")
	}

	if config.TableName == "" {
		config.TableName = "metrics"
	}

	if config.BatchSize == 0 {
		config.BatchSize = 1000
	}

	if config.MaxRetry == 0 {
		config.MaxRetry = 3
	}

	// Open database connection
	db, err := sql.Open("postgres", config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	exporter := &PostgresExporter{
		name:   config.Name,
		config: config,
		logger: logger,
		db:     db,
	}

	// Optionally create table if it doesn't exist
	if err := exporter.ensureTable(context.Background()); err != nil {
		logger.Warnw("Failed to ensure metrics table exists",
			"exporter", config.Name,
			"error", err)
	}

	return exporter, nil
}

// ensureTable creates the metrics table if it doesn't exist
func (e *PostgresExporter) ensureTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			counter_id INTEGER NOT NULL,
			value DOUBLE PRECISION NOT NULL,
			cause_code VARCHAR(100),
			hostname VARCHAR(255) NOT NULL,
			system_name VARCHAR(100) NOT NULL,
			timestamp TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`, e.config.TableName)

	if _, err := e.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create indexes
	indexes := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_counter_time ON %s(counter_id, timestamp DESC)", e.config.TableName, e.config.TableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_hostname_time ON %s(hostname, timestamp DESC)", e.config.TableName, e.config.TableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_cause_code ON %s(cause_code) WHERE cause_code IS NOT NULL", e.config.TableName, e.config.TableName),
	}

	for _, indexQuery := range indexes {
		if _, err := e.db.ExecContext(ctx, indexQuery); err != nil {
			e.logger.Warnw("Failed to create index",
				"exporter", e.name,
				"error", err)
		}
	}

	return nil
}

// Export inserts metric records into PostgreSQL
func (e *PostgresExporter) Export(ctx context.Context, records []MetricRecord) error {
	if len(records) == 0 {
		return nil
	}

	startTime := time.Now()

	// Process records in batches
	for i := 0; i < len(records); i += e.config.BatchSize {
		end := i + e.config.BatchSize
		if end > len(records) {
			end = len(records)
		}

		batch := records[i:end]
		if err := e.insertBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to insert batch: %w", err)
		}
	}

	duration := time.Since(startTime)
	e.logger.Debugw("Exported metrics to PostgreSQL",
		"exporter", e.name,
		"records", len(records),
		"duration_ms", duration.Milliseconds())

	return nil
}

// insertBatch inserts a batch of records using a single multi-row INSERT
func (e *PostgresExporter) insertBatch(ctx context.Context, records []MetricRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Build multi-row INSERT statement
	placeholders := make([]string, len(records))
	values := make([]interface{}, 0, len(records)*6)

	for i, record := range records {
		offset := i * 6
		placeholders[i] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)",
			offset+1, offset+2, offset+3, offset+4, offset+5, offset+6)

		values = append(values,
			record.CounterID,
			record.Value,
			nullInt(record.CauseCode),
			record.Hostname,
			record.SystemName,
			record.Timestamp,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (counter_id, value, cause_code, hostname, system_name, timestamp)
		VALUES %s
	`, e.config.TableName, strings.Join(placeholders, ", "))

	// Execute with retry
	var lastErr error
	for attempt := 1; attempt <= e.config.MaxRetry; attempt++ {
		_, err := e.db.ExecContext(ctx, query, values...)
		if err == nil {
			return nil
		}

		lastErr = err
		e.logger.Warnw("PostgreSQL insert attempt failed",
			"exporter", e.name,
			"attempt", attempt,
			"max_retry", e.config.MaxRetry,
			"error", err)

		if attempt < e.config.MaxRetry {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", e.config.MaxRetry, lastErr)
}

// nullString returns sql.NullString for empty strings
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// nullInt returns nil for zero values (0 = no cause code)
func nullInt(i int) interface{} {
	if i == 0 {
		return nil
	}
	return i
}

// Name returns the exporter name
func (e *PostgresExporter) Name() string {
	return e.name
}

// Close closes the database connection
func (e *PostgresExporter) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}
