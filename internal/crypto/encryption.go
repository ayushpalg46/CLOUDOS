// Package crypto provides encryption, key management, and device identity for CloudOS.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

const (
	// KeySize is the AES-256 key size in bytes.
	KeySize = 32
	// NonceSize is the GCM nonce size.
	NonceSize = 12
	// SaltSize is the salt size for key derivation.
	SaltSize = 32
)

// DeriveKey derives an AES-256 key from a passphrase using Argon2id.
func DeriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, 3, 64*1024, 4, KeySize)
}

// GenerateSalt generates a cryptographically secure random salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return salt, nil
}

// GenerateKey generates a random AES-256 key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return key, nil
}

// EncryptData encrypts data using AES-256-GCM.
func EncryptData(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptData decrypts data encrypted with AES-256-GCM.
func DecryptData(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// EncryptFile encrypts a file in-place or to a destination path.
func EncryptFile(srcPath, destPath string, key []byte) error {
	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	ciphertext, err := EncryptData(plaintext, key)
	if err != nil {
		return err
	}

	if destPath == "" {
		destPath = srcPath + ".enc"
	}

	return os.WriteFile(destPath, ciphertext, 0600)
}

// DecryptFile decrypts a file.
func DecryptFile(srcPath, destPath string, key []byte) error {
	ciphertext, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	plaintext, err := DecryptData(ciphertext, key)
	if err != nil {
		return err
	}

	if destPath == "" {
		destPath = srcPath // Overwrite
	}

	return os.WriteFile(destPath, plaintext, 0600)
}

// EncryptFileStream encrypts a file using streaming for large files.
func EncryptFileStream(srcPath, destPath string, key []byte) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	if destPath == "" {
		destPath = srcPath + ".enc"
	}
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return err
	}
	dst.Write(iv) // Prepend IV

	stream := cipher.NewCTR(block, iv)
	writer := &cipher.StreamWriter{S: stream, W: dst}
	if _, err := io.Copy(writer, src); err != nil {
		return err
	}

	return nil
}

// HashData returns the SHA-256 hash of data.
func HashData(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
