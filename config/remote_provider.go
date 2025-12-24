package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
)

// RemoteProviderType defines remote config backend types
type RemoteProviderType string

const (
	ProviderConsul RemoteProviderType = "consul"
	ProviderEtcd   RemoteProviderType = "etcd"
	ProviderConfd  RemoteProviderType = "confd"
)

// RemoteProviderConfig configures a remote configuration provider
type RemoteProviderConfig struct {
	// Type of remote provider (consul, etcd)
	Type RemoteProviderType

	// Endpoints for the remote service
	Endpoints []string

	// Key path in the remote store
	Key string

	// Timeout for operations
	Timeout time.Duration

	// RetryConfig for resilient operations
	RetryConfig RetryConfig

	// TLS configuration (optional)
	TLSConfig *TLSConfig
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

// ConsulProvider implements Provider for HashiCorp Consul
type ConsulProvider struct {
	client *api.Client
	key    string
	config RemoteProviderConfig
}

// NewConsulProvider creates a Consul-based configuration provider
func NewConsulProvider(cfg RemoteProviderConfig) (*ConsulProvider, error) {
	consulConfig := api.DefaultConfig()

	if len(cfg.Endpoints) > 0 {
		consulConfig.Address = cfg.Endpoints[0]
	}

	// Configure TLS if provided
	if cfg.TLSConfig != nil {
		consulConfig.TLSConfig = api.TLSConfig{
			CertFile: cfg.TLSConfig.CertFile,
			KeyFile:  cfg.TLSConfig.KeyFile,
			CAFile:   cfg.TLSConfig.CAFile,
		}
	}

	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	return &ConsulProvider{
		client: client,
		key:    cfg.Key,
		config: cfg,
	}, nil
}

// Load retrieves configuration from Consul
func (c *ConsulProvider) Load(ctx context.Context) (map[string]interface{}, error) {
	kv := c.client.KV()

	var lastErr error
	retries := 0
	wait := c.config.RetryConfig.InitialWait

	for retries <= c.config.RetryConfig.MaxRetries {
		pair, _, err := kv.Get(c.key, &api.QueryOptions{})
		if err != nil {
			lastErr = err
			retries++

			if retries > c.config.RetryConfig.MaxRetries {
				break
			}

			time.Sleep(wait)
			wait = time.Duration(float64(wait) * c.config.RetryConfig.Multiplier)
			if wait > c.config.RetryConfig.MaxWait {
				wait = c.config.RetryConfig.MaxWait
			}
			continue
		}

		if pair == nil {
			return nil, fmt.Errorf("key not found: %s", c.key)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(pair.Value, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		return result, nil
	}

	return nil, fmt.Errorf("failed to load config after %d retries: %w", retries, lastErr)
}

// Name returns the provider name
func (c *ConsulProvider) Name() string {
	return fmt.Sprintf("consul(%s)", c.key)
}

// Close closes the Consul client
func (c *ConsulProvider) Close() error {
	// Consul client doesn't need explicit cleanup
	return nil
}

// ConsulWatcher watches Consul for configuration changes
type ConsulWatcher struct {
	client   *api.Client
	key      string
	stopCh   chan struct{}
	interval time.Duration
}

// NewConsulWatcher creates a watcher for Consul configuration changes
func NewConsulWatcher(client *api.Client, key string, interval time.Duration) *ConsulWatcher {
	if interval == 0 {
		interval = 10 * time.Second // Default polling interval
	}

	return &ConsulWatcher{
		client:   client,
		key:      key,
		stopCh:   make(chan struct{}),
		interval: interval,
	}
}

// Watch monitors Consul for configuration changes using blocking queries
func (w *ConsulWatcher) Watch(ctx context.Context, callback func(map[string]interface{})) error {
	kv := w.client.KV()

	// Get initial index
	_, meta, err := kv.Get(w.key, &api.QueryOptions{})
	if err != nil {
		return fmt.Errorf("failed to get initial config: %w", err)
	}

	lastIndex := meta.LastIndex

	go func() {
		for {
			select {
			case <-w.stopCh:
				return
			case <-ctx.Done():
				return
			default:
				// Use blocking query with wait index
				pair, meta, err := kv.Get(w.key, &api.QueryOptions{
					WaitIndex: lastIndex,
					WaitTime:  w.interval,
				})

				if err != nil {
					// Log error and continue watching
					time.Sleep(w.interval)
					continue
				}

				// Check if index changed (config updated)
				if meta.LastIndex != lastIndex {
					lastIndex = meta.LastIndex

					if pair != nil {
						var config map[string]interface{}
						if err := json.Unmarshal(pair.Value, &config); err != nil {
							// Log unmarshal error and continue
							continue
						}

						callback(config)
					}
				}
			}
		}
	}()

	return nil
}

// Stop halts the watcher
func (w *ConsulWatcher) Stop() error {
	close(w.stopCh)
	return nil
}

// EtcdProvider implements Provider for etcd
// TODO: Implement etcd support when needed
type EtcdProvider struct {
	endpoints []string
	key       string
}

// NewEtcdProvider creates an etcd-based configuration provider
func NewEtcdProvider(cfg RemoteProviderConfig) (*EtcdProvider, error) {
	// TODO: Implement etcd client setup
	return &EtcdProvider{
		endpoints: cfg.Endpoints,
		key:       cfg.Key,
	}, nil
}

// Load retrieves configuration from etcd
func (e *EtcdProvider) Load(ctx context.Context) (map[string]interface{}, error) {
	// TODO: Implement etcd loading
	return nil, fmt.Errorf("etcd provider not implemented yet")
}

// Name returns the provider name
func (e *EtcdProvider) Name() string {
	return fmt.Sprintf("etcd(%s)", e.key)
}

// Close closes the etcd client
func (e *EtcdProvider) Close() error {
	// TODO: Implement etcd cleanup
	return nil
}

// ConfdProvider implements Provider for confd-compatible backends
// Confd supports multiple backends: etcd, consul, redis, dynamodb, etc.
type ConfdProvider struct {
	backend   string
	endpoints []string
	key       string
	config    RemoteProviderConfig
}

// NewConfdProvider creates a confd-compatible configuration provider
// Confd can use various backends (etcd, consul, redis, etc.)
func NewConfdProvider(cfg RemoteProviderConfig) (*ConfdProvider, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("confd provider requires at least one endpoint")
	}

	// Default to etcd backend for confd
	backend := "etcd"
	if len(cfg.Endpoints) > 0 && cfg.Endpoints[0] != "" {
		// You can specify backend in endpoint format: "etcd://host:port"
		// or "consul://host:port", etc.
		backend = "etcd" // TODO: Parse from endpoint URL scheme
	}

	return &ConfdProvider{
		backend:   backend,
		endpoints: cfg.Endpoints,
		key:       cfg.Key,
		config:    cfg,
	}, nil
}

// Load retrieves configuration from confd backend
func (c *ConfdProvider) Load(ctx context.Context) (map[string]interface{}, error) {
	// Confd typically uses etcd or consul as backend
	// For now, we'll delegate to consul if available
	// In production, you'd detect the backend type and use appropriate client

	// Simple implementation: try consul first
	consulProvider, err := NewConsulProvider(c.config)
	if err == nil {
		return consulProvider.Load(ctx)
	}

	// Fallback to etcd
	etcdProvider, err := NewEtcdProvider(c.config)
	if err != nil {
		return nil, fmt.Errorf("confd provider: failed to initialize backend: %w", err)
	}

	return etcdProvider.Load(ctx)
}

// Name returns the provider name
func (c *ConfdProvider) Name() string {
	return fmt.Sprintf("confd(%s:%s)", c.backend, c.key)
}

// Close closes the confd provider
func (c *ConfdProvider) Close() error {
	return nil
}

// ConfdWatcher watches confd backend for configuration changes
type ConfdWatcher struct {
	backend  string
	watcher  Watcher
	stopCh   chan struct{}
	interval time.Duration
}

// NewConfdWatcher creates a watcher for confd configuration changes
func NewConfdWatcher(backend string, watcher Watcher, interval time.Duration) *ConfdWatcher {
	if interval == 0 {
		interval = 10 * time.Second
	}

	return &ConfdWatcher{
		backend:  backend,
		watcher:  watcher,
		stopCh:   make(chan struct{}),
		interval: interval,
	}
}

// Watch monitors confd backend for configuration changes
func (cw *ConfdWatcher) Watch(ctx context.Context, callback func(map[string]interface{})) error {
	if cw.watcher != nil {
		return cw.watcher.Watch(ctx, callback)
	}

	// Fallback to polling if no watcher available
	go func() {
		ticker := time.NewTicker(cw.interval)
		defer ticker.Stop()

		for {
			select {
			case <-cw.stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Polling-based watching
				// In production, you'd reload and compare configs
			}
		}
	}()

	return nil
}

// Stop halts the watcher
func (cw *ConfdWatcher) Stop() error {
	close(cw.stopCh)
	if cw.watcher != nil {
		return cw.watcher.Stop()
	}
	return nil
}
