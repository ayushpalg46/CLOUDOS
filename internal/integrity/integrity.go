// Package integrity provides data integrity verification and secure sharing for uniteOS.
package integrity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ayushgpal/uniteos/internal/storage"
)

// VerificationResult holds the result of an integrity check for a single file.
type VerificationResult struct {
	Path       string `json:"path"`
	Expected   string `json:"expected_hash"`
	Actual     string `json:"actual_hash"`
	Match      bool   `json:"match"`
	Error      string `json:"error,omitempty"`
	Size       int64  `json:"size"`
	VerifiedAt time.Time `json:"verified_at"`
}

// VerificationReport is the full integrity report.
type VerificationReport struct {
	TotalFiles int                  `json:"total_files"`
	Passed     int                  `json:"passed"`
	Failed     int                  `json:"failed"`
	Errors     int                  `json:"errors"`
	Results    []VerificationResult `json:"results"`
	Duration   string               `json:"duration"`
	VerifiedAt time.Time            `json:"verified_at"`
}

// Verifier performs data integrity verification.
type Verifier struct {
	store  *storage.Store
	logger *slog.Logger
}

// NewVerifier creates a new integrity verifier.
func NewVerifier(store *storage.Store, logger *slog.Logger) *Verifier {
	return &Verifier{store: store, logger: logger}
}

// VerifyAll checks the integrity of all tracked files.
func (v *Verifier) VerifyAll() (*VerificationReport, error) {
	start := time.Now()
	files, err := v.store.DB.ListTrackedFiles()
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	report := &VerificationReport{
		VerifiedAt: time.Now(),
	}

	for _, f := range files {
		if f.IsDir {
			continue
		}

		report.TotalFiles++
		result := VerificationResult{
			Path:       f.RelativePath,
			Expected:   f.Hash,
			Size:       f.Size,
			VerifiedAt: time.Now(),
		}

		actualHash, err := storage.HashFile(f.Path)
		if err != nil {
			result.Error = err.Error()
			result.Match = false
			report.Errors++
		} else {
			result.Actual = actualHash
			result.Match = actualHash == f.Hash
			if result.Match {
				report.Passed++
			} else {
				report.Failed++
				v.logger.Warn("integrity mismatch",
					"path", f.RelativePath,
					"expected", f.Hash[:16],
					"actual", actualHash[:16],
				)
			}
		}
		report.Results = append(report.Results, result)
	}

	report.Duration = time.Since(start).String()

	v.logger.Info("integrity verification complete",
		"total", report.TotalFiles,
		"passed", report.Passed,
		"failed", report.Failed,
		"errors", report.Errors,
		"duration", report.Duration,
	)

	return report, nil
}

// ─── Secure Sharing ────────────────────────────────────────────

// ShareToken represents a temporary file sharing token.
type ShareToken struct {
	Token     string    `json:"token"`
	FilePath  string    `json:"file_path"`
	FileHash  string    `json:"file_hash"`
	CreatedBy string    `json:"created_by"` // Device ID
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	MaxUses   int       `json:"max_uses"`
	UsedCount int       `json:"used_count"`
}

// ShareManager handles secure file sharing via temporary tokens.
type ShareManager struct {
	tokens   map[string]*ShareToken
	mu       sync.RWMutex
	deviceID string
	logger   *slog.Logger
}

// NewShareManager creates a new share manager.
func NewShareManager(deviceID string, logger *slog.Logger) *ShareManager {
	return &ShareManager{
		tokens:   make(map[string]*ShareToken),
		deviceID: deviceID,
		logger:   logger,
	}
}

// GenerateToken creates a temporary sharing token for a file.
func (sm *ShareManager) GenerateToken(filePath, fileHash string, duration time.Duration, maxUses int) (*ShareToken, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	if maxUses <= 0 {
		maxUses = 5
	}
	if duration <= 0 {
		duration = 24 * time.Hour
	}

	token := &ShareToken{
		Token:     hex.EncodeToString(tokenBytes),
		FilePath:  filePath,
		FileHash:  fileHash,
		CreatedBy: sm.deviceID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(duration),
		MaxUses:   maxUses,
	}

	sm.mu.Lock()
	sm.tokens[token.Token] = token
	sm.mu.Unlock()

	sm.logger.Info("share token generated",
		"file", filePath,
		"expires", token.ExpiresAt.Format(time.RFC3339),
		"max_uses", maxUses,
	)

	return token, nil
}

// ValidateToken checks if a token is valid and increments usage.
func (sm *ShareManager) ValidateToken(tokenStr string) (*ShareToken, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	token, ok := sm.tokens[tokenStr]
	if !ok {
		return nil, fmt.Errorf("token not found")
	}

	if time.Now().After(token.ExpiresAt) {
		delete(sm.tokens, tokenStr)
		return nil, fmt.Errorf("token expired")
	}

	if token.UsedCount >= token.MaxUses {
		delete(sm.tokens, tokenStr)
		return nil, fmt.Errorf("token usage limit exceeded")
	}

	token.UsedCount++
	return token, nil
}

// RevokeToken invalidates a sharing token.
func (sm *ShareManager) RevokeToken(tokenStr string) {
	sm.mu.Lock()
	delete(sm.tokens, tokenStr)
	sm.mu.Unlock()
}

// ListTokens returns all active tokens.
func (sm *ShareManager) ListTokens() []ShareToken {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var tokens []ShareToken
	now := time.Now()
	for _, t := range sm.tokens {
		if now.Before(t.ExpiresAt) && t.UsedCount < t.MaxUses {
			tokens = append(tokens, *t)
		}
	}
	return tokens
}

// CleanExpired removes expired tokens.
func (sm *ShareManager) CleanExpired() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	count := 0
	now := time.Now()
	for key, t := range sm.tokens {
		if now.After(t.ExpiresAt) || t.UsedCount >= t.MaxUses {
			delete(sm.tokens, key)
			count++
		}
	}
	return count
}
