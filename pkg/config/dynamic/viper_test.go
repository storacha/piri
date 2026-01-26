package dynamic

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/config"
)

func TestNewTOMLPersister(t *testing.T) {
	p := NewTOMLPersister("/some/path/config.toml")
	require.NotNil(t, p)
	require.Equal(t, "/some/path/config.toml", p.filePath)
}

func TestTOMLPersister_Persist(t *testing.T) {
	t.Run("creates file if doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "config.toml")

		p := NewTOMLPersister(filePath)
		err := p.Persist(map[config.Key]any{
			"test.value": "hello",
		})
		require.NoError(t, err)

		// Verify file was created
		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var cfg map[string]any
		err = toml.Unmarshal(data, &cfg)
		require.NoError(t, err)

		testSection, ok := cfg["test"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "hello", testSection["value"])
	})

	t.Run("updates existing file preserving other content", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "config.toml")

		// Create initial file with some content
		initialContent := `
[server]
port = 8080
host = "localhost"

[database]
connection = "postgres://localhost"
`
		err := os.WriteFile(filePath, []byte(initialContent), 0644)
		require.NoError(t, err)

		p := NewTOMLPersister(filePath)
		err = p.Persist(map[config.Key]any{
			"server.timeout": "30s",
		})
		require.NoError(t, err)

		// Read and verify
		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var cfg map[string]any
		err = toml.Unmarshal(data, &cfg)
		require.NoError(t, err)

		// Original values should be preserved
		server, ok := cfg["server"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, int64(8080), server["port"])
		require.Equal(t, "localhost", server["host"])
		require.Equal(t, "30s", server["timeout"]) // New value

		// Other sections should be preserved
		database, ok := cfg["database"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "postgres://localhost", database["connection"])
	})

	t.Run("handles nested keys with dot notation", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "config.toml")

		p := NewTOMLPersister(filePath)
		err := p.Persist(map[config.Key]any{
			"pdp.aggregation.manager.poll_interval": "5m",
		})
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var cfg map[string]any
		err = toml.Unmarshal(data, &cfg)
		require.NoError(t, err)

		pdp, ok := cfg["pdp"].(map[string]any)
		require.True(t, ok)
		aggregation, ok := pdp["aggregation"].(map[string]any)
		require.True(t, ok)
		manager, ok := aggregation["manager"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "5m", manager["poll_interval"])
	})

	t.Run("formats duration as string", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "config.toml")

		p := NewTOMLPersister(filePath)
		err := p.Persist(map[config.Key]any{
			"test.duration": 5 * time.Minute,
		})
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var cfg map[string]any
		err = toml.Unmarshal(data, &cfg)
		require.NoError(t, err)

		testSection, ok := cfg["test"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "5m0s", testSection["duration"])
	})

	t.Run("handles multiple updates in single call", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "config.toml")

		p := NewTOMLPersister(filePath)
		err := p.Persist(map[config.Key]any{
			"server.port":    8080,
			"server.timeout": 30 * time.Second,
			"database.max":   100,
		})
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var cfg map[string]any
		err = toml.Unmarshal(data, &cfg)
		require.NoError(t, err)

		server, ok := cfg["server"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, int64(8080), server["port"])
		require.Equal(t, "30s", server["timeout"])

		database, ok := cfg["database"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, int64(100), database["max"])
	})

	t.Run("returns error for invalid file path", func(t *testing.T) {
		p := NewTOMLPersister("/nonexistent/directory/config.toml")
		err := p.Persist(map[config.Key]any{
			"test.value": "hello",
		})
		require.Error(t, err)
	})

	t.Run("returns error for malformed TOML file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "config.toml")

		// Write invalid TOML
		err := os.WriteFile(filePath, []byte("this is not valid toml {{{{"), 0644)
		require.NoError(t, err)

		p := NewTOMLPersister(filePath)
		err = p.Persist(map[config.Key]any{
			"test.value": "hello",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "parsing config file")
	})

	t.Run("handles concurrent calls safely", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "config.toml")

		p := NewTOMLPersister(filePath)

		var wg sync.WaitGroup
		iterations := 50

		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				key := config.Key("test.value" + string(rune('a'+i%26)))
				_ = p.Persist(map[config.Key]any{
					key: i,
				})
			}(i)
		}

		wg.Wait()

		// File should be readable and valid TOML
		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var cfg map[string]any
		err = toml.Unmarshal(data, &cfg)
		require.NoError(t, err)
		require.NotEmpty(t, cfg)
	})
}

func TestSetNestedValue(t *testing.T) {
	t.Run("creates nested structure for dotted key", func(t *testing.T) {
		m := make(map[string]any)
		setNestedValue(m, "a.b.c", "value")

		a, ok := m["a"].(map[string]any)
		require.True(t, ok)
		b, ok := a["b"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "value", b["c"])
	})

	t.Run("handles single-level key", func(t *testing.T) {
		m := make(map[string]any)
		setNestedValue(m, "key", "value")

		require.Equal(t, "value", m["key"])
	})

	t.Run("overwrites non-map intermediate value", func(t *testing.T) {
		m := map[string]any{
			"a": "string-value", // Not a map
		}
		setNestedValue(m, "a.b.c", "value")

		// "a" should now be a map
		a, ok := m["a"].(map[string]any)
		require.True(t, ok)
		b, ok := a["b"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "value", b["c"])
	})

	t.Run("preserves existing nested values", func(t *testing.T) {
		m := map[string]any{
			"a": map[string]any{
				"existing": "value",
			},
		}
		setNestedValue(m, "a.new", "new-value")

		a, ok := m["a"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "value", a["existing"])
		require.Equal(t, "new-value", a["new"])
	})

	t.Run("formats duration as string", func(t *testing.T) {
		m := make(map[string]any)
		setNestedValue(m, "config.timeout", 5*time.Minute)

		config, ok := m["config"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "5m0s", config["timeout"])
	})

	t.Run("handles deep nesting", func(t *testing.T) {
		m := make(map[string]any)
		setNestedValue(m, "level1.level2.level3.level4.level5", "deep-value")

		current := m
		for _, level := range []string{"level1", "level2", "level3", "level4"} {
			next, ok := current[level].(map[string]any)
			require.True(t, ok, "expected map at %s", level)
			current = next
		}
		require.Equal(t, "deep-value", current["level5"])
	})

	t.Run("handles various value types", func(t *testing.T) {
		m := make(map[string]any)

		setNestedValue(m, "types.string", "hello")
		setNestedValue(m, "types.int", 42)
		setNestedValue(m, "types.uint", uint(100))
		setNestedValue(m, "types.float", 3.14)
		setNestedValue(m, "types.bool", true)

		types, ok := m["types"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "hello", types["string"])
		require.Equal(t, 42, types["int"])
		require.Equal(t, uint(100), types["uint"])
		require.Equal(t, 3.14, types["float"])
		require.Equal(t, true, types["bool"])
	})
}
