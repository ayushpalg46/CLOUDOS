package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DeviceIdentity represents this device's identity in the CloudOS network.
type DeviceIdentity struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Platform   string    `json:"platform"`
	MasterKey  string    `json:"master_key"` // Encrypted master key
	Salt       string    `json:"salt"`
	CreatedAt  time.Time `json:"created_at"`
}

// KeyManager handles encryption key lifecycle.
type KeyManager struct {
	identity *DeviceIdentity
	keyPath  string
	masterKey []byte // In-memory only, never persisted in plaintext
}

// NewKeyManager creates or loads a key manager.
func NewKeyManager(dataDir string) (*KeyManager, error) {
	keyPath := filepath.Join(dataDir, "identity.json")
	km := &KeyManager{keyPath: keyPath}

	if _, err := os.Stat(keyPath); err == nil {
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read identity: %w", err)
		}
		var identity DeviceIdentity
		if err := json.Unmarshal(data, &identity); err != nil {
			return nil, fmt.Errorf("parse identity: %w", err)
		}
		km.identity = &identity
	}

	return km, nil
}

// Initialize creates a new device identity with a passphrase.
func (km *KeyManager) Initialize(deviceName, platform, passphrase string) error {
	// Generate device ID
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return err
	}

	// Generate master key
	masterKey, err := GenerateKey()
	if err != nil {
		return err
	}

	// Generate salt and derive encryption key from passphrase
	salt, err := GenerateSalt()
	if err != nil {
		return err
	}

	derivedKey := DeriveKey(passphrase, salt)

	// Encrypt master key with derived key
	encMasterKey, err := EncryptData(masterKey, derivedKey)
	if err != nil {
		return err
	}

	km.identity = &DeviceIdentity{
		DeviceID:   hex.EncodeToString(idBytes),
		DeviceName: deviceName,
		Platform:   platform,
		MasterKey:  hex.EncodeToString(encMasterKey),
		Salt:       hex.EncodeToString(salt),
		CreatedAt:  time.Now(),
	}
	km.masterKey = masterKey

	return km.save()
}

// Unlock decrypts the master key using the passphrase.
func (km *KeyManager) Unlock(passphrase string) error {
	if km.identity == nil {
		return fmt.Errorf("no identity found — run init first")
	}

	salt, err := hex.DecodeString(km.identity.Salt)
	if err != nil {
		return fmt.Errorf("decode salt: %w", err)
	}

	derivedKey := DeriveKey(passphrase, salt)

	encMasterKey, err := hex.DecodeString(km.identity.MasterKey)
	if err != nil {
		return fmt.Errorf("decode master key: %w", err)
	}

	masterKey, err := DecryptData(encMasterKey, derivedKey)
	if err != nil {
		return fmt.Errorf("incorrect passphrase or corrupted key")
	}

	km.masterKey = masterKey
	return nil
}

// GetMasterKey returns the master key (must be unlocked first).
func (km *KeyManager) GetMasterKey() ([]byte, error) {
	if km.masterKey == nil {
		return nil, fmt.Errorf("key manager is locked")
	}
	return km.masterKey, nil
}

// IsInitialized checks if the identity has been created.
func (km *KeyManager) IsInitialized() bool {
	return km.identity != nil
}

// IsUnlocked checks if the master key is available.
func (km *KeyManager) IsUnlocked() bool {
	return km.masterKey != nil
}

// GetIdentity returns the device identity.
func (km *KeyManager) GetIdentity() *DeviceIdentity {
	return km.identity
}

func (km *KeyManager) save() error {
	data, err := json.MarshalIndent(km.identity, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(km.keyPath), 0700)
	return os.WriteFile(km.keyPath, data, 0600)
}
