package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Registry maintains a thread-safe collection of loaded plugins.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// NewRegistry creates a new empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

// Register adds a plugin to the registry. Returns ErrPluginAlreadyRegistered if the name is taken.
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return fmt.Errorf("cannot register nil plugin")
	}
	name := p.Name()
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("%w: %s", ErrPluginAlreadyRegistered, name)
	}

	r.plugins[name] = p
	return nil
}

// Unregister removes a plugin from the registry by name.
// Returns ErrPluginNotFound if the plugin does not exist.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; !exists {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, name)
	}

	delete(r.plugins, name)
	return nil
}

// Get retrieves a plugin by name. Returns ErrPluginNotFound if absent.
func (r *Registry) Get(name string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, name)
	}
	return p, nil
}

// List returns a snapshot of all registered plugins.
func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p)
	}
	return out
}

// InitializeAll calls Initialize on every registered plugin.
// Errors are collected and returned as a combined error; initialization is attempted for all plugins.
func (r *Registry) InitializeAll(ctx context.Context, configs map[string]map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []error
	for name, p := range r.plugins {
		cfg := configs[name]
		if cfg == nil {
			cfg = make(map[string]interface{})
		}
		if err := p.Initialize(ctx, cfg); err != nil {
			errs = append(errs, fmt.Errorf("plugin %s initialization failed: %w", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("plugin initialization errors: %v", errs)
	}
	return nil
}

// DispatchEvent sends an event to every registered plugin that implements OnEvent.
func (r *Registry) DispatchEvent(ctx context.Context, event Event) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []error
	for name, p := range r.plugins {
		if err := p.OnEvent(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("plugin %s event handler failed: %w", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("event dispatch errors: %v", errs)
	}
	return nil
}
