package config

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// EnvProviderConfig configures environment variable provider
type EnvProviderConfig struct {
	// Prefix for environment variables (e.g., "EIR_" or "DIAMGW_")
	Prefix string

	// Separator for nested keys (default: "_")
	Separator string

	// AutomaticEnv enables automatic environment variable binding
	AutomaticEnv bool
}

// EnvProvider implements Provider for environment variables
// It overlays environment variables on top of base configuration
type EnvProvider struct {
	prefix    string
	separator string
	config    EnvProviderConfig
}

// NewEnvProvider creates an environment variable configuration provider
func NewEnvProvider(cfg EnvProviderConfig) *EnvProvider {
	if cfg.Separator == "" {
		cfg.Separator = "_"
	}

	return &EnvProvider{
		prefix:    cfg.Prefix,
		separator: cfg.Separator,
		config:    cfg,
	}
}

// Load reads environment variables and converts them to nested map
// Example: EIR_SERVER_PORT=8080 -> {"server": {"port": 8080}}
func (e *EnvProvider) Load(ctx context.Context) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Get all environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]

		// Skip if doesn't match prefix
		if e.prefix != "" && !strings.HasPrefix(key, e.prefix) {
			continue
		}

		// Remove prefix and convert to nested path
		if e.prefix != "" {
			key = strings.TrimPrefix(key, e.prefix)
		}

		// Convert KEY_NAME to nested structure
		path := strings.Split(strings.ToLower(key), strings.ToLower(e.separator))
		e.setNestedValue(result, path, value)
	}

	return result, nil
}

// setNestedValue sets a value in a nested map structure
func (e *EnvProvider) setNestedValue(m map[string]interface{}, path []string, value string) {
	if len(path) == 0 {
		return
	}

	key := path[0]

	if len(path) == 1 {
		// Leaf value - try to parse as appropriate type
		m[key] = e.parseValue(value)
		return
	}

	// Create nested map if it doesn't exist
	if _, ok := m[key]; !ok {
		m[key] = make(map[string]interface{})
	}

	// Ensure it's a map
	if nested, ok := m[key].(map[string]interface{}); ok {
		e.setNestedValue(nested, path[1:], value)
	}
}

// parseValue attempts to parse string value to appropriate type
func (e *EnvProvider) parseValue(value string) interface{} {
	// Try boolean
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}

	// Try integer
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}

	// Try float
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}

	// Return as string
	return value
}

// Name returns the provider name
func (e *EnvProvider) Name() string {
	return fmt.Sprintf("env(%s*)", e.prefix)
}

// Close cleans up resources
func (e *EnvProvider) Close() error {
	return nil
}

// BindEnv binds environment variables to a struct using tags
// Example: type Config struct { Port int `env:"PORT" envDefault:"8080"` }
func BindEnv(target interface{}, prefix string) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be pointer to struct")
	}

	return bindEnvStruct(v.Elem(), prefix)
}

// bindEnvStruct recursively binds environment variables to struct fields
func bindEnvStruct(v reflect.Value, prefix string) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			// Use field name if no tag
			envTag = strings.ToUpper(fieldType.Name)
		}

		// Build full environment variable name
		envName := envTag
		if prefix != "" {
			envName = prefix + "_" + envTag
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			if err := bindEnvStruct(field, envName); err != nil {
				return err
			}
			continue
		}

		// Get value from environment
		envValue := os.Getenv(envName)
		if envValue == "" {
			// Try default value
			if defaultVal := fieldType.Tag.Get("envDefault"); defaultVal != "" {
				envValue = defaultVal
			} else {
				continue
			}
		}

		// Set field value based on type
		if err := setFieldValue(field, envValue); err != nil {
			return fmt.Errorf("failed to set field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// setFieldValue sets a struct field value from string
func setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(i)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(u)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(f)

	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)

	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}

// UnmarshalEnv unmarshals a map into a struct, handling environment variable naming
func UnmarshalEnv(data map[string]interface{}, target interface{}) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be pointer to struct")
	}

	return unmarshalMap(data, v.Elem())
}

// unmarshalMap recursively unmarshals map into struct
func unmarshalMap(data map[string]interface{}, v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		// Get field name (lowercase to match env provider)
		fieldName := strings.ToLower(fieldType.Name)

		// Check if value exists in map
		value, ok := data[fieldName]
		if !ok {
			continue
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			if nestedMap, ok := value.(map[string]interface{}); ok {
				if err := unmarshalMap(nestedMap, field); err != nil {
					return err
				}
			}
			continue
		}

		// Set field value
		if err := setFieldValueFromInterface(field, value); err != nil {
			return fmt.Errorf("failed to set field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// setFieldValueFromInterface sets field from interface{} value
func setFieldValueFromInterface(field reflect.Value, value interface{}) error {
	v := reflect.ValueOf(value)

	// Handle type conversion
	if v.Type().ConvertibleTo(field.Type()) {
		field.Set(v.Convert(field.Type()))
		return nil
	}

	return fmt.Errorf("cannot convert %T to %s", value, field.Type())
}
