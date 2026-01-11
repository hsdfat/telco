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
		t.Errorf("Expected total requests = 100, got %.0f", recs[0].Value)
	}

	// Check successful requests counter
	if recs, ok := counterMap[CounterSuccessfulRequests]; !ok {
		t.Error("Missing CounterSuccessfulRequests")
	} else if recs[0].Value != 90 {
		t.Errorf("Expected successful requests = 90, got %.0f", recs[0].Value)
	}

	// Check diameter result codes
	diamResultCodeRecs := counterMap[CounterDiameterResultCode]
	if len(diamResultCodeRecs) != 2 {
		t.Errorf("Expected 2 diameter result code records, got %d", len(diamResultCodeRecs))
	}

	foundDiam2001 := false
	for _, rec := range diamResultCodeRecs {
		if rec.CauseCode == "diameter:2001" && rec.Value == 45 {
			foundDiam2001 = true
		}
	}
	if !foundDiam2001 {
		t.Error("Missing diameter:2001 result code record")
	}

	// Check HTTP status codes
	httpStatusCodeRecs := counterMap[CounterHTTPStatusCode]
	if len(httpStatusCodeRecs) != 2 {
		t.Errorf("Expected 2 HTTP status code records, got %d", len(httpStatusCodeRecs))
	}

	foundHTTP200 := false
	for _, rec := range httpStatusCodeRecs {
		if rec.CauseCode == "http:200" && rec.Value == 45 {
			foundHTTP200 = true
		}
	}
	if !foundHTTP200 {
		t.Error("Missing http:200 status code record")
	}

	// Check cache stats
	if recs, ok := counterMap[CounterCacheHits]; !ok {
		t.Error("Missing CounterCacheHits")
	} else if recs[0].Value != 80 {
		t.Errorf("Expected cache hits = 80, got %.0f", recs[0].Value)
	}

	// Check equipment status
	if recs, ok := counterMap[CounterWhitelisted]; !ok {
		t.Error("Missing CounterWhitelisted")
	} else if recs[0].Value != 80 {
		t.Errorf("Expected whitelisted = 80, got %.0f", recs[0].Value)
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
