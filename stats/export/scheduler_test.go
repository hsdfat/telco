package export

import (
	"testing"
	"time"

	statsmodel "github.com/hsdfat/telco/stats"
)

// mockLogger for testing
type mockLogger struct{}

func (m *mockLogger) Infow(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Errorw(msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) Debugw(msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) Warnw(msg string, keysAndValues ...interface{})  {}

// mockStatsCollector for testing
type mockStatsCollector struct {
	stats *statsmodel.ServiceStats
}

func (m *mockStatsCollector) GetStats() interface{} {
	return m.stats
}

func (m *mockStatsCollector) RecordRequest(source string, success bool)    {}
func (m *mockStatsCollector) RecordResultCode(source string, code int)     {}
func (m *mockStatsCollector) RecordCacheHit(hit bool)                      {}
func (m *mockStatsCollector) RecordDatabaseOperation(operation string)     {}
func (m *mockStatsCollector) RecordEquipmentStatus(status string)          {}
func (m *mockStatsCollector) SetActiveConnections(count int64)             {}
func (m *mockStatsCollector) IncrementActiveConnections()                  {}
func (m *mockStatsCollector) DecrementActiveConnections()                  {}

// TestDeltaCalculation tests that the scheduler correctly calculates delta metrics
func TestDeltaCalculation(t *testing.T) {
	// Create mock logger and stats collector
	logger := &mockLogger{}
	transformer := NewTransformer("test-host", "test-system")

	// Initial stats (first snapshot)
	initialStats := &statsmodel.ServiceStats{
		ServiceName:    "EIR",
		ServiceVersion: "1.0.0",
		Uptime:         "1m",
		Timestamp:      time.Now(),
		Requests: statsmodel.RequestStats{
			Total:   100,
			Success: 90,
			Failed:  10,
			BySource: map[string]statsmodel.SourceStats{
				"diameter": {Total: 60, Success: 55, Failed: 5},
				"http":     {Total: 40, Success: 35, Failed: 5},
			},
		},
		Connections: statsmodel.ConnectionStats{
			Active: 5,
			Total:  100,
			Failed: 2,
		},
		CustomMetrics: map[string]interface{}{
			"eir": &statsmodel.EIRStats{
				EquipmentChecks: statsmodel.EquipmentCheckStats{
					Total:   100,
					Success: 90,
					Failed:  10,
					ByInterface: map[string]statsmodel.InterfaceCheckStats{
						"diameter": {
							Total:        60,
							Success:      55,
							Failed:       5,
							ByResultCode: map[int]uint64{2001: 55, 5012: 5},
						},
						"http": {
							Total:        40,
							Success:      35,
							Failed:       5,
							ByResultCode: map[int]uint64{200: 35, 404: 5},
						},
					},
				},
				CacheStats: statsmodel.CacheStats{
					Hits:   80,
					Misses: 20,
					Size:   1000,
				},
				DatabaseOps: statsmodel.DatabaseOperationStats{
					Queries: 50,
					Inserts: 30,
					Updates: 10,
					Deletes: 5,
				},
				ByEquipmentStatus: map[string]uint64{
					"whitelisted": 70,
					"blacklisted": 20,
					"greylisted":  10,
				},
			},
		},
	}

	mockStats := &mockStatsCollector{stats: initialStats}
	scheduler := NewExportScheduler(1*time.Minute, mockStats, transformer, logger)

	// First export cycle - should return full stats (no previous snapshot)
	t.Run("FirstExport_ReturnsFullStats", func(t *testing.T) {
		delta := scheduler.calculateDeltaStats(initialStats)

		if delta.Requests.Total != 100 {
			t.Errorf("First export: Expected Total=100, got %d", delta.Requests.Total)
		}
		if delta.Requests.Success != 90 {
			t.Errorf("First export: Expected Success=90, got %d", delta.Requests.Success)
		}

		// Store this as previous snapshot
		scheduler.updatePreviousSnapshot(initialStats)
	})

	// Simulate some time passing and new requests coming in
	updatedStats := &statsmodel.ServiceStats{
		ServiceName:    "EIR",
		ServiceVersion: "1.0.0",
		Uptime:         "2m",
		Timestamp:      time.Now(),
		Requests: statsmodel.RequestStats{
			Total:   150, // +50 new requests
			Success: 135, // +45 successful
			Failed:  15,  // +5 failed
			BySource: map[string]statsmodel.SourceStats{
				"diameter": {Total: 90, Success: 82, Failed: 8},   // +30 total, +27 success, +3 failed
				"http":     {Total: 60, Success: 53, Failed: 7},   // +20 total, +18 success, +2 failed
			},
		},
		Connections: statsmodel.ConnectionStats{
			Active: 8,   // Current active connections (gauge)
			Total:  150, // +50 new connections
			Failed: 5,   // +3 failed connections
		},
		CustomMetrics: map[string]interface{}{
			"eir": &statsmodel.EIRStats{
				EquipmentChecks: statsmodel.EquipmentCheckStats{
					Total:   150,
					Success: 135,
					Failed:  15,
					ByInterface: map[string]statsmodel.InterfaceCheckStats{
						"diameter": {
							Total:        90,
							Success:      82,
							Failed:       8,
							ByResultCode: map[int]uint64{2001: 82, 5012: 8}, // +27, +3
						},
						"http": {
							Total:        60,
							Success:      53,
							Failed:       7,
							ByResultCode: map[int]uint64{200: 53, 404: 7}, // +18, +2
						},
					},
				},
				CacheStats: statsmodel.CacheStats{
					Hits:   120, // +40 hits
					Misses: 30,  // +10 misses
					Size:   1200, // Current size (gauge)
				},
				DatabaseOps: statsmodel.DatabaseOperationStats{
					Queries: 75,  // +25 queries
					Inserts: 45,  // +15 inserts
					Updates: 15,  // +5 updates
					Deletes: 8,   // +3 deletes
				},
				ByEquipmentStatus: map[string]uint64{
					"whitelisted": 105, // +35
					"blacklisted": 30,  // +10
					"greylisted":  15,  // +5
				},
			},
		},
	}

	// Second export cycle - should return only deltas
	t.Run("SecondExport_ReturnsDeltaOnly", func(t *testing.T) {
		delta := scheduler.calculateDeltaStats(updatedStats)

		// Verify counter deltas
		if delta.Requests.Total != 50 {
			t.Errorf("Delta: Expected Total=50 (150-100), got %d", delta.Requests.Total)
		}
		if delta.Requests.Success != 45 {
			t.Errorf("Delta: Expected Success=45 (135-90), got %d", delta.Requests.Success)
		}
		if delta.Requests.Failed != 5 {
			t.Errorf("Delta: Expected Failed=5 (15-10), got %d", delta.Requests.Failed)
		}

		// Verify gauge values (should be current values, not deltas)
		if delta.Connections.Active != 8 {
			t.Errorf("Delta: Expected Active=8 (current gauge), got %d", delta.Connections.Active)
		}

		// Verify connection counter deltas
		if delta.Connections.Total != 50 {
			t.Errorf("Delta: Expected Connections.Total=50 (150-100), got %d", delta.Connections.Total)
		}
		if delta.Connections.Failed != 3 {
			t.Errorf("Delta: Expected Connections.Failed=3 (5-2), got %d", delta.Connections.Failed)
		}

		// Verify BySource deltas
		diamDelta := delta.Requests.BySource["diameter"]
		if diamDelta.Total != 30 {
			t.Errorf("Delta: Expected diameter.Total=30 (90-60), got %d", diamDelta.Total)
		}
		if diamDelta.Success != 27 {
			t.Errorf("Delta: Expected diameter.Success=27 (82-55), got %d", diamDelta.Success)
		}
		if diamDelta.Failed != 3 {
			t.Errorf("Delta: Expected diameter.Failed=3 (8-5), got %d", diamDelta.Failed)
		}

		httpDelta := delta.Requests.BySource["http"]
		if httpDelta.Total != 20 {
			t.Errorf("Delta: Expected http.Total=20 (60-40), got %d", httpDelta.Total)
		}

		// Verify EIR-specific deltas
		eirDelta := delta.CustomMetrics["eir"].(*statsmodel.EIRStats)
		if eirDelta.EquipmentChecks.Total != 50 {
			t.Errorf("Delta: Expected EIR.Total=50, got %d", eirDelta.EquipmentChecks.Total)
		}

		// Verify cache deltas
		if eirDelta.CacheStats.Hits != 40 {
			t.Errorf("Delta: Expected CacheHits=40 (120-80), got %d", eirDelta.CacheStats.Hits)
		}
		if eirDelta.CacheStats.Misses != 10 {
			t.Errorf("Delta: Expected CacheMisses=10 (30-20), got %d", eirDelta.CacheStats.Misses)
		}
		// Cache size is a gauge, should be current value
		if eirDelta.CacheStats.Size != 1200 {
			t.Errorf("Delta: Expected CacheSize=1200 (gauge), got %d", eirDelta.CacheStats.Size)
		}

		// Verify DB operation deltas
		if eirDelta.DatabaseOps.Queries != 25 {
			t.Errorf("Delta: Expected DBQueries=25 (75-50), got %d", eirDelta.DatabaseOps.Queries)
		}
		if eirDelta.DatabaseOps.Inserts != 15 {
			t.Errorf("Delta: Expected DBInserts=15 (45-30), got %d", eirDelta.DatabaseOps.Inserts)
		}

		// Verify equipment status deltas
		if eirDelta.ByEquipmentStatus["whitelisted"] != 35 {
			t.Errorf("Delta: Expected Whitelisted=35 (105-70), got %d", eirDelta.ByEquipmentStatus["whitelisted"])
		}
		if eirDelta.ByEquipmentStatus["blacklisted"] != 10 {
			t.Errorf("Delta: Expected Blacklisted=10 (30-20), got %d", eirDelta.ByEquipmentStatus["blacklisted"])
		}

		// Verify interface result code deltas
		diamInterface := eirDelta.EquipmentChecks.ByInterface["diameter"]
		if diamInterface.ByResultCode[2001] != 27 {
			t.Errorf("Delta: Expected diameter:2001=27 (82-55), got %d", diamInterface.ByResultCode[2001])
		}
		if diamInterface.ByResultCode[5012] != 3 {
			t.Errorf("Delta: Expected diameter:5012=3 (8-5), got %d", diamInterface.ByResultCode[5012])
		}

		httpInterface := eirDelta.EquipmentChecks.ByInterface["http"]
		if httpInterface.ByResultCode[200] != 18 {
			t.Errorf("Delta: Expected http:200=18 (53-35), got %d", httpInterface.ByResultCode[200])
		}
		if httpInterface.ByResultCode[404] != 2 {
			t.Errorf("Delta: Expected http:404=2 (7-5), got %d", httpInterface.ByResultCode[404])
		}
	})
}

// TestSafeSubtraction tests the safeSub64 helper function
func TestSafeSubtraction(t *testing.T) {
	tests := []struct {
		name     string
		a, b     uint64
		expected uint64
	}{
		{"Normal subtraction", 100, 50, 50},
		{"Equal values", 100, 100, 0},
		{"Would be negative", 50, 100, 0},
		{"Zero subtraction", 100, 0, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeSub64(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("safeSub64(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// TestMapDeltaCalculation tests map delta calculation helpers
func TestMapDeltaCalculation(t *testing.T) {
	t.Run("StringMap", func(t *testing.T) {
		current := map[string]uint64{"a": 100, "b": 50, "c": 30}
		prev := map[string]uint64{"a": 70, "b": 50, "c": 20}

		delta := calculateMapDelta64(current, prev)

		if delta["a"] != 30 {
			t.Errorf("Expected a=30, got %d", delta["a"])
		}
		if delta["b"] != 0 { // No change
			t.Errorf("Expected b=0, got %d", delta["b"])
		}
		if delta["c"] != 10 {
			t.Errorf("Expected c=10, got %d", delta["c"])
		}
	})

	t.Run("IntMap", func(t *testing.T) {
		current := map[int]uint64{200: 100, 404: 50, 500: 30}
		prev := map[int]uint64{200: 70, 404: 50, 500: 20}

		delta := calculateMapDeltaInt64(current, prev)

		if delta[200] != 30 {
			t.Errorf("Expected 200=30, got %d", delta[200])
		}
		if delta[404] != 0 { // No change
			t.Errorf("Expected 404=0, got %d", delta[404])
		}
		if delta[500] != 10 {
			t.Errorf("Expected 500=10, got %d", delta[500])
		}
	})

	t.Run("NewKeyInCurrent", func(t *testing.T) {
		current := map[string]uint64{"a": 100, "b": 50}
		prev := map[string]uint64{"a": 70}

		delta := calculateMapDelta64(current, prev)

		if delta["a"] != 30 {
			t.Errorf("Expected a=30, got %d", delta["a"])
		}
		if delta["b"] != 50 { // New key
			t.Errorf("Expected b=50 (new key), got %d", delta["b"])
		}
	})
}
