# Telco Configuration Package

A flexible, switchable configuration system for telco services (EIR, diam-gw) with support for multiple configuration sources, hot reloading, and environment-based overrides.

## Features

- **Multiple Configuration Sources**: File (YAML/JSON), Environment Variables, Remote (Consul/etcd/confd)
- **Priority-Based Merging**: Higher priority sources override lower priority
- **Hot Reloading**: Automatic configuration updates without service restart
- **Validation**: Basic validation with struct tags
- **Environment Variable Overlay**: 12-factor app style configuration
- **Interface-Based Design**: Easy to extend with new providers
- **Confd Compatible**: Works with confd-managed configurations

## Architecture

### Core Interfaces

```go
type Provider interface {
    Load(ctx context.Context) (map[string]interface{}, error)
    Name() string
    Close() error
}

type Watcher interface {
    Watch(ctx context.Context, callback func(map[string]interface{})) error
    Stop() error
}

type Validator interface {
    Validate(config interface{}) error
}
```

### Configuration Priority (High to Low)

1. **Environment Variables** (highest priority)
2. **Remote Config Server** (Consul/etcd)
3. **Local Config File** (YAML/JSON, lowest priority)

Environment variables always override file and remote config.

## Usage Examples

### Basic File-Based Configuration

```go
import (
    eirconfig "github.com/hsdfat/telco/go-eir/pkg/config"
)

func main() {
    loader, err := eirconfig.NewLoader(eirconfig.LoaderConfig{
        ConfigFile: "config.yaml",
        ConfigFileSearchPaths: []string{".", "./config", "/etc/eir"},
        EnvPrefix: "EIR_",
        EnableHotReload: false,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer loader.Close()

    cfg, err := loader.Load(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    // Use configuration
    fmt.Printf("Server port: %d\n", cfg.Server.Port)
}
```

### Remote Configuration (Consul)

```go
loader, err := eirconfig.NewLoader(eirconfig.LoaderConfig{
    EnvPrefix: "EIR_",
    RemoteConfig: &eirconfig.RemoteConfig{
        Provider:  "consul",
        Endpoints: []string{"localhost:8500"},
        Key:       "config/eir/production",
    },
    EnableHotReload: true,
    ReloadCallback: func(cfg *eirconfig.Config) error {
        log.Println("Configuration reloaded!")
        // Apply new configuration
        return nil
    },
})
```

### With Hot Reload Watching

```go
ctx := context.Background()

// Load initial config
cfg, err := loader.Load(ctx)
if err != nil {
    log.Fatal(err)
}

// Start watching for changes
go func() {
    err := loader.Watch(ctx, func(newCfg *eirconfig.Config) error {
        log.Println("Config updated!")
        log.Printf("New server port: %d\n", newCfg.Server.Port)
        // Gracefully apply new configuration
        return nil
    })
    if err != nil {
        log.Printf("Watch error: %v\n", err)
    }
}()

// Application continues running
```

### Environment Variable Overlay

Configuration can be overridden via environment variables:

```bash
# Override server port
export EIR_SERVER_PORT=9090

# Override database settings
export EIR_DATABASE_HOST=db.example.com
export EIR_DATABASE_PORT=5432
export EIR_DATABASE_USERNAME=eir_user
export EIR_DATABASE_PASSWORD=secret

# Override nested settings
export EIR_DIAMETER_ORIGIN_HOST=eir.example.com
export EIR_DIAMETER_ORIGIN_REALM=example.com
```

Environment variables use the pattern: `PREFIX_SECTION_FIELD`

### Diameter Gateway Configuration

```go
import (
    gwconfig "github.com/hsdfat/telco/diam-gw/pkg/config"
)

loader, err := gwconfig.NewLoader(gwconfig.LoaderConfig{
    ConfigFile: "gateway.yaml",
    EnvPrefix: "DIAMGW_",
    RemoteConfig: &gwconfig.RemoteConfig{
        Provider:  "consul",
        Endpoints: []string{"consul.example.com:8500"},
        Key:       "config/diam-gw/production",
    },
    EnableHotReload: true,
})

cfg, err := loader.Load(context.Background())

// Access DRA configurations
for _, dra := range cfg.DRAs {
    fmt.Printf("DRA: %s:%d (priority: %d)\n", dra.Host, dra.Port, dra.Priority)
}
```

## Configuration Validation

Configurations are validated using struct tags:

```go
type ServerConfig struct {
    Port int `validate:"required,min=1,max=65535"`
    Host string `validate:"required"`
}
```

Supported validation tags:
- `required` - Field must be set
- `min=X` - Minimum value/length
- `max=X` - Maximum value/length
- `oneof=A B C` - Value must be one of the options

## Configuration File Formats

### YAML (Recommended)

```yaml
# EIR Configuration
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30s

database:
  type: postgres
  host: localhost
  port: 5432
  database: eir
  username: eir_user
  password: ${DB_PASSWORD}  # Can reference env vars

diameter:
  enabled: true
  listen_addr: 0.0.0.0:3868
  origin_host: eir.example.com
  origin_realm: example.com

cache:
  provider: redis
  ttl: 5m
  redis_addr: localhost:6379
```

### JSON

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "database": {
    "type": "postgres",
    "host": "localhost"
  }
}
```

## Consul Configuration Storage

Store configuration in Consul KV store:

```bash
# Store EIR production config
consul kv put config/eir/production @config.json

# Store Diameter Gateway config
consul kv put config/diam-gw/production @gateway-config.json
```

The config package will automatically watch for changes and reload.

## Custom Providers

Extend with custom configuration providers:

```go
type MyCustomProvider struct {}

func (p *MyCustomProvider) Load(ctx context.Context) (map[string]interface{}, error) {
    // Load config from custom source
    return map[string]interface{}{
        "server": map[string]interface{}{
            "port": 8080,
        },
    }, nil
}

func (p *MyCustomProvider) Name() string {
    return "custom"
}

func (p *MyCustomProvider) Close() error {
    return nil
}
```

## Best Practices

1. **Use Environment Variables for Secrets**: Never commit passwords/keys to files
2. **Enable Hot Reload in Production**: Allows config changes without downtime
3. **Use Remote Config for Centralized Management**: Consul/etcd for multi-instance deployments
4. **Validate Early**: Use struct tags to catch config errors at startup
5. **Use Defaults**: Provide sensible defaults for all optional settings
6. **Layer Your Config**: Base config in files, environment-specific overrides via env vars

## Package Structure

```
pkg/config/
├── provider.go          # Core interfaces (Provider, Watcher, Validator, Manager)
├── remote_provider.go   # Consul and etcd providers
├── file_provider.go     # File-based provider (YAML/JSON)
├── env_provider.go      # Environment variable provider
├── validator.go         # Validation framework
├── go.mod              # Go module definition
└── README.md           # This file
```

## Dependencies

- `github.com/fsnotify/fsnotify` - File system watching
- `github.com/hashicorp/consul/api` - Consul client
- `gopkg.in/yaml.v3` - YAML parsing

## Testing

```go
// Use in-memory config for testing
testConfig := &Config{
    Server: ServerConfig{Port: 8080},
}

// Or use file provider with test config
loader, _ := NewLoader(LoaderConfig{
    ConfigFile: "testdata/config.test.yaml",
})
```

## Future Enhancements

- [ ] etcd provider implementation
- [ ] Config encryption at rest
- [ ] Config versioning and rollback
- [ ] Config diff and change tracking
- [ ] Schema validation with JSON Schema
- [ ] Config templates with variable substitution
- [ ] Multi-environment support (dev/staging/prod profiles)
