package plugin

import (
	"context"
	"fmt"
)

// EventType represents the type of event dispatched by the event bus.
type EventType string

// Common event types used across AetherStream plugins.
const (
	EventLibraryScanComplete EventType = "library.scan.complete"
	EventMediaAdded          EventType = "media.added"
	EventMediaUpdated        EventType = "media.updated"
	EventMediaRemoved        EventType = "media.removed"
	EventPlaybackStart       EventType = "playback.start"
	EventPlaybackStop        EventType = "playback.stop"
	EventUserLogin           EventType = "user.login"
	EventUserLogout          EventType = "user.logout"
	EventSubtitleFound       EventType = "subtitle.found"
	EventMetadataFetched     EventType = "metadata.fetched"
	EventNotification        EventType = "notification"
)

// Event is a generic event structure passed through the event bus.
type Event struct {
	Type    EventType
	Payload interface{}
}

// Plugin is the interface that all AetherStream plugins must implement.
type Plugin interface {
	// Initialize is called once when the plugin is loaded.
	// The config map contains plugin-specific settings from the global config.
	Initialize(ctx context.Context, config map[string]interface{}) error

	// Name returns the unique name of the plugin.
	Name() string

	// Version returns the semantic version string of the plugin.
	Version() string

	// OnEvent is called whenever an event the plugin cares about is published.
	OnEvent(ctx context.Context, event Event) error
}

// BasePlugin provides a minimal embeddable implementation of Plugin.
// Concrete plugins can embed this and override methods as needed.
type BasePlugin struct {
	PluginName    string
	PluginVersion string
}

func (b *BasePlugin) Name() string    { return b.PluginName }
func (b *BasePlugin) Version() string { return b.PluginVersion }
func (b *BasePlugin) Initialize(ctx context.Context, config map[string]interface{}) error {
	return nil
}
func (b *BasePlugin) OnEvent(ctx context.Context, event Event) error {
	return nil
}

// ErrPluginNotFound is returned when a requested plugin does not exist in the registry.
var ErrPluginNotFound = fmt.Errorf("plugin not found")

// ErrPluginAlreadyRegistered is returned when attempting to register a plugin with a duplicate name.
var ErrPluginAlreadyRegistered = fmt.Errorf("plugin already registered")
