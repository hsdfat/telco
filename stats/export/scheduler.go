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
	stats, ok := statsInterface.(*statsmodel.ServiceStats)
	if !ok {
		s.logger.Errorw("Failed to cast stats to ServiceStats",
			"type", fmt.Sprintf("%T", statsInterface))
		return
	}

	// Transform stats to metric records
	records := s.transformer.Transform(stats)
	if len(records) == 0 {
		s.logger.Debugw("No metrics to export")
		return
	}

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
