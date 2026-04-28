package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
)

// VectorEntry represents a stored document with its embedding.
type VectorEntry struct {
	ID       string         `json:"id"`        // File path or document ID
	Content  string         `json:"content"`    // Original text for display
	Tags     []string       `json:"tags"`       // Associated tags
	Sparse   map[int]float64 `json:"sparse"`    // Sparse TF-IDF vector
	Metadata map[string]string `json:"metadata"` // Extra metadata
}

// SearchResult is a scored match from a vector search.
type SearchResult struct {
	Entry      VectorEntry `json:"entry"`
	Score      float64     `json:"score"` // Cosine similarity [0, 1]
}

// VectorStore is an in-memory vector database for semantic search.
type VectorStore struct {
	entries map[string]*VectorEntry
	mu      sync.RWMutex
	engine  *EmbeddingEngine
	logger  *slog.Logger
	path    string // Persistence path
}

// NewVectorStore creates a new vector store.
func NewVectorStore(engine *EmbeddingEngine, persistPath string, logger *slog.Logger) *VectorStore {
	return &VectorStore{
		entries: make(map[string]*VectorEntry),
		engine:  engine,
		logger:  logger,
		path:    persistPath,
	}
}

// Upsert adds or updates a document in the vector store.
func (vs *VectorStore) Upsert(id, content string, tags []string, metadata map[string]string) {
	sparse := vs.engine.EmbedSparse(content)

	vs.mu.Lock()
	vs.entries[id] = &VectorEntry{
		ID:       id,
		Content:  truncate(content, 500),
		Tags:     tags,
		Sparse:   sparse,
		Metadata: metadata,
	}
	vs.mu.Unlock()
}

// Delete removes a document from the store.
func (vs *VectorStore) Delete(id string) {
	vs.mu.Lock()
	delete(vs.entries, id)
	vs.mu.Unlock()
}

// Search finds the top-k most similar documents to a query.
func (vs *VectorStore) Search(query string, topK int) []SearchResult {
	queryVec := vs.engine.EmbedSparse(query)
	if len(queryVec) == 0 {
		return nil
	}

	vs.mu.RLock()
	defer vs.mu.RUnlock()

	var results []SearchResult
	for _, entry := range vs.entries {
		score := CosineSimilaritySparse(queryVec, entry.Sparse)
		if score > 0.001 { // Minimum relevance threshold
			results = append(results, SearchResult{Entry: *entry, Score: score})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results
}

// SearchByTag finds documents that match any of the given tags.
func (vs *VectorStore) SearchByTag(tags []string, topK int) []VectorEntry {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	vs.mu.RLock()
	defer vs.mu.RUnlock()

	var results []VectorEntry
	for _, entry := range vs.entries {
		for _, t := range entry.Tags {
			if tagSet[t] {
				results = append(results, *entry)
				break
			}
		}
	}

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results
}

// Count returns the number of entries.
func (vs *VectorStore) Count() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.entries)
}

// Get retrieves a specific entry by ID.
func (vs *VectorStore) Get(id string) *VectorEntry {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	if e, ok := vs.entries[id]; ok {
		return e
	}
	return nil
}

// Save persists the vector store to disk.
func (vs *VectorStore) Save() error {
	vs.mu.RLock()
	data, err := json.Marshal(vs.entries)
	vs.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal vector store: %w", err)
	}
	return os.WriteFile(vs.path, data, 0600)
}

// Load restores the vector store from disk.
func (vs *VectorStore) Load() error {
	data, err := os.ReadFile(vs.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Fresh start
		}
		return err
	}

	var entries map[string]*VectorEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	vs.mu.Lock()
	vs.entries = entries
	vs.mu.Unlock()

	vs.logger.Info("vector store loaded", "entries", len(entries))
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
