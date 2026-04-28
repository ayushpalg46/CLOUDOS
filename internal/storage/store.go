package storage

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ayushgpal/uniteos/internal/core"
)

// Store is the unified storage manager that combines SQLite, BlobStore, and events.
type Store struct {
	DB        *DB
	Blobs     *BlobStore
	Config    *core.Config
	EventBus  *core.EventBus
	Logger    *slog.Logger
}

// NewStore initializes the complete storage system.
func NewStore(config *core.Config, eventBus *core.EventBus, logger *slog.Logger) (*Store, error) {
	db, err := NewDB(config.DatabasePath, logger)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	blobs, err := NewBlobStore(config.BlobStorePath, config.ChunkSizeBytes, logger)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("init blob store: %w", err)
	}

	return &Store{
		DB: db, Blobs: blobs,
		Config: config, EventBus: eventBus, Logger: logger,
	}, nil
}

// Close closes the storage system.
func (s *Store) Close() error {
	return s.DB.Close()
}

// TrackFile adds a file or directory to tracking.
func (s *Store) TrackFile(absPath string) error {
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	relPath, err := filepath.Rel(s.Config.WorkspaceDir, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") || filepath.VolumeName(absPath) != filepath.VolumeName(s.Config.WorkspaceDir) {
		// File is outside the workspace. Copy it in to ensure safe syncing!
		if info.IsDir() {
			return fmt.Errorf("cannot import directories from outside workspace directly")
		}
		
		fileName := filepath.Base(absPath)
		newAbsPath := filepath.Join(s.Config.WorkspaceDir, "Imports", fileName)
		
		// Create Imports directory if it doesn't exist
		os.MkdirAll(filepath.Dir(newAbsPath), 0755)
		
		// Read from original, write to workspace
		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("read external file: %w", err)
		}
		if err := os.WriteFile(newAbsPath, data, 0644); err != nil {
			return fmt.Errorf("copy to workspace: %w", err)
		}
		
		// Update variables to point to the new safe copy
		absPath = newAbsPath
		relPath = filepath.Join("Imports", fileName)
		info, _ = os.Stat(absPath)
	}

	var hash string
	if !info.IsDir() {
		hash, err = HashFile(absPath)
		if err != nil {
			return fmt.Errorf("hash file: %w", err)
		}
	}

	file := &TrackedFile{
		Path:         absPath,
		RelativePath: relPath,
		Hash:         hash,
		Size:         info.Size(),
		ModTime:      info.ModTime(),
		IsDir:        info.IsDir(),
		Status:       "active",
	}

	fileID, err := s.DB.AddTrackedFile(file)
	if err != nil {
		return err
	}

	// Store blob for files
	if !info.IsDir() {
		blobInfo, err := s.Blobs.StoreFile(absPath)
		if err != nil {
			s.Logger.Warn("failed to store blob", "path", relPath, "error", err)
		} else {
			// Create initial version
			blobRefs, _ := json.Marshal(blobInfo.ChunkHashes)
			s.DB.AddFileVersion(&FileVersion{
				FileID:     fileID,
				VersionNum: 1,
				Hash:       hash,
				Size:       info.Size(),
				BlobRefs:   string(blobRefs),
				ChangeType: "created",
			})
		}
	}

	s.EventBus.Publish(core.NewEvent(core.EventFileTracked, "storage", map[string]interface{}{
		"path": relPath, "hash": hash, "size": info.Size(),
	}))

	return nil
}

// TrackDirectory recursively tracks all files in a directory.
func (s *Store) TrackDirectory(dirPath string) (int, error) {
	count := 0
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		// Skip .uniteos directory
		if info.IsDir() && info.Name() == ".uniteos" {
			return filepath.SkipDir
		}
		// Skip hidden files/dirs
		if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if err := s.TrackFile(path); err != nil {
			s.Logger.Warn("skip file", "path", path, "error", err)
			return nil
		}
		count++
		return nil
	})
	return count, err
}

// UpdateFile checks for changes and creates a new version if modified.
func (s *Store) UpdateFile(absPath string) (bool, error) {
	existing, err := s.DB.GetTrackedFile(absPath)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return false, fmt.Errorf("file not tracked: %s", absPath)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.DB.RemoveTrackedFile(absPath)
			s.EventBus.Publish(core.NewEvent(core.EventFileDeleted, "storage", map[string]interface{}{
				"path": existing.RelativePath,
			}))
			return true, nil
		}
		return false, err
	}

	newHash, err := HashFile(absPath)
	if err != nil {
		return false, err
	}

	if newHash == existing.Hash {
		return false, nil // No changes
	}

	// File has changed — store new blob and create version
	blobInfo, err := s.Blobs.StoreFile(absPath)
	if err != nil {
		return false, err
	}

	latestVer, _ := s.DB.GetLatestVersionNum(existing.ID)
	blobRefs, _ := json.Marshal(blobInfo.ChunkHashes)

	s.DB.AddFileVersion(&FileVersion{
		FileID:     existing.ID,
		VersionNum: latestVer + 1,
		Hash:       newHash,
		Size:       info.Size(),
		BlobRefs:   string(blobRefs),
		ChangeType: "modified",
	})

	// Update tracked file record
	existing.Hash = newHash
	existing.Size = info.Size()
	existing.ModTime = info.ModTime()
	s.DB.AddTrackedFile(existing)

	s.EventBus.Publish(core.NewEvent(core.EventFileModified, "storage", map[string]interface{}{
		"path": existing.RelativePath, "hash": newHash, "version": latestVer + 1,
	}))

	return true, nil
}

// CreateSnapshot creates a point-in-time snapshot of all tracked files.
func (s *Store) CreateSnapshot(name, description string) (*Snapshot, error) {
	files, err := s.DB.ListTrackedFiles()
	if err != nil {
		return nil, err
	}

	snapshotID := fmt.Sprintf("snap-%d", time.Now().UnixNano())

	var totalSize int64
	fileCount := 0

	for _, f := range files {
		if f.IsDir {
			continue
		}

		latestVer, _ := s.DB.GetLatestVersionNum(f.ID)
		versions, _ := s.DB.GetFileVersions(f.ID)
		var blobRefs string
		for _, v := range versions {
			if v.VersionNum == latestVer {
				blobRefs = v.BlobRefs
				break
			}
		}

		s.DB.AddSnapshotFile(&SnapshotFile{
			SnapshotID: snapshotID,
			FilePath:   f.RelativePath,
			FileHash:   f.Hash,
			FileSize:   f.Size,
			BlobRefs:   blobRefs,
		})

		totalSize += f.Size
		fileCount++
	}

	snapshot := &Snapshot{
		SnapshotID:  snapshotID,
		Name:        name,
		Description: description,
		FileCount:   fileCount,
		TotalSize:   totalSize,
	}

	if err := s.DB.AddSnapshot(snapshot); err != nil {
		return nil, err
	}

	s.EventBus.Publish(core.NewEvent(core.EventSnapshotCreated, "storage", map[string]interface{}{
		"snapshot_id": snapshotID, "files": fileCount, "size": totalSize,
	}))

	return snapshot, nil
}

// RestoreSnapshot restores files from a snapshot.
func (s *Store) RestoreSnapshot(snapshotID string) error {
	files, err := s.DB.GetSnapshotFiles(snapshotID)
	if err != nil {
		return err
	}

	for _, sf := range files {
		var chunkHashes []string
		if err := json.Unmarshal([]byte(sf.BlobRefs), &chunkHashes); err != nil {
			s.Logger.Warn("invalid blob refs", "file", sf.FilePath)
			continue
		}

		destPath := filepath.Join(s.Config.WorkspaceDir, sf.FilePath)
		blobInfo := &BlobInfo{
			Hash: sf.FileHash, Size: sf.FileSize,
			ChunkCount: len(chunkHashes), ChunkHashes: chunkHashes,
		}

		if err := s.Blobs.RestoreFile(blobInfo, destPath); err != nil {
			s.Logger.Error("restore file failed", "file", sf.FilePath, "error", err)
			continue
		}
	}

	s.EventBus.Publish(core.NewEvent(core.EventSnapshotRestored, "storage", map[string]interface{}{
		"snapshot_id": snapshotID, "files": len(files),
	}))

	return nil
}

// GetStatus returns the status of all tracked files (changed, unchanged, deleted).
func (s *Store) GetStatus() ([]FileStatus, error) {
	files, err := s.DB.ListTrackedFiles()
	if err != nil {
		return nil, err
	}

	var statuses []FileStatus
	for _, f := range files {
		if f.IsDir {
			continue
		}

		status := FileStatus{
			Path:     f.RelativePath,
			OldHash:  f.Hash,
			Size:     f.Size,
			ModTime:  f.ModTime,
			Status:   "unchanged",
		}

		info, err := os.Stat(f.Path)
		if err != nil {
			if os.IsNotExist(err) {
				status.Status = "deleted"
			} else {
				status.Status = "error"
			}
			statuses = append(statuses, status)
			continue
		}

		if info.ModTime().After(f.ModTime) {
			newHash, err := HashFile(f.Path)
			if err == nil && newHash != f.Hash {
				status.Status = "modified"
				status.NewHash = newHash
				status.Size = info.Size()
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// FileStatus represents the current status of a tracked file.
type FileStatus struct {
	Path    string    `json:"path"`
	OldHash string    `json:"old_hash"`
	NewHash string    `json:"new_hash,omitempty"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Status  string    `json:"status"` // unchanged, modified, deleted, error
}
