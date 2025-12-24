# Configuration Package Testing

This document describes the test coverage for the configuration package.

## Running Tests

```bash
cd /Users/hsdfat8/data/projects/telco/utils
go test ./config/... -v
```

## Test Coverage

### Provider Tests ([provider_test.go](provider_test.go))

**Mock Provider**
- MockProvider implementation for testing
- Used across all test suites

**Manager Tests**
- `TestManager_Load` - Configuration loading from multiple providers
  - Single provider loading
  - Multiple providers with merge priority
  - Nested configuration merge

**Merge Function Tests**
- `TestMerge` - Deep merge functionality
  - Simple merge
  - Override values
  - Nested merge
  - Deep nested merge

**Utilities**
- `TestDefaultRetryConfig` - Default retry configuration

### File Provider Tests ([file_provider_test.go](file_provider_test.go))

**File Loading**
- `TestFileProvider_Load_YAML` - YAML file parsing
- `TestFileProvider_Load_JSON` - JSON file parsing
- `TestFileProvider_AutoDetectFormat` - Format auto-detection (.yaml, .yml, .json)
- `TestFileProvider_SearchPaths` - File search in multiple directories
- `TestFileProvider_NotRequired` - Optional file handling

**File Watching**
- `TestFileWatcher_Watch` - File change detection and hot reload

**Utilities**
- `TestResolveFilePath` - Path resolution logic
- `TestFileProvider_Name` - Provider naming

### Environment Variable Provider Tests ([env_provider_test.go](env_provider_test.go))

**Environment Loading**
- `TestEnvProvider_Load` - Environment variable parsing
  - Hierarchical structure (PREFIX_SECTION_FIELD)
  - Type conversion (int, bool, string)
  - Nested configuration

**Value Parsing**
- `TestEnvProvider_ParseValue` - Automatic type detection
  - Boolean (true/false)
  - Integer (positive/negative)
  - Float
  - String (fallback)

**Binding**
- `TestBindEnv` - Struct binding from environment variables
  - Environment variable override
  - Default values
  - Partial override
- `TestBindEnv_NestedStruct` - Nested struct binding

**Field Setting**
- `TestSetFieldValue` - Type conversion for struct fields
  - String, int, bool, float
  - Error handling for invalid values

**Utilities**
- `TestEnvProvider_Name` - Provider naming

### Validator Tests ([validator_test.go](validator_test.go))

**Required Field Validation**
- `TestStructValidator_Required`
  - All required fields present
  - Missing required field detection
  - Empty string detection

**Min/Max Validation**
- `TestStructValidator_MinMax`
  - Numeric range validation (port: 1-65535)
  - String length validation (min/max characters)

**Enum Validation**
- `TestStructValidator_OneOf`
  - Valid enum values
  - Invalid enum detection

**Nested Validation**
- `TestStructValidator_Nested`
  - Nested struct validation
  - Missing nested required fields
  - Invalid nested values

**Custom Validators**
- `TestFuncValidator` - Custom validation functions
- `TestChainValidator` - Multiple validator chaining

**Utilities**
- `TestValidationErrors` - Error formatting
- `TestIsZeroValue` - Zero value detection

## Test Statistics

```
Provider Tests:       4 tests, 8 sub-tests
File Provider Tests:  8 tests, 7 sub-tests
Env Provider Tests:   7 tests, 13 sub-tests
Validator Tests:      8 tests, 17 sub-tests
---
Total:               27 tests, 45 sub-tests
```

## Coverage Areas

### ✅ Fully Tested
- File-based configuration (YAML/JSON)
- Environment variable parsing and binding
- Configuration merging with priority
- Validation framework (required, min, max, oneof)
- File watching for hot reload
- Type conversion and reflection-based binding

### ⚠️ Partially Tested
- Remote providers (Consul/etcd/confd)
  - Mock implementations exist
  - Integration tests with real Consul/etcd not included
  - Would require running actual Consul/etcd servers

### 📝 Future Test Coverage
- Integration tests with real Consul server
- Integration tests with real etcd server
- Confd backend testing
- TLS configuration testing
- Remote watcher real-time update tests
- Error recovery and retry logic
- Concurrent configuration updates

## Running Specific Tests

```bash
# Run only file provider tests
go test ./config -run TestFileProvider -v

# Run only validation tests
go test ./config -run TestValidator -v

# Run only environment tests
go test ./config -run TestEnvProvider -v

# Run with coverage
go test ./config -cover

# Run with coverage report
go test ./config -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Mock Objects

### MockProvider
```go
type MockProvider struct {
    name string
    data map[string]interface{}
    err  error
}
```

Used for:
- Testing Manager merge logic
- Testing priority ordering
- Simulating provider errors

## Test Patterns

### 1. Table-Driven Tests

All tests use table-driven patterns:

```go
tests := []struct {
    name    string
    input   InputType
    want    OutputType
    wantErr bool
}{
    {"case 1", input1, want1, false},
    {"case 2", input2, want2, true},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // Test implementation
    })
}
```

### 2. Temporary Directories

File tests use `t.TempDir()` for isolation:

```go
tmpDir := t.TempDir()
configFile := filepath.Join(tmpDir, "config.yaml")
```

### 3. Environment Cleanup

Environment tests clean up after themselves:

```go
os.Setenv("TEST_VAR", "value")
defer os.Unsetenv("TEST_VAR")
```

### 4. Subtest Organization

Complex tests use subtests for better organization:

```go
t.Run("valid config", func(t *testing.T) {
    // Test implementation
})

t.Run("invalid config", func(t *testing.T) {
    // Test implementation
})
```

## Test Examples

### Testing Configuration Loading

```go
provider := NewMockProvider("test", map[string]interface{}{
    "server": map[string]interface{}{
        "port": 8080,
    },
})

manager := NewManager(ManagerConfig{
    Providers: []Provider{provider},
})

data, err := manager.Load(context.Background())
// Assert on data and err
```

### Testing File Watching

```go
watcher, _ := NewFileWatcher([]string{configFile}, 50*time.Millisecond)
defer watcher.Stop()

callbackCalled := make(chan bool, 1)
go watcher.Watch(ctx, func(data map[string]interface{}) {
    callbackCalled <- true
})

// Modify file
os.WriteFile(configFile, []byte("new content"), 0644)

// Wait for callback
select {
case <-callbackCalled:
    // Success
case <-time.After(1 * time.Second):
    t.Error("callback not called")
}
```

### Testing Validation

```go
type Config struct {
    Port int `validate:"required,min=1,max=65535"`
}

validator := NewStructValidator(&Config{})

err := validator.Validate(map[string]interface{}{
    "port": 8080,
})
// Assert err is nil

err = validator.Validate(map[string]interface{}{
    "port": 70000,
})
// Assert err is not nil
```

## Continuous Integration

Add to CI pipeline:

```yaml
# .github/workflows/test.yml
- name: Run Config Tests
  run: |
    cd utils
    go test ./config/... -v -race -coverprofile=coverage.out
    go tool cover -func=coverage.out
```

## Best Practices

1. **Always use table-driven tests** for multiple test cases
2. **Clean up resources** (temp files, env vars) in defer
3. **Use subtests** for logical grouping
4. **Test error cases** as well as success cases
5. **Use meaningful test names** that describe what's being tested
6. **Avoid test interdependencies** - each test should be independent
7. **Use t.Helper()** in assertion helpers to show correct line numbers

## Contributing Tests

When adding new features:

1. Write tests first (TDD)
2. Ensure tests are table-driven
3. Test both success and error paths
4. Add test documentation
5. Update this file with new test categories
