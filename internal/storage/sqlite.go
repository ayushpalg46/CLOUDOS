// Package storage provides the storage layer for UNITEos.
// It handles structured data (SQLite), blob storage, metadata indexing,
// and deduplication.
package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database with UNITEos-specific operations.
type DB struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewDB opens (or creates) a SQLite database at the given path.
func NewDB(dbPath string, logger *slog.Logger) (*DB, error) {
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite only supports 1 writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // Keep connections alive

	sqldb := &DB{db: db, logger: logger}

	if err := sqldb.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("database initialized", "path", dbPath)
	return sqldb, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// migrate runs all database migrations.
func (d *DB) migrate() error {
	migrations := []string{
		// Tracked files table
		`CREATE TABLE IF NOT EXISTS tracked_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			relative_path TEXT NOT NULL,
			hash TEXT NOT NULL DEFAULT '',
			size INTEGER NOT NULL DEFAULT 0,
			mod_time DATETIME NOT NULL,
			is_dir BOOLEAN NOT NULL DEFAULT FALSE,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		// File versions table
		`CREATE TABLE IF NOT EXISTS file_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_id INTEGER NOT NULL,
			version_num INTEGER NOT NULL,
			hash TEXT NOT NULL,
			size INTEGER NOT NULL,
			blob_refs TEXT NOT NULL DEFAULT '[]',
			change_type TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (file_id) REFERENCES tracked_files(id) ON DELETE CASCADE,
			UNIQUE(file_id, version_num)
		)`,

		// Snapshots table
		`CREATE TABLE IF NOT EXISTS snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			snapshot_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			file_count INTEGER NOT NULL DEFAULT 0,
			total_size INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		// Snapshot files (junction table)
		`CREATE TABLE IF NOT EXISTS snapshot_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			snapshot_id TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_hash TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			blob_refs TEXT NOT NULL DEFAULT '[]',
			FOREIGN KEY (snapshot_id) REFERENCES snapshots(snapshot_id) ON DELETE CASCADE
		)`,

		// Blob metadata table
		`CREATE TABLE IF NOT EXISTS blobs (
			hash TEXT PRIMARY KEY,
			size INTEGER NOT NULL,
			chunk_count INTEGER NOT NULL DEFAULT 1,
			compressed BOOLEAN NOT NULL DEFAULT FALSE,
			encrypted BOOLEAN NOT NULL DEFAULT FALSE,
			ref_count INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		// File metadata / tags
		`CREATE TABLE IF NOT EXISTS file_metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_id INTEGER NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			FOREIGN KEY (file_id) REFERENCES tracked_files(id) ON DELETE CASCADE,
			UNIQUE(file_id, key)
		)`,

		// Sync state table
		`CREATE TABLE IF NOT EXISTS sync_state (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_hash TEXT NOT NULL,
			version_num INTEGER NOT NULL,
			synced_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, file_path)
		)`,

		// Device registry
		`CREATE TABLE IF NOT EXISTS devices (
			device_id TEXT PRIMARY KEY,
			device_name TEXT NOT NULL,
			platform TEXT NOT NULL DEFAULT '',
			last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			public_key TEXT NOT NULL DEFAULT '',
			trusted BOOLEAN NOT NULL DEFAULT FALSE
		)`,

		// Indexes for performance
		`CREATE INDEX IF NOT EXISTS idx_tracked_files_path ON tracked_files(path)`,
		`CREATE INDEX IF NOT EXISTS idx_tracked_files_hash ON tracked_files(hash)`,
		`CREATE INDEX IF NOT EXISTS idx_tracked_files_status ON tracked_files(status)`,
		`CREATE INDEX IF NOT EXISTS idx_file_versions_file_id ON file_versions(file_id)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_files_snapshot ON snapshot_files(snapshot_id)`,
		`CREATE INDEX IF NOT EXISTS idx_blobs_ref_count ON blobs(ref_count)`,
		`CREATE INDEX IF NOT EXISTS idx_file_metadata_file ON file_metadata(file_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_state_device ON sync_state(device_id)`,
	}

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, migration := range migrations {
		if _, err := tx.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, migration)
		}
	}

	return tx.Commit()
}

// TrackedFile represents a file being tracked by UNITEos.
type TrackedFile struct {
	ID           int64     `json:"id"`
	Path         string    `json:"path"`
	RelativePath string    `json:"relative_path"`
	Hash         string    `json:"hash"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	IsDir        bool      `json:"is_dir"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// FileVersion represents a version of a tracked file.
type FileVersion struct {
	ID         int64     `json:"id"`
	FileID     int64     `json:"file_id"`
	VersionNum int       `json:"version_num"`
	Hash       string    `json:"hash"`
	Size       int64     `json:"size"`
	BlobRefs   string    `json:"blob_refs"`
	ChangeType string    `json:"change_type"`
	CreatedAt  time.Time `json:"created_at"`
}

// Snapshot represents a point-in-time snapshot of tracked files.
type Snapshot struct {
	ID          int64     `json:"id"`
	SnapshotID  string    `json:"snapshot_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	FileCount   int       `json:"file_count"`
	TotalSize   int64     `json:"total_size"`
	CreatedAt   time.Time `json:"created_at"`
}

// SnapshotFile represents a file within a snapshot.
type SnapshotFile struct {
	ID         int64  `json:"id"`
	SnapshotID string `json:"snapshot_id"`
	FilePath   string `json:"file_path"`
	FileHash   string `json:"file_hash"`
	FileSize   int64  `json:"file_size"`
	BlobRefs   string `json:"blob_refs"`
}

// AddTrackedFile adds or updates a tracked file record.
func (d *DB) AddTrackedFile(file *TrackedFile) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO tracked_files (path, relative_path, hash, size, mod_time, is_dir, status, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(path) DO UPDATE SET
			hash = excluded.hash,
			size = excluded.size,
			mod_time = excluded.mod_time,
			status = excluded.status,
			updated_at = CURRENT_TIMESTAMP
	`, file.Path, file.RelativePath, file.Hash, file.Size, file.ModTime, file.IsDir, file.Status)
	if err != nil {
		return 0, fmt.Errorf("failed to add tracked file: %w", err)
	}
	return result.LastInsertId()
}

// GetTrackedFile retrieves a tracked file by its path.
func (d *DB) GetTrackedFile(path string) (*TrackedFile, error) {
	var file TrackedFile
	err := d.db.QueryRow(`
		SELECT id, path, relative_path, hash, size, mod_time, is_dir, status, created_at, updated_at
		FROM tracked_files WHERE path = ?
	`, path).Scan(&file.ID, &file.Path, &file.RelativePath, &file.Hash, &file.Size,
		&file.ModTime, &file.IsDir, &file.Status, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get tracked file: %w", err)
	}
	return &file, nil
}

// ListTrackedFiles returns all active tracked files.
func (d *DB) ListTrackedFiles() ([]TrackedFile, error) {
	rows, err := d.db.Query(`
		SELECT id, path, relative_path, hash, size, mod_time, is_dir, status, created_at, updated_at
		FROM tracked_files WHERE status = 'active'
		ORDER BY relative_path
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list tracked files: %w", err)
	}
	defer rows.Close()

	var files []TrackedFile
	for rows.Next() {
		var f TrackedFile
		if err := rows.Scan(&f.ID, &f.Path, &f.RelativePath, &f.Hash, &f.Size,
			&f.ModTime, &f.IsDir, &f.Status, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tracked file: %w", err)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// RemoveTrackedFile marks a file as removed (soft delete).
func (d *DB) RemoveTrackedFile(path string) error {
	_, err := d.db.Exec(`
		UPDATE tracked_files SET status = 'removed', updated_at = CURRENT_TIMESTAMP
		WHERE path = ?
	`, path)
	return err
}

// AddFileVersion adds a new version record for a file.
func (d *DB) AddFileVersion(version *FileVersion) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO file_versions (file_id, version_num, hash, size, blob_refs, change_type)
		VALUES (?, ?, ?, ?, ?, ?)
	`, version.FileID, version.VersionNum, version.Hash, version.Size, version.BlobRefs, version.ChangeType)
	if err != nil {
		return 0, fmt.Errorf("failed to add file version: %w", err)
	}
	return result.LastInsertId()
}

// GetFileVersions retrieves all versions of a file.
func (d *DB) GetFileVersions(fileID int64) ([]FileVersion, error) {
	rows, err := d.db.Query(`
		SELECT id, file_id, version_num, hash, size, blob_refs, change_type, created_at
		FROM file_versions WHERE file_id = ? ORDER BY version_num DESC
	`, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file versions: %w", err)
	}
	defer rows.Close()

	var versions []FileVersion
	for rows.Next() {
		var v FileVersion
		if err := rows.Scan(&v.ID, &v.FileID, &v.VersionNum, &v.Hash, &v.Size,
			&v.BlobRefs, &v.ChangeType, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan file version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// GetLatestVersionNum returns the latest version number for a file.
func (d *DB) GetLatestVersionNum(fileID int64) (int, error) {
	var versionNum sql.NullInt64
	err := d.db.QueryRow(`
		SELECT MAX(version_num) FROM file_versions WHERE file_id = ?
	`, fileID).Scan(&versionNum)
	if err != nil {
		return 0, err
	}
	if !versionNum.Valid {
		return 0, nil
	}
	return int(versionNum.Int64), nil
}

// AddSnapshot creates a new snapshot record.
func (d *DB) AddSnapshot(snapshot *Snapshot) error {
	_, err := d.db.Exec(`
		INSERT INTO snapshots (snapshot_id, name, description, file_count, total_size)
		VALUES (?, ?, ?, ?, ?)
	`, snapshot.SnapshotID, snapshot.Name, snapshot.Description, snapshot.FileCount, snapshot.TotalSize)
	return err
}

// AddSnapshotFile adds a file to a snapshot.
func (d *DB) AddSnapshotFile(sf *SnapshotFile) error {
	_, err := d.db.Exec(`
		INSERT INTO snapshot_files (snapshot_id, file_path, file_hash, file_size, blob_refs)
		VALUES (?, ?, ?, ?, ?)
	`, sf.SnapshotID, sf.FilePath, sf.FileHash, sf.FileSize, sf.BlobRefs)
	return err
}

// ListSnapshots returns all snapshots ordered by creation time.
func (d *DB) ListSnapshots() ([]Snapshot, error) {
	rows, err := d.db.Query(`
		SELECT id, snapshot_id, name, description, file_count, total_size, created_at
		FROM snapshots ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []Snapshot
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.ID, &s.SnapshotID, &s.Name, &s.Description,
			&s.FileCount, &s.TotalSize, &s.CreatedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, rows.Err()
}

// GetSnapshot retrieves a snapshot by its ID.
func (d *DB) GetSnapshot(snapshotID string) (*Snapshot, error) {
	var s Snapshot
	err := d.db.QueryRow(`
		SELECT id, snapshot_id, name, description, file_count, total_size, created_at
		FROM snapshots WHERE snapshot_id = ?
	`, snapshotID).Scan(&s.ID, &s.SnapshotID, &s.Name, &s.Description,
		&s.FileCount, &s.TotalSize, &s.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// GetSnapshotFiles returns all files in a snapshot.
func (d *DB) GetSnapshotFiles(snapshotID string) ([]SnapshotFile, error) {
	rows, err := d.db.Query(`
		SELECT id, snapshot_id, file_path, file_hash, file_size, blob_refs
		FROM snapshot_files WHERE snapshot_id = ?
	`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []SnapshotFile
	for rows.Next() {
		var f SnapshotFile
		if err := rows.Scan(&f.ID, &f.SnapshotID, &f.FilePath, &f.FileHash,
			&f.FileSize, &f.BlobRefs); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// SearchFiles searches tracked files by name pattern.
func (d *DB) SearchFiles(query string) ([]TrackedFile, error) {
	rows, err := d.db.Query(`
		SELECT id, path, relative_path, hash, size, mod_time, is_dir, status, created_at, updated_at
		FROM tracked_files 
		WHERE status = 'active' AND (relative_path LIKE ? OR path LIKE ?)
		ORDER BY relative_path
	`, "%"+query+"%", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []TrackedFile
	for rows.Next() {
		var f TrackedFile
		if err := rows.Scan(&f.ID, &f.Path, &f.RelativePath, &f.Hash, &f.Size,
			&f.ModTime, &f.IsDir, &f.Status, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// SetFileMetadata sets a metadata key-value pair for a file.
func (d *DB) SetFileMetadata(fileID int64, key, value string) error {
	_, err := d.db.Exec(`
		INSERT INTO file_metadata (file_id, key, value)
		VALUES (?, ?, ?)
		ON CONFLICT(file_id, key) DO UPDATE SET value = excluded.value
	`, fileID, key, value)
	return err
}

// GetFileMetadata retrieves all metadata for a file.
func (d *DB) GetFileMetadata(fileID int64) (map[string]string, error) {
	rows, err := d.db.Query(`
		SELECT key, value FROM file_metadata WHERE file_id = ?
	`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	meta := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		meta[key] = value
	}
	return meta, rows.Err()
}

// GetStats returns database statistics.
func (d *DB) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// File counts
	var fileCount, dirCount, removedCount int64
	d.db.QueryRow(`SELECT COUNT(*) FROM tracked_files WHERE status = 'active' AND is_dir = FALSE`).Scan(&fileCount)
	d.db.QueryRow(`SELECT COUNT(*) FROM tracked_files WHERE status = 'active' AND is_dir = TRUE`).Scan(&dirCount)
	d.db.QueryRow(`SELECT COUNT(*) FROM tracked_files WHERE status = 'removed'`).Scan(&removedCount)

	// Total sizes
	var totalSize sql.NullInt64
	d.db.QueryRow(`SELECT SUM(size) FROM tracked_files WHERE status = 'active'`).Scan(&totalSize)

	// Version count
	var versionCount int64
	d.db.QueryRow(`SELECT COUNT(*) FROM file_versions`).Scan(&versionCount)

	// Snapshot count
	var snapshotCount int64
	d.db.QueryRow(`SELECT COUNT(*) FROM snapshots`).Scan(&snapshotCount)

	// Blob count
	var blobCount int64
	d.db.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&blobCount)

	stats["tracked_files"] = fileCount
	stats["tracked_dirs"] = dirCount
	stats["removed_files"] = removedCount
	stats["total_size_bytes"] = totalSize.Int64
	stats["versions"] = versionCount
	stats["snapshots"] = snapshotCount
	stats["blobs"] = blobCount

	return stats, nil
}
