// Package core provides the core runtime engine for uniteOS.
// It manages the application lifecycle, configuration, and event system.
package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	// AppName is the application identifier.
	AppName = "uniteos"
	// Version is the current application version.
	Version = "0.1.0"
	// ConfigFileName is the default configuration file name.
	ConfigFileName = "config.json"
	// DefaultDBName is the default database file name.
	DefaultDBName = "uniteos.db"
	// DefaultBlobDir is the default blob storage directory.
	DefaultBlobDir = "blobs"
	// DefaultSnapshotDir is the default snapshot directory.
	DefaultSnapshotDir = "snapshots"
	// DefaultLogFile is the default log file name.
	DefaultLogFile = "uniteos.log"
)

// Config holds all uniteOS configuration values.
type Config struct {
	// DataDir is the root directory for all uniteOS data.
	DataDir string `json:"data_dir"`
	// WorkspaceDir is the directory being tracked by uniteOS.
	WorkspaceDir string `json:"workspace_dir"`
	// DatabasePath is the path to the SQLite database.
	DatabasePath string `json:"database_path"`
	// BlobStorePath is the path to the blob storage directory.
	BlobStorePath string `json:"blob_store_path"`
	// SnapshotPath is the path to the snapshot directory.
	SnapshotPath string `json:"snapshot_path"`
	// LogPath is the path to the log file.
	LogPath string `json:"log_path"`
	// EncryptionEnabled controls whether file encryption is active.
	EncryptionEnabled bool `json:"encryption_enabled"`
	// CompressionEnabled controls whether blob compression is active.
	CompressionEnabled bool `json:"compression_enabled"`
	// ChunkSizeBytes is the size of each blob chunk (default 4MB).
	ChunkSizeBytes int64 `json:"chunk_size_bytes"`
	// APIPort is the port for the local REST API server.
	APIPort int `json:"api_port"`
	// DeviceID is a unique identifier for this device.
	DeviceID string `json:"device_id"`
	// DeviceName is a human-readable name for this device.
	DeviceName string `json:"device_name"`
	// MaxVersions is the maximum number of versions to keep per file.
	MaxVersions int `json:"max_versions"`

	mu sync.RWMutex `json:"-"`
}

// DefaultConfig creates a configuration with sensible defaults.
func DefaultConfig(workspaceDir string) *Config {
	dataDir := filepath.Join(workspaceDir, ".uniteos")

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	return &Config{
		DataDir:            dataDir,
		WorkspaceDir:       workspaceDir,
		DatabasePath:       filepath.Join(dataDir, DefaultDBName),
		BlobStorePath:      filepath.Join(dataDir, DefaultBlobDir),
		SnapshotPath:       filepath.Join(dataDir, DefaultSnapshotDir),
		LogPath:            filepath.Join(dataDir, DefaultLogFile),
		EncryptionEnabled:  false,
		CompressionEnabled: true,
		ChunkSizeBytes:     4 * 1024 * 1024, // 4MB
		APIPort:            7890,
		DeviceID:           "", // Generated on first run
		DeviceName:         fmt.Sprintf("%s-%s", hostname, runtime.GOOS),
		MaxVersions:        100,
	}
}

// Save writes the configuration to disk as JSON.
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	configPath := filepath.Join(c.DataDir, ConfigFileName)

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Load reads the configuration from disk.
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// Update safely modifies the configuration.
func (c *Config) Update(fn func(*Config)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c)
}

// GetDataDir returns the data directory path.
func (c *Config) GetDataDir() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.DataDir
}

// GetWorkspaceDir returns the workspace directory path.
func (c *Config) GetWorkspaceDir() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.WorkspaceDir
}

// Ensure all required directories exist.
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.DataDir,
		c.BlobStorePath,
		c.SnapshotPath,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
