package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchStats fetches stats from a service endpoint and converts to unified model
func FetchStats(url string, serviceType string) (*ServiceStats, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	switch serviceType {
	case "prometheus":
		return ParsePrometheusMetrics(string(body))
	case "json":
		var stats ServiceStats
		if err := json.Unmarshal(body, &stats); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON stats: %w", err)
		}
		return &stats, nil
	default:
		return nil, fmt.Errorf("unsupported service type: %s", serviceType)
	}
}

// ParsePrometheusMetrics converts Prometheus text format to unified stats model
func ParsePrometheusMetrics(metricsText string) (*ServiceStats, error) {
	stats := &ServiceStats{
		ServiceName: "EIR",
		Timestamp:   time.Now(),
		Connections: ConnectionStats{},
		Requests:    RequestStats{
			BySource:    make(map[string]SourceStats),
			ByOperation: make(map[string]OperationStats),
		},
		Performance: PerformanceStats{},
		Errors:      ErrorStats{
			ByType:      make(map[string]uint64),
			ByInterface: make(map[string]uint64),
		},
		CustomMetrics: make(map[string]interface{}),
	}

	lines := strings.Split(metricsText, "\n")
	metricsMap := make(map[string]float64)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse metric line: metric_name{labels} value
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			metricName := parts[0]
			var value float64
			fmt.Sscanf(parts[1], "%f", &value)
			metricsMap[metricName] = value
		}
	}

	// Parse EIR-specific metrics
	stats.Requests.Total = uint64(metricsMap["eir_equipment_check_total{source=\"diameter\",status=\"success\"}"] +
		metricsMap["eir_equipment_check_total{source=\"diameter\",status=\"error\"}"] +
		metricsMap["eir_equipment_check_total{source=\"http\",status=\"success\"}"] +
		metricsMap["eir_equipment_check_total{source=\"http\",status=\"error\"}"])

	stats.Requests.Success = uint64(metricsMap["eir_equipment_check_total{source=\"diameter\",status=\"success\"}"] +
		metricsMap["eir_equipment_check_total{source=\"http\",status=\"success\"}"])

	stats.Requests.Failed = uint64(metricsMap["eir_equipment_check_total{source=\"diameter\",status=\"error\"}"] +
		metricsMap["eir_equipment_check_total{source=\"http\",status=\"error\"}"])

	// By source stats
	stats.Requests.BySource["diameter"] = SourceStats{
		Total:   uint64(metricsMap["eir_equipment_check_total{source=\"diameter\",status=\"success\"}"] + metricsMap["eir_equipment_check_total{source=\"diameter\",status=\"error\"}"]),
		Success: uint64(metricsMap["eir_equipment_check_total{source=\"diameter\",status=\"success\"}"]),
		Failed:  uint64(metricsMap["eir_equipment_check_total{source=\"diameter\",status=\"error\"}"]),
	}

	stats.Requests.BySource["http"] = SourceStats{
		Total:   uint64(metricsMap["eir_equipment_check_total{source=\"http\",status=\"success\"}"] + metricsMap["eir_equipment_check_total{source=\"http\",status=\"error\"}"]),
		Success: uint64(metricsMap["eir_equipment_check_total{source=\"http\",status=\"success\"}"]),
		Failed:  uint64(metricsMap["eir_equipment_check_total{source=\"http\",status=\"error\"}"]),
	}

	// Connection stats
	stats.Connections.Active = uint64(metricsMap["eir_active_diameter_connections"])

	// Cache stats
	cacheHits := metricsMap["eir_cache_hit_total{result=\"hit\"}"]
	cacheMisses := metricsMap["eir_cache_hit_total{result=\"miss\"}"]
	cacheTotal := cacheHits + cacheMisses
	hitRate := 0.0
	if cacheTotal > 0 {
		hitRate = (cacheHits / cacheTotal) * 100
	}

	stats.CustomMetrics["cache"] = CacheStats{
		Hits:    uint64(cacheHits),
		Misses:  uint64(cacheMisses),
		HitRate: hitRate,
	}

	// Equipment by status
	eirStats := EIRStats{
		ByEquipmentStatus: map[string]uint64{
			"whitelisted": uint64(metricsMap["eir_equipment_by_status{status=\"whitelisted\"}"]),
			"blacklisted": uint64(metricsMap["eir_equipment_by_status{status=\"blacklisted\"}"]),
			"greylisted":  uint64(metricsMap["eir_equipment_by_status{status=\"greylisted\"}"]),
		},
	}
	stats.CustomMetrics["eir"] = eirStats

	return stats, nil
}

// CompareStats compares two ServiceStats and returns the difference
func CompareStats(before, after *ServiceStats) *ServiceStats {
	diff := &ServiceStats{
		ServiceName: after.ServiceName,
		Timestamp:   after.Timestamp,
		Connections: ConnectionStats{
			Total:  after.Connections.Total - before.Connections.Total,
			Active: after.Connections.Active,
			Failed: after.Connections.Failed - before.Connections.Failed,
			Closed: after.Connections.Closed - before.Connections.Closed,
		},
		Requests: RequestStats{
			Total:     after.Requests.Total - before.Requests.Total,
			Success:   after.Requests.Success - before.Requests.Success,
			Failed:    after.Requests.Failed - before.Requests.Failed,
			Pending:   after.Requests.Pending,
			BytesSent: after.Requests.BytesSent - before.Requests.BytesSent,
			BytesRecv: after.Requests.BytesRecv - before.Requests.BytesRecv,
			BySource:  make(map[string]SourceStats),
		},
		Performance: after.Performance,
		Errors: ErrorStats{
			Total:  after.Errors.Total - before.Errors.Total,
			ByType: make(map[string]uint64),
		},
	}

	// Calculate by-source differences
	for source, afterStats := range after.Requests.BySource {
		beforeStats := before.Requests.BySource[source]
		diff.Requests.BySource[source] = SourceStats{
			Total:   afterStats.Total - beforeStats.Total,
			Success: afterStats.Success - beforeStats.Success,
			Failed:  afterStats.Failed - beforeStats.Failed,
		}
	}

	return diff
}

// ValidateStats checks if the actual stats match expected values within tolerance
func ValidateStats(expected, actual uint64, tolerance uint64) (bool, string) {
	var diff uint64
	if actual > expected {
		diff = actual - expected
	} else {
		diff = expected - actual
	}

	if diff <= tolerance {
		return true, fmt.Sprintf("✓ Match (expected: %d, actual: %d, diff: %d)", expected, actual, diff)
	}

	return false, fmt.Sprintf("✗ Mismatch (expected: %d, actual: %d, diff: %d, tolerance: %d)", expected, actual, diff, tolerance)
}

// FormatStatsReport generates a human-readable stats report
func FormatStatsReport(stats *ServiceStats) string {
	report := strings.Builder{}
	report.WriteString(fmt.Sprintf("=== %s Statistics ===\n", stats.ServiceName))
	report.WriteString(fmt.Sprintf("Timestamp: %s\n\n", stats.Timestamp.Format(time.RFC3339)))

	// Connections
	report.WriteString("Connections:\n")
	report.WriteString(fmt.Sprintf("  Total: %d\n", stats.Connections.Total))
	report.WriteString(fmt.Sprintf("  Active: %d\n", stats.Connections.Active))
	report.WriteString(fmt.Sprintf("  Failed: %d\n", stats.Connections.Failed))
	report.WriteString(fmt.Sprintf("  Closed: %d\n\n", stats.Connections.Closed))

	// Requests
	report.WriteString("Requests:\n")
	report.WriteString(fmt.Sprintf("  Total: %d\n", stats.Requests.Total))
	report.WriteString(fmt.Sprintf("  Success: %d\n", stats.Requests.Success))
	report.WriteString(fmt.Sprintf("  Failed: %d\n", stats.Requests.Failed))
	report.WriteString(fmt.Sprintf("  Pending: %d\n", stats.Requests.Pending))
	report.WriteString(fmt.Sprintf("  Bytes Sent: %d\n", stats.Requests.BytesSent))
	report.WriteString(fmt.Sprintf("  Bytes Recv: %d\n\n", stats.Requests.BytesRecv))

	// By source
	if len(stats.Requests.BySource) > 0 {
		report.WriteString("By Source:\n")
		for source, srcStats := range stats.Requests.BySource {
			report.WriteString(fmt.Sprintf("  %s:\n", source))
			report.WriteString(fmt.Sprintf("    Total: %d\n", srcStats.Total))
			report.WriteString(fmt.Sprintf("    Success: %d\n", srcStats.Success))
			report.WriteString(fmt.Sprintf("    Failed: %d\n", srcStats.Failed))
		}
		report.WriteString("\n")
	}

	// Performance
	report.WriteString("Performance:\n")
	report.WriteString(fmt.Sprintf("  Requests/sec: %.2f\n", stats.Performance.RequestsPerSecond))
	report.WriteString(fmt.Sprintf("  Avg Latency: %.3f ms\n", stats.Performance.AvgLatencyMs))
	report.WriteString(fmt.Sprintf("  Min Latency: %.3f ms\n", stats.Performance.MinLatencyMs))
	report.WriteString(fmt.Sprintf("  Max Latency: %.3f ms\n\n", stats.Performance.MaxLatencyMs))

	// Errors
	report.WriteString("Errors:\n")
	report.WriteString(fmt.Sprintf("  Total: %d\n", stats.Errors.Total))

	return report.String()
}
