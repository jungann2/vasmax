package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// encPrefix is prepended to encrypted credentials.
	encPrefix = "ENC:"

	// nonceSize is the AES-GCM nonce length in bytes.
	nonceSize = 12
)

// machineIDPath is the path to the machine-id file used for key derivation.
// It can be overridden in tests.
var machineIDPath = "/etc/machine-id"

// testKey allows tests to inject a fixed key without reading machine-id.
// When non-nil it takes precedence over machineIDPath.
var testKey []byte

// SetKeyForTesting sets a fixed 32-byte AES key for use in tests.
// Pass nil to revert to the default machine-id based derivation.
func SetKeyForTesting(key []byte) {
	testKey = key
}

// deriveKey returns a 32-byte AES-256 key.
// If testKey is set it is returned directly; otherwise the key is derived
// by computing SHA-256 of the contents of machineIDPath.
func deriveKey() ([]byte, error) {
	if testKey != nil {
		if len(testKey) != 32 {
			return nil, fmt.Errorf("test key must be exactly 32 bytes, got %d", len(testKey))
		}
		return testKey, nil
	}

	data, err := os.ReadFile(machineIDPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read machine-id from %s: %w", machineIDPath, err)
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// EncryptCredential encrypts plaintext using AES-256-GCM.
// The key is derived from /etc/machine-id (SHA-256).
// Returns "ENC:" + base64(nonce + ciphertext + tag).
func EncryptCredential(plaintext string) (string, error) {
	key, err := deriveKey()
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encryption failed: could not generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag after nonce.
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	encoded := base64.StdEncoding.EncodeToString(sealed)
	return encPrefix + encoded, nil
}

// DecryptCredential decrypts a credential string that has the "ENC:" prefix.
// It strips the prefix, base64-decodes, extracts the 12-byte nonce, and
// decrypts using AES-256-GCM.
func DecryptCredential(encrypted string) (string, error) {
	if !IsEncrypted(encrypted) {
		return "", fmt.Errorf("decryption failed: input does not have %s prefix", encPrefix)
	}

	b64 := strings.TrimPrefix(encrypted, encPrefix)
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("decryption failed: invalid base64: %w", err)
	}

	if len(raw) < nonceSize {
		return "", fmt.Errorf("decryption failed: ciphertext too short")
	}

	key, err := deriveKey()
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	nonce := raw[:nonceSize]
	ciphertext := raw[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// IsEncrypted returns true if s starts with the "ENC:" prefix.
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, encPrefix)
}
