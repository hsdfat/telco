# Unified Statistics Model

This package provides a unified statistics model that can be used across all telco applications (EIR, HTTP-GW, Diam-GW) for consistent metrics collection, reporting, and validation.

## Overview

The unified stats model provides:
- **Common data structures** for statistics across all services
- **Format converters** for different metrics formats (Prometheus, JSON, etc.)
- **Validation utilities** for comparing expected vs actual metrics
- **Human-readable formatting** for reports

## Usage

### 1. Fetching Stats

```go
import "github.com/hsdfat/telco/test-framework/stats"

// Fetch stats from Prometheus endpoint
serviceStats, err := stats.FetchStats("http://localhost:9090/metrics", "prometheus")
if err != nil {
    log.Fatal(err)
}

// Fetch stats from JSON API
serviceStats, err := stats.FetchStats("http://localhost:8080/api/stats", "json")
if err != nil {
    log.Fatal(err)
}
```

### 2. Comparing Stats

```go
// Get initial stats
initialStats, _ := stats.FetchStats(metricsURL, "prometheus")

// ... perform operations ...

// Get final stats
finalStats, _ := stats.FetchStats(metricsURL, "prometheus")

// Calculate difference
diff := stats.CompareStats(initialStats, finalStats)

fmt.Printf("Requests processed: %d\n", diff.Requests.Total)
fmt.Printf("Success: %d\n", diff.Requests.Success)
fmt.Printf("Failed: %d\n", diff.Requests.Failed)
```

### 3. Validating Stats

```go
expected := uint64(100)
actual := uint64(98)
tolerance := uint64(5)

valid, message := stats.ValidateStats(expected, actual, tolerance)
if !valid {
    log.Printf("Validation failed: %s", message)
} else {
    log.Printf("Validation passed: %s", message)
}
```

### 4. Generating Reports

```go
report := stats.FormatStatsReport(serviceStats)
fmt.Println(report)
```

## Data Structures

### ServiceStats

The main statistics container:

```go
type ServiceStats struct {
    ServiceName     string                // e.g., "EIR", "Diam-GW"
    ServiceVersion  string
    Uptime          string
    Timestamp       time.Time
    Connections     ConnectionStats       // Connection statistics
    Requests        RequestStats          // Request/response statistics
    Performance     PerformanceStats      // Performance metrics
    Errors          ErrorStats            // Error tracking
    InterfaceStats  map[string]interface{} // Interface-specific stats
    CustomMetrics   map[string]interface{} // Service-specific metrics
}
```

### RequestStats

Tracks request/response statistics with breakdown by source:

```go
type RequestStats struct {
    Total       uint64
    Success     uint64
    Failed      uint64
    Pending     uint64
    BytesSent   uint64
    BytesRecv   uint64
    BySource    map[string]SourceStats      // e.g., "diameter", "http"
    ByOperation map[string]OperationStats   // e.g., "check", "provision"
}

type SourceStats struct {
    Total   uint64
    Success uint64
    Failed  uint64
}
```

### PerformanceStats

Performance-related metrics:

```go
type PerformanceStats struct {
    RequestsPerSecond float64
    AvgLatencyMs      float64
    MinLatencyMs      float64
    MaxLatencyMs      float64
    P50LatencyMs      float64  // Optional
    P95LatencyMs      float64  // Optional
    P99LatencyMs      float64  // Optional
}
```

## Format Converters

### Prometheus Metrics

The package automatically parses Prometheus text format and maps to unified model:

```
# Prometheus format
eir_equipment_check_total{source="diameter",status="success"} 150
eir_equipment_check_total{source="http",status="success"} 75

# Maps to
ServiceStats.Requests.BySource["diameter"].Success = 150
ServiceStats.Requests.BySource["http"].Success = 75
```

### JSON Stats

Services can expose stats as JSON matching the ServiceStats structure:

```json
{
  "service_name": "EIR",
  "timestamp": "2026-01-07T10:00:00Z",
  "requests": {
    "total": 1000,
    "success": 980,
    "failed": 20,
    "by_source": {
      "diameter": {
        "total": 600,
        "success": 590,
        "failed": 10
      },
      "http": {
        "total": 400,
        "success": 390,
        "failed": 10
      }
    }
  }
}
```

## Integration with Applications

### EIR (Prometheus)

EIR exposes Prometheus metrics at `/metrics` which are automatically converted:

- `eir_equipment_check_total{source,status}` → `Requests.BySource[source]`
- `eir_active_diameter_connections` → `Connections.Active`
- `eir_cache_hit_total{result}` → `CustomMetrics["cache"]`

### Diam-GW (Programmatic)

Diam-GW provides `GetStats()` method. To expose via HTTP:

```go
import "github.com/hsdfat/telco/test-framework/stats"

// Convert Diam-GW stats to unified format
func convertDiamGWStats(gwStats ServerStatsSnapshot) *stats.ServiceStats {
    return &stats.ServiceStats{
        ServiceName: "Diam-GW",
        Timestamp:   time.Now(),
        Connections: stats.ConnectionStats{
            Total:  gwStats.TotalConnections,
            Active: gwStats.ActiveConnections,
        },
        Requests: stats.RequestStats{
            Total:     gwStats.TotalMessages,
            BytesSent: gwStats.TotalBytesSent,
            BytesRecv: gwStats.TotalBytesReceived,
        },
        Errors: stats.ErrorStats{
            Total: gwStats.Errors,
        },
    }
}

// HTTP endpoint
http.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
    gwStats := server.GetStats()
    unifiedStats := convertDiamGWStats(gwStats)

    response := stats.StatsResponse{
        Status: "success",
        Data:   *unifiedStats,
    }

    json.NewEncoder(w).Encode(response)
})
```

### HTTP-GW (JSON)

HTTP-GW already exposes JSON stats. Add converter:

```go
func convertHTTPGWStats(gwStats FrontendStats) *stats.ServiceStats {
    return &stats.ServiceStats{
        ServiceName: "HTTP-GW",
        Timestamp:   time.Now(),
        Requests: stats.RequestStats{
            Total:   gwStats.RequestsTotal,
            Success: gwStats.RequestsSuccess,
            Failed:  gwStats.RequestsError,
        },
        Performance: stats.PerformanceStats{
            AvgLatencyMs: gwStats.AvgLatencyMs,
            MinLatencyMs: gwStats.MinLatencyMs,
            MaxLatencyMs: gwStats.MaxLatencyMs,
        },
    }
}
```

## Performance Testing

The performance test suite uses the unified stats model:

```go
// Get initial stats
initialStats, _ := suite.GetMetrics(ctx)

// Run tests
// ...

// Get final stats and compare
finalStats, _ := suite.GetMetrics(ctx)
diff := stats.CompareStats(initialStats, finalStats)

// Validate
expectedRequests := uint64(100)
actualRequests := diff.Requests.BySource["diameter"].Success
valid, msg := stats.ValidateStats(expectedRequests, actualRequests, 5)
```

## Benefits

1. **Consistency**: Same data structure across all services
2. **Flexibility**: Support for multiple metrics formats
3. **Testability**: Built-in validation with tolerance
4. **Extensibility**: Custom metrics support per service
5. **Observability**: Unified reporting format

## Future Enhancements

- [ ] Add support for time-series data (historical stats)
- [ ] Implement stats aggregation across multiple services
- [ ] Add support for OpenTelemetry format
- [ ] Create Grafana dashboard templates using unified model
- [ ] Add percentile calculations (P50, P95, P99)
