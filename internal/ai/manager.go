package ai

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ayushgpal/uniteos/internal/core"
	"github.com/ayushgpal/uniteos/internal/storage"
)

// Manager is the central AI orchestrator for uniteOS.
// It coordinates embeddings, semantic search, smart tagging,
// auto-organization, and context-aware suggestions.
type Manager struct {
	engine      *EmbeddingEngine
	vectorStore *VectorStore
	organizer   *Organizer
	store       *storage.Store
	eventBus    *core.EventBus
	logger      *slog.Logger
	dataDir     string
	mu          sync.Mutex
	indexedAt   time.Time
}

// NewManager creates a new AI manager.
func NewManager(store *storage.Store, eventBus *core.EventBus, dataDir string, logger *slog.Logger) *Manager {
	engine := NewEmbeddingEngine()
	vectorPath := filepath.Join(dataDir, "vectors.json")
	vs := NewVectorStore(engine, vectorPath, logger)

	return &Manager{
		engine:      engine,
		vectorStore: vs,
		organizer:   NewOrganizer(vs),
		store:       store,
		eventBus:    eventBus,
		logger:      logger,
		dataDir:     dataDir,
	}
}

// Start initializes the AI system and subscribes to events.
func (m *Manager) Start() error {
	// Load persisted vectors
	if err := m.vectorStore.Load(); err != nil {
		m.logger.Warn("could not load vector store", "error", err)
	}

	// Subscribe to file events for real-time indexing
	m.eventBus.Subscribe(core.EventFileCreated, func(e core.Event) {
		if path, ok := e.Data["path"].(string); ok {
			m.IndexFile(path)
		}
	})
	m.eventBus.Subscribe(core.EventFileModified, func(e core.Event) {
		if path, ok := e.Data["path"].(string); ok {
			m.IndexFile(path)
		}
	})
	m.eventBus.Subscribe(core.EventFileDeleted, func(e core.Event) {
		if path, ok := e.Data["path"].(string); ok {
			m.vectorStore.Delete(path)
		}
	})

	m.logger.Info("AI manager started", "vectors", m.vectorStore.Count())
	return nil
}

// Stop persists state and shuts down.
func (m *Manager) Stop() {
	m.vectorStore.Save()
	m.logger.Info("AI manager stopped")
}

// IndexAll indexes all tracked files for semantic search.
func (m *Manager) IndexAll() (*IndexReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	start := time.Now()
	files, err := m.store.DB.ListTrackedFiles()
	if err != nil {
		return nil, err
	}

	// Collect text content for TF-IDF fitting
	var documents []string
	for _, f := range files {
		if f.IsDir {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Path))
		docContent := f.RelativePath + " " + filepath.Base(f.Path)
		if isTextFile(ext) {
			data, err := os.ReadFile(f.Path)
			if err == nil {
				docContent += " " + string(data)
			}
		}
		documents = append(documents, docContent)
	}

	// Fit TF-IDF on all documents
	if len(documents) > 0 {
		m.engine.FitDocuments(documents)
	}

	// Index each file
	report := &IndexReport{StartedAt: start}
	for i, f := range files {
		if f.IsDir {
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Path))
		var content string

		if isTextFile(ext) {
			data, _ := os.ReadFile(f.Path)
			content = string(data)
		}

		// Analyze file
		analysis, err := AnalyzeFile(f.Path)
		if err != nil {
			report.Errors++
			continue
		}

		// Build document text for embedding
		docText := content + " " + f.RelativePath + " " +
			strings.Join(analysis.Tags, " ") + " " +
			strings.Join(analysis.Keywords, " ")

		// Store in vector DB
		metadata := map[string]string{
			"category": analysis.Category,
			"size":     fmt.Sprintf("%d", f.Size),
		}
		if analysis.Language != "" {
			metadata["language"] = analysis.Language
		}

		m.vectorStore.Upsert(f.RelativePath, docText, analysis.Tags, metadata)
		report.Indexed++

		if (i+1)%100 == 0 {
			m.logger.Debug("indexing progress", "indexed", report.Indexed, "total", len(files))
		}
	}

	// Persist
	m.vectorStore.Save()
	m.indexedAt = time.Now()

	report.Duration = time.Since(start).String()
	report.VocabularySize = m.engine.VocabularySize()
	report.TotalVectors = m.vectorStore.Count()

	m.logger.Info("AI indexing complete",
		"indexed", report.Indexed,
		"errors", report.Errors,
		"vocabulary", report.VocabularySize,
		"duration", report.Duration,
	)

	m.eventBus.Publish(core.NewEvent("ai.index.complete", "ai", map[string]interface{}{
		"indexed":    report.Indexed,
		"vocabulary": report.VocabularySize,
		"duration":   report.Duration,
	}))

	return report, nil
}

// IndexFile indexes a single file (for real-time updates).
func (m *Manager) IndexFile(path string) {
	ext := strings.ToLower(filepath.Ext(path))
	var content string

	if isTextFile(ext) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		content = string(data)
	}

	analysis, err := AnalyzeFile(path)
	if err != nil {
		return
	}

	relPath := filepath.Base(path)
	docText := content + " " + relPath + " " +
		strings.Join(analysis.Tags, " ") + " " +
		strings.Join(analysis.Keywords, " ")

	metadata := map[string]string{"category": analysis.Category}
	if analysis.Language != "" {
		metadata["language"] = analysis.Language
	}

	m.vectorStore.Upsert(relPath, docText, analysis.Tags, metadata)
}

// SemanticSearch performs a natural-language search across indexed files.
func (m *Manager) SemanticSearch(query string, topK int) []SearchResult {
	if topK <= 0 {
		topK = 10
	}
	results := m.vectorStore.Search(query, topK)

	m.eventBus.Publish(core.NewEvent("ai.search", "ai", map[string]interface{}{
		"query":   query,
		"results": len(results),
	}))

	return results
}

// SearchByTags finds files matching specific tags.
func (m *Manager) SearchByTags(tags []string, topK int) []VectorEntry {
	return m.vectorStore.SearchByTag(tags, topK)
}

// AnalyzeWorkspace performs a full AI analysis of the workspace.
func (m *Manager) AnalyzeWorkspace() (*WorkspaceAnalysis, error) {
	files, err := m.store.DB.ListTrackedFiles()
	if err != nil {
		return nil, err
	}

	wa := &WorkspaceAnalysis{
		AnalyzedAt:  time.Now(),
		Categories:  make(map[string]int),
		Languages:   make(map[string]int),
		TagCloud:    make(map[string]int),
	}

	var analyses []*FileAnalysis
	for _, f := range files {
		if f.IsDir {
			continue
		}

		analysis, err := AnalyzeFile(f.Path)
		if err != nil {
			continue
		}
		analyses = append(analyses, analysis)
		wa.TotalFiles++
		wa.Categories[analysis.Category]++
		if analysis.Language != "" {
			wa.Languages[analysis.Language]++
		}
		for _, tag := range analysis.Tags {
			wa.TagCloud[tag]++
		}
	}

	// Organization suggestions
	wa.Organization = m.organizer.SuggestOrganization(analyses)

	// Duplicate detection
	wa.Duplicates = m.organizer.FindDuplicateContent(0.85)

	// Generate insights
	wa.Insights = m.generateInsights(wa)

	return wa, nil
}

// generateInsights creates human-readable observations about the workspace.
func (m *Manager) generateInsights(wa *WorkspaceAnalysis) []string {
	var insights []string

	// Largest category
	maxCat, maxCount := "", 0
	for cat, count := range wa.Categories {
		if count > maxCount {
			maxCat = cat
			maxCount = count
		}
	}
	if maxCat != "" {
		insights = append(insights, fmt.Sprintf("Your workspace is primarily composed of %s files (%d files, %.0f%%)",
			maxCat, maxCount, float64(maxCount)/float64(wa.TotalFiles)*100))
	}

	// Language diversity
	if len(wa.Languages) > 1 {
		var langs []string
		for lang := range wa.Languages {
			langs = append(langs, lang)
		}
		insights = append(insights, fmt.Sprintf("Multi-language project: %s", strings.Join(langs, ", ")))
	} else if len(wa.Languages) == 1 {
		for lang := range wa.Languages {
			insights = append(insights, fmt.Sprintf("Single-language project: %s", lang))
		}
	}

	// Duplicates
	if len(wa.Duplicates) > 0 {
		totalDups := 0
		for _, g := range wa.Duplicates {
			totalDups += len(g)
		}
		insights = append(insights, fmt.Sprintf("Found %d groups of similar files (%d files total) — consider deduplication",
			len(wa.Duplicates), totalDups))
	}

	// Organization suggestions
	if wa.Organization != nil && len(wa.Organization.Suggestions) > 0 {
		insights = append(insights, fmt.Sprintf("%d files could be better organized",
			len(wa.Organization.Suggestions)))
	}

	return insights
}

// GetStats returns AI system statistics.
func (m *Manager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"indexed_files":   m.vectorStore.Count(),
		"vocabulary_size": m.engine.VocabularySize(),
		"last_indexed":    m.indexedAt.Format(time.RFC3339),
		"status":          "active",
	}
}

// ─── Report Types ─────────────────────────────────────────────

// IndexReport contains results from an indexing operation.
type IndexReport struct {
	Indexed        int       `json:"indexed"`
	Errors         int       `json:"errors"`
	VocabularySize int       `json:"vocabulary_size"`
	TotalVectors   int       `json:"total_vectors"`
	Duration       string    `json:"duration"`
	StartedAt      time.Time `json:"started_at"`
}

// WorkspaceAnalysis is a comprehensive AI analysis of the workspace.
type WorkspaceAnalysis struct {
	TotalFiles   int                `json:"total_files"`
	Categories   map[string]int     `json:"categories"`
	Languages    map[string]int     `json:"languages"`
	TagCloud     map[string]int     `json:"tag_cloud"`
	Organization *OrganizationPlan  `json:"organization"`
	Duplicates   [][]string         `json:"duplicates"`
	Insights     []string           `json:"insights"`
	AnalyzedAt   time.Time          `json:"analyzed_at"`
}
