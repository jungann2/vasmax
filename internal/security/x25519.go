package security

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// X25519KeyPair holds a Reality X25519 key pair.
type X25519KeyPair struct {
	PrivateKey string // base64 encoded
	PublicKey  string // base64 encoded
}

// GenerateX25519KeyPair generates a new X25519 key pair for Reality protocol.
func GenerateX25519KeyPair() (*X25519KeyPair, error) {
	privateKey := make([]byte, curve25519.ScalarSize)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, fmt.Errorf("生成私钥失败: %w", err)
	}

	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("生成公钥失败: %w", err)
	}

	return &X25519KeyPair{
		PrivateKey: base64.RawURLEncoding.EncodeToString(privateKey),
		PublicKey:  base64.RawURLEncoding.EncodeToString(publicKey),
	}, nil
}

// GenerateShortID generates a random 8-byte hex short ID for Reality.
func GenerateShortID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成 ShortID 失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}
