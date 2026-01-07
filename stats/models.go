package stats

import "time"

// ServiceStats represents unified statistics for any service (EIR, Diam-GW, HTTP-GW)
type ServiceStats struct {
	ServiceName     string                 `json:"service_name"`
	ServiceVersion  string                 `json:"service_version,omitempty"`
	Uptime          string                 `json:"uptime"`
	Timestamp       time.Time              `json:"timestamp"`
	Connections     ConnectionStats        `json:"connections"`
	Requests        RequestStats           `json:"requests"`
	Performance     PerformanceStats       `json:"performance"`
	Errors          ErrorStats             `json:"errors"`
	InterfaceStats  map[string]interface{} `json:"interface_stats,omitempty"`  // Interface-specific stats
	CustomMetrics   map[string]interface{} `json:"custom_metrics,omitempty"`   // Service-specific metrics
}

// ConnectionStats tracks connection-related statistics
type ConnectionStats struct {
	Total   uint64 `json:"total"`    // Total connections ever established
	Active  uint64 `json:"active"`   // Currently active connections
	Failed  uint64 `json:"failed"`   // Failed connection attempts
	Closed  uint64 `json:"closed"`   // Gracefully closed connections
}

// RequestStats tracks request/response statistics
type RequestStats struct {
	Total       uint64 `json:"total"`        // Total requests processed
	Success     uint64 `json:"success"`      // Successful requests
	Failed      uint64 `json:"failed"`       // Failed requests
	Pending     uint64 `json:"pending"`      // Requests in progress
	BytesSent   uint64 `json:"bytes_sent"`   // Total bytes sent
	BytesRecv   uint64 `json:"bytes_recv"`   // Total bytes received
	BySource    map[string]SourceStats `json:"by_source,omitempty"`  // Stats by source (diameter, http, etc)
	ByOperation map[string]OperationStats `json:"by_operation,omitempty"` // Stats by operation type
}

// SourceStats tracks statistics by source interface
type SourceStats struct {
	Total   uint64 `json:"total"`
	Success uint64 `json:"success"`
	Failed  uint64 `json:"failed"`
}

// OperationStats tracks statistics by operation type
type OperationStats struct {
	Total   uint64  `json:"total"`
	Success uint64  `json:"success"`
	Failed  uint64  `json:"failed"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// PerformanceStats tracks performance-related statistics
type PerformanceStats struct {
	RequestsPerSecond float64       `json:"requests_per_second"`
	AvgLatencyMs      float64       `json:"avg_latency_ms"`
	MinLatencyMs      float64       `json:"min_latency_ms"`
	MaxLatencyMs      float64       `json:"max_latency_ms"`
	P50LatencyMs      float64       `json:"p50_latency_ms,omitempty"`
	P95LatencyMs      float64       `json:"p95_latency_ms,omitempty"`
	P99LatencyMs      float64       `json:"p99_latency_ms,omitempty"`
}

// ErrorStats tracks error-related statistics
type ErrorStats struct {
	Total       uint64            `json:"total"`
	ByType      map[string]uint64 `json:"by_type,omitempty"`      // Errors by type/code
	ByInterface map[string]uint64 `json:"by_interface,omitempty"` // Errors by interface
	LastError   *ErrorInfo        `json:"last_error,omitempty"`
}

// ErrorInfo contains information about an error
type ErrorInfo struct {
	Message   string    `json:"message"`
	Code      string    `json:"code,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Count     uint64    `json:"count"` // How many times this error occurred
}

// DiameterStats contains Diameter-specific statistics
type DiameterStats struct {
	Applications map[int]ApplicationStats `json:"applications"` // Stats by Application-ID
}

// ApplicationStats tracks statistics for a Diameter application
type ApplicationStats struct {
	ApplicationID int                    `json:"application_id"`
	Name          string                 `json:"name,omitempty"`
	MessagesSent  uint64                 `json:"messages_sent"`
	MessagesRecv  uint64                 `json:"messages_recv"`
	BytesSent     uint64                 `json:"bytes_sent"`
	BytesRecv     uint64                 `json:"bytes_recv"`
	Errors        uint64                 `json:"errors"`
	Commands      map[int]CommandStats   `json:"commands,omitempty"` // Stats by Command-Code
}

// CommandStats tracks statistics for a Diameter command
type CommandStats struct {
	CommandCode  int    `json:"command_code"`
	Name         string `json:"name,omitempty"`
	RequestsSent uint64 `json:"requests_sent"`
	RequestsRecv uint64 `json:"requests_recv"`
	AnswersSent  uint64 `json:"answers_sent"`
	AnswersRecv  uint64 `json:"answers_recv"`
	Errors       uint64 `json:"errors"`
}

// HTTPStats contains HTTP-specific statistics
type HTTPStats struct {
	ByEndpoint map[string]EndpointStats `json:"by_endpoint,omitempty"`
	ByMethod   map[string]MethodStats   `json:"by_method,omitempty"`
	ByStatus   map[int]uint64           `json:"by_status,omitempty"` // Status code distribution
}

// EndpointStats tracks statistics for an HTTP endpoint
type EndpointStats struct {
	Path         string  `json:"path"`
	Requests     uint64  `json:"requests"`
	Success      uint64  `json:"success"`
	Errors       uint64  `json:"errors"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// MethodStats tracks statistics for an HTTP method
type MethodStats struct {
	Method   string `json:"method"`
	Requests uint64 `json:"requests"`
	Success  uint64 `json:"success"`
	Errors   uint64 `json:"errors"`
}

// EIRStats contains EIR-specific statistics
type EIRStats struct {
	EquipmentChecks   EquipmentCheckStats      `json:"equipment_checks"`
	DatabaseOps       DatabaseOperationStats   `json:"database_operations"`
	CacheStats        CacheStats               `json:"cache_stats"`
	ByEquipmentStatus map[string]uint64        `json:"by_equipment_status,omitempty"` // whitelisted, blacklisted, greylisted
}

// EquipmentCheckStats tracks equipment check statistics
type EquipmentCheckStats struct {
	Total        uint64            `json:"total"`
	Success      uint64            `json:"success"`
	Failed       uint64            `json:"failed"`
	ByInterface  map[string]uint64 `json:"by_interface,omitempty"`  // diameter, http
	ByResultCode map[int]uint64    `json:"by_result_code,omitempty"` // Result code distribution
}

// DatabaseOperationStats tracks database operation statistics
type DatabaseOperationStats struct {
	Queries        uint64  `json:"queries"`
	Inserts        uint64  `json:"inserts"`
	Updates        uint64  `json:"updates"`
	Deletes        uint64  `json:"deletes"`
	Errors         uint64  `json:"errors"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	ActiveQueries  uint64  `json:"active_queries"`
}

// CacheStats tracks cache statistics
type CacheStats struct {
	Hits        uint64  `json:"hits"`
	Misses      uint64  `json:"misses"`
	HitRate     float64 `json:"hit_rate"` // Percentage
	Size        uint64  `json:"size"`     // Number of entries
	MaxSize     uint64  `json:"max_size"`
	Evictions   uint64  `json:"evictions"`
}

// StatsResponse is the standard HTTP response format for stats endpoints
type StatsResponse struct {
	Status  string       `json:"status"`  // "success" or "error"
	Message string       `json:"message,omitempty"`
	Data    ServiceStats `json:"data"`
}

// HealthStatus represents the health status of a service
type HealthStatus struct {
	Status    string            `json:"status"` // "healthy", "degraded", "unhealthy"
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]Check  `json:"checks,omitempty"`
}

// Check represents a health check result
type Check struct {
	Status   string `json:"status"` // "pass", "warn", "fail"
	Message  string `json:"message,omitempty"`
	Duration string `json:"duration,omitempty"`
}
