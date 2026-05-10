package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ExternalPluginConfig holds metadata for a subprocess-based plugin.
type ExternalPluginConfig struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	TimeoutSecs int               `json:"timeout_secs,omitempty"`
}

// ExternalPlugin wraps a subprocess plugin communicating via JSON-RPC over stdin/stdout.
type ExternalPlugin struct {
	cfg    ExternalPluginConfig
	cmd    *exec.Cmd
	client *rpc.Client
	mu     sync.Mutex
}

// NewExternalPlugin creates a wrapper for a subprocess plugin.
func NewExternalPlugin(cfg ExternalPluginConfig) *ExternalPlugin {
	return &ExternalPlugin{cfg: cfg}
}

// Initialize starts the subprocess and establishes the RPC client.
func (ep *ExternalPlugin) Initialize(ctx context.Context, config map[string]interface{}) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.client != nil {
		return fmt.Errorf("external plugin %s already initialized", ep.cfg.Name)
	}

	// #nosec G204 - plugin commands are validated during registration
	cmd := exec.CommandContext(ctx, ep.cfg.Command, ep.cfg.Args...)
	cmd.Env = os.Environ()
	for k, v := range ep.cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start external plugin: %w", err)
	}

	ep.cmd = cmd
	// Use an io.Pipe or net/rpc over a combined ReadWriteCloser. Here we use a simple pipe pair.
	// net/rpc.NewClient requires an io.ReadWriteCloser. We can use a custom wrapper.
	var stdoutFile, stdinFile *os.File
	if f, ok := stdout.(*os.File); ok {
		stdoutFile = f
	}
	if f, ok := stdin.(*os.File); ok {
		stdinFile = f
	}
	rwc := &readWriteCloser{Reader: stdoutFile, Writer: stdinFile, closer: func() error {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil
	}}
	ep.client = rpc.NewClient(rwc)

	// Handshake: call Plugin.Initialize with serialized config.
	var reply string
	configBytes, _ := json.Marshal(config)
	if err := ep.client.Call("Plugin.Initialize", string(configBytes), &reply); err != nil {
		_ = ep.shutdown()
		return fmt.Errorf("external plugin handshake failed: %w", err)
	}

	return nil
}

func (ep *ExternalPlugin) Name() string    { return ep.cfg.Name }
func (ep *ExternalPlugin) Version() string { return ep.cfg.Version }

// OnEvent forwards the event to the external process via RPC.
func (ep *ExternalPlugin) OnEvent(ctx context.Context, event Event) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.client == nil {
		return fmt.Errorf("external plugin %s not initialized", ep.cfg.Name)
	}

	payload, _ := json.Marshal(event)
	var reply string

	timeout := 30 * time.Second
	if ep.cfg.TimeoutSecs > 0 {
		timeout = time.Duration(ep.cfg.TimeoutSecs) * time.Second
	}

	call := ep.client.Go("Plugin.OnEvent", string(payload), &reply, nil)
	select {
	case <-call.Done:
		if call.Error != nil {
			return fmt.Errorf("external plugin OnEvent failed: %w", call.Error)
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("external plugin OnEvent timed out after %v", timeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown stops the external process.
func (ep *ExternalPlugin) Shutdown() error {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.shutdown()
}

func (ep *ExternalPlugin) shutdown() error {
	if ep.client != nil {
		_ = ep.client.Close()
		ep.client = nil
	}
	if ep.cmd != nil && ep.cmd.Process != nil {
		_ = ep.cmd.Process.Kill()
		_, _ = ep.cmd.Process.Wait()
		ep.cmd = nil
	}
	return nil
}

// readWriteCloser adapts separate Reader/Writer to io.ReadWriteCloser.
type readWriteCloser struct {
	Reader *os.File
	Writer *os.File
	closer func() error
}

func (r *readWriteCloser) Read(p []byte) (n int, err error)  { return r.Reader.Read(p) }
func (r *readWriteCloser) Write(p []byte) (n int, err error) { return r.Writer.Write(p) }
func (r *readWriteCloser) Close() error                       { return r.closer() }

// ExternalPluginManager loads and manages subprocess plugins from a directory.
type ExternalPluginManager struct {
	registry  *Registry
	mu        sync.Mutex
	externals map[string]*ExternalPlugin
}

// NewExternalPluginManager creates a manager bound to a registry.
func NewExternalPluginManager(registry *Registry) *ExternalPluginManager {
	return &ExternalPluginManager{
		registry:  registry,
		externals: make(map[string]*ExternalPlugin),
	}
}

// LoadDirectory scans a directory for .json plugin manifests and starts each external plugin.
func (em *ExternalPluginManager) LoadDirectory(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read external plugin directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[plugin] failed to read manifest %s: %v\n", path, err)
			continue
		}

		var cfg ExternalPluginConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "[plugin] invalid manifest %s: %v\n", path, err)
			continue
		}

		if cfg.Command == "" || cfg.Name == "" {
			fmt.Fprintf(os.Stderr, "[plugin] manifest %s missing command or name\n", path)
			continue
		}

		plug := NewExternalPlugin(cfg)
		if err := plug.Initialize(ctx, make(map[string]interface{})); err != nil {
			fmt.Fprintf(os.Stderr, "[plugin] failed to init external plugin %s: %v\n", cfg.Name, err)
			continue
		}

		if err := em.registry.Register(plug); err != nil {
			fmt.Fprintf(os.Stderr, "[plugin] failed to register external plugin %s: %v\n", cfg.Name, err)
			_ = plug.Shutdown()
			continue
		}

		em.mu.Lock()
		em.externals[cfg.Name] = plug
		em.mu.Unlock()
	}

	return nil
}

// ShutdownAll stops all managed external plugins.
func (em *ExternalPluginManager) ShutdownAll() {
	em.mu.Lock()
	defer em.mu.Unlock()

	for _, plug := range em.externals {
		_ = plug.Shutdown()
	}
	em.externals = make(map[string]*ExternalPlugin)
}
