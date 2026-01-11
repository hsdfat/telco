package export

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// LoadExportConfig loads export configuration from viper
func LoadExportConfig(v *viper.Viper) (*ExportConfig, error) {
	if !v.IsSet("stats_export") {
		return &ExportConfig{Enabled: false}, nil
	}

	config := &ExportConfig{}

	// Load enabled flag
	config.Enabled = v.GetBool("stats_export.enabled")
	if !config.Enabled {
		return config, nil
	}

	// Load interval
	intervalStr := v.GetString("stats_export.interval")
	if intervalStr == "" {
		intervalStr = "30s" // default
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid interval: %w", err)
	}
	config.Interval = interval

	// Load hostname (auto-detect if empty)
	config.Hostname = v.GetString("stats_export.hostname")
	if config.Hostname == "" {
		hostname, _ := os.Hostname()
		config.Hostname = hostname
	}

	// Load system name
	config.SystemName = v.GetString("stats_export.system_name")
	if config.SystemName == "" {
		config.SystemName = "EIR" // default
	}

	// Load exporters
	exportersConfig := v.Get("stats_export.exporters")
	if exportersConfig == nil {
		return config, nil
	}

	exportersList, ok := exportersConfig.([]interface{})
	if !ok {
		return nil, fmt.Errorf("exporters must be a list")
	}

	for i, exporterInterface := range exportersList {
		exporterMap, ok := exporterInterface.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("exporter %d is not a map", i)
		}

		exporterConfig, err := parseExporterConfig(exporterMap)
		if err != nil {
			return nil, fmt.Errorf("failed to parse exporter %d: %w", i, err)
		}

		config.Exporters = append(config.Exporters, exporterConfig)
	}

	return config, nil
}

// parseExporterConfig parses a single exporter configuration
func parseExporterConfig(m map[string]interface{}) (ExporterConfig, error) {
	config := ExporterConfig{}

	// Type (required)
	typeVal, ok := m["type"].(string)
	if !ok {
		return config, fmt.Errorf("exporter type is required")
	}
	config.Type = typeVal

	// Name (required)
	nameVal, ok := m["name"].(string)
	if !ok {
		return config, fmt.Errorf("exporter name is required")
	}
	config.Name = nameVal

	// Enabled (default true)
	if enabledVal, ok := m["enabled"].(bool); ok {
		config.Enabled = enabledVal
	} else {
		config.Enabled = true
	}

	// Config map
	if configVal, ok := m["config"].(map[string]interface{}); ok {
		// Expand environment variables in config values
		config.Config = expandEnvVars(configVal)
	} else {
		config.Config = make(map[string]interface{})
	}

	return config, nil
}

// expandEnvVars recursively expands environment variables in config values
func expandEnvVars(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case string:
			result[k] = os.ExpandEnv(val)
		case map[string]interface{}:
			result[k] = expandEnvVars(val)
		default:
			result[k] = v
		}
	}
	return result
}

// ParseExportConfigFromEnv parses export configuration from environment variables
// This is useful for containerized environments where YAML config is not available
func ParseExportConfigFromEnv() (*ExportConfig, error) {
	config := &ExportConfig{}

	// Check if export is enabled
	enabled := os.Getenv("STATS_EXPORT_ENABLED")
	if enabled == "" || strings.ToLower(enabled) == "false" {
		config.Enabled = false
		return config, nil
	}
	config.Enabled = true

	// Parse interval
	intervalStr := os.Getenv("STATS_EXPORT_INTERVAL")
	if intervalStr == "" {
		intervalStr = "30s"
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid STATS_EXPORT_INTERVAL: %w", err)
	}
	config.Interval = interval

	// Get hostname
	config.Hostname = os.Getenv("STATS_EXPORT_HOSTNAME")
	if config.Hostname == "" {
		hostname, _ := os.Hostname()
		config.Hostname = hostname
	}

	// Get system name
	config.SystemName = os.Getenv("STATS_EXPORT_SYSTEM_NAME")
	if config.SystemName == "" {
		config.SystemName = "EIR"
	}

	// Parse exporters from environment
	// Format: STATS_EXPORT_EXPORTERS=http:metrics-http,postgres:metrics-db,file:metrics-file
	exportersEnv := os.Getenv("STATS_EXPORT_EXPORTERS")
	if exportersEnv != "" {
		exporterPairs := strings.Split(exportersEnv, ",")
		for _, pair := range exporterPairs {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) != 2 {
				continue
			}

			exporterType := strings.TrimSpace(parts[0])
			exporterName := strings.TrimSpace(parts[1])

			exporterConfig := ExporterConfig{
				Type:    exporterType,
				Name:    exporterName,
				Enabled: true,
				Config:  make(map[string]interface{}),
			}

			// Load exporter-specific config from environment
			// e.g., STATS_EXPORT_HTTP_METRICS_HTTP_URL, STATS_EXPORT_POSTGRES_METRICS_DB_CONNECTION_STRING
			prefix := fmt.Sprintf("STATS_EXPORT_%s_%s_", strings.ToUpper(exporterType), strings.ToUpper(strings.ReplaceAll(exporterName, "-", "_")))
			loadExporterConfigFromEnv(&exporterConfig, prefix)

			config.Exporters = append(config.Exporters, exporterConfig)
		}
	}

	return config, nil
}

// loadExporterConfigFromEnv loads exporter-specific configuration from environment variables
func loadExporterConfigFromEnv(config *ExporterConfig, prefix string) {
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}

		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}

		key := strings.TrimPrefix(pair[0], prefix)
		key = strings.ToLower(key)
		value := pair[1]

		config.Config[key] = value
	}
}
