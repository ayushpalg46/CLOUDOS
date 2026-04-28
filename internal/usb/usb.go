// Package usb provides USB-based sync for uniteOS.
// Supports two modes:
//   1. USB Drive (sneakernet) — export/import sync bundles via flash drive
//   2. USB Tethering — works automatically via the existing P2P sync
package usb

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ayushgpal/uniteos/internal/storage"
	csync "github.com/ayushgpal/uniteos/internal/sync"
)

// SyncBundle is a portable sync package that can be transferred via USB drive.
type SyncBundle struct {
	Version    string                      `json:"version"`
	DeviceID   string                      `json:"device_id"`
	DeviceName string                      `json:"device_name"`
	CreatedAt  time.Time                   `json:"created_at"`
	FileStates map[string]*csync.FileState `json:"file_states"`
	Files      []BundleFile                `json:"files"`
}

// BundleFile represents a file in the sync bundle.
type BundleFile struct {
	RelativePath string    `json:"relative_path"`
	Hash         string    `json:"hash"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	IsDir        bool      `json:"is_dir"`
}

// USBSync handles USB drive-based synchronization.
type USBSync struct {
	store        *storage.Store
	syncManager  *csync.SyncManager
	workspaceDir string
	deviceID     string
	deviceName   string
	logger       *slog.Logger
}

// NewUSBSync creates a new USB sync handler.
func NewUSBSync(
	store *storage.Store,
	syncManager *csync.SyncManager,
	workspaceDir, deviceID, deviceName string,
	logger *slog.Logger,
) *USBSync {
	return &USBSync{
		store:        store,
		syncManager:  syncManager,
		workspaceDir: workspaceDir,
		deviceID:     deviceID,
		deviceName:   deviceName,
		logger:       logger,
	}
}

// Export creates a sync bundle directory on a USB drive / target path.
// The bundle includes a manifest and all tracked files.
func (u *USBSync) Export(targetDir string) (*ExportReport, error) {
	start := time.Now()

	// Create bundle directory
	bundleDir := filepath.Join(targetDir, fmt.Sprintf("uniteos-sync-%s", u.deviceID[:8]))
	os.MkdirAll(bundleDir, 0755)
	filesDir := filepath.Join(bundleDir, "files")
	os.MkdirAll(filesDir, 0755)

	// Get all tracked files
	trackedFiles, err := u.store.DB.ListTrackedFiles()
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	// Build bundle manifest
	bundle := SyncBundle{
		Version:    "1.0",
		DeviceID:   u.deviceID,
		DeviceName: u.deviceName,
		CreatedAt:  time.Now(),
		FileStates: u.syncManager.GetStatesForSync(),
	}

	report := &ExportReport{BundlePath: bundleDir, StartedAt: start}

	// Copy files to bundle
	for _, f := range trackedFiles {
		if f.IsDir {
			continue
		}

		bundle.Files = append(bundle.Files, BundleFile{
			RelativePath: f.RelativePath,
			Hash:         f.Hash,
			Size:         f.Size,
			ModTime:      f.ModTime,
			IsDir:        false,
		})

		// Copy file to bundle
		srcPath := f.Path
		destPath := filepath.Join(filesDir, f.RelativePath)
		os.MkdirAll(filepath.Dir(destPath), 0755)

		if err := copyFile(srcPath, destPath); err != nil {
			u.logger.Warn("failed to copy file", "path", f.RelativePath, "error", err)
			report.Errors++
			continue
		}
		report.FilesCopied++
		report.BytesCopied += f.Size
	}

	// Write manifest
	manifestPath := filepath.Join(bundleDir, "manifest.json")
	manifestData, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	report.Duration = time.Since(start).String()

	u.logger.Info("USB export complete",
		"bundle", bundleDir,
		"files", report.FilesCopied,
		"bytes", report.BytesCopied,
	)

	return report, nil
}

// Import reads a sync bundle from a USB drive and merges it into the workspace.
func (u *USBSync) Import(bundleDir string) (*ImportReport, error) {
	start := time.Now()

	// Read manifest
	manifestPath := filepath.Join(bundleDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w (is this a uniteOS sync bundle?)", err)
	}

	var bundle SyncBundle
	if err := json.Unmarshal(manifestData, &bundle); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if bundle.DeviceID == u.deviceID {
		return nil, fmt.Errorf("cannot import from the same device (ID: %s)", u.deviceID[:8])
	}

	report := &ImportReport{
		SourceDevice: bundle.DeviceName,
		SourceID:     bundle.DeviceID,
		BundleDate:   bundle.CreatedAt,
		StartedAt:    start,
	}

	filesDir := filepath.Join(bundleDir, "files")

	// Merge CRDT states via peer sync
	if bundle.FileStates != nil {
		for _, remoteState := range bundle.FileStates {
			u.syncManager.ReceiveRemoteState(remoteState)
		}
		report.StatesMerged = len(bundle.FileStates)
	}

	// Copy new/updated files
	for _, f := range bundle.Files {
		if f.IsDir {
			continue
		}

		srcPath := filepath.Join(filesDir, f.RelativePath)
		destPath := filepath.Join(u.workspaceDir, f.RelativePath)

		// Check if file exists and is different
		existingHash := ""
		if existing, err := u.store.DB.GetTrackedFile(f.RelativePath); err == nil && existing != nil {
			existingHash = existing.Hash
		}

		if existingHash == f.Hash {
			report.Skipped++
			continue // Same content, skip
		}

		// Copy file
		os.MkdirAll(filepath.Dir(destPath), 0755)
		if err := copyFile(srcPath, destPath); err != nil {
			u.logger.Warn("failed to import file", "path", f.RelativePath, "error", err)
			report.Errors++
			continue
		}

		// Re-track the updated file
		u.store.TrackFile(destPath)

		if existingHash == "" {
			report.NewFiles++
		} else {
			report.Updated++
		}
		report.BytesCopied += f.Size
	}

	report.Duration = time.Since(start).String()

	u.logger.Info("USB import complete",
		"source", bundle.DeviceName,
		"new", report.NewFiles,
		"updated", report.Updated,
		"skipped", report.Skipped,
	)

	return report, nil
}

// ListBundles scans a USB drive path for available sync bundles.
func ListBundles(drivePath string) ([]BundleInfo, error) {
	entries, err := os.ReadDir(drivePath)
	if err != nil {
		return nil, err
	}

	var bundles []BundleInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(drivePath, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var bundle SyncBundle
		if err := json.Unmarshal(data, &bundle); err != nil {
			continue
		}
		bundles = append(bundles, BundleInfo{
			Path:       filepath.Join(drivePath, entry.Name()),
			DeviceID:   bundle.DeviceID,
			DeviceName: bundle.DeviceName,
			CreatedAt:  bundle.CreatedAt,
			FileCount:  len(bundle.Files),
		})
	}
	return bundles, nil
}

// ─── Report Types ──────────────────────────────────────────────

// ExportReport contains the results of a USB export.
type ExportReport struct {
	BundlePath  string    `json:"bundle_path"`
	FilesCopied int       `json:"files_copied"`
	BytesCopied int64     `json:"bytes_copied"`
	Errors      int       `json:"errors"`
	Duration    string    `json:"duration"`
	StartedAt   time.Time `json:"started_at"`
}

// ImportReport contains the results of a USB import.
type ImportReport struct {
	SourceDevice string    `json:"source_device"`
	SourceID     string    `json:"source_id"`
	BundleDate   time.Time `json:"bundle_date"`
	NewFiles     int       `json:"new_files"`
	Updated      int       `json:"updated"`
	Skipped      int       `json:"skipped"`
	StatesMerged int       `json:"states_merged"`
	BytesCopied  int64     `json:"bytes_copied"`
	Errors       int       `json:"errors"`
	Duration     string    `json:"duration"`
	StartedAt    time.Time `json:"started_at"`
}

// BundleInfo describes a discovered sync bundle on a USB drive.
type BundleInfo struct {
	Path       string    `json:"path"`
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	CreatedAt  time.Time `json:"created_at"`
	FileCount  int       `json:"file_count"`
}

// ─── Helpers ───────────────────────────────────────────────────

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	return err
}
