package export

import (
	"context"
	"fmt"
	"sync"
	"time"

	statsmodel "github.com/hsdfat/telco/stats"
)

// ExportScheduler periodically collects stats and exports metrics
type ExportScheduler struct {
	interval       time.Duration
	exporters      []Exporter
	transformer    *Transformer
	statsCollector StatsCollectorInterface
	logger         Logger
	stopChan       chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex
	running        bool

	// Delta tracking: stores previous snapshot for calculating differences
	prevSnapshot   *statsmodel.ServiceStats
	snapshotMutex  sync.RWMutex
}

// NewExportScheduler creates a new export scheduler
func NewExportScheduler(
	interval time.Duration,
	statsCollector StatsCollectorInterface,
	transformer *Transformer,
	logger Logger,
) *ExportScheduler {
	return &ExportScheduler{
		interval:       interval,
		exporters:      make([]Exporter, 0),
		transformer:    transformer,
		statsCollector: statsCollector,
		logger:         logger,
		stopChan:       make(chan struct{}),
		running:        false,
	}
}

// AddExporter adds an exporter to the scheduler
func (s *ExportScheduler) AddExporter(exporter Exporter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exporters = append(s.exporters, exporter)
}

// Start begins the export scheduler
func (s *ExportScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run(ctx)
}

// Stop stops the export scheduler
func (s *ExportScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopChan)
	s.wg.Wait()

	// Close all exporters
	s.mu.RLock()
	exporters := s.exporters
	s.mu.RUnlock()

	for _, exporter := range exporters {
		if err := exporter.Close(); err != nil {
			s.logger.Errorw("Failed to close exporter",
				"exporter", exporter.Name(),
				"error", err)
		}
	}
}

// run is the main loop of the scheduler
func (s *ExportScheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Infow("Export scheduler started",
		"interval", s.interval.String(),
		"exporters", len(s.exporters))

	for {
		select {
		case <-ctx.Done():
			s.logger.Infow("Export scheduler stopped due to context cancellation")
			return
		case <-s.stopChan:
			s.logger.Infow("Export scheduler stopped")
			return
		case <-ticker.C:
			s.exportCycle(ctx)
		}
	}
}

// exportCycle performs a single export cycle
func (s *ExportScheduler) exportCycle(ctx context.Context) {
	startTime := time.Now()

	// Get current stats
	statsInterface := s.statsCollector.GetStats()
	currentStats, ok := statsInterface.(*statsmodel.ServiceStats)
	if !ok {
		s.logger.Errorw("Failed to cast stats to ServiceStats",
			"type", fmt.Sprintf("%T", statsInterface))
		return
	}

	// Calculate delta stats (difference since last export)
	deltaStats := s.calculateDeltaStats(currentStats)

	// Transform delta stats to metric records
	records := s.transformer.Transform(deltaStats)
	if len(records) == 0 {
		s.logger.Debugw("No metrics to export")
		return
	}

	// Store current stats as previous snapshot for next cycle
	s.updatePreviousSnapshot(currentStats)

	// Get exporters safely
	s.mu.RLock()
	exporters := make([]Exporter, len(s.exporters))
	copy(exporters, s.exporters)
	s.mu.RUnlock()

	// Export to all exporters in parallel
	var wg sync.WaitGroup
	for _, exporter := range exporters {
		wg.Add(1)
		go func(exp Exporter) {
			defer wg.Done()
			s.exportToExporter(ctx, exp, records)
		}(exporter)
	}

	wg.Wait()

	duration := time.Since(startTime)
	s.logger.Debugw("Export cycle completed",
		"records", len(records),
		"exporters", len(exporters),
		"duration_ms", duration.Milliseconds())
}

// exportToExporter exports records to a single exporter
func (s *ExportScheduler) exportToExporter(ctx context.Context, exporter Exporter, records []MetricRecord) {
	exportCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := exporter.Export(exportCtx, records); err != nil {
		s.logger.Errorw("Failed to export metrics",
			"exporter", exporter.Name(),
			"error", err)
		return
	}

	s.logger.Debugw("Successfully exported metrics",
		"exporter", exporter.Name(),
		"records", len(records))
}

// calculateDeltaStats calculates the difference between current and previous stats
func (s *ExportScheduler) calculateDeltaStats(current *statsmodel.ServiceStats) *statsmodel.ServiceStats {
	s.snapshotMutex.RLock()
	prev := s.prevSnapshot
	s.snapshotMutex.RUnlock()

	// If no previous snapshot exists, return current stats (first export)
	if prev == nil {
		s.logger.Debugw("No previous snapshot, exporting current stats")
		return current
	}

	// Create delta stats by subtracting previous from current
	delta := &statsmodel.ServiceStats{
		ServiceName:    current.ServiceName,
		ServiceVersion: current.ServiceVersion,
		Uptime:         current.Uptime,
		Timestamp:      current.Timestamp,
		Connections: statsmodel.ConnectionStats{
			Active: current.Connections.Active, // Use current value for gauges
			Total:  safeSub64(current.Connections.Total, prev.Connections.Total),
			Failed: safeSub64(current.Connections.Failed, prev.Connections.Failed),
		},
		Requests: statsmodel.RequestStats{
			Total:   safeSub64(current.Requests.Total, prev.Requests.Total),
			Success: safeSub64(current.Requests.Success, prev.Requests.Success),
			Failed:  safeSub64(current.Requests.Failed, prev.Requests.Failed),
			Pending: current.Requests.Pending, // Use current value for gauges
			BySource: make(map[string]statsmodel.SourceStats),
			ByOperation: make(map[string]statsmodel.OperationStats),
		},
		Performance: statsmodel.PerformanceStats{
			RequestsPerSecond: current.Performance.RequestsPerSecond, // Use current value
			AvgLatencyMs:      current.Performance.AvgLatencyMs,      // Use current value
			MinLatencyMs:      current.Performance.MinLatencyMs,      // Use current value
			MaxLatencyMs:      current.Performance.MaxLatencyMs,      // Use current value
			P50LatencyMs:      current.Performance.P50LatencyMs,      // Use current value
			P95LatencyMs:      current.Performance.P95LatencyMs,      // Use current value
			P99LatencyMs:      current.Performance.P99LatencyMs,      // Use current value
		},
		Errors: statsmodel.ErrorStats{
			Total:       safeSub64(current.Errors.Total, prev.Errors.Total),
			ByType:      calculateMapDelta64(current.Errors.ByType, prev.Errors.ByType),
			ByInterface: calculateMapDelta64(current.Errors.ByInterface, prev.Errors.ByInterface),
		},
		CustomMetrics: make(map[string]interface{}),
	}

	// Calculate delta for BySource
	for source, currStat := range current.Requests.BySource {
		prevStat := prev.Requests.BySource[source]
		delta.Requests.BySource[source] = statsmodel.SourceStats{
			Total:   safeSub64(currStat.Total, prevStat.Total),
			Success: safeSub64(currStat.Success, prevStat.Success),
			Failed:  safeSub64(currStat.Failed, prevStat.Failed),
		}
	}

	// Calculate delta for ByOperation
	for op, currStat := range current.Requests.ByOperation {
		prevStat := prev.Requests.ByOperation[op]
		delta.Requests.ByOperation[op] = statsmodel.OperationStats{
			Total:   safeSub64(currStat.Total, prevStat.Total),
			Success: safeSub64(currStat.Success, prevStat.Success),
			Failed:  safeSub64(currStat.Failed, prevStat.Failed),
		}
	}

	// Calculate delta for EIR-specific metrics
	if currEIR, ok := current.CustomMetrics["eir"].(*statsmodel.EIRStats); ok {
		var prevEIR *statsmodel.EIRStats
		if prev.CustomMetrics != nil {
			if p, ok := prev.CustomMetrics["eir"].(*statsmodel.EIRStats); ok {
				prevEIR = p
			}
		}

		delta.CustomMetrics["eir"] = s.calculateEIRDelta(currEIR, prevEIR)
	}

	return delta
}

// calculateEIRDelta calculates delta for EIR-specific stats
func (s *ExportScheduler) calculateEIRDelta(current *statsmodel.EIRStats, prev *statsmodel.EIRStats) *statsmodel.EIRStats {
	if prev == nil {
		return current
	}

	deltaEIR := &statsmodel.EIRStats{
		EquipmentChecks: statsmodel.EquipmentCheckStats{
			Total:       safeSub64(current.EquipmentChecks.Total, prev.EquipmentChecks.Total),
			Success:     safeSub64(current.EquipmentChecks.Success, prev.EquipmentChecks.Success),
			Failed:      safeSub64(current.EquipmentChecks.Failed, prev.EquipmentChecks.Failed),
			ByInterface: make(map[string]statsmodel.InterfaceCheckStats),
		},
		CacheStats: statsmodel.CacheStats{
			Hits:    safeSub64(current.CacheStats.Hits, prev.CacheStats.Hits),
			Misses:  safeSub64(current.CacheStats.Misses, prev.CacheStats.Misses),
			HitRate: current.CacheStats.HitRate, // Use current value
			Size:    current.CacheStats.Size,    // Use current value (gauge)
		},
		DatabaseOps: statsmodel.DatabaseOperationStats{
			Queries: safeSub64(current.DatabaseOps.Queries, prev.DatabaseOps.Queries),
			Inserts: safeSub64(current.DatabaseOps.Inserts, prev.DatabaseOps.Inserts),
			Updates: safeSub64(current.DatabaseOps.Updates, prev.DatabaseOps.Updates),
			Deletes: safeSub64(current.DatabaseOps.Deletes, prev.DatabaseOps.Deletes),
		},
		ByEquipmentStatus: calculateMapDelta64(current.ByEquipmentStatus, prev.ByEquipmentStatus),
	}

	// Calculate delta for interface-specific stats
	for ifName, currIf := range current.EquipmentChecks.ByInterface {
		prevIf := prev.EquipmentChecks.ByInterface[ifName]
		deltaEIR.EquipmentChecks.ByInterface[ifName] = statsmodel.InterfaceCheckStats{
			Total:        safeSub64(currIf.Total, prevIf.Total),
			Success:      safeSub64(currIf.Success, prevIf.Success),
			Failed:       safeSub64(currIf.Failed, prevIf.Failed),
			ByResultCode: calculateMapDeltaInt64(currIf.ByResultCode, prevIf.ByResultCode),
		}
	}

	return deltaEIR
}

// updatePreviousSnapshot stores current stats as previous snapshot
func (s *ExportScheduler) updatePreviousSnapshot(current *statsmodel.ServiceStats) {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()
	s.prevSnapshot = current
}

// safeSub64 safely subtracts two uint64 values (returns 0 if result would be negative)
func safeSub64(a, b uint64) uint64 {
	if a >= b {
		return a - b
	}
	return 0
}

// calculateMapDelta64 calculates delta for map[string]uint64
func calculateMapDelta64(current, prev map[string]uint64) map[string]uint64 {
	delta := make(map[string]uint64)
	for key, currVal := range current {
		prevVal := prev[key]
		if diff := safeSub64(currVal, prevVal); diff > 0 {
			delta[key] = diff
		}
	}
	return delta
}

// calculateMapDeltaInt64 calculates delta for map[int]uint64
func calculateMapDeltaInt64(current, prev map[int]uint64) map[int]uint64 {
	delta := make(map[int]uint64)
	for key, currVal := range current {
		prevVal := prev[key]
		if diff := safeSub64(currVal, prevVal); diff > 0 {
			delta[key] = diff
		}
	}
	return delta
}
