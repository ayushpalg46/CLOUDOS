package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// BlobStore manages chunk-based file storage with content-addressable hashing.
type BlobStore struct {
	basePath  string
	chunkSize int64
	logger    *slog.Logger
}

// BlobInfo contains metadata about a stored blob.
type BlobInfo struct {
	Hash        string   `json:"hash"`
	Size        int64    `json:"size"`
	ChunkCount  int      `json:"chunk_count"`
	ChunkHashes []string `json:"chunk_hashes"`
}

// NewBlobStore creates a new blob store at the specified path.
func NewBlobStore(basePath string, chunkSize int64, logger *slog.Logger) (*BlobStore, error) {
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create blob store: %w", err)
	}
	for i := 0; i < 256; i++ {
		dir := filepath.Join(basePath, fmt.Sprintf("%02x", i))
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	if chunkSize <= 0 {
		chunkSize = 4 * 1024 * 1024
	}
	return &BlobStore{basePath: basePath, chunkSize: chunkSize, logger: logger}, nil
}

// StoreFile stores a file as content-addressable chunks.
func (bs *BlobStore) StoreFile(filePath string) (*BlobInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	fullHasher := sha256.New()
	if _, err := io.Copy(fullHasher, f); err != nil {
		return nil, err
	}
	fullHash := hex.EncodeToString(fullHasher.Sum(nil))
	f.Seek(0, 0)

	var chunkHashes []string
	buf := make([]byte, bs.chunkSize)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			h := sha256.New()
			h.Write(chunk)
			chunkHash := hex.EncodeToString(h.Sum(nil))
			chunkPath := bs.chunkPath(chunkHash)
			if _, serr := os.Stat(chunkPath); os.IsNotExist(serr) {
				if werr := os.WriteFile(chunkPath, chunk, 0600); werr != nil {
					return nil, werr
				}
			}
			chunkHashes = append(chunkHashes, chunkHash)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return &BlobInfo{
		Hash: fullHash, Size: stat.Size(),
		ChunkCount: len(chunkHashes), ChunkHashes: chunkHashes,
	}, nil
}

// RestoreFile reconstructs a file from its chunks.
func (bs *BlobStore) RestoreFile(info *BlobInfo, destPath string) error {
	os.MkdirAll(filepath.Dir(destPath), 0755)
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, hash := range info.ChunkHashes {
		data, err := os.ReadFile(bs.chunkPath(hash))
		if err != nil {
			return fmt.Errorf("read chunk %s: %w", hash[:12], err)
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
	}
	return nil
}

// HasBlob checks if a blob exists in the store.
func (bs *BlobStore) HasBlob(hash string) bool {
	_, err := os.Stat(bs.chunkPath(hash))
	return err == nil
}

func (bs *BlobStore) chunkPath(hash string) string {
	if len(hash) < 2 {
		return filepath.Join(bs.basePath, "00", hash)
	}
	return filepath.Join(bs.basePath, hash[:2], hash)
}

// HashFile computes the SHA-256 hash of a file.
func HashFile(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// GetStoreSize returns total size of all blobs.
func (bs *BlobStore) GetStoreSize() (int64, error) {
	var total int64
	filepath.Walk(bs.basePath, func(_ string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, nil
}
