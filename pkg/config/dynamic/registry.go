package dynamic

import (
	"fmt"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/storacha/piri/pkg/config"
)

var log = logging.Logger("config/dynamic")

// Registry is the central store for all dynamic configuration values.
// It provides thread-safe access to configuration and notifies observers of changes.
type Registry struct {
	mu             sync.RWMutex
	config         map[config.Key]ConfigEntry
	observers      map[config.Key][]observerEntry
	nextObserverID uint64

	// persister handles writing config changes to disk
	persister Persister
}

// Persister handles persisting configuration changes.
type Persister interface {
	// Persist writes only the specified config changes to persistent storage.
	// This enables surgical updates that preserve existing config file content.
	Persist(updates map[config.Key]any) error
}

// ConfigEntry holds a configuration value and its schema for validation.
type ConfigEntry struct {
	Value  any
	Schema ConfigSchema
}

// RegistryOption configures a Registry.
type RegistryOption func(*Registry)

// WithPersister sets the persister for writing config changes to disk.
func WithPersister(p Persister) RegistryOption {
	return func(r *Registry) {
		r.persister = p
	}
}

// NewRegistry creates a new Registry with the given configuration entries.
// If cfgs is nil, an empty registry is created that can be populated via RegisterEntries.
func NewRegistry(cfgs map[config.Key]ConfigEntry, opts ...RegistryOption) *Registry {
	if cfgs == nil {
		cfgs = make(map[config.Key]ConfigEntry)
	}
	r := &Registry{
		config:    cfgs,
		observers: make(map[config.Key][]observerEntry),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Update accepts raw string keys and untyped values from the API.
// It handles all parsing and validation internally.
// If persist is true, the changes are written to persistent storage.
// Returns an error if validation fails for any value.
func (r *Registry) Update(updates map[string]any, persist bool, source ChangeSource) error {
	if len(updates) == 0 {
		return nil
	}

	r.mu.Lock()

	// Phase 1: Parse and validate ALL updates before applying ANY
	parsedUpdates := make(map[config.Key]any)

	for keyStr, rawValue := range updates {
		key := config.Key(keyStr)

		entry, exists := r.config[key]
		if !exists {
			r.mu.Unlock()
			return &UnknownKeyError{Key: keyStr}
		}

		// Schema handles both parsing (string → typed) and validation
		typedValue, err := entry.Schema.ParseAndValidate(rawValue)
		if err != nil {
			r.mu.Unlock()
			return &ValidationError{Key: key, Cause: err}
		}

		parsedUpdates[key] = typedValue
	}

	// Phase 2: Capture old values for rollback and notifications
	oldValues := make(map[config.Key]any)
	for key := range parsedUpdates {
		oldValues[key] = r.config[key].Value
	}

	// Phase 3: Apply updates
	for key, typedValue := range parsedUpdates {
		entry := r.config[key]
		entry.Value = typedValue
		r.config[key] = entry
	}

	// Copy observers while holding the lock
	observersCopy := make(map[config.Key][]observerEntry)
	for key := range parsedUpdates {
		if entries, ok := r.observers[key]; ok {
			observersCopy[key] = append([]observerEntry{}, entries...)
		}
	}

	r.mu.Unlock()

	// Phase 4: Persist if requested
	if persist && r.persister != nil {
		if err := r.persister.Persist(parsedUpdates); err != nil {
			// Rollback in-memory changes
			r.mu.Lock()
			for key, oldValue := range oldValues {
				entry := r.config[key]
				entry.Value = oldValue
				r.config[key] = entry
			}
			r.mu.Unlock()
			return &PersistError{Cause: err}
		}
	}

	// Phase 5: Notify observers outside of lock
	for key, typedValue := range parsedUpdates {
		event := ChangeEvent{
			Key:      key,
			OldValue: oldValues[key],
			NewValue: typedValue,
			Source:   source,
		}

		log.Infow("Config value changed",
			"key", event.Key,
			"old_value", event.OldValue,
			"new_value", event.NewValue,
			"source", event.Source,
		)

		for _, entry := range observersCopy[key] {
			entry.observer.OnConfigChange(event)
		}
	}

	return nil
}

// GetDuration returns the duration value for the given key.
// If the key doesn't exist or the value is not a duration, returns the fallback.
func (r *Registry) GetDuration(key config.Key, fallback time.Duration) time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if entry, ok := r.config[key]; ok {
		if d, ok := entry.Value.(time.Duration); ok {
			return d
		}
	}
	return fallback
}

// GetInt returns the int value for the given key.
// If the key doesn't exist or the value is not an int, returns the fallback.
func (r *Registry) GetInt(key config.Key, fallback int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if entry, ok := r.config[key]; ok {
		if i, ok := entry.Value.(int); ok {
			return i
		}
	}
	return fallback
}

// GetUint returns the uint value for the given key.
// If the key doesn't exist or the value is not a uint, returns the fallback.
func (r *Registry) GetUint(key config.Key, fallback uint) uint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if entry, ok := r.config[key]; ok {
		if u, ok := entry.Value.(uint); ok {
			return u
		}
	}
	return fallback
}

// GetAll returns all config values as a map (for API response).
// Duration values are formatted as strings.
func (r *Registry) GetAll() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]any)
	for key, entry := range r.config {
		// Format values for JSON response
		switch v := entry.Value.(type) {
		case time.Duration:
			result[string(key)] = v.String()
		default:
			result[string(key)] = v
		}
	}
	return result
}

// Keys returns all registered configuration keys.
func (r *Registry) Keys() []config.Key {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]config.Key, 0, len(r.config))
	for key := range r.config {
		keys = append(keys, key)
	}
	return keys
}

// RegisterEntries adds new configuration entries to the registry.
// This allows packages to register their own dynamic config keys.
// Returns an error if any key is already registered.
func (r *Registry) RegisterEntries(entries map[config.Key]ConfigEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for conflicts first
	for key := range entries {
		if _, exists := r.config[key]; exists {
			return fmt.Errorf("config key %s already registered", key)
		}
	}

	// Add all entries
	for key, entry := range entries {
		r.config[key] = entry
	}

	return nil
}

// observerEntry wraps an observer with an ID for unsubscription.
type observerEntry struct {
	id       uint64
	observer Observer
}

// Subscribe registers an observer for changes to the specified key.
// Returns a function to unsubscribe the observer.
func (r *Registry) Subscribe(key config.Key, observer Observer) (func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, has := r.config[key]; !has {
		return nil, fmt.Errorf("key %s does not exist", key)
	}

	// Generate a unique ID for this subscription
	id := r.nextObserverID
	r.nextObserverID++

	entry := observerEntry{id: id, observer: observer}
	r.observers[key] = append(r.observers[key], entry)

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		entries := r.observers[key]
		for i, e := range entries {
			if e.id == id {
				r.observers[key] = append(entries[:i], entries[i+1:]...)
				break
			}
		}
	}, nil
}

// SubscribeFunc is a convenience method that wraps a function as an Observer.
func (r *Registry) SubscribeFunc(key config.Key, fn func(ChangeEvent)) (func(), error) {
	return r.Subscribe(key, ObserverFunc(fn))
}
