// Package sync provides the CRDT-based synchronization engine for UNITEos.
// It enables conflict-free multi-device data replication.
package sync

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ─── Vector Clock ──────────────────────────────────────────────

// VectorClock tracks causal ordering of events across devices.
type VectorClock map[string]uint64

// NewVectorClock creates a new empty vector clock.
func NewVectorClock() VectorClock {
	return make(VectorClock)
}

// Increment advances the clock for a given device.
func (vc VectorClock) Increment(deviceID string) {
	vc[deviceID]++
}

// Merge combines two vector clocks, taking the maximum of each entry.
func (vc VectorClock) Merge(other VectorClock) VectorClock {
	merged := make(VectorClock)
	for k, v := range vc {
		merged[k] = v
	}
	for k, v := range other {
		if existing, ok := merged[k]; !ok || v > existing {
			merged[k] = v
		}
	}
	return merged
}

// HappensBefore returns true if this clock happens-before other (strict ordering).
func (vc VectorClock) HappensBefore(other VectorClock) bool {
	atLeastOneLess := false
	for k, v := range vc {
		otherV := other[k]
		if v > otherV {
			return false
		}
		if v < otherV {
			atLeastOneLess = true
		}
	}
	// Check keys in other that aren't in vc
	for k, v := range other {
		if _, ok := vc[k]; !ok && v > 0 {
			atLeastOneLess = true
		}
	}
	return atLeastOneLess
}

// IsConcurrent returns true if neither clock happens-before the other.
func (vc VectorClock) IsConcurrent(other VectorClock) bool {
	return !vc.HappensBefore(other) && !other.HappensBefore(vc) && !vc.Equal(other)
}

// Equal returns true if both clocks are identical.
func (vc VectorClock) Equal(other VectorClock) bool {
	if len(vc) != len(other) {
		return false
	}
	for k, v := range vc {
		if other[k] != v {
			return false
		}
	}
	return true
}

// Clone creates a deep copy of the vector clock.
func (vc VectorClock) Clone() VectorClock {
	c := make(VectorClock, len(vc))
	for k, v := range vc {
		c[k] = v
	}
	return c
}

// ─── LWW Register (Last-Writer-Wins) ──────────────────────────

// LWWRegister is a Last-Writer-Wins register CRDT for file metadata.
type LWWRegister struct {
	Value     interface{} `json:"value"`
	Timestamp time.Time   `json:"timestamp"`
	DeviceID  string      `json:"device_id"`
}

// NewLWWRegister creates a new LWW register.
func NewLWWRegister(value interface{}, deviceID string) *LWWRegister {
	return &LWWRegister{
		Value:     value,
		Timestamp: time.Now(),
		DeviceID:  deviceID,
	}
}

// Merge merges two LWW registers, keeping the latest value.
func (r *LWWRegister) Merge(other *LWWRegister) *LWWRegister {
	if other.Timestamp.After(r.Timestamp) {
		return other
	}
	if other.Timestamp.Equal(r.Timestamp) && other.DeviceID > r.DeviceID {
		return other // Deterministic tie-breaking by device ID
	}
	return r
}

// ─── G-Counter (Grow-Only Counter) ─────────────────────────────

// GCounter is a grow-only counter CRDT, useful for tracking version numbers.
type GCounter struct {
	Counts map[string]uint64 `json:"counts"`
	mu     sync.RWMutex
}

// NewGCounter creates a new grow-only counter.
func NewGCounter() *GCounter {
	return &GCounter{Counts: make(map[string]uint64)}
}

// Increment increases the counter for a device.
func (gc *GCounter) Increment(deviceID string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.Counts[deviceID]++
}

// Value returns the total count across all devices.
func (gc *GCounter) Value() uint64 {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	var total uint64
	for _, v := range gc.Counts {
		total += v
	}
	return total
}

// Merge combines two G-Counters.
func (gc *GCounter) Merge(other *GCounter) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	for k, v := range other.Counts {
		if v > gc.Counts[k] {
			gc.Counts[k] = v
		}
	}
}

// ─── OR-Set (Observed-Remove Set) ──────────────────────────────

// ORSetElement represents an element in an OR-Set with a unique tag.
type ORSetElement struct {
	Value    string `json:"value"`
	UniqueID string `json:"unique_id"` // DeviceID + counter
	Added    bool   `json:"added"`
}

// ORSet is an Observed-Remove Set CRDT for tracking file paths.
type ORSet struct {
	Elements map[string][]ORSetElement `json:"elements"` // value -> list of tagged entries
	mu       sync.RWMutex
}

// NewORSet creates a new OR-Set.
func NewORSet() *ORSet {
	return &ORSet{Elements: make(map[string][]ORSetElement)}
}

// Add inserts a value with a unique tag.
func (s *ORSet) Add(value, uniqueID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Elements[value] = append(s.Elements[value], ORSetElement{
		Value: value, UniqueID: uniqueID, Added: true,
	})
}

// Remove marks all instances of a value as removed.
func (s *ORSet) Remove(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if elems, ok := s.Elements[value]; ok {
		for i := range elems {
			elems[i].Added = false
		}
	}
}

// Contains checks if a value is in the set (has at least one active tag).
func (s *ORSet) Contains(value string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, elem := range s.Elements[value] {
		if elem.Added {
			return true
		}
	}
	return false
}

// Values returns all active values in the set.
func (s *ORSet) Values() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []string
	for value, elems := range s.Elements {
		for _, elem := range elems {
			if elem.Added {
				result = append(result, value)
				break
			}
		}
	}
	return result
}

// Merge combines two OR-Sets.
func (s *ORSet) Merge(other *ORSet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	seen := make(map[string]bool)
	for value, otherElems := range other.Elements {
		existingElems := s.Elements[value]
		for _, oe := range otherElems {
			key := oe.UniqueID
			if !seen[key] {
				found := false
				for _, ee := range existingElems {
					if ee.UniqueID == oe.UniqueID {
						found = true
						break
					}
				}
				if !found {
					s.Elements[value] = append(s.Elements[value], oe)
				}
				seen[key] = true
			}
		}
	}
}

// ─── File State CRDT ──────────────────────────────────────────

// FileState represents the CRDT state for a single file across devices.
type FileState struct {
	Path         string      `json:"path"`
	Hash         LWWRegister `json:"hash"`
	Size         LWWRegister `json:"size"`
	ModTime      LWWRegister `json:"mod_time"`
	Deleted      LWWRegister `json:"deleted"`
	VectorClock  VectorClock `json:"vector_clock"`
	VersionCount *GCounter   `json:"version_count"`
}

// NewFileState creates a new file state CRDT for a given path and device.
func NewFileState(path, hash string, size int64, modTime time.Time, deviceID string) *FileState {
	vc := NewVectorClock()
	vc.Increment(deviceID)

	return &FileState{
		Path:         path,
		Hash:         *NewLWWRegister(hash, deviceID),
		Size:         *NewLWWRegister(size, deviceID),
		ModTime:      *NewLWWRegister(modTime.Unix(), deviceID),
		Deleted:      *NewLWWRegister(false, deviceID),
		VectorClock:  vc,
		VersionCount: NewGCounter(),
	}
}

// MergeFileState merges two file states, returning the merged result and whether a conflict occurred.
func MergeFileState(local, remote *FileState) (*FileState, bool) {
	if local.Path != remote.Path {
		return local, false
	}

	isConflict := local.VectorClock.IsConcurrent(remote.VectorClock)

	merged := &FileState{
		Path:         local.Path,
		Hash:         *local.Hash.Merge(&remote.Hash),
		Size:         *local.Size.Merge(&remote.Size),
		ModTime:      *local.ModTime.Merge(&remote.ModTime),
		Deleted:      *local.Deleted.Merge(&remote.Deleted),
		VectorClock:  local.VectorClock.Merge(remote.VectorClock),
		VersionCount: NewGCounter(),
	}

	merged.VersionCount.Merge(local.VersionCount)
	merged.VersionCount.Merge(remote.VersionCount)

	return merged, isConflict
}

// Serialize converts file state to JSON bytes.
func (fs *FileState) Serialize() ([]byte, error) {
	return json.Marshal(fs)
}

// DeserializeFileState parses JSON bytes into a FileState.
func DeserializeFileState(data []byte) (*FileState, error) {
	var fs FileState
	if err := json.Unmarshal(data, &fs); err != nil {
		return nil, fmt.Errorf("deserialize file state: %w", err)
	}
	return &fs, nil
}
