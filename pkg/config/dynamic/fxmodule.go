package dynamic

import (
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Module provides the dynamic configuration registry and viper bridge.
// Packages register their own config entries via Registry.RegisterEntries.
var Module = fx.Module("config/dynamic",
	fx.Provide(
		ProvideRegistry,
		ProvideViperBridge,
	),
)

// ProvideRegistry creates an empty Registry.
// Packages register their own entries via RegisterEntries during fx startup.
func ProvideRegistry() *Registry {
	v := viper.GetViper()

	// Create persister using surgical TOML updates (only if config file is used)
	var persister Persister
	if configFile := v.ConfigFileUsed(); configFile != "" {
		persister = NewTOMLPersister(configFile)
	}

	return NewRegistry(nil, WithPersister(persister))
}

// ProvideViperBridge creates a ViperBridge for config file reloads.
func ProvideViperBridge(registry *Registry) *ViperBridge {
	return NewViperBridge(viper.GetViper(), registry)
}
