package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"
)

// Loader discovers and loads Go plugin shared objects (.so) from a directory.
// It also supports loading built-in plugins (pre-compiled Go structs) directly.
type Loader struct {
	registry *Registry
	bus      *EventBus
}

// NewLoader creates a Loader bound to a registry and event bus.
func NewLoader(registry *Registry, bus *EventBus) *Loader {
	return &Loader{registry: registry, bus: bus}
}

// LoadBuiltIn registers a pre-instantiated plugin directly.
func (l *Loader) LoadBuiltIn(p Plugin) error {
	if err := l.registry.Register(p); err != nil {
		return err
	}
	return nil
}

// LoadDirectory scans a directory for .so files and attempts to load each as a Go plugin.
// The plugin shared object must export a symbol named "Plugin" of type plugin.Plugin.
func (l *Loader) LoadDirectory(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no plugin directory is not an error
		}
		return fmt.Errorf("failed to read plugin directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".so") {
			continue
		}

		path := filepath.Join(dir, name)
		if err := l.loadSharedObject(ctx, path); err != nil {
			// Log and continue; one bad plugin should not stop others from loading.
			fmt.Fprintf(os.Stderr, "[plugin] failed to load %s: %v\n", path, err)
		}
	}
	return nil
}

func (l *Loader) loadSharedObject(ctx context.Context, path string) error {
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("plugin.Open failed: %w", err)
	}

	sym, err := p.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("missing 'Plugin' symbol: %w", err)
	}

	plug, ok := sym.(Plugin)
	if !ok {
		return fmt.Errorf("'Plugin' symbol does not implement plugin.Plugin interface")
	}

	if err := l.registry.Register(plug); err != nil {
		return err
	}

	// Initialize with empty config; in a real deployment configs could be read from a sidecar JSON/YAML.
	if err := plug.Initialize(ctx, make(map[string]interface{})); err != nil {
		// Unregister on failed init to keep registry clean.
		_ = l.registry.Unregister(plug.Name())
		return fmt.Errorf("plugin %s initialization failed: %w", plug.Name(), err)
	}

	return nil
}
