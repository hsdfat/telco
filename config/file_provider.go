package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// FileFormat defines supported file formats
type FileFormat string

const (
	FormatYAML FileFormat = "yaml"
	FormatJSON FileFormat = "json"
)

// FileProviderConfig configures file-based configuration provider
type FileProviderConfig struct {
	// Path to the configuration file
	Path string

	// Format of the file (yaml, json)
	Format FileFormat

	// SearchPaths to look for the config file if Path is relative
	SearchPaths []string

	// Required indicates if the file must exist
	Required bool
}

// FileProvider implements Provider for file-based configuration
type FileProvider struct {
	path   string
	format FileFormat
	config FileProviderConfig
}

// NewFileProvider creates a file-based configuration provider
func NewFileProvider(cfg FileProviderConfig) (*FileProvider, error) {
	// Auto-detect format from extension if not specified
	if cfg.Format == "" {
		ext := strings.ToLower(filepath.Ext(cfg.Path))
		switch ext {
		case ".yaml", ".yml":
			cfg.Format = FormatYAML
		case ".json":
			cfg.Format = FormatJSON
		default:
			return nil, fmt.Errorf("cannot detect format from extension: %s", ext)
		}
	}

	// Resolve file path using search paths
	resolvedPath, err := resolveFilePath(cfg.Path, cfg.SearchPaths)
	if err != nil && cfg.Required {
		return nil, err
	}

	return &FileProvider{
		path:   resolvedPath,
		format: cfg.Format,
		config: cfg,
	}, nil
}

// Load reads and parses the configuration file
func (f *FileProvider) Load(ctx context.Context) (map[string]interface{}, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if !f.config.Required && os.IsNotExist(err) {
			return make(map[string]interface{}), nil // Return empty config
		}
		return nil, fmt.Errorf("failed to read file %s: %w", f.path, err)
	}

	var result map[string]interface{}

	switch f.format {
	case FormatYAML:
		if err := yaml.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	case FormatJSON:
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", f.format)
	}

	return result, nil
}

// Name returns the provider name
func (f *FileProvider) Name() string {
	return fmt.Sprintf("file(%s)", f.path)
}

// Close cleans up resources
func (f *FileProvider) Close() error {
	return nil
}

// resolveFilePath finds the config file in search paths
func resolveFilePath(path string, searchPaths []string) (string, error) {
	// If absolute path, use it directly
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			return "", err
		}
		return path, nil
	}

	// Search in provided paths
	for _, searchPath := range searchPaths {
		fullPath := filepath.Join(searchPath, path)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}

	// Try current directory
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("config file not found: %s", path)
}

// FileWatcher watches file system for configuration changes
type FileWatcher struct {
	watcher  *fsnotify.Watcher
	paths    []string
	stopCh   chan struct{}
	debounce time.Duration
}

// NewFileWatcher creates a file system watcher
func NewFileWatcher(paths []string, debounce time.Duration) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	if debounce == 0 {
		debounce = 100 * time.Millisecond
	}

	return &FileWatcher{
		watcher:  watcher,
		paths:    paths,
		stopCh:   make(chan struct{}),
		debounce: debounce,
	}, nil
}

// Watch monitors files for changes
func (fw *FileWatcher) Watch(ctx context.Context, callback func(map[string]interface{})) error {
	// Add all paths to watcher
	for _, path := range fw.paths {
		if err := fw.watcher.Add(path); err != nil {
			return fmt.Errorf("failed to watch %s: %w", path, err)
		}
	}

	go func() {
		var debounceTimer *time.Timer

		for {
			select {
			case <-fw.stopCh:
				return
			case <-ctx.Done():
				return
			case event, ok := <-fw.watcher.Events:
				if !ok {
					return
				}

				// Only trigger on write or create events
				if event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Create == fsnotify.Create {

					// Debounce rapid file changes
					if debounceTimer != nil {
						debounceTimer.Stop()
					}

					debounceTimer = time.AfterFunc(fw.debounce, func() {
						// Reload config from file
						// This is simplified - in practice, you'd reload from the provider
						data, err := os.ReadFile(event.Name)
						if err != nil {
							return
						}

						var config map[string]interface{}
						// Detect format and unmarshal
						ext := strings.ToLower(filepath.Ext(event.Name))
						if ext == ".yaml" || ext == ".yml" {
							yaml.Unmarshal(data, &config)
						} else if ext == ".json" {
							json.Unmarshal(data, &config)
						}

						if config != nil {
							callback(config)
						}
					})
				}
			case err, ok := <-fw.watcher.Errors:
				if !ok {
					return
				}
				// Log error but continue watching
				_ = err
			}
		}
	}()

	return nil
}

// Stop halts the file watcher
func (fw *FileWatcher) Stop() error {
	close(fw.stopCh)
	return fw.watcher.Close()
}
