// Package ai provides local-first AI features for uniteOS.
// All AI operations work fully offline using TF-IDF embeddings,
// cosine similarity search, and rule-based content analysis.
package ai

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// ─── TF-IDF Embedding Engine ──────────────────────────────────

// EmbeddingEngine computes TF-IDF vector embeddings for text documents.
// Operates entirely locally — no external API or model dependencies.
type EmbeddingEngine struct {
	vocabulary map[string]int // term -> index in vector
	idf        map[string]float64
	vocabList  []string
	docCount   int
	docFreq    map[string]int // how many documents contain each term
}

// NewEmbeddingEngine creates a new TF-IDF embedding engine.
func NewEmbeddingEngine() *EmbeddingEngine {
	return &EmbeddingEngine{
		vocabulary: make(map[string]int),
		idf:        make(map[string]float64),
		docFreq:    make(map[string]int),
	}
}

// Tokenize splits text into normalized tokens.
func Tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			current.WriteRune(r)
		} else {
			if current.Len() > 1 { // Skip single-char tokens
				tokens = append(tokens, current.String())
			}
			current.Reset()
		}
	}
	if current.Len() > 1 {
		tokens = append(tokens, current.String())
	}

	return filterStopWords(tokens)
}

// Common English stop words to filter out.
var stopWords = map[string]bool{
	"the": true, "is": true, "at": true, "which": true, "on": true,
	"a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "with": true, "to": true, "for": true, "of": true,
	"not": true, "no": true, "it": true, "be": true, "as": true,
	"do": true, "if": true, "by": true, "from": true, "up": true,
	"so": true, "we": true, "he": true, "she": true, "this": true,
	"that": true, "are": true, "was": true, "were": true, "been": true,
	"have": true, "has": true, "had": true, "will": true, "would": true,
	"can": true, "could": true, "may": true, "might": true, "shall": true,
	"should": true, "must": true, "am": true, "its": true, "my": true,
	"your": true, "our": true, "their": true, "them": true, "they": true,
	"you": true, "me": true, "him": true, "her": true, "us": true,
}

func filterStopWords(tokens []string) []string {
	var filtered []string
	for _, t := range tokens {
		if !stopWords[t] && len(t) > 1 {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// termFrequency computes normalized term frequencies for a document.
func termFrequency(tokens []string) map[string]float64 {
	tf := make(map[string]float64)
	for _, t := range tokens {
		tf[t]++
	}
	// Normalize by max frequency
	maxFreq := 0.0
	for _, v := range tf {
		if v > maxFreq {
			maxFreq = v
		}
	}
	if maxFreq > 0 {
		for k := range tf {
			tf[k] = 0.5 + 0.5*(tf[k]/maxFreq) // augmented TF
		}
	}
	return tf
}

// FitDocuments builds the vocabulary and IDF values from a corpus.
func (e *EmbeddingEngine) FitDocuments(documents []string) {
	e.docCount = len(documents)
	e.docFreq = make(map[string]int)
	allTerms := make(map[string]bool)

	for _, doc := range documents {
		tokens := Tokenize(doc)
		seen := make(map[string]bool)
		for _, t := range tokens {
			allTerms[t] = true
			if !seen[t] {
				e.docFreq[t]++
				seen[t] = true
			}
		}
	}

	// Build sorted vocabulary
	e.vocabList = make([]string, 0, len(allTerms))
	for term := range allTerms {
		e.vocabList = append(e.vocabList, term)
	}
	sort.Strings(e.vocabList)

	e.vocabulary = make(map[string]int, len(e.vocabList))
	for i, term := range e.vocabList {
		e.vocabulary[term] = i
	}

	// Compute IDF
	e.idf = make(map[string]float64, len(e.vocabList))
	for _, term := range e.vocabList {
		df := float64(e.docFreq[term])
		e.idf[term] = math.Log(1 + float64(e.docCount)/(1+df))
	}
}

// Embed computes the TF-IDF embedding vector for a text document.
func (e *EmbeddingEngine) Embed(text string) []float64 {
	tokens := Tokenize(text)
	tf := termFrequency(tokens)

	vec := make([]float64, len(e.vocabList))
	for term, tfVal := range tf {
		if idx, ok := e.vocabulary[term]; ok {
			idfVal := e.idf[term]
			vec[idx] = tfVal * idfVal
		}
	}

	// L2 normalize
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec
}

// EmbedSparse returns a sparse TF-IDF vector (only non-zero entries).
func (e *EmbeddingEngine) EmbedSparse(text string) map[int]float64 {
	tokens := Tokenize(text)
	tf := termFrequency(tokens)

	sparse := make(map[int]float64)
	norm := 0.0

	for term, tfVal := range tf {
		if idx, ok := e.vocabulary[term]; ok {
			val := tfVal * e.idf[term]
			sparse[idx] = val
			norm += val * val
		}
	}

	norm = math.Sqrt(norm)
	if norm > 0 {
		for k := range sparse {
			sparse[k] /= norm
		}
	}

	return sparse
}

// VocabularySize returns the number of terms in the vocabulary.
func (e *EmbeddingEngine) VocabularySize() int {
	return len(e.vocabList)
}

// ─── Similarity Functions ─────────────────────────────────────

// CosineSimilarity computes cosine similarity between two dense vectors.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// CosineSimilaritySparse computes cosine similarity between two sparse vectors.
func CosineSimilaritySparse(a, b map[int]float64) float64 {
	var dot float64
	for k, va := range a {
		if vb, ok := b[k]; ok {
			dot += va * vb
		}
	}
	// Vectors are already L2-normalized, so dot product = cosine similarity
	return dot
}
