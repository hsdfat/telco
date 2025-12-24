package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileProvider_Load_YAML(t *testing.T) {
	// Create temporary YAML file
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
server:
  host: localhost
  port: 8080
database:
  type: postgres
  host: db.example.com
`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	provider, err := NewFileProvider(FileProviderConfig{
		Path:   yamlFile,
		Format: FormatYAML,
	})
	if err != nil {
		t.Fatalf("NewFileProvider() error = %v", err)
	}

	data, err := provider.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify server section
	server, ok := data["server"].(map[string]interface{})
	if !ok {
		t.Fatal("server section not found or wrong type")
	}

	if host := server["host"]; host != "localhost" {
		t.Errorf("server.host = %v, want localhost", host)
	}

	if port := server["port"]; port != 8080 {
		t.Errorf("server.port = %v, want 8080", port)
	}

	// Verify database section
	database, ok := data["database"].(map[string]interface{})
	if !ok {
		t.Fatal("database section not found or wrong type")
	}

	if dbType := database["type"]; dbType != "postgres" {
		t.Errorf("database.type = %v, want postgres", dbType)
	}
}

func TestFileProvider_Load_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "config.json")

	jsonContent := `{
  "server": {
    "host": "0.0.0.0",
    "port": 9090
  },
  "enabled": true
}`

	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	provider, err := NewFileProvider(FileProviderConfig{
		Path:   jsonFile,
		Format: FormatJSON,
	})
	if err != nil {
		t.Fatalf("NewFileProvider() error = %v", err)
	}

	data, err := provider.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	server := data["server"].(map[string]interface{})
	if host := server["host"]; host != "0.0.0.0" {
		t.Errorf("server.host = %v, want 0.0.0.0", host)
	}

	if port := server["port"]; port != float64(9090) { // JSON numbers are float64
		t.Errorf("server.port = %v, want 9090", port)
	}

	if enabled := data["enabled"]; enabled != true {
		t.Errorf("enabled = %v, want true", enabled)
	}
}

func TestFileProvider_AutoDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     FileFormat
		wantErr  bool
	}{
		{
			name:     "yaml extension",
			filename: "config.yaml",
			want:     FormatYAML,
			wantErr:  false,
		},
		{
			name:     "yml extension",
			filename: "config.yml",
			want:     FormatYAML,
			wantErr:  false,
		},
		{
			name:     "json extension",
			filename: "config.json",
			want:     FormatJSON,
			wantErr:  false,
		},
		{
			name:     "unknown extension",
			filename: "config.txt",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, tt.filename)

			// Create empty file
			if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
				t.Fatal(err)
			}

			provider, err := NewFileProvider(FileProviderConfig{
				Path: path,
				// Format not specified - should auto-detect
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("NewFileProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && provider.format != tt.want {
				t.Errorf("format = %v, want %v", provider.format, tt.want)
			}
		})
	}
}

func TestFileProvider_SearchPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config file in subdirectory
	configDir := filepath.Join(tmpDir, "config")
	if err := os.Mkdir(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(configDir, "app.yaml")
	if err := os.WriteFile(configFile, []byte("key: value"), 0644); err != nil {
		t.Fatal(err)
	}

	// Use relative path with search paths
	provider, err := NewFileProvider(FileProviderConfig{
		Path:        "app.yaml",
		Format:      FormatYAML,
		SearchPaths: []string{configDir},
	})
	if err != nil {
		t.Fatalf("NewFileProvider() error = %v", err)
	}

	data, err := provider.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if data["key"] != "value" {
		t.Errorf("key = %v, want value", data["key"])
	}
}

func TestFileProvider_NotRequired(t *testing.T) {
	provider, err := NewFileProvider(FileProviderConfig{
		Path:     "/nonexistent/config.yaml",
		Format:   FormatYAML,
		Required: false,
	})
	if err != nil {
		t.Fatalf("NewFileProvider() should not error for non-required file, got %v", err)
	}

	// Load should return empty config
	data, err := provider.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(data) != 0 {
		t.Errorf("expected empty config, got %v", data)
	}
}

func TestFileProvider_Name(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.yaml")
	os.WriteFile(path, []byte("{}"), 0644)

	provider, _ := NewFileProvider(FileProviderConfig{
		Path:   path,
		Format: FormatYAML,
	})

	name := provider.Name()
	if name != "file("+path+")" {
		t.Errorf("Name() = %v, want file(%s)", name, path)
	}
}

func TestFileWatcher_Watch(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Write initial config
	initialConfig := "key: initial"
	if err := os.WriteFile(configFile, []byte(initialConfig), 0644); err != nil {
		t.Fatal(err)
	}

	watcher, err := NewFileWatcher([]string{configFile}, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("NewFileWatcher() error = %v", err)
	}
	defer watcher.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	callbackCalled := make(chan bool, 1)

	// Start watching
	go func() {
		watcher.Watch(ctx, func(data map[string]interface{}) {
			select {
			case callbackCalled <- true:
			default:
			}
		})
	}()

	// Wait a bit for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Modify file
	updatedConfig := "key: updated"
	if err := os.WriteFile(configFile, []byte(updatedConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for callback
	select {
	case <-callbackCalled:
		// Success - callback was called
	case <-time.After(1 * time.Second):
		t.Error("callback was not called after file modification")
	}
}

func TestResolveFilePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file in search path
	searchDir := filepath.Join(tmpDir, "search")
	os.Mkdir(searchDir, 0755)
	testFile := filepath.Join(searchDir, "config.yaml")
	os.WriteFile(testFile, []byte("test"), 0644)

	tests := []struct {
		name        string
		path        string
		searchPaths []string
		wantPath    string
		wantErr     bool
	}{
		{
			name:        "absolute path exists",
			path:        testFile,
			searchPaths: nil,
			wantPath:    testFile,
			wantErr:     false,
		},
		{
			name:        "relative path in search paths",
			path:        "config.yaml",
			searchPaths: []string{searchDir},
			wantPath:    testFile,
			wantErr:     false,
		},
		{
			name:        "file not found",
			path:        "nonexistent.yaml",
			searchPaths: []string{tmpDir},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveFilePath(tt.path, tt.searchPaths)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveFilePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantPath {
				t.Errorf("resolveFilePath() = %v, want %v", got, tt.wantPath)
			}
		})
	}
}
