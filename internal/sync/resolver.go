package sync

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// ConflictStrategy defines how to resolve sync conflicts.
type ConflictStrategy string

const (
	StrategyLastWriterWins ConflictStrategy = "lww"       // Latest timestamp wins
	StrategyKeepBoth       ConflictStrategy = "keep_both" // Keep both versions
	StrategyManual         ConflictStrategy = "manual"    // Flag for manual resolution
)

// Conflict represents a sync conflict between two devices.
type Conflict struct {
	ID            string           `json:"id"`
	FilePath      string           `json:"file_path"`
	LocalState    *FileState       `json:"local_state"`
	RemoteState   *FileState       `json:"remote_state"`
	Strategy      ConflictStrategy `json:"strategy"`
	Resolved      bool             `json:"resolved"`
	Resolution    string           `json:"resolution"` // "local", "remote", "both"
	ResolvedAt    *time.Time       `json:"resolved_at,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
}

// ConflictResolver handles sync conflicts between devices.
type ConflictResolver struct {
	strategy     ConflictStrategy
	workspaceDir string
	conflicts    []Conflict
	logger       *slog.Logger
}

// NewConflictResolver creates a new conflict resolver.
func NewConflictResolver(strategy ConflictStrategy, workspaceDir string, logger *slog.Logger) *ConflictResolver {
	return &ConflictResolver{
		strategy:     strategy,
		workspaceDir: workspaceDir,
		logger:       logger,
	}
}

// Resolve resolves a conflict between local and remote file states.
func (cr *ConflictResolver) Resolve(local, remote *FileState) (*Conflict, error) {
	conflictID := fmt.Sprintf("conflict-%d", time.Now().UnixNano())

	conflict := Conflict{
		ID:          conflictID,
		FilePath:    local.Path,
		LocalState:  local,
		RemoteState: remote,
		Strategy:    cr.strategy,
		CreatedAt:   time.Now(),
	}

	switch cr.strategy {
	case StrategyLastWriterWins:
		return cr.resolveByLWW(&conflict)
	case StrategyKeepBoth:
		return cr.resolveByKeepBoth(&conflict)
	case StrategyManual:
		// Don't auto-resolve; flag for user
		cr.conflicts = append(cr.conflicts, conflict)
		cr.logger.Warn("sync conflict requires manual resolution",
			"file", conflict.FilePath,
			"conflict_id", conflictID,
		)
		return &conflict, nil
	default:
		return cr.resolveByLWW(&conflict)
	}
}

// resolveByLWW resolves by picking the latest modification.
func (cr *ConflictResolver) resolveByLWW(conflict *Conflict) (*Conflict, error) {
	localTime := conflict.LocalState.ModTime.Timestamp
	remoteTime := conflict.RemoteState.ModTime.Timestamp

	if remoteTime.After(localTime) {
		conflict.Resolution = "remote"
	} else {
		conflict.Resolution = "local"
	}

	conflict.Resolved = true
	now := time.Now()
	conflict.ResolvedAt = &now

	cr.logger.Info("conflict resolved by LWW",
		"file", conflict.FilePath,
		"winner", conflict.Resolution,
	)
	return conflict, nil
}

// resolveByKeepBoth saves the conflicting version alongside the original.
func (cr *ConflictResolver) resolveByKeepBoth(conflict *Conflict) (*Conflict, error) {
	// Create a conflict copy: filename.conflict-{deviceID}.ext
	filePath := filepath.Join(cr.workspaceDir, conflict.FilePath)
	ext := filepath.Ext(filePath)
	base := filePath[:len(filePath)-len(ext)]
	remoteDevice := conflict.RemoteState.Hash.DeviceID

	conflictPath := fmt.Sprintf("%s.conflict-%s%s", base, remoteDevice[:8], ext)

	// If the remote version's data needs to be written, the sync manager handles
	// the actual file transfer. Here we just record the decision.
	conflict.Resolution = "both"
	conflict.Resolved = true
	now := time.Now()
	conflict.ResolvedAt = &now

	cr.logger.Info("conflict resolved by keeping both",
		"file", conflict.FilePath,
		"conflict_copy", conflictPath,
	)
	return conflict, nil
}

// SaveConflictCopy saves a conflicting file version to disk.
func (cr *ConflictResolver) SaveConflictCopy(originalPath string, data []byte, deviceID string) (string, error) {
	ext := filepath.Ext(originalPath)
	base := originalPath[:len(originalPath)-len(ext)]
	shortDevice := deviceID
	if len(shortDevice) > 8 {
		shortDevice = shortDevice[:8]
	}

	conflictPath := fmt.Sprintf("%s.conflict-%s-%s%s",
		base, shortDevice, time.Now().Format("20060102-150405"), ext)

	if err := os.WriteFile(conflictPath, data, 0644); err != nil {
		return "", fmt.Errorf("save conflict copy: %w", err)
	}

	return conflictPath, nil
}

// GetUnresolved returns all unresolved conflicts.
func (cr *ConflictResolver) GetUnresolved() []Conflict {
	var unresolved []Conflict
	for _, c := range cr.conflicts {
		if !c.Resolved {
			unresolved = append(unresolved, c)
		}
	}
	return unresolved
}

// ResolveManual manually resolves a conflict.
func (cr *ConflictResolver) ResolveManual(conflictID, resolution string) error {
	for i, c := range cr.conflicts {
		if c.ID == conflictID {
			if resolution != "local" && resolution != "remote" && resolution != "both" {
				return fmt.Errorf("invalid resolution: must be 'local', 'remote', or 'both'")
			}
			cr.conflicts[i].Resolution = resolution
			cr.conflicts[i].Resolved = true
			now := time.Now()
			cr.conflicts[i].ResolvedAt = &now
			cr.logger.Info("conflict manually resolved",
				"conflict_id", conflictID,
				"resolution", resolution,
			)
			return nil
		}
	}
	return fmt.Errorf("conflict not found: %s", conflictID)
}
