package config

import (
	"context"
	"testing"
)

// MockProvider is a test provider implementation
type MockProvider struct {
	name string
	data map[string]interface{}
	err  error
}

func NewMockProvider(name string, data map[string]interface{}) *MockProvider {
	return &MockProvider{
		name: name,
		data: data,
	}
}

func (m *MockProvider) Load(ctx context.Context) (map[string]interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.data, nil
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) Close() error {
	return nil
}

func TestManager_Load(t *testing.T) {
	tests := []struct {
		name      string
		providers []Provider
		want      map[string]interface{}
		wantErr   bool
	}{
		{
			name: "single provider",
			providers: []Provider{
				NewMockProvider("test", map[string]interface{}{
					"key": "value",
				}),
			},
			want: map[string]interface{}{
				"key": "value",
			},
			wantErr: false,
		},
		{
			name: "multiple providers with merge",
			providers: []Provider{
				// Higher priority (loaded last)
				NewMockProvider("high", map[string]interface{}{
					"key1": "high",
					"key2": "high",
				}),
				// Lower priority (loaded first)
				NewMockProvider("low", map[string]interface{}{
					"key1": "low",
					"key3": "low",
				}),
			},
			want: map[string]interface{}{
				"key1": "high", // Overridden by high priority
				"key2": "high",
				"key3": "low", // Only in low priority
			},
			wantErr: false,
		},
		{
			name: "nested merge",
			providers: []Provider{
				NewMockProvider("high", map[string]interface{}{
					"server": map[string]interface{}{
						"port": 9090,
					},
				}),
				NewMockProvider("low", map[string]interface{}{
					"server": map[string]interface{}{
						"host": "localhost",
						"port": 8080,
					},
				}),
			},
			want: map[string]interface{}{
				"server": map[string]interface{}{
					"host": "localhost",
					"port": 9090, // Overridden
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(ManagerConfig{
				Providers: tt.providers,
			})

			got, err := m.Load(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Manager.Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				assertMapEqual(t, got, tt.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name string
		dst  map[string]interface{}
		src  map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "simple merge",
			dst: map[string]interface{}{
				"a": 1,
			},
			src: map[string]interface{}{
				"b": 2,
			},
			want: map[string]interface{}{
				"a": 1,
				"b": 2,
			},
		},
		{
			name: "override value",
			dst: map[string]interface{}{
				"a": 1,
			},
			src: map[string]interface{}{
				"a": 2,
			},
			want: map[string]interface{}{
				"a": 2,
			},
		},
		{
			name: "nested merge",
			dst: map[string]interface{}{
				"server": map[string]interface{}{
					"host": "localhost",
				},
			},
			src: map[string]interface{}{
				"server": map[string]interface{}{
					"port": 8080,
				},
			},
			want: map[string]interface{}{
				"server": map[string]interface{}{
					"host": "localhost",
					"port": 8080,
				},
			},
		},
		{
			name: "deep nested merge",
			dst: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": 1,
					},
				},
			},
			src: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"d": 2,
					},
				},
			},
			want: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": 1,
						"d": 2,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merge(tt.dst, tt.src)
			assertMapEqual(t, tt.dst, tt.want)
		})
	}
}

// Helper function to compare maps
func assertMapEqual(t *testing.T, got, want map[string]interface{}) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("map length mismatch: got %d, want %d", len(got), len(want))
		t.Errorf("got: %+v", got)
		t.Errorf("want: %+v", want)
		return
	}

	for k, wantV := range want {
		gotV, ok := got[k]
		if !ok {
			t.Errorf("missing key %q in result", k)
			continue
		}

		// Handle nested maps
		if wantMap, ok := wantV.(map[string]interface{}); ok {
			if gotMap, ok := gotV.(map[string]interface{}); ok {
				assertMapEqual(t, gotMap, wantMap)
				continue
			}
		}

		if gotV != wantV {
			t.Errorf("key %q: got %v, want %v", k, gotV, wantV)
		}
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}

	if cfg.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", cfg.Multiplier)
	}
}
