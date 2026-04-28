package sync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// DeltaType indicates the kind of delta operation.
type DeltaType string

const (
	DeltaFull   DeltaType = "full"   // Send entire file
	DeltaPatch  DeltaType = "patch"  // Send only changed blocks
	DeltaDelete DeltaType = "delete" // File was deleted
)

// BlockSignature represents a hash signature for a block of data.
type BlockSignature struct {
	Index  int    `json:"index"`
	Offset int64  `json:"offset"`
	Size   int    `json:"size"`
	Hash   string `json:"hash"`
}

// Delta represents the difference between two versions of a file.
type Delta struct {
	FilePath    string           `json:"file_path"`
	Type        DeltaType        `json:"type"`
	OldHash     string           `json:"old_hash"`
	NewHash     string           `json:"new_hash"`
	BlockSize   int              `json:"block_size"`
	OldBlocks   []BlockSignature `json:"old_blocks,omitempty"`
	NewBlocks   []BlockSignature `json:"new_blocks,omitempty"`
	ChangedData []DeltaBlock     `json:"changed_data,omitempty"`
	TotalSize   int64            `json:"total_size"`
}

// DeltaBlock represents a changed block of data.
type DeltaBlock struct {
	Index int    `json:"index"`
	Hash  string `json:"hash"`
	Data  []byte `json:"data"`
}

const defaultBlockSize = 64 * 1024 // 64KB blocks for delta comparison

// ComputeBlockSignatures computes block-level hashes for a file.
func ComputeBlockSignatures(filePath string, blockSize int) ([]BlockSignature, error) {
	if blockSize <= 0 {
		blockSize = defaultBlockSize
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var sigs []BlockSignature
	buf := make([]byte, blockSize)
	index := 0
	offset := int64(0)

	for {
		n, err := f.Read(buf)
		if n > 0 {
			h := sha256.Sum256(buf[:n])
			sigs = append(sigs, BlockSignature{
				Index:  index,
				Offset: offset,
				Size:   n,
				Hash:   hex.EncodeToString(h[:]),
			})
			index++
			offset += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return sigs, nil
}

// ComputeDelta calculates the delta between old and new versions of a file.
func ComputeDelta(oldPath, newPath string) (*Delta, error) {
	newInfo, err := os.Stat(newPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Delta{
				FilePath: oldPath,
				Type:     DeltaDelete,
			}, nil
		}
		return nil, err
	}

	// If old file doesn't exist, send full file
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		newData, err := os.ReadFile(newPath)
		if err != nil {
			return nil, err
		}
		h := sha256.Sum256(newData)
		return &Delta{
			FilePath:  newPath,
			Type:      DeltaFull,
			NewHash:   hex.EncodeToString(h[:]),
			TotalSize: newInfo.Size(),
			ChangedData: []DeltaBlock{{
				Index: 0,
				Hash:  hex.EncodeToString(h[:]),
				Data:  newData,
			}},
		}, nil
	}

	blockSize := defaultBlockSize
	oldSigs, err := ComputeBlockSignatures(oldPath, blockSize)
	if err != nil {
		return nil, err
	}
	newSigs, err := ComputeBlockSignatures(newPath, blockSize)
	if err != nil {
		return nil, err
	}

	// Build lookup of old block hashes
	oldMap := make(map[string]BlockSignature)
	for _, sig := range oldSigs {
		oldMap[sig.Hash] = sig
	}

	// Find changed blocks
	var changedBlocks []DeltaBlock
	newFile, err := os.Open(newPath)
	if err != nil {
		return nil, err
	}
	defer newFile.Close()

	buf := make([]byte, blockSize)
	for _, newSig := range newSigs {
		if _, exists := oldMap[newSig.Hash]; !exists {
			// This block is new or changed — include its data
			newFile.Seek(newSig.Offset, 0)
			n, err := newFile.Read(buf)
			if err != nil && err != io.EOF {
				return nil, err
			}
			changedBlocks = append(changedBlocks, DeltaBlock{
				Index: newSig.Index,
				Hash:  newSig.Hash,
				Data:  append([]byte{}, buf[:n]...),
			})
		}
	}

	// Compute file hashes
	oldHashStr := computeFileHash(oldPath)
	newHashStr := computeFileHash(newPath)

	deltaType := DeltaPatch
	if len(changedBlocks) == len(newSigs) {
		deltaType = DeltaFull // Everything changed, just send full file
	}

	return &Delta{
		FilePath:    newPath,
		Type:        deltaType,
		OldHash:     oldHashStr,
		NewHash:     newHashStr,
		BlockSize:   blockSize,
		OldBlocks:   oldSigs,
		NewBlocks:   newSigs,
		ChangedData: changedBlocks,
		TotalSize:   newInfo.Size(),
	}, nil
}

// ApplyDelta applies a delta to reconstruct the new version of a file.
func ApplyDelta(oldPath string, delta *Delta, destPath string) error {
	if delta.Type == DeltaDelete {
		return os.Remove(destPath)
	}

	if delta.Type == DeltaFull {
		// Full replacement
		if len(delta.ChangedData) > 0 {
			return os.WriteFile(destPath, delta.ChangedData[0].Data, 0644)
		}
		return fmt.Errorf("full delta has no data")
	}

	// Patch: reconstruct from old blocks + changed blocks
	oldFile, err := os.Open(oldPath)
	if err != nil {
		return err
	}
	defer oldFile.Close()

	// Build changed block lookup
	changedMap := make(map[int]DeltaBlock)
	for _, cb := range delta.ChangedData {
		changedMap[cb.Index] = cb
	}

	// Build old block lookup
	oldBlockMap := make(map[string]BlockSignature)
	for _, ob := range delta.OldBlocks {
		oldBlockMap[ob.Hash] = ob
	}

	var result bytes.Buffer
	buf := make([]byte, delta.BlockSize)

	for _, newBlock := range delta.NewBlocks {
		if cb, ok := changedMap[newBlock.Index]; ok {
			// Use changed data
			result.Write(cb.Data)
		} else if oldBlock, ok := oldBlockMap[newBlock.Hash]; ok {
			// Reuse old block
			oldFile.Seek(oldBlock.Offset, 0)
			n, err := oldFile.Read(buf)
			if err != nil && err != io.EOF {
				return err
			}
			result.Write(buf[:n])
		} else {
			return fmt.Errorf("block %d not found in old or changed data", newBlock.Index)
		}
	}

	return os.WriteFile(destPath, result.Bytes(), 0644)
}

// DeltaSize returns the total size of data in the delta (for bandwidth estimation).
func (d *Delta) DeltaSize() int64 {
	var size int64
	for _, cb := range d.ChangedData {
		size += int64(len(cb.Data))
	}
	return size
}

// Savings returns the percentage of bandwidth saved by using delta sync.
func (d *Delta) Savings() float64 {
	if d.TotalSize == 0 {
		return 0
	}
	return (1.0 - float64(d.DeltaSize())/float64(d.TotalSize)) * 100
}

func computeFileHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}
