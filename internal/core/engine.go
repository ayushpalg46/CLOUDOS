package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Engine is the central orchestrator for the UNITEos runtime.
// It manages the lifecycle of all subsystems.
type Engine struct {
	Config    *Config
	EventBus  *EventBus
	Logger    *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	started   time.Time
	ollamaCmd *exec.Cmd
}

// NewEngine creates and initializes a new UNITEos engine.
func NewEngine(workspaceDir string) (*Engine, error) {
	// Resolve absolute path
	absPath, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve workspace path: %w", err)
	}

	// Check workspace exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("workspace directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace path is not a directory: %s", absPath)
	}

	// Load or create config
	config := DefaultConfig(absPath)
	configPath := filepath.Join(config.DataDir, ConfigFileName)

	if _, err := os.Stat(configPath); err == nil {
		// Config exists, load it
		loaded, err := LoadConfig(configPath)
		if err != nil {
			// Log warning but continue with defaults
			fmt.Fprintf(os.Stderr, "warning: failed to load config, using defaults: %v\n", err)
		} else {
			config = loaded
		}
	}

	// Generate device ID if needed
	if config.DeviceID == "" {
		id := make([]byte, 16)
		if _, err := rand.Read(id); err != nil {
			return nil, fmt.Errorf("failed to generate device ID: %w", err)
		}
		config.DeviceID = hex.EncodeToString(id)
	}

	// Ensure directories exist
	if err := config.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create data directories: %w", err)
	}

	// Setup logger
	var logger *slog.Logger
	logFile, err := os.OpenFile(config.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Fallback to stderr if log file is locked or inaccessible
		fmt.Fprintf(os.Stderr, "warning: could not open log file %s: %v. Falling back to console.\n", config.LogPath, err)
		logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		logger = slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	// Create event bus
	eventBus := NewEventBus(logger)

	engine := &Engine{
		Config:   config,
		EventBus: eventBus,
		Logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Save config
	if err := config.Save(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	logger.Info("UNITEos engine initialized",
		"workspace", absPath,
		"device_id", config.DeviceID,
		"device_name", config.DeviceName,
		"version", Version,
	)

	return engine, nil
}

// Start begins the engine's runtime operations.
func (e *Engine) Start() error {
	e.started = time.Now()
	e.Logger.Info("UNITEos engine starting", "version", Version)

	// User requested that UNITEos completely owns the AI process lifecycle
	// 1. Kill any existing Ollama background processes
	e.Logger.Info("Taking full control of AI processes...")
	exec.Command("taskkill", "/F", "/IM", "ollama.exe").Run()
	exec.Command("taskkill", "/F", "/IM", "ollama_llama_server.exe").Run()

	// 2. Start Ollama exclusively as a child of UNITEos
	e.ollamaCmd = exec.Command("ollama", "serve")
	// Hide the terminal window on Windows if possible
	if err := e.ollamaCmd.Start(); err != nil {
		e.Logger.Warn("Failed to start embedded Ollama server", "error", err)
	} else {
		e.Logger.Info("Successfully booted embedded AI server", "pid", e.ollamaCmd.Process.Pid)
	}

	e.EventBus.Publish(NewEvent(EventEngineStarted, "engine", map[string]interface{}{
		"version":   Version,
		"device_id": e.Config.DeviceID,
	}))

	return nil
}

// Stop gracefully shuts down the engine.
func (e *Engine) Stop() {
	e.Logger.Info("UNITEos engine stopping",
		"uptime", time.Since(e.started).String(),
	)

	// Kill the child AI process so it dies when UNITEos dies
	if e.ollamaCmd != nil && e.ollamaCmd.Process != nil {
		e.Logger.Info("Shutting down embedded AI server...")
		e.ollamaCmd.Process.Kill()
		exec.Command("taskkill", "/F", "/IM", "ollama.exe").Run()
		exec.Command("taskkill", "/F", "/IM", "ollama_llama_server.exe").Run()
	}

	e.cancel()

	e.EventBus.Publish(NewEvent(EventEngineStopped, "engine", map[string]interface{}{
		"uptime_seconds": time.Since(e.started).Seconds(),
	}))
}

// Context returns the engine's context.
func (e *Engine) Context() context.Context {
	return e.ctx
}

// Uptime returns how long the engine has been running.
func (e *Engine) Uptime() time.Duration {
	if e.started.IsZero() {
		return 0
	}
	return time.Since(e.started)
}

// IsInitialized checks if a UNITEos workspace has been initialized at the given path.
func IsInitialized(workspaceDir string) bool {
	dataDir := filepath.Join(workspaceDir, ".uniteos")
	configPath := filepath.Join(dataDir, ConfigFileName)
	_, err := os.Stat(configPath)
	return err == nil
}
