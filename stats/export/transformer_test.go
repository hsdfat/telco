package export

import (
	"testing"
	"time"

	statsmodel "github.com/hsdfat/telco/stats"
)

func TestTransformer_Transform(t *testing.T) {
	transformer := NewTransformer("test-host", "EIR")

	now := time.Now()
	stats := &statsmodel.ServiceStats{
		Requests: statsmodel.RequestStats{
			Total:   100,
			Success: 90,
			Failed:  10,
		},
		Timestamp: now,
		Performance: statsmodel.PerformanceStats{
			RequestsPerSecond: 50.5,
		},
		CustomMetrics: map[string]interface{}{
			"eir": &statsmodel.EIRStats{
				EquipmentChecks: statsmodel.EquipmentCheckStats{
					Total:   100,
					Success: 90,
					Failed:  10,
					ByInterface: map[string]statsmodel.InterfaceCheckStats{
						"diameter": {
							Total:   50,
							Success: 45,
							Failed:  5,
							ByResultCode: map[int]uint64{
				2001: 45,
								5012: 5,
							},
						},
						"http": {
							Total:   50,
							Success: 45,
							Failed:  5,
							ByResultCode: map[int]uint64{
								200: 45,
								500: 5,
							},
						},
					},
				},
				CacheStats: statsmodel.CacheStats{
					Hits:    80,
					Misses:  20,
					HitRate: 80.0,
				},
				DatabaseOps: statsmodel.DatabaseOperationStats{
					Queries: 150,
					Inserts: 10,
					Updates: 5,
				},
				ByEquipmentStatus: map[string]uint64{
					"whitelisted": 80,
					"blacklisted": 15,
					"greylisted":  5,
				},
			},
		},
	}

	records := transformer.Transform(stats)

	// Verify we got records
	if len(records) == 0 {
		t.Fatal("Expected records, got none")
	}

	// Verify basic fields are set correctly
	for _, record := range records {
		if record.Hostname != "test-host" {
			t.Errorf("Expected hostname 'test-host', got '%s'", record.Hostname)
		}
		if record.SystemName != "EIR" {
			t.Errorf("Expected system_name 'EIR', got '%s'", record.SystemName)
		}
		if record.Timestamp != now {
			t.Errorf("Expected timestamp %v, got %v", now, record.Timestamp)
		}
	}

	// Verify specific counters exist
	counterMap := make(map[int][]MetricRecord)
	for _, record := range records {
		counterMap[record.CounterID] = append(counterMap[record.CounterID], record)
	}

	// Check total requests counter
	if recs, ok := counterMap[CounterTotalRequests]; !ok {
		t.Error("Missing CounterTotalRequests")
	} else if recs[0].Value != 100 {
		t.Errorf("Expected total requests = 100, got %d", recs[0].Value)
	}

	// Check successful requests counter
	if recs, ok := counterMap[CounterSuccessfulRequests]; !ok {
		t.Error("Missing CounterSuccessfulRequests")
	} else if recs[0].Value != 90 {
		t.Errorf("Expected successful requests = 90, got %d", recs[0].Value)
	}

	// Check diameter result codes
	diamResultCodeRecs := counterMap[CounterDiameterResultCode]
	if len(diamResultCodeRecs) != 2 {
		t.Errorf("Expected 2 diameter result code records, got %d", len(diamResultCodeRecs))
	}

	foundDiam2001 := false
	for _, rec := range diamResultCodeRecs {
		if rec.CauseCode == 2001 && rec.Value == 45 {
			foundDiam2001 = true
		}
	}
	if !foundDiam2001 {
		t.Error("Missing diameter result code 2001 record")
	}

	// Check HTTP status codes
	httpStatusCodeRecs := counterMap[CounterHTTPStatusCode]
	if len(httpStatusCodeRecs) != 2 {
		t.Errorf("Expected 2 HTTP status code records, got %d", len(httpStatusCodeRecs))
	}

	foundHTTP200 := false
	for _, rec := range httpStatusCodeRecs {
		if rec.CauseCode == 200 && rec.Value == 45 {
			foundHTTP200 = true
		}
	}
	if !foundHTTP200 {
		t.Error("Missing HTTP status code 200 record")
	}

	// Check cache stats
	if recs, ok := counterMap[CounterCacheHits]; !ok {
		t.Error("Missing CounterCacheHits")
	} else if recs[0].Value != 80 {
		t.Errorf("Expected cache hits = 80, got %d", recs[0].Value)
	}

	// Check equipment status
	if recs, ok := counterMap[CounterWhitelisted]; !ok {
		t.Error("Missing CounterWhitelisted")
	} else if recs[0].Value != 80 {
		t.Errorf("Expected whitelisted = 80, got %d", recs[0].Value)
	}
}

func TestTransformer_FilterRecords(t *testing.T) {
	// Test with include filter
	config := TransformerConfig{
		IncludeCounters: []int{CounterTotalRequests, CounterSuccessfulRequests},
	}
	transformer := NewTransformerWithConfig("test-host", "EIR", config)

	stats := &statsmodel.ServiceStats{
		Requests: statsmodel.RequestStats{
			Total:   100,
			Success: 90,
			Failed:  10,
		},
		Timestamp: time.Now(),
		CustomMetrics: map[string]interface{}{
			"eir": &statsmodel.EIRStats{
				CacheStats: statsmodel.CacheStats{
					Hits: 50,
				},
			},
		},
	}

	records := transformer.Transform(stats)

	// Should only have 2 records (total and successful)
	if len(records) != 2 {
		t.Errorf("Expected 2 filtered records, got %d", len(records))
	}

	for _, record := range records {
		if record.CounterID != CounterTotalRequests && record.CounterID != CounterSuccessfulRequests {
			t.Errorf("Unexpected counter ID %d in filtered results", record.CounterID)
		}
	}
}

func TestTransformer_ExcludeRecords(t *testing.T) {
	// Test with exclude filter
	config := TransformerConfig{
		ExcludeCounters: []int{CounterFailedRequests},
	}
	transformer := NewTransformerWithConfig("test-host", "EIR", config)

	stats := &statsmodel.ServiceStats{
		Requests: statsmodel.RequestStats{
			Total:   100,
			Success: 90,
			Failed:  10,
		},
		Timestamp: time.Now(),
		CustomMetrics: map[string]interface{}{
			"eir": &statsmodel.EIRStats{},
		},
	}

	records := transformer.Transform(stats)

	// Should not have failed requests counter
	for _, record := range records {
		if record.CounterID == CounterFailedRequests {
			t.Error("CounterFailedRequests should be excluded")
		}
	}
}

func TestGetCounterName(t *testing.T) {
	tests := []struct {
		counterID int
		expected  string
	}{
		{CounterTotalRequests, "total_requests"},
		{CounterDiameterResultCode, "diameter_result_code"},
		{CounterCacheHits, "cache_hits"},
		{9999, "unknown"},
	}

	for _, tt := range tests {
		result := GetCounterName(tt.counterID)
		if result != tt.expected {
			t.Errorf("GetCounterName(%d) = %s, want %s", tt.counterID, result, tt.expected)
		}
	}
}

// TestTransformer_ZeroValueFiltering tests that zero-value counters are not exported
func TestTransformer_ZeroValueFiltering(t *testing.T) {
	transformer := NewTransformer("test-host", "test-system")

	// Create stats with some zero values (typical after delta calculation where no activity occurred)
	stats := &statsmodel.ServiceStats{
		ServiceName:    "EIR",
		ServiceVersion: "1.0.0",
		Timestamp:      time.Now(),
		Requests: statsmodel.RequestStats{
			Total:   100, // Non-zero
			Success: 0,   // Zero - should not export
			Failed:  5,   // Non-zero
			Pending: 0,   // Zero - should not export
		},
		Connections: statsmodel.ConnectionStats{
			Active: 0,  // Zero but GAUGE - should ALWAYS export
			Total:  10, // Non-zero
			Failed: 0,  // Zero - should not export
		},
		Performance: statsmodel.PerformanceStats{
			RequestsPerSecond: 10.5, // Gauge - always export
			AvgLatencyMs:      0,    // Zero - no latency data, don't export latency metrics
		},
		CustomMetrics: map[string]interface{}{
			"eir": &statsmodel.EIRStats{
				EquipmentChecks: statsmodel.EquipmentCheckStats{
					Total:   100,
					Success: 95,
					Failed:  5,
					ByInterface: map[string]statsmodel.InterfaceCheckStats{
						"diameter": {
							Total:        60,
							Success:      58,
							Failed:       2,
							ByResultCode: map[int]uint64{2001: 58, 5012: 2},
						},
						"http": {
							Total:        40,
							Success:      37,
							Failed:       3,
							ByResultCode: map[int]uint64{200: 37, 404: 3},
						},
					},
				},
				CacheStats: statsmodel.CacheStats{
					Hits:    50,
					Misses:  0,    // Zero - should not export
					HitRate: 100.0, // Gauge - always export
					Size:    1000,  // Gauge - always export
				},
				DatabaseOps: statsmodel.DatabaseOperationStats{
					Queries: 25,
					Inserts: 0, // Zero - should not export
					Updates: 0, // Zero - should not export
					Deletes: 0, // Zero - should not export
				},
				ByEquipmentStatus: map[string]uint64{
					"whitelisted": 80,
					"blacklisted": 20,
					"greylisted":  0, // Zero - should not export
				},
			},
		},
	}

	records := transformer.Transform(stats)

	// Create a map of counter IDs for easy checking
	recordMap := make(map[int]MetricRecord)
	for _, record := range records {
		recordMap[record.CounterID] = record
	}

	// Verify zero-value counters are NOT exported
	t.Run("ZeroCountersNotExported", func(t *testing.T) {
		zeroCounters := []int{
			CounterSuccessfulRequests, // Success = 0
			CounterPendingRequests,    // Pending = 0
			CounterFailedConnections,  // Failed connections = 0
			CounterCacheMisses,        // Misses = 0
			CounterDBInserts,          // Inserts = 0
			CounterDBUpdates,          // Updates = 0
			CounterDBDeletes,          // Deletes = 0
			CounterGreylisted,         // Greylisted = 0
		}

		for _, counterID := range zeroCounters {
			if _, exists := recordMap[counterID]; exists {
				t.Errorf("Counter %d (%s) should not be exported when value is 0",
					counterID, GetCounterName(counterID))
			}
		}
	})

	// Verify non-zero counters ARE exported
	t.Run("NonZeroCountersExported", func(t *testing.T) {
		nonZeroCounters := map[int]uint64{
			CounterTotalRequests:     100,
			CounterFailedRequests:    5,
			CounterTotalConnections:  10,
			CounterCacheHits:         50,
			CounterDBQueries:         25,
			CounterWhitelisted:       80,
			CounterBlacklisted:       20,
		}

		for counterID, expectedValue := range nonZeroCounters {
			record, exists := recordMap[counterID]
			if !exists {
				t.Errorf("Counter %d (%s) should be exported when value is non-zero",
					counterID, GetCounterName(counterID))
				continue
			}
			if record.Value != expectedValue {
				t.Errorf("Counter %d: expected value %v, got %v",
					counterID, expectedValue, record.Value)
			}
		}
	})

	// Verify gauges are ALWAYS exported (even if zero)
	t.Run("GaugesAlwaysExported", func(t *testing.T) {
		// Note: Float values are multiplied by 100 for 2 decimal precision
		gaugeCounters := map[int]uint64{
			CounterActiveConnections:  0,     // Zero but should export (gauge!)
			CounterRequestsPerSecond:  1050,  // 10.5 * 100
			CounterCacheHitRate:       10000, // 100.0 * 100
			CounterCacheSize:          1000,  // Already integer
		}

		for counterID, expectedValue := range gaugeCounters {
			record, exists := recordMap[counterID]
			if !exists {
				t.Errorf("Gauge %d (%s) should always be exported (value=%v)",
					counterID, GetCounterName(counterID), expectedValue)
				continue
			}
			if record.Value != expectedValue {
				t.Errorf("Gauge %d: expected value %v, got %v",
					counterID, expectedValue, record.Value)
			}
		}
	})

	// Verify latency metrics are not exported when AvgLatencyMs is 0
	t.Run("LatencyNotExportedWhenZero", func(t *testing.T) {
		latencyCounters := []int{
			CounterAvgLatencyMs,
			CounterMinLatencyMs,
			CounterMaxLatencyMs,
			CounterP50LatencyMs,
			CounterP95LatencyMs,
			CounterP99LatencyMs,
		}

		for _, counterID := range latencyCounters {
			if _, exists := recordMap[counterID]; exists {
				t.Errorf("Latency counter %d (%s) should not be exported when AvgLatencyMs is 0",
					counterID, GetCounterName(counterID))
			}
		}
	})

	t.Logf("Total records exported: %d (filtered zero-value counters)", len(records))
}
