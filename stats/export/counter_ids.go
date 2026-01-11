package export

// Counter ID constants for metrics
const (
	// General request counters (1000-1099)
	CounterTotalRequests      = 1000
	CounterSuccessfulRequests = 1001
	CounterFailedRequests     = 1002
	CounterPendingRequests    = 1003

	// Diameter counters (1100-1199)
	CounterDiameterTotal      = 1100
	CounterDiameterSuccess    = 1101
	CounterDiameterFailed     = 1102
	CounterDiameterResultCode = 1103 // Use CauseCode for specific result code

	// HTTP counters (1200-1299)
	CounterHTTPTotal      = 1200
	CounterHTTPSuccess    = 1201
	CounterHTTPFailed     = 1202
	CounterHTTPStatusCode = 1203 // Use CauseCode for specific status code

	// Performance counters (1300-1399)
	CounterRequestsPerSecond = 1300
	CounterAvgLatencyMs      = 1301
	CounterMinLatencyMs      = 1302
	CounterMaxLatencyMs      = 1303
	CounterP50LatencyMs      = 1304
	CounterP95LatencyMs      = 1305
	CounterP99LatencyMs      = 1306

	// Cache counters (1400-1499)
	CounterCacheHits    = 1400
	CounterCacheMisses  = 1401
	CounterCacheHitRate = 1402
	CounterCacheSize    = 1403

	// Database counters (1500-1599)
	CounterDBQueries = 1500
	CounterDBInserts = 1501
	CounterDBUpdates = 1502
	CounterDBDeletes = 1503

	// Equipment status counters (1600-1699)
	CounterWhitelisted = 1600
	CounterBlacklisted = 1601
	CounterGreylisted  = 1602

	// Connection counters (1700-1799)
	CounterActiveConnections = 1700
	CounterTotalConnections  = 1701
	CounterFailedConnections = 1702
)

// CounterMetadata provides metadata about counter IDs
type CounterMetadata struct {
	ID          int
	Name        string
	Description string
	Unit        string
	Type        string // "counter", "gauge", "rate"
}

// GetCounterMetadata returns metadata for all defined counters
func GetCounterMetadata() []CounterMetadata {
	return []CounterMetadata{
		// General request counters
		{CounterTotalRequests, "total_requests", "Total number of requests processed", "count", "counter"},
		{CounterSuccessfulRequests, "successful_requests", "Total number of successful requests", "count", "counter"},
		{CounterFailedRequests, "failed_requests", "Total number of failed requests", "count", "counter"},
		{CounterPendingRequests, "pending_requests", "Number of requests currently pending", "count", "gauge"},

		// Diameter counters
		{CounterDiameterTotal, "diameter_total", "Total Diameter requests", "count", "counter"},
		{CounterDiameterSuccess, "diameter_success", "Successful Diameter requests", "count", "counter"},
		{CounterDiameterFailed, "diameter_failed", "Failed Diameter requests", "count", "counter"},
		{CounterDiameterResultCode, "diameter_result_code", "Diameter result code distribution", "count", "counter"},

		// HTTP counters
		{CounterHTTPTotal, "http_total", "Total HTTP requests", "count", "counter"},
		{CounterHTTPSuccess, "http_success", "Successful HTTP requests", "count", "counter"},
		{CounterHTTPFailed, "http_failed", "Failed HTTP requests", "count", "counter"},
		{CounterHTTPStatusCode, "http_status_code", "HTTP status code distribution", "count", "counter"},

		// Performance counters
		{CounterRequestsPerSecond, "requests_per_second", "Request throughput rate", "requests/sec", "gauge"},
		{CounterAvgLatencyMs, "avg_latency_ms", "Average request latency", "milliseconds", "gauge"},
		{CounterMinLatencyMs, "min_latency_ms", "Minimum request latency", "milliseconds", "gauge"},
		{CounterMaxLatencyMs, "max_latency_ms", "Maximum request latency", "milliseconds", "gauge"},
		{CounterP50LatencyMs, "p50_latency_ms", "50th percentile latency", "milliseconds", "gauge"},
		{CounterP95LatencyMs, "p95_latency_ms", "95th percentile latency", "milliseconds", "gauge"},
		{CounterP99LatencyMs, "p99_latency_ms", "99th percentile latency", "milliseconds", "gauge"},

		// Cache counters
		{CounterCacheHits, "cache_hits", "Number of cache hits", "count", "counter"},
		{CounterCacheMisses, "cache_misses", "Number of cache misses", "count", "counter"},
		{CounterCacheHitRate, "cache_hit_rate", "Cache hit rate percentage", "percent", "gauge"},
		{CounterCacheSize, "cache_size", "Current cache size", "entries", "gauge"},

		// Database counters
		{CounterDBQueries, "db_queries", "Total database queries", "count", "counter"},
		{CounterDBInserts, "db_inserts", "Total database inserts", "count", "counter"},
		{CounterDBUpdates, "db_updates", "Total database updates", "count", "counter"},
		{CounterDBDeletes, "db_deletes", "Total database deletes", "count", "counter"},

		// Equipment status counters
		{CounterWhitelisted, "whitelisted", "Whitelisted equipment checks", "count", "counter"},
		{CounterBlacklisted, "blacklisted", "Blacklisted equipment checks", "count", "counter"},
		{CounterGreylisted, "greylisted", "Greylisted equipment checks", "count", "counter"},

		// Connection counters
		{CounterActiveConnections, "active_connections", "Currently active connections", "count", "gauge"},
		{CounterTotalConnections, "total_connections", "Total connections established", "count", "counter"},
		{CounterFailedConnections, "failed_connections", "Failed connection attempts", "count", "counter"},
	}
}

// GetCounterName returns the human-readable name for a counter ID
func GetCounterName(counterID int) string {
	metadata := GetCounterMetadata()
	for _, m := range metadata {
		if m.ID == counterID {
			return m.Name
		}
	}
	return "unknown"
}
