// Package plugins provides an extensible plugin system for UNITEos.
// Plugins can hook into the event system and extend functionality.
package plugins

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/ayushgpal/uniteos/internal/core"
)

// PluginState represents the lifecycle state of a plugin.
type PluginState string

const (
	StateLoaded    PluginState = "loaded"
	StateActive    PluginState = "active"
	StateStopped   PluginState = "stopped"
	StateError     PluginState = "error"
)

// PluginManifest describes a plugin.
type PluginManifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Events      []string `json:"events"`  // Events this plugin listens to
	Hooks       []string `json:"hooks"`   // Hook points
}

// PluginHook is the function signature for plugin hooks.
type PluginHook func(event core.Event) error

// Plugin represents a loaded plugin.
type Plugin struct {
	Manifest PluginManifest `json:"manifest"`
	State    PluginState    `json:"state"`
	Path     string         `json:"path"`
	hooks    map[string]PluginHook
	onStart  func() error
	onStop   func()
}

// PluginManager manages the plugin lifecycle.
type PluginManager struct {
	plugins  map[string]*Plugin
	mu       sync.RWMutex
	eventBus *core.EventBus
	logger   *slog.Logger
	pluginDir string
}

// NewPluginManager creates a new plugin manager.
func NewPluginManager(eventBus *core.EventBus, pluginDir string, logger *slog.Logger) *PluginManager {
	os.MkdirAll(pluginDir, 0700)
	return &PluginManager{
		plugins:   make(map[string]*Plugin),
		eventBus:  eventBus,
		logger:    logger,
		pluginDir: pluginDir,
	}
}

// RegisterPlugin registers a built-in plugin.
func (pm *PluginManager) RegisterPlugin(manifest PluginManifest, hooks map[string]PluginHook, onStart func() error, onStop func()) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.plugins[manifest.Name]; exists {
		return fmt.Errorf("plugin already registered: %s", manifest.Name)
	}

	plugin := &Plugin{
		Manifest: manifest,
		State:    StateLoaded,
		hooks:    hooks,
		onStart:  onStart,
		onStop:   onStop,
	}

	pm.plugins[manifest.Name] = plugin
	pm.logger.Info("plugin registered", "name", manifest.Name, "version", manifest.Version)
	return nil
}

// StartPlugin activates a registered plugin.
func (pm *PluginManager) StartPlugin(name string) error {
	pm.mu.Lock()
	plugin, ok := pm.plugins[name]
	pm.mu.Unlock()

	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if plugin.State == StateActive {
		return nil
	}

	// Run start callback
	if plugin.onStart != nil {
		if err := plugin.onStart(); err != nil {
			plugin.State = StateError
			return fmt.Errorf("plugin start failed: %w", err)
		}
	}

	// Subscribe to events
	for _, eventName := range plugin.Manifest.Events {
		eventType := core.EventType(eventName)
		hook := plugin.hooks[eventName]
		if hook != nil {
			pm.eventBus.Subscribe(eventType, func(event core.Event) {
				if err := hook(event); err != nil {
					pm.logger.Error("plugin hook error",
						"plugin", name,
						"event", eventName,
						"error", err,
					)
				}
			})
		}
	}

	plugin.State = StateActive
	pm.logger.Info("plugin started", "name", name)
	return nil
}

// StopPlugin deactivates a plugin.
func (pm *PluginManager) StopPlugin(name string) error {
	pm.mu.Lock()
	plugin, ok := pm.plugins[name]
	pm.mu.Unlock()

	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if plugin.onStop != nil {
		plugin.onStop()
	}

	plugin.State = StateStopped
	pm.logger.Info("plugin stopped", "name", name)
	return nil
}

// StartAll starts all registered plugins.
func (pm *PluginManager) StartAll() {
	pm.mu.RLock()
	names := make([]string, 0, len(pm.plugins))
	for name := range pm.plugins {
		names = append(names, name)
	}
	pm.mu.RUnlock()

	for _, name := range names {
		if err := pm.StartPlugin(name); err != nil {
			pm.logger.Error("failed to start plugin", "name", name, "error", err)
		}
	}
}

// StopAll stops all active plugins.
func (pm *PluginManager) StopAll() {
	pm.mu.RLock()
	names := make([]string, 0)
	for name, p := range pm.plugins {
		if p.State == StateActive {
			names = append(names, name)
		}
	}
	pm.mu.RUnlock()

	for _, name := range names {
		pm.StopPlugin(name)
	}
}

// ListPlugins returns all registered plugins.
func (pm *PluginManager) ListPlugins() []Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []Plugin
	for _, p := range pm.plugins {
		result = append(result, *p)
	}
	return result
}

// LoadPluginManifests scans the plugin directory for manifest files.
func (pm *PluginManager) LoadPluginManifests() ([]PluginManifest, error) {
	var manifests []PluginManifest

	entries, err := os.ReadDir(pm.pluginDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			manifestPath := filepath.Join(pm.pluginDir, entry.Name(), "manifest.json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				continue
			}
			var m PluginManifest
			if err := json.Unmarshal(data, &m); err != nil {
				continue
			}
			manifests = append(manifests, m)
		}
	}

	return manifests, nil
}

// ─── Built-in Plugins ──────────────────────────────────────────

// RegisterAutoVersionPlugin registers a plugin that auto-versions on file changes.
func RegisterAutoVersionPlugin(pm *PluginManager) {
	pm.RegisterPlugin(
		PluginManifest{
			Name:        "auto-version",
			Version:     "1.0.0",
			Description: "Automatically creates file versions when changes are detected",
			Author:      "UNITEos",
			Events:      []string{string(core.EventFileModified)},
		},
		map[string]PluginHook{
			string(core.EventFileModified): func(event core.Event) error {
				path, _ := event.Data["path"].(string)
				version, _ := event.Data["version"]
				pm.logger.Debug("auto-version: new version created",
					"path", path,
					"version", version,
				)
				return nil
			},
		},
		func() error {
			pm.logger.Info("auto-version plugin started")
			return nil
		},
		func() {
			pm.logger.Info("auto-version plugin stopped")
		},
	)
}

// RegisterAuditLogPlugin registers a plugin that logs all file events.
func RegisterAuditLogPlugin(pm *PluginManager) {
	pm.RegisterPlugin(
		PluginManifest{
			Name:        "audit-log",
			Version:     "1.0.0",
			Description: "Logs all file operations for compliance and auditing",
			Author:      "UNITEos",
			Events: []string{
				string(core.EventFileCreated),
				string(core.EventFileModified),
				string(core.EventFileDeleted),
				string(core.EventFileEncrypted),
				string(core.EventFileDecrypted),
			},
		},
		map[string]PluginHook{
			string(core.EventFileCreated): func(e core.Event) error {
				pm.logger.Info("[AUDIT] file created", "data", e.Data)
				return nil
			},
			string(core.EventFileModified): func(e core.Event) error {
				pm.logger.Info("[AUDIT] file modified", "data", e.Data)
				return nil
			},
			string(core.EventFileDeleted): func(e core.Event) error {
				pm.logger.Info("[AUDIT] file deleted", "data", e.Data)
				return nil
			},
			string(core.EventFileEncrypted): func(e core.Event) error {
				pm.logger.Info("[AUDIT] file encrypted", "data", e.Data)
				return nil
			},
			string(core.EventFileDecrypted): func(e core.Event) error {
				pm.logger.Info("[AUDIT] file decrypted", "data", e.Data)
				return nil
			},
		},
		nil, nil,
	)
}
