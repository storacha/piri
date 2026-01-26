package dynamic

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"

	"github.com/storacha/piri/pkg/config"
)

// ViperBridge bridges between Viper configuration and the dynamic Registry.
// It provides methods to explicitly reload configuration from the config file.
type ViperBridge struct {
	mu       sync.RWMutex
	v        *viper.Viper
	registry *Registry
}

// NewViperBridge creates a new ViperBridge.
func NewViperBridge(v *viper.Viper, registry *Registry) *ViperBridge {
	return &ViperBridge{
		v:        v,
		registry: registry,
	}
}

// Reload explicitly reloads config from the file.
// Call this in response to admin API request or CLI command.
// Returns an error if the config file cannot be read or if any value fails validation.
// On validation failure, no values are updated (fail-fast behavior).
func (vb *ViperBridge) Reload() error {
	vb.mu.Lock()
	defer vb.mu.Unlock()

	if vb.registry == nil {
		return fmt.Errorf("registry not set")
	}

	// Re-read the config file
	if err := vb.v.ReadInConfig(); err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	log.Info("Reloading config from file")

	// Build updates map from viper values for all registered keys
	updates := make(map[string]any)

	for _, key := range vb.registry.Keys() {
		keyStr := string(key)
		if vb.v.IsSet(keyStr) {
			// Get the raw value from viper - it will be parsed by the registry's schema
			updates[keyStr] = vb.v.Get(keyStr)
		}
	}

	if len(updates) == 0 {
		log.Info("No config values to reload")
		return nil
	}

	// Don't persist since we're loading from file
	// Registry will validate all values - if any fail, the entire reload fails
	if err := vb.registry.Update(updates, false, SourceFile); err != nil {
		return fmt.Errorf("applying config changes: %w", err)
	}

	log.Infow("Config reloaded successfully", "keys_updated", len(updates))
	return nil
}

// TOMLPersister implements surgical TOML file updates.
// Unlike Viper's WriteConfig which dumps all values, this persister
// reads the existing file, updates only the changed keys, and writes back
// preserving all other content.
type TOMLPersister struct {
	mu       sync.Mutex
	filePath string
}

// NewTOMLPersister creates a new TOMLPersister.
func NewTOMLPersister(filePath string) *TOMLPersister {
	return &TOMLPersister{filePath: filePath}
}

// Persist writes only the specified config changes to the config file.
func (p *TOMLPersister) Persist(updates map[config.Key]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Read existing file
	data, err := os.ReadFile(p.filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config file: %w", err)
	}

	// Parse TOML into a map to preserve structure
	var cfg map[string]any
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parsing config file: %w", err)
		}
	} else {
		cfg = make(map[string]any)
	}

	// Apply only the updates
	for key, value := range updates {
		setNestedValue(cfg, string(key), value)
	}

	// Write back
	out, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(p.filePath, out, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	log.Infow("Config persisted to file", "file", p.filePath, "updates", len(updates))
	return nil
}

// setNestedValue sets a value in a nested map using dot-notation key.
// e.g., "pdp.aggregation.poll_interval" -> config["pdp"]["aggregation"]["poll_interval"]
func setNestedValue(m map[string]any, key string, value any) {
	parts := strings.Split(key, ".")
	current := m

	// Navigate/create the nested structure
	for _, part := range parts[:len(parts)-1] {
		if _, ok := current[part]; !ok {
			current[part] = make(map[string]any)
		}
		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			// Path exists but isn't a map, need to overwrite
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		}
	}

	// Format duration values as strings for TOML
	switch v := value.(type) {
	case time.Duration:
		current[parts[len(parts)-1]] = v.String()
	default:
		current[parts[len(parts)-1]] = value
	}
}
