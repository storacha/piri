package dynamic

import (
	"github.com/storacha/piri/pkg/config"
)

// ChangeSource indicates where a configuration change originated.
type ChangeSource string

const (
	// SourceAPI indicates the change came from the admin API.
	SourceAPI ChangeSource = "api"
	// SourceFile indicates the change came from a config file update.
	SourceFile ChangeSource = "file"
)

// ChangeEvent represents a configuration change notification.
type ChangeEvent struct {
	// Key is the configuration key that changed.
	Key config.Key
	// OldValue is the previous value.
	OldValue any
	// NewValue is the new value.
	NewValue any
	// Source indicates where the change originated.
	Source ChangeSource
}

// Observer is notified when configuration values change.
type Observer interface {
	// OnConfigChange is called when a configuration value changes.
	// Implementations should be non-blocking and return quickly.
	OnConfigChange(event ChangeEvent)
}

// ObserverFunc is a function adapter for the Observer interface.
type ObserverFunc func(event ChangeEvent)

// OnConfigChange implements Observer.
func (f ObserverFunc) OnConfigChange(event ChangeEvent) {
	f(event)
}
