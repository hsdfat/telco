package config

import (
	"context"
	"os"
	"reflect"
	"testing"
)

func TestEnvProvider_Load(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_SERVER_HOST", "localhost")
	os.Setenv("TEST_SERVER_PORT", "8080")
	os.Setenv("TEST_DATABASE_TYPE", "postgres")
	os.Setenv("TEST_DATABASE_ENABLED", "true")
	os.Setenv("TEST_CACHE_TTL", "300")
	defer func() {
		os.Unsetenv("TEST_SERVER_HOST")
		os.Unsetenv("TEST_SERVER_PORT")
		os.Unsetenv("TEST_DATABASE_TYPE")
		os.Unsetenv("TEST_DATABASE_ENABLED")
		os.Unsetenv("TEST_CACHE_TTL")
	}()

	provider := NewEnvProvider(EnvProviderConfig{
		Prefix:    "TEST_",
		Separator: "_",
	})

	data, err := provider.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check nested structure
	server, ok := data["server"].(map[string]interface{})
	if !ok {
		t.Fatal("server section not found or wrong type")
	}

	if host := server["host"]; host != "localhost" {
		t.Errorf("server.host = %v, want localhost", host)
	}

	if port := server["port"]; port != int64(8080) {
		t.Errorf("server.port = %v (type %T), want 8080", port, port)
	}

	database, ok := data["database"].(map[string]interface{})
	if !ok {
		t.Fatal("database section not found")
	}

	if dbType := database["type"]; dbType != "postgres" {
		t.Errorf("database.type = %v, want postgres", dbType)
	}

	if enabled := database["enabled"]; enabled != true {
		t.Errorf("database.enabled = %v, want true", enabled)
	}

	cache, ok := data["cache"].(map[string]interface{})
	if !ok {
		t.Fatal("cache section not found")
	}

	if ttl := cache["ttl"]; ttl != int64(300) {
		t.Errorf("cache.ttl = %v, want 300", ttl)
	}
}

func TestEnvProvider_ParseValue(t *testing.T) {
	provider := NewEnvProvider(EnvProviderConfig{})

	tests := []struct {
		name  string
		input string
		want  interface{}
	}{
		{"boolean true", "true", true},
		{"boolean false", "false", false},
		{"integer", "42", int64(42)},
		{"negative integer", "-10", int64(-10)},
		{"float", "3.14", float64(3.14)},
		{"string", "hello", "hello"},
		{"string number", "not_a_number", "not_a_number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.parseValue(tt.input)
			if got != tt.want {
				t.Errorf("parseValue(%q) = %v (type %T), want %v (type %T)",
					tt.input, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestEnvProvider_NoPrefix(t *testing.T) {
	os.Setenv("MY_VAR", "value")
	defer os.Unsetenv("MY_VAR")

	provider := NewEnvProvider(EnvProviderConfig{
		Prefix: "",
	})

	data, err := provider.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should include MY_VAR
	if _, ok := data["my"]; !ok {
		t.Error("expected 'my' key in result")
	}
}

func TestBindEnv(t *testing.T) {
	type TestConfig struct {
		Port     int    `env:"PORT" envDefault:"8080"`
		Host     string `env:"HOST" envDefault:"localhost"`
		Enabled  bool   `env:"ENABLED" envDefault:"true"`
		MaxConns int    `env:"MAX_CONNS" envDefault:"100"`
	}

	tests := []struct {
		name    string
		setEnv  map[string]string
		prefix  string
		want    TestConfig
		wantErr bool
	}{
		{
			name: "with environment variables",
			setEnv: map[string]string{
				"APP_PORT":      "9090",
				"APP_HOST":      "0.0.0.0",
				"APP_ENABLED":   "false",
				"APP_MAX_CONNS": "200",
			},
			prefix: "APP",
			want: TestConfig{
				Port:     9090,
				Host:     "0.0.0.0",
				Enabled:  false,
				MaxConns: 200,
			},
			wantErr: false,
		},
		{
			name:   "with defaults",
			setEnv: map[string]string{},
			prefix: "APP",
			want: TestConfig{
				Port:     8080,
				Host:     "localhost",
				Enabled:  true,
				MaxConns: 100,
			},
			wantErr: false,
		},
		{
			name: "partial override",
			setEnv: map[string]string{
				"APP_PORT": "3000",
			},
			prefix: "APP",
			want: TestConfig{
				Port:     3000,
				Host:     "localhost",
				Enabled:  true,
				MaxConns: 100,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.setEnv {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tt.setEnv {
					os.Unsetenv(k)
				}
			}()

			var cfg TestConfig
			err := BindEnv(&cfg, tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("BindEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !reflect.DeepEqual(cfg, tt.want) {
				t.Errorf("BindEnv() got = %+v, want %+v", cfg, tt.want)
			}
		})
	}
}

func TestBindEnv_NestedStruct(t *testing.T) {
	type DatabaseConfig struct {
		Host string `env:"HOST" envDefault:"localhost"`
		Port int    `env:"PORT" envDefault:"5432"`
	}

	type Config struct {
		Database DatabaseConfig
	}

	os.Setenv("APP_DATABASE_HOST", "db.example.com")
	os.Setenv("APP_DATABASE_PORT", "3306")
	defer func() {
		os.Unsetenv("APP_DATABASE_HOST")
		os.Unsetenv("APP_DATABASE_PORT")
	}()

	var cfg Config
	err := BindEnv(&cfg, "APP")
	if err != nil {
		t.Fatalf("BindEnv() error = %v", err)
	}

	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host = %v, want db.example.com", cfg.Database.Host)
	}

	if cfg.Database.Port != 3306 {
		t.Errorf("Database.Port = %v, want 3306", cfg.Database.Port)
	}
}

func TestSetFieldValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		kind    reflect.Kind
		want    interface{}
		wantErr bool
	}{
		{"string", "hello", reflect.String, "hello", false},
		{"int", "42", reflect.Int, 42, false},
		{"bool true", "true", reflect.Bool, true, false},
		{"bool false", "false", reflect.Bool, false, false},
		{"float", "3.14", reflect.Float64, 3.14, false},
		{"invalid int", "not_a_number", reflect.Int, nil, true},
		{"invalid bool", "maybe", reflect.Bool, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target reflect.Value

			switch tt.kind {
			case reflect.String:
				var s string
				target = reflect.ValueOf(&s).Elem()
			case reflect.Int:
				var i int
				target = reflect.ValueOf(&i).Elem()
			case reflect.Bool:
				var b bool
				target = reflect.ValueOf(&b).Elem()
			case reflect.Float64:
				var f float64
				target = reflect.ValueOf(&f).Elem()
			}

			err := setFieldValue(target, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("setFieldValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				got := target.Interface()
				if got != tt.want {
					t.Errorf("setFieldValue() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestEnvProvider_Name(t *testing.T) {
	provider := NewEnvProvider(EnvProviderConfig{
		Prefix: "TEST_",
	})

	name := provider.Name()
	if name != "env(TEST_*)" {
		t.Errorf("Name() = %v, want env(TEST_*)", name)
	}
}
