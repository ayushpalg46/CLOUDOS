package ai

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ─── Auto-Organization Engine ─────────────────────────────────

// OrganizeSuggestion recommends a folder structure for a file.
type OrganizeSuggestion struct {
	FilePath        string   `json:"file_path"`
	CurrentDir      string   `json:"current_dir"`
	SuggestedDir    string   `json:"suggested_dir"`
	Reason          string   `json:"reason"`
	Confidence      float64  `json:"confidence"`
	RelatedFiles    []string `json:"related_files,omitempty"`
	AlternativeDirs []string `json:"alternative_dirs,omitempty"`
}

// ClusterGroup represents a group of related files.
type ClusterGroup struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Files    []string `json:"files"`
	Tags     []string `json:"tags"`
	Size     int      `json:"size"`
}

// OrganizationPlan is a full workspace reorganization plan.
type OrganizationPlan struct {
	Suggestions []OrganizeSuggestion `json:"suggestions"`
	Clusters    []ClusterGroup       `json:"clusters"`
	Stats       map[string]int       `json:"stats"`
}

// Organizer provides intelligent file organization suggestions.
type Organizer struct {
	vectorStore *VectorStore
}

// NewOrganizer creates a new auto-organizer.
func NewOrganizer(vs *VectorStore) *Organizer {
	return &Organizer{vectorStore: vs}
}

// SuggestOrganization analyzes files and suggests a folder structure.
func (o *Organizer) SuggestOrganization(analyses []*FileAnalysis) *OrganizationPlan {
	plan := &OrganizationPlan{
		Stats: make(map[string]int),
	}

	// Group files by category
	categoryFiles := make(map[string][]*FileAnalysis)
	for _, a := range analyses {
		categoryFiles[a.Category] = append(categoryFiles[a.Category], a)
		plan.Stats[a.Category]++
	}

	// Generate clusters
	for cat, files := range categoryFiles {
		cluster := ClusterGroup{
			Name:     formatClusterName(cat),
			Category: cat,
			Size:     len(files),
		}
		tagSet := make(map[string]bool)
		for _, f := range files {
			cluster.Files = append(cluster.Files, f.Path)
			for _, t := range f.Tags {
				tagSet[t] = true
			}
		}
		for t := range tagSet {
			cluster.Tags = append(cluster.Tags, t)
		}
		sort.Strings(cluster.Tags)
		plan.Clusters = append(plan.Clusters, cluster)
	}

	// Sort clusters by size
	sort.Slice(plan.Clusters, func(i, j int) bool {
		return plan.Clusters[i].Size > plan.Clusters[j].Size
	})

	// Generate per-file suggestions
	for _, a := range analyses {
		suggestion := o.suggestForFile(a, analyses)
		if suggestion != nil {
			plan.Suggestions = append(plan.Suggestions, *suggestion)
		}
	}

	return plan
}

func (o *Organizer) suggestForFile(analysis *FileAnalysis, allFiles []*FileAnalysis) *OrganizeSuggestion {
	currentDir := filepath.Dir(analysis.Path)
	suggestedDir := o.computeSuggestedDir(analysis)

	// Only suggest if the file would benefit from moving
	if filepath.ToSlash(currentDir) == filepath.ToSlash(suggestedDir) {
		return nil
	}

	// Find related files (same category or overlapping tags)
	var related []string
	for _, other := range allFiles {
		if other.Path == analysis.Path {
			continue
		}
		if other.Category == analysis.Category || hasOverlap(analysis.Tags, other.Tags) {
			related = append(related, other.Path)
		}
	}
	if len(related) > 5 {
		related = related[:5]
	}

	return &OrganizeSuggestion{
		FilePath:     analysis.Path,
		CurrentDir:   currentDir,
		SuggestedDir: suggestedDir,
		Reason:       fmt.Sprintf("File categorized as '%s'; suggested directory groups similar files", analysis.Category),
		Confidence:   analysis.Confidence * 0.8,
		RelatedFiles: related,
		AlternativeDirs: []string{
			filepath.Join(suggestedDir, analysis.Language),
		},
	}
}

func (o *Organizer) computeSuggestedDir(analysis *FileAnalysis) string {
	// Standard directory conventions
	dirMap := map[string]string{
		"code":     "src",
		"script":   "scripts",
		"web":      "web",
		"document": "docs",
		"data":     "data",
		"config":   "config",
		"image":    "assets/images",
		"video":    "assets/videos",
		"audio":    "assets/audio",
		"archive":  "archives",
		"binary":   "bin",
		"database": "db",
		"log":      "logs",
		"lock":     ".",
		"test":     "tests",
	}

	// Check for test files
	baseName := strings.ToLower(filepath.Base(analysis.Path))
	if strings.Contains(baseName, "test") || strings.HasSuffix(baseName, "_test.go") {
		return "tests"
	}

	if dir, ok := dirMap[analysis.Category]; ok {
		// Sub-organize by language for code
		if analysis.Category == "code" && analysis.Language != "" {
			return filepath.Join(dir, strings.ToLower(analysis.Language))
		}
		return dir
	}

	return "misc"
}

// FindDuplicateContent identifies files with similar content.
func (o *Organizer) FindDuplicateContent(threshold float64) [][]string {
	if o.vectorStore == nil {
		return nil
	}

	entries := make([]*VectorEntry, 0)
	// Collect all entries
	o.vectorStore.mu.RLock()
	for _, e := range o.vectorStore.entries {
		entries = append(entries, e)
	}
	o.vectorStore.mu.RUnlock()

	var groups [][]string
	used := make(map[string]bool)

	for i, a := range entries {
		if used[a.ID] {
			continue
		}
		var group []string
		group = append(group, a.ID)

		for j := i + 1; j < len(entries); j++ {
			b := entries[j]
			if used[b.ID] {
				continue
			}
			sim := CosineSimilaritySparse(a.Sparse, b.Sparse)
			if sim >= threshold {
				group = append(group, b.ID)
				used[b.ID] = true
			}
		}

		if len(group) > 1 {
			groups = append(groups, group)
			used[a.ID] = true
		}
	}

	return groups
}

func formatClusterName(category string) string {
	names := map[string]string{
		"code":     "Source Code",
		"document": "Documents",
		"data":     "Data Files",
		"config":   "Configuration",
		"image":    "Images",
		"video":    "Videos",
		"audio":    "Audio",
		"archive":  "Archives",
		"binary":   "Binaries",
		"web":      "Web Assets",
		"script":   "Scripts",
		"database": "Databases",
		"log":      "Logs",
		"lock":     "Lock Files",
	}
	if name, ok := names[category]; ok {
		return name
	}
	return strings.Title(category)
}

func hasOverlap(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		if set[v] {
			return true
		}
	}
	return false
}
