package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ─── Content Analyzer ─────────────────────────────────────────

// FileAnalysis contains the AI-extracted metadata for a file.
type FileAnalysis struct {
	Path       string            `json:"path"`
	Category   string            `json:"category"`
	Tags       []string          `json:"tags"`
	Language   string            `json:"language,omitempty"`
	Summary    string            `json:"summary"`
	Keywords   []string          `json:"keywords"`
	Confidence float64           `json:"confidence"`
	Metadata   map[string]string `json:"metadata"`
}

// CategoryMap maps file extensions to categories.
var CategoryMap = map[string]string{
	// Documents
	".txt": "document", ".md": "document", ".doc": "document", ".docx": "document",
	".pdf": "document", ".rtf": "document", ".tex": "document", ".odt": "document",
	".csv": "data", ".tsv": "data", ".json": "data", ".xml": "data",
	".yaml": "config", ".yml": "config", ".toml": "config", ".ini": "config",
	".conf": "config", ".cfg": "config", ".env": "config",
	// Code
	".go": "code", ".py": "code", ".js": "code", ".ts": "code", ".jsx": "code",
	".tsx": "code", ".java": "code", ".c": "code", ".cpp": "code", ".h": "code",
	".rs": "code", ".rb": "code", ".php": "code", ".swift": "code", ".kt": "code",
	".cs": "code", ".r": "code", ".scala": "code", ".lua": "code", ".dart": "code",
	".sh": "script", ".bash": "script", ".ps1": "script", ".bat": "script",
	".cmd": "script", ".zsh": "script",
	".html": "web", ".css": "web", ".scss": "web", ".less": "web", ".svg": "web",
	".sql": "database",
	// Media
	".jpg": "image", ".jpeg": "image", ".png": "image", ".gif": "image",
	".bmp": "image", ".webp": "image", ".ico": "image", ".tiff": "image",
	".mp4": "video", ".avi": "video", ".mkv": "video", ".mov": "video",
	".webm": "video", ".flv": "video",
	".mp3": "audio", ".wav": "audio", ".flac": "audio", ".ogg": "audio",
	".aac": "audio", ".m4a": "audio",
	// Archives
	".zip": "archive", ".tar": "archive", ".gz": "archive", ".rar": "archive",
	".7z": "archive", ".bz2": "archive", ".xz": "archive",
	// Executables
	".exe": "binary", ".dll": "binary", ".so": "binary", ".dylib": "binary",
	// Other
	".log": "log", ".lock": "lock",
	".mod": "config", ".sum": "config",
}

// LanguageMap maps extensions to programming languages.
var LanguageMap = map[string]string{
	".go": "Go", ".py": "Python", ".js": "JavaScript", ".ts": "TypeScript",
	".java": "Java", ".c": "C", ".cpp": "C++", ".rs": "Rust", ".rb": "Ruby",
	".php": "PHP", ".swift": "Swift", ".kt": "Kotlin", ".cs": "C#",
	".r": "R", ".scala": "Scala", ".lua": "Lua", ".dart": "Dart",
	".sh": "Shell", ".ps1": "PowerShell", ".sql": "SQL",
	".html": "HTML", ".css": "CSS", ".jsx": "React", ".tsx": "React",
}

// AnalyzeFile performs AI analysis on a file.
func AnalyzeFile(filePath string) (*FileAnalysis, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	baseName := filepath.Base(filePath)
	relDir := filepath.Dir(filePath)

	analysis := &FileAnalysis{
		Path:       filePath,
		Confidence: 0.8,
		Metadata:   make(map[string]string),
	}

	// Determine category
	if cat, ok := CategoryMap[ext]; ok {
		analysis.Category = cat
	} else {
		analysis.Category = "other"
		analysis.Confidence = 0.5
	}

	// Determine language
	if lang, ok := LanguageMap[ext]; ok {
		analysis.Language = lang
	}

	// Generate tags from path and extension
	analysis.Tags = generateFileTags(baseName, ext, relDir, analysis.Category)

	// Extract keywords and summary from text files
	if isTextFile(ext) && info.Size() < 5*1024*1024 { // < 5MB
		content, err := os.ReadFile(filePath)
		if err == nil {
			text := string(content)
			analysis.Keywords = extractKeywords(text, 15)
			analysis.Summary = generateSummary(text, baseName)
		}
	} else {
		analysis.Summary = fmt.Sprintf("%s file (%s)", analysis.Category, formatFileSize(info.Size()))
		analysis.Keywords = []string{analysis.Category, ext}
	}

	// Metadata
	analysis.Metadata["extension"] = ext
	analysis.Metadata["size"] = fmt.Sprintf("%d", info.Size())
	analysis.Metadata["category"] = analysis.Category
	if analysis.Language != "" {
		analysis.Metadata["language"] = analysis.Language
	}

	return analysis, nil
}

// generateFileTags creates tags from file properties.
func generateFileTags(name, ext, dir string, category string) []string {
	tags := []string{category}

	if ext != "" {
		tags = append(tags, strings.TrimPrefix(ext, "."))
	}

	// Add tags from directory path
	parts := strings.Split(filepath.ToSlash(dir), "/")
	for _, p := range parts {
		p = strings.ToLower(p)
		if p != "" && p != "." && p != ".." && len(p) > 1 {
			tags = append(tags, p)
		}
	}

	// Add tags from filename patterns
	lowerName := strings.ToLower(name)
	if strings.Contains(lowerName, "test") {
		tags = append(tags, "test")
	}
	if strings.Contains(lowerName, "readme") {
		tags = append(tags, "documentation")
	}
	if strings.Contains(lowerName, "config") || strings.Contains(lowerName, "setting") {
		tags = append(tags, "configuration")
	}
	if strings.Contains(lowerName, "main") || strings.Contains(lowerName, "index") {
		tags = append(tags, "entry-point")
	}
	if strings.HasPrefix(lowerName, ".") {
		tags = append(tags, "hidden")
	}

	return uniqueStrings(tags)
}

// extractKeywords returns the top-N most important words.
func extractKeywords(text string, topN int) []string {
	tokens := Tokenize(text)
	freq := make(map[string]int)
	for _, t := range tokens {
		freq[t]++
	}

	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range freq {
		sorted = append(sorted, kv{k, v})
	}

	// Sort by frequency descending
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Value > sorted[i].Value {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var keywords []string
	for i, kv := range sorted {
		if i >= topN {
			break
		}
		keywords = append(keywords, kv.Key)
	}
	return keywords
}

// generateSummary creates a brief summary of text content.
func generateSummary(text, filename string) string {
	lines := strings.Split(text, "\n")

	// Try to find a title (first heading or non-empty line)
	var title string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			title = strings.TrimPrefix(line, "# ")
			break
		}
		if strings.HasPrefix(line, "// ") || strings.HasPrefix(line, "/* ") {
			title = strings.TrimPrefix(strings.TrimPrefix(line, "// "), "/* ")
			break
		}
		if len(line) > 0 && len(line) < 200 {
			title = line
			break
		}
	}

	if title == "" {
		title = filename
	}

	// Count stats
	wordCount := len(Tokenize(text))
	lineCount := len(lines)

	return fmt.Sprintf("%s — %d words, %d lines", truncate(title, 80), wordCount, lineCount)
}

// isTextFile checks if a file extension is likely text-based.
func isTextFile(ext string) bool {
	textExts := map[string]bool{
		".txt": true, ".md": true, ".go": true, ".py": true, ".js": true,
		".ts": true, ".java": true, ".c": true, ".cpp": true, ".h": true,
		".rs": true, ".rb": true, ".php": true, ".html": true, ".css": true,
		".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
		".sql": true, ".sh": true, ".bat": true, ".ps1": true, ".csv": true,
		".tsx": true, ".jsx": true, ".swift": true, ".kt": true, ".cs": true,
		".r": true, ".scala": true, ".lua": true, ".dart": true,
		".ini": true, ".conf": true, ".cfg": true, ".env": true,
		".log": true, ".tex": true, ".rtf": true, ".svg": true,
		".mod": true, ".sum": true, ".lock": true, ".scss": true, ".less": true,
	}
	return textExts[ext]
}

func formatFileSize(bytes int64) string {
	const u = 1024
	if bytes < u {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(u), 0
	for n := bytes / u; n >= u; n /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]bool, len(s))
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
