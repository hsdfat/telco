package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestStructValidator_Required(t *testing.T) {
	type Config struct {
		Name string `validate:"required"`
		Port int    `validate:"required"`
	}

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "all required fields present",
			config: map[string]interface{}{
				"name": "test",
				"port": 8080,
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			config: map[string]interface{}{
				"name": "test",
			},
			wantErr: true,
			errMsg:  "Port",
		},
		{
			name: "empty string",
			config: map[string]interface{}{
				"name": "",
				"port": 8080,
			},
			wantErr: true,
			errMsg:  "Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			validator := NewStructValidator(cfg)

			err := validator.Validate(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message should contain %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestStructValidator_MinMax(t *testing.T) {
	type Config struct {
		Port     int    `validate:"min=1,max=65535"`
		Name     string `validate:"min=3,max=10"`
		MaxConns int    `validate:"min=1"`
	}

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "all within range",
			config: map[string]interface{}{
				"port":     8080,
				"name":     "test",
				"maxconns": 100,
			},
			wantErr: false,
		},
		{
			name: "port too small",
			config: map[string]interface{}{
				"port":     0,
				"name":     "test",
				"maxconns": 100,
			},
			wantErr: true,
			errMsg:  "Port",
		},
		{
			name: "port too large",
			config: map[string]interface{}{
				"port":     70000,
				"name":     "test",
				"maxconns": 100,
			},
			wantErr: true,
			errMsg:  "Port",
		},
		{
			name: "name too short",
			config: map[string]interface{}{
				"port":     8080,
				"name":     "ab",
				"maxconns": 100,
			},
			wantErr: true,
			errMsg:  "Name",
		},
		{
			name: "name too long",
			config: map[string]interface{}{
				"port":     8080,
				"name":     "verylongname",
				"maxconns": 100,
			},
			wantErr: true,
			errMsg:  "Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			validator := NewStructValidator(cfg)

			err := validator.Validate(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message should contain %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestStructValidator_OneOf(t *testing.T) {
	type Config struct {
		Environment string `validate:"oneof=dev staging prod"`
		LogLevel    string `validate:"oneof=debug info warn error"`
	}

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid values",
			config: map[string]interface{}{
				"environment": "prod",
				"loglevel":    "info",
			},
			wantErr: false,
		},
		{
			name: "invalid environment",
			config: map[string]interface{}{
				"environment": "local",
				"loglevel":    "info",
			},
			wantErr: true,
			errMsg:  "Environment",
		},
		{
			name: "invalid log level",
			config: map[string]interface{}{
				"environment": "dev",
				"loglevel":    "trace",
			},
			wantErr: true,
			errMsg:  "LogLevel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			validator := NewStructValidator(cfg)

			err := validator.Validate(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message should contain %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestStructValidator_Nested(t *testing.T) {
	type DatabaseConfig struct {
		Host string `validate:"required"`
		Port int    `validate:"min=1,max=65535"`
	}

	type Config struct {
		Database DatabaseConfig
	}

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid nested config",
			config: map[string]interface{}{
				"database": map[string]interface{}{
					"host": "localhost",
					"port": 5432,
				},
			},
			wantErr: false,
		},
		{
			name: "missing nested required field",
			config: map[string]interface{}{
				"database": map[string]interface{}{
					"port": 5432,
				},
			},
			wantErr: true,
			errMsg:  "Database.Host",
		},
		{
			name: "invalid nested value",
			config: map[string]interface{}{
				"database": map[string]interface{}{
					"host": "localhost",
					"port": 70000,
				},
			},
			wantErr: true,
			errMsg:  "Database.Port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			validator := NewStructValidator(cfg)

			err := validator.Validate(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message should contain %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestFuncValidator(t *testing.T) {
	validator := NewFuncValidator(func(config interface{}) error {
		data := config.(map[string]interface{})
		if data["invalid"] == true {
			return ValidationError{Field: "custom", Message: "custom validation failed"}
		}
		return nil
	})

	// Valid config
	err := validator.Validate(map[string]interface{}{"invalid": false})
	if err != nil {
		t.Errorf("Validate() should not error for valid config, got %v", err)
	}

	// Invalid config
	err = validator.Validate(map[string]interface{}{"invalid": true})
	if err == nil {
		t.Error("Validate() should error for invalid config")
	}
}

func TestChainValidator(t *testing.T) {
	type Config struct {
		Port int    `validate:"required,min=1,max=65535"`
		Name string `validate:"required"`
	}

	funcValidator := NewFuncValidator(func(config interface{}) error {
		// Custom validation
		return nil
	})

	// Test 1: Valid config
	t.Run("valid config", func(t *testing.T) {
		structValidator := NewStructValidator(&Config{})
		chain := NewChainValidator(structValidator, funcValidator)

		err := chain.Validate(map[string]interface{}{
			"port": 8080,
			"name": "test",
		})
		if err != nil {
			t.Errorf("Validate() should not error for valid config, got %v", err)
		}
	})

	// Test 2: Invalid config (missing required field)
	t.Run("invalid config", func(t *testing.T) {
		structValidator := NewStructValidator(&Config{})
		chain := NewChainValidator(structValidator, funcValidator)

		err := chain.Validate(map[string]interface{}{
			"port": 8080,
		})
		if err == nil {
			t.Error("Validate() should error for invalid config")
		}
	})
}

func TestValidationErrors(t *testing.T) {
	errors := ValidationErrors{
		ValidationError{Field: "field1", Message: "error 1"},
		ValidationError{Field: "field2", Message: "error 2"},
	}

	errMsg := errors.Error()
	if !strings.Contains(errMsg, "field1") {
		t.Error("error message should contain field1")
	}
	if !strings.Contains(errMsg, "field2") {
		t.Error("error message should contain field2")
	}
}

func TestIsZeroValue(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"zero int", 0, true},
		{"non-zero int", 42, false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"zero float", 0.0, true},
		{"non-zero float", 3.14, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := reflect.ValueOf(tt.value)
			if got := isZeroValue(v); got != tt.want {
				t.Errorf("isZeroValue(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
