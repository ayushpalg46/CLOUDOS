package sync

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ayushgpal/uniteos/internal/core"
	"github.com/ayushgpal/uniteos/internal/storage"
)

// SyncStatus represents the overall sync state.
type SyncStatus string

const (
	StatusIdle      SyncStatus = "idle"
	StatusSyncing   SyncStatus = "syncing"
	StatusConflict  SyncStatus = "conflict"
	StatusError     SyncStatus = "error"
)

// PeerInfo represents a connected peer device.
type PeerInfo struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Address    string    `json:"address"`
	LastSeen   time.Time `json:"last_seen"`
	Connected  bool      `json:"connected"`
}

// SyncManager orchestrates multi-device synchronization.
type SyncManager struct {
	deviceID     string
	workspaceDir string
	store        *storage.Store
	eventBus     *core.EventBus
	resolver     *ConflictResolver
	logger       *slog.Logger

	// CRDT state for all tracked files
	fileStates map[string]*FileState
	statesMu   sync.RWMutex

	// Connected peers
	peers   map[string]*PeerInfo
	peersMu sync.RWMutex

	status   SyncStatus
	statusMu sync.RWMutex

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewSyncManager creates a new sync manager.
func NewSyncManager(
	deviceID, workspaceDir string,
	store *storage.Store,
	eventBus *core.EventBus,
	strategy ConflictStrategy,
	logger *slog.Logger,
) *SyncManager {
	return &SyncManager{
		deviceID:     deviceID,
		workspaceDir: workspaceDir,
		store:        store,
		eventBus:     eventBus,
		resolver:     NewConflictResolver(strategy, workspaceDir, logger),
		logger:       logger,
		fileStates:   make(map[string]*FileState),
		peers:        make(map[string]*PeerInfo),
		status:       StatusIdle,
		stopCh:       make(chan struct{}),
	}
}

// Start begins the sync manager's background operations.
func (sm *SyncManager) Start() error {
	sm.logger.Info("sync manager starting", "device_id", sm.deviceID)

	// Load local file states
	if err := sm.loadLocalStates(); err != nil {
		return fmt.Errorf("load local states: %w", err)
	}

	// Subscribe to file events
	sm.eventBus.Subscribe(core.EventFileCreated, sm.onFileEvent)
	sm.eventBus.Subscribe(core.EventFileModified, sm.onFileEvent)
	sm.eventBus.Subscribe(core.EventFileDeleted, sm.onFileEvent)
	sm.eventBus.Subscribe(core.EventFileTracked, sm.onFileEvent)

	// Start periodic state persistence
	sm.wg.Add(1)
	go sm.persistenceLoop()

	sm.eventBus.Publish(core.NewEvent(core.EventSyncStarted, "sync", map[string]interface{}{
		"device_id": sm.deviceID,
	}))

	return nil
}

// Stop shuts down the sync manager.
func (sm *SyncManager) Stop() {
	close(sm.stopCh)
	sm.wg.Wait()
	sm.saveStates()
	sm.logger.Info("sync manager stopped")
}

// loadLocalStates initializes CRDT states from the local database.
func (sm *SyncManager) loadLocalStates() error {
	// Try to load persisted states
	statePath := filepath.Join(sm.workspaceDir, ".uniteos", "sync_state.json")
	if data, err := os.ReadFile(statePath); err == nil {
		var states map[string]*FileState
		if err := json.Unmarshal(data, &states); err == nil {
			sm.statesMu.Lock()
			sm.fileStates = states
			sm.statesMu.Unlock()
			sm.logger.Info("loaded persisted sync states", "count", len(states))
			return nil
		}
	}

	// Build from tracked files
	files, err := sm.store.DB.ListTrackedFiles()
	if err != nil {
		return err
	}

	sm.statesMu.Lock()
	defer sm.statesMu.Unlock()

	for _, f := range files {
		if f.IsDir {
			continue
		}
		modUnix := f.ModTime
		state := NewFileState(f.RelativePath, f.Hash, f.Size, modUnix, sm.deviceID)
		sm.fileStates[f.RelativePath] = state
	}

	sm.logger.Info("initialized sync states from database", "count", len(sm.fileStates))
	return nil
}

// onFileEvent handles file change events and updates CRDT states.
func (sm *SyncManager) onFileEvent(event core.Event) {
	path, _ := event.Data["path"].(string)
	if path == "" {
		return
	}

	sm.statesMu.Lock()
	defer sm.statesMu.Unlock()

	existing := sm.fileStates[path]

	switch event.Type {
	case core.EventFileCreated, core.EventFileTracked:
		hash, _ := event.Data["hash"].(string)
		size, _ := event.Data["size"].(int64)
		if existing != nil {
			if existing.Hash.Value.(string) == hash {
				return // File is identical, prevent ping-pong loop
			}
			existing.Hash = *NewLWWRegister(hash, sm.deviceID)
			existing.Size = *NewLWWRegister(size, sm.deviceID)
			existing.ModTime = *NewLWWRegister(time.Now().Unix(), sm.deviceID)
			existing.VectorClock.Increment(sm.deviceID)
			existing.VersionCount.Increment(sm.deviceID)
			return
		}
		state := NewFileState(path, hash, size, time.Now(), sm.deviceID)
		sm.fileStates[path] = state
		sm.logger.Debug("sync state created", "path", path)

	case core.EventFileModified:
		hash, _ := event.Data["hash"].(string)
		if existing != nil {
			existing.Hash = *NewLWWRegister(hash, sm.deviceID)
			existing.ModTime = *NewLWWRegister(time.Now().Unix(), sm.deviceID)
			existing.VectorClock.Increment(sm.deviceID)
			existing.VersionCount.Increment(sm.deviceID)
		}

	case core.EventFileDeleted:
		if existing != nil {
			existing.Deleted = *NewLWWRegister(true, sm.deviceID)
			existing.VectorClock.Increment(sm.deviceID)
		}
	}
}

// ReceiveRemoteState processes a file state received from a peer device.
func (sm *SyncManager) ReceiveRemoteState(remoteState *FileState) (bool, *Conflict, error) {
	sm.statesMu.Lock()
	defer sm.statesMu.Unlock()

	localState, exists := sm.fileStates[remoteState.Path]

	if !exists {
		// New file from remote — accept it
		sm.fileStates[remoteState.Path] = remoteState
		sm.logger.Info("accepted new remote file", "path", remoteState.Path)
		return true, nil, nil // Return true to indicate we need to download this file
	}

	// Merge CRDT states
	merged, isConflict := MergeFileState(localState, remoteState)
	sm.fileStates[remoteState.Path] = merged

	if isConflict {
		sm.setStatus(StatusConflict)
		conflict, err := sm.resolver.Resolve(localState, remoteState)
		if err != nil {
			return false, nil, err
		}

		sm.eventBus.Publish(core.NewEvent(core.EventSyncConflict, "sync", map[string]interface{}{
			"path":        remoteState.Path,
			"conflict_id": conflict.ID,
			"resolution":  conflict.Resolution,
		}))

		return false, conflict, nil
	}

	// If the merged state's vector clock equals the remote state's vector clock,
	// and it's not equal to the local state's clock, it means the remote won the merge.
	// We should download the file.
	needsDownload := false
	if !localState.VectorClock.Equal(merged.VectorClock) && merged.VectorClock.Equal(remoteState.VectorClock) {
		needsDownload = true
	}

	sm.logger.Debug("merged remote state", "path", remoteState.Path)
	return needsDownload, nil, nil
}

// GetStatesForSync returns all local CRDT states for sending to a peer.
func (sm *SyncManager) GetStatesForSync() map[string]*FileState {
	sm.statesMu.RLock()
	defer sm.statesMu.RUnlock()

	states := make(map[string]*FileState, len(sm.fileStates))
	for k, v := range sm.fileStates {
		states[k] = v
	}
	return states
}

// GetChangedSince returns file states that changed after a given vector clock.
func (sm *SyncManager) GetChangedSince(peerClock VectorClock) map[string]*FileState {
	sm.statesMu.RLock()
	defer sm.statesMu.RUnlock()

	changed := make(map[string]*FileState)
	for path, state := range sm.fileStates {
		if peerClock.HappensBefore(state.VectorClock) {
			changed[path] = state
		}
	}
	return changed
}

// SyncWithPeer performs a full sync exchange with a peer.
func (sm *SyncManager) SyncWithPeer(peerID string, peerStates map[string]*FileState) ([]*FileState, []Conflict, error) {
	sm.setStatus(StatusSyncing)
	defer sm.setStatus(StatusIdle)

	sm.logger.Info("syncing with peer", "peer_id", peerID, "remote_files", len(peerStates))

	var conflicts []Conflict
	var toDownload []*FileState

	for path, remoteState := range peerStates {
		needsDownload, conflict, err := sm.ReceiveRemoteState(remoteState)
		if err != nil {
			sm.logger.Error("sync error", "path", path, "error", err)
			continue
		}
		if needsDownload {
			toDownload = append(toDownload, remoteState)
		}
		if conflict != nil && !conflict.Resolved {
			conflicts = append(conflicts, *conflict)
		}
	}

	sm.eventBus.Publish(core.NewEvent(core.EventSyncCompleted, "sync", map[string]interface{}{
		"peer_id":    peerID,
		"files":      len(peerStates),
		"downloads":  len(toDownload),
		"conflicts":  len(conflicts),
	}))

	sm.logger.Info("sync completed",
		"peer_id", peerID,
		"downloads", len(toDownload),
		"conflicts", len(conflicts),
	)

	return toDownload, conflicts, nil
}

// AddPeer registers a discovered peer.
func (sm *SyncManager) AddPeer(peer *PeerInfo) {
	sm.peersMu.Lock()
	defer sm.peersMu.Unlock()
	sm.peers[peer.DeviceID] = peer
	sm.eventBus.Publish(core.NewEvent(core.EventDeviceDetected, "sync", map[string]interface{}{
		"device_id":   peer.DeviceID,
		"device_name": peer.DeviceName,
		"address":     peer.Address,
	}))
}

// RemovePeer removes a peer.
func (sm *SyncManager) RemovePeer(deviceID string) {
	sm.peersMu.Lock()
	defer sm.peersMu.Unlock()
	delete(sm.peers, deviceID)
	sm.eventBus.Publish(core.NewEvent(core.EventDeviceLost, "sync", map[string]interface{}{
		"device_id": deviceID,
	}))
}

// GetPeer returns a known peer by ID.
func (sm *SyncManager) GetPeer(deviceID string) (*PeerInfo, bool) {
	sm.peersMu.RLock()
	defer sm.peersMu.RUnlock()
	peer, exists := sm.peers[deviceID]
	if exists {
		pCopy := *peer
		return &pCopy, true
	}
	return nil, false
}

// GetPeers returns all known peers.
func (sm *SyncManager) GetPeers() []PeerInfo {
	sm.peersMu.RLock()
	defer sm.peersMu.RUnlock()
	var peers []PeerInfo
	for _, p := range sm.peers {
		peers = append(peers, *p)
	}
	return peers
}

// GetStatus returns the current sync status.
func (sm *SyncManager) GetStatus() SyncStatus {
	sm.statusMu.RLock()
	defer sm.statusMu.RUnlock()
	return sm.status
}

func (sm *SyncManager) setStatus(status SyncStatus) {
	sm.statusMu.Lock()
	sm.status = status
	sm.statusMu.Unlock()
}

// GetConflicts returns unresolved conflicts.
func (sm *SyncManager) GetConflicts() []Conflict {
	return sm.resolver.GetUnresolved()
}

// ResolveConflict manually resolves a conflict.
func (sm *SyncManager) ResolveConflict(conflictID, resolution string) error {
	return sm.resolver.ResolveManual(conflictID, resolution)
}

// persistenceLoop periodically saves CRDT states to disk.
func (sm *SyncManager) persistenceLoop() {
	defer sm.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sm.stopCh:
			return
		case <-ticker.C:
			sm.saveStates()
		}
	}
}

// saveStates persists CRDT states to disk.
func (sm *SyncManager) saveStates() {
	sm.statesMu.RLock()
	data, err := json.MarshalIndent(sm.fileStates, "", "  ")
	sm.statesMu.RUnlock()

	if err != nil {
		sm.logger.Error("failed to marshal sync states", "error", err)
		return
	}

	statePath := filepath.Join(sm.workspaceDir, ".uniteos", "sync_state.json")
	if err := os.WriteFile(statePath, data, 0600); err != nil {
		sm.logger.Error("failed to save sync states", "error", err)
	}
}
