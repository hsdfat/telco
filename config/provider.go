package config

import (
	"context"
	"time"
)

// Provider defines the interface for configuration sources
// Implementations can load config from files, remote servers, etc.
type Provider interface {
	// Load retrieves the configuration data
	Load(ctx context.Context) (map[string]interface{}, error)

	// Name returns the provider name for logging
	Name() string

	// Close cleans up provider resources
	Close() error
}

// Watcher monitors configuration changes and triggers callbacks
type Watcher interface {
	// Watch starts monitoring for configuration changes
	// The callback is invoked when changes are detected
	Watch(ctx context.Context, callback func(map[string]interface{})) error

	// Stop halts the watcher
	Stop() error
}

// Validator validates configuration data
type Validator interface {
	// Validate checks if the configuration is valid
	// Returns nil if valid, error describing issues if invalid
	Validate(config interface{}) error
}

// Manager orchestrates multiple providers with priority
type Manager struct {
	providers []Provider
	validator Validator
	watcher   Watcher
	current   map[string]interface{}
}

// ManagerConfig configures the config manager
type ManagerConfig struct {
	// Providers in priority order (first = highest priority)
	Providers []Provider

	// Validator for config validation
	Validator Validator

	// Watcher for hot reload
	Watcher Watcher

	// EnableHotReload enables automatic config reloading
	EnableHotReload bool

	// ReloadCallback is called after successful config reload
	ReloadCallback func(map[string]interface{}) error
}

// NewManager creates a new configuration manager
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		providers: cfg.Providers,
		validator: cfg.Validator,
		watcher:   cfg.Watcher,
	}
}

// Load loads configuration from all providers with priority merging
// Higher priority providers (earlier in slice) override lower priority
func (m *Manager) Load(ctx context.Context) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Load from providers in reverse order (lower priority first)
	for i := len(m.providers) - 1; i >= 0; i-- {
		data, err := m.providers[i].Load(ctx)
		if err != nil {
			return nil, err
		}

		// Merge with deep merge strategy
		merge(result, data)
	}

	// Validate if validator is configured
	if m.validator != nil {
		if err := m.validator.Validate(result); err != nil {
			return nil, err
		}
	}

	m.current = result
	return result, nil
}

// Watch starts watching for configuration changes
func (m *Manager) Watch(ctx context.Context, callback func(map[string]interface{}) error) error {
	if m.watcher == nil {
		return nil // No watcher configured
	}

	return m.watcher.Watch(ctx, func(data map[string]interface{}) {
		// Validate before callback
		if m.validator != nil {
			if err := m.validator.Validate(data); err != nil {
				// Log validation error but don't crash
				return
			}
		}

		m.current = data
		if callback != nil {
			callback(data)
		}
	})
}

// Close closes all providers and watcher
func (m *Manager) Close() error {
	for _, p := range m.providers {
		if err := p.Close(); err != nil {
			return err
		}
	}

	if m.watcher != nil {
		return m.watcher.Stop()
	}

	return nil
}

// merge performs a deep merge of src into dst
func merge(dst, src map[string]interface{}) {
	for k, v := range src {
		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				merge(dstMap, srcMap)
				continue
			}
		}
		dst[k] = v
	}
}

// RetryConfig configures retry behavior for providers
type RetryConfig struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns sensible retry defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     5 * time.Second,
		Multiplier:  2.0,
	}
}
