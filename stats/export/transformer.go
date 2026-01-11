package export

import (
	"fmt"
	"time"

	statsmodel "github.com/hsdfat/telco/stats"
)

// Transformer converts ServiceStats into a list of MetricRecords
type Transformer struct {
	hostname   string
	systemName string
	config     TransformerConfig
}

// NewTransformer creates a transformer with hostname and system name
func NewTransformer(hostname, systemName string) *Transformer {
	return &Transformer{
		hostname:   hostname,
		systemName: systemName,
		config: TransformerConfig{
			SampleRate: 1.0, // Default: export all metrics
		},
	}
}

// NewTransformerWithConfig creates a transformer with custom configuration
func NewTransformerWithConfig(hostname, systemName string, config TransformerConfig) *Transformer {
	return &Transformer{
		hostname:   hostname,
		systemName: systemName,
		config:     config,
	}
}

// Transform converts ServiceStats to MetricRecords
func (t *Transformer) Transform(stats *statsmodel.ServiceStats) []MetricRecord {
	records := make([]MetricRecord, 0, 100)
	timestamp := stats.Timestamp

	// General request metrics
	records = append(records, t.createRecord(CounterTotalRequests, float64(stats.Requests.Total), "", timestamp))
	records = append(records, t.createRecord(CounterSuccessfulRequests, float64(stats.Requests.Success), "", timestamp))
	records = append(records, t.createRecord(CounterFailedRequests, float64(stats.Requests.Failed), "", timestamp))

	if stats.Requests.Pending > 0 {
		records = append(records, t.createRecord(CounterPendingRequests, float64(stats.Requests.Pending), "", timestamp))
	}

	// Connection metrics
	if stats.Connections.Active > 0 {
		records = append(records, t.createRecord(CounterActiveConnections, float64(stats.Connections.Active), "", timestamp))
	}
	if stats.Connections.Total > 0 {
		records = append(records, t.createRecord(CounterTotalConnections, float64(stats.Connections.Total), "", timestamp))
	}
	if stats.Connections.Failed > 0 {
		records = append(records, t.createRecord(CounterFailedConnections, float64(stats.Connections.Failed), "", timestamp))
	}

	// Performance metrics
	if stats.Performance.RequestsPerSecond > 0 {
		records = append(records, t.createRecord(CounterRequestsPerSecond, stats.Performance.RequestsPerSecond, "", timestamp))
	}
	if stats.Performance.AvgLatencyMs > 0 {
		records = append(records, t.createRecord(CounterAvgLatencyMs, stats.Performance.AvgLatencyMs, "", timestamp))
	}
	if stats.Performance.MinLatencyMs > 0 {
		records = append(records, t.createRecord(CounterMinLatencyMs, stats.Performance.MinLatencyMs, "", timestamp))
	}
	if stats.Performance.MaxLatencyMs > 0 {
		records = append(records, t.createRecord(CounterMaxLatencyMs, stats.Performance.MaxLatencyMs, "", timestamp))
	}
	if stats.Performance.P50LatencyMs > 0 {
		records = append(records, t.createRecord(CounterP50LatencyMs, stats.Performance.P50LatencyMs, "", timestamp))
	}
	if stats.Performance.P95LatencyMs > 0 {
		records = append(records, t.createRecord(CounterP95LatencyMs, stats.Performance.P95LatencyMs, "", timestamp))
	}
	if stats.Performance.P99LatencyMs > 0 {
		records = append(records, t.createRecord(CounterP99LatencyMs, stats.Performance.P99LatencyMs, "", timestamp))
	}

	// EIR-specific metrics
	if eirStats, ok := stats.CustomMetrics["eir"].(*statsmodel.EIRStats); ok {
		records = append(records, t.transformEIRStats(eirStats, timestamp)...)
	}

	// Filter records based on configuration
	return t.filterRecords(records)
}

// transformEIRStats transforms EIR-specific statistics
func (t *Transformer) transformEIRStats(eirStats *statsmodel.EIRStats, timestamp time.Time) []MetricRecord {
	records := make([]MetricRecord, 0, 50)

	// Interface-specific metrics
	for ifName, ifStats := range eirStats.EquipmentChecks.ByInterface {
		var totalCounter, successCounter, failedCounter, resultCodeCounter int

		// Determine counter IDs based on interface
		switch ifName {
		case "diameter":
			totalCounter = CounterDiameterTotal
			successCounter = CounterDiameterSuccess
			failedCounter = CounterDiameterFailed
			resultCodeCounter = CounterDiameterResultCode
		case "http":
			totalCounter = CounterHTTPTotal
			successCounter = CounterHTTPSuccess
			failedCounter = CounterHTTPFailed
			resultCodeCounter = CounterHTTPStatusCode
		default:
			continue
		}

		// Total per interface
		if ifStats.Total > 0 {
			records = append(records, t.createRecord(totalCounter, float64(ifStats.Total), ifName, timestamp))
		}

		// Success per interface
		if ifStats.Success > 0 {
			records = append(records, t.createRecord(successCounter, float64(ifStats.Success), ifName, timestamp))
		}

		// Failed per interface
		if ifStats.Failed > 0 {
			records = append(records, t.createRecord(failedCounter, float64(ifStats.Failed), ifName, timestamp))
		}

		// Result codes per interface
		for code, count := range ifStats.ByResultCode {
			if count > 0 {
				causeCode := fmt.Sprintf("%s:%d", ifName, code)
				records = append(records, t.createRecord(resultCodeCounter, float64(count), causeCode, timestamp))
			}
		}
	}

	// Cache statistics
	if eirStats.CacheStats.Hits > 0 {
		records = append(records, t.createRecord(CounterCacheHits, float64(eirStats.CacheStats.Hits), "", timestamp))
	}
	if eirStats.CacheStats.Misses > 0 {
		records = append(records, t.createRecord(CounterCacheMisses, float64(eirStats.CacheStats.Misses), "", timestamp))
	}
	if eirStats.CacheStats.HitRate > 0 {
		records = append(records, t.createRecord(CounterCacheHitRate, eirStats.CacheStats.HitRate, "", timestamp))
	}
	if eirStats.CacheStats.Size > 0 {
		records = append(records, t.createRecord(CounterCacheSize, float64(eirStats.CacheStats.Size), "", timestamp))
	}

	// Database operations
	if eirStats.DatabaseOps.Queries > 0 {
		records = append(records, t.createRecord(CounterDBQueries, float64(eirStats.DatabaseOps.Queries), "", timestamp))
	}
	if eirStats.DatabaseOps.Inserts > 0 {
		records = append(records, t.createRecord(CounterDBInserts, float64(eirStats.DatabaseOps.Inserts), "", timestamp))
	}
	if eirStats.DatabaseOps.Updates > 0 {
		records = append(records, t.createRecord(CounterDBUpdates, float64(eirStats.DatabaseOps.Updates), "", timestamp))
	}
	if eirStats.DatabaseOps.Deletes > 0 {
		records = append(records, t.createRecord(CounterDBDeletes, float64(eirStats.DatabaseOps.Deletes), "", timestamp))
	}

	// Equipment status distribution
	if count, ok := eirStats.ByEquipmentStatus["whitelisted"]; ok && count > 0 {
		records = append(records, t.createRecord(CounterWhitelisted, float64(count), "", timestamp))
	}
	if count, ok := eirStats.ByEquipmentStatus["blacklisted"]; ok && count > 0 {
		records = append(records, t.createRecord(CounterBlacklisted, float64(count), "", timestamp))
	}
	if count, ok := eirStats.ByEquipmentStatus["greylisted"]; ok && count > 0 {
		records = append(records, t.createRecord(CounterGreylisted, float64(count), "", timestamp))
	}

	return records
}

// createRecord creates a MetricRecord with proper timestamp handling
func (t *Transformer) createRecord(counterID int, value float64, causeCode string, timestamp time.Time) MetricRecord {
	return MetricRecord{
		CounterID:  counterID,
		Value:      value,
		CauseCode:  causeCode,
		Hostname:   t.hostname,
		SystemName: t.systemName,
		Timestamp:  timestamp,
	}
}

// filterRecords filters records based on configuration
func (t *Transformer) filterRecords(records []MetricRecord) []MetricRecord {
	if len(t.config.IncludeCounters) == 0 && len(t.config.ExcludeCounters) == 0 {
		return records
	}

	filtered := make([]MetricRecord, 0, len(records))

	for _, record := range records {
		// Check if counter is in exclude list
		if t.isExcluded(record.CounterID) {
			continue
		}

		// If include list is specified, only include those counters
		if len(t.config.IncludeCounters) > 0 && !t.isIncluded(record.CounterID) {
			continue
		}

		filtered = append(filtered, record)
	}

	return filtered
}

// isIncluded checks if a counter ID is in the include list
func (t *Transformer) isIncluded(counterID int) bool {
	for _, id := range t.config.IncludeCounters {
		if id == counterID {
			return true
		}
	}
	return false
}

// isExcluded checks if a counter ID is in the exclude list
func (t *Transformer) isExcluded(counterID int) bool {
	for _, id := range t.config.ExcludeCounters {
		if id == counterID {
			return true
		}
	}
	return false
}
