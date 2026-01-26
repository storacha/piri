package manager

import (
	"fmt"
	"time"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/config/dynamic"
)

// ConfigProvider provides dynamic configuration for the Manager.
type ConfigProvider interface {
	// PollInterval returns the current poll interval for the aggregation manager.
	PollInterval() time.Duration

	// BatchSize returns the current maximum batch size.
	BatchSize() uint

	// Subscribe registers an observer for config changes on the specified key.
	// Use config.Key constants (e.g., config.ManagerPollInterval).
	// Returns a function to unsubscribe.
	Subscribe(key config.Key, fn func(old, new any)) (func(), error)
}

// dynamicConfigProvider implements ConfigProvider using the dynamic registry.
type dynamicConfigProvider struct {
	registry *dynamic.Registry
	fallback app.AggregateManagerConfig
}

// NewConfigProvider creates a ConfigProvider backed by the dynamic registry.
// It registers the manager's config entries with the registry during creation.
func NewConfigProvider(
	registry *dynamic.Registry,
	cfg app.AggregateManagerConfig,
) (ConfigProvider, error) {

	// Register this package's config entries with their schemas
	if err := registry.RegisterEntries(map[config.Key]dynamic.ConfigEntry{
		config.ManagerPollInterval: {
			Value:  cfg.PollInterval,
			Schema: dynamic.DurationSchema{Min: time.Second, Max: 365 * 24 * time.Hour},
		},
		config.ManagerBatchSize: {
			Value:  cfg.BatchSize,
			Schema: dynamic.UintSchema{Min: 1, Max: 500},
		},
	}); err != nil {
		return nil, fmt.Errorf("registering config entries: %w", err)
	}

	return &dynamicConfigProvider{
		registry: registry,
		fallback: cfg,
	}, nil
}

func (p *dynamicConfigProvider) PollInterval() time.Duration {
	return p.registry.GetDuration(config.ManagerPollInterval, p.fallback.PollInterval)
}

func (p *dynamicConfigProvider) BatchSize() uint {
	return p.registry.GetUint(config.ManagerBatchSize, p.fallback.BatchSize)
}

func (p *dynamicConfigProvider) Subscribe(key config.Key, fn func(old, new any)) (func(), error) {
	return p.registry.SubscribeFunc(key, func(event dynamic.ChangeEvent) {
		fn(event.OldValue, event.NewValue)
	})
}
