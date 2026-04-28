// Package watcher provides real-time file system monitoring for uniteOS.
// It watches tracked directories for changes and emits events through the event bus.
package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ayushgpal/uniteos/internal/core"
	"github.com/ayushgpal/uniteos/internal/storage"
	"github.com/fsnotify/fsnotify"
)

// Watcher monitors the file system for changes in the workspace.
type Watcher struct {
	fsWatcher    *fsnotify.Watcher
	store        *storage.Store
	eventBus     *core.EventBus
	logger       *slog.Logger
	workspaceDir string

	// Debounce duplicate events
	pending   map[string]time.Time
	pendingMu sync.Mutex
	debounce  time.Duration

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewWatcher creates a new file system watcher.
func NewWatcher(store *storage.Store, eventBus *core.EventBus, workspaceDir string, logger *slog.Logger) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		fsWatcher:    fsw,
		store:        store,
		eventBus:     eventBus,
		logger:       logger,
		workspaceDir: workspaceDir,
		pending:      make(map[string]time.Time),
		debounce:     500 * time.Millisecond,
		stopCh:       make(chan struct{}),
	}, nil
}

// Start begins watching the workspace directory recursively.
func (w *Watcher) Start() error {
	// Add all directories recursively
	err := filepath.Walk(w.workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".uniteos" || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return w.fsWatcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	w.wg.Add(2)
	go w.eventLoop()
	go w.debounceLoop()

	w.logger.Info("file watcher started", "workspace", w.workspaceDir)
	return nil
}

// Stop stops the file watcher.
func (w *Watcher) Stop() {
	close(w.stopCh)
	w.fsWatcher.Close()
	w.wg.Wait()
	w.logger.Info("file watcher stopped")
}

// eventLoop processes raw fsnotify events.
func (w *Watcher) eventLoop() {
	defer w.wg.Done()

	for {
		select {
		case <-w.stopCh:
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleFSEvent(event)
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.logger.Error("watcher error", "error", err)
		}
	}
}

// handleFSEvent processes a single file system event with debouncing.
func (w *Watcher) handleFSEvent(event fsnotify.Event) {
	path := event.Name

	// Skip .uniteos internal directory
	rel, _ := filepath.Rel(w.workspaceDir, path)
	if strings.HasPrefix(rel, ".uniteos") || strings.HasPrefix(filepath.Base(path), ".") {
		return
	}

	// If a new directory is created, watch it too
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			w.fsWatcher.Add(path)
		}
	}

	// Debounce: record the event timestamp
	w.pendingMu.Lock()
	w.pending[path] = time.Now()
	w.pendingMu.Unlock()
}

// debounceLoop flushes debounced events periodically.
func (w *Watcher) debounceLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.flushPending()
		}
	}
}

// flushPending processes events that have been debounced long enough.
func (w *Watcher) flushPending() {
	w.pendingMu.Lock()
	now := time.Now()
	var ready []string
	for path, ts := range w.pending {
		if now.Sub(ts) >= w.debounce {
			ready = append(ready, path)
			delete(w.pending, path)
		}
	}
	w.pendingMu.Unlock()

	for _, path := range ready {
		w.processChange(path)
	}
}

// processChange handles a confirmed file change.
func (w *Watcher) processChange(path string) {
	absPath, _ := filepath.Abs(path)
	relPath, _ := filepath.Rel(w.workspaceDir, absPath)

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File was deleted
			w.eventBus.Publish(core.NewEvent(core.EventFileDeleted, "watcher", map[string]interface{}{
				"path":     relPath,
				"abs_path": absPath,
			}))
			w.store.DB.RemoveTrackedFile(absPath)
			w.logger.Info("file deleted", "path", relPath)
			return
		}
		return
	}

	if info.IsDir() {
		return
	}

	// Check if tracked
	existing, _ := w.store.DB.GetTrackedFile(absPath)

	if existing == nil {
		// New untracked file — auto-track it
		if err := w.store.TrackFile(absPath); err != nil {
			w.logger.Warn("auto-track failed", "path", relPath, "error", err)
			return
		}
		w.eventBus.Publish(core.NewEvent(core.EventFileCreated, "watcher", map[string]interface{}{
			"path":     relPath,
			"abs_path": absPath,
			"size":     info.Size(),
		}))
		w.logger.Info("new file detected and tracked", "path", relPath)
	} else {
		// Existing file — check for modifications
		changed, err := w.store.UpdateFile(absPath)
		if err != nil {
			w.logger.Warn("update check failed", "path", relPath, "error", err)
			return
		}
		if changed {
			w.logger.Info("file modified", "path", relPath)
		}
	}
}

// GetWatchedPaths returns all paths being watched.
func (w *Watcher) GetWatchedPaths() []string {
	return w.fsWatcher.WatchList()
}
