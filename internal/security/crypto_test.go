package security

import (
	"crypto/sha256"
	"testing"
)

// testSetup sets a deterministic key for all crypto tests and returns a cleanup function.
func testSetup(t *testing.T) {
	t.Helper()
	key := sha256.Sum256([]byte("test-machine-id-for-unit-tests"))
	SetKeyForTesting(key[:])
	t.Cleanup(func() { SetKeyForTesting(nil) })
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	testSetup(t)

	cases := []string{
		"my-secret-token",
		"",
		"a",
		"hello world 你好世界",
		"ENC:not-really-encrypted",
		"special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?",
	}

	for _, plaintext := range cases {
		encrypted, err := EncryptCredential(plaintext)
		if err != nil {
			t.Fatalf("EncryptCredential(%q) error: %v", plaintext, err)
		}

		if !IsEncrypted(encrypted) {
			t.Errorf("EncryptCredential(%q) result %q does not have ENC: prefix", plaintext, encrypted)
		}

		decrypted, err := DecryptCredential(encrypted)
		if err != nil {
			t.Fatalf("DecryptCredential(%q) error: %v", encrypted, err)
		}

		if decrypted != plaintext {
			t.Errorf("round-trip failed: got %q, want %q", decrypted, plaintext)
		}
	}
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ENC:abc123", true},
		{"ENC:", true},
		{"enc:abc", false},
		{"plaintext", false},
		{"", false},
		{"ENCRYPTED:abc", false},
	}

	for _, tt := range tests {
		got := IsEncrypted(tt.input)
		if got != tt.want {
			t.Errorf("IsEncrypted(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDecryptInvalidInput(t *testing.T) {
	testSetup(t)

	tests := []struct {
		name  string
		input string
	}{
		{"no prefix", "not-encrypted"},
		{"invalid base64", "ENC:!!!invalid-base64!!!"},
		{"too short ciphertext", "ENC:AQID"},
		{"corrupted ciphertext", "ENC:AQIDBAUG" + "BwgJCgsMDQ4PEBESExQVFhcYGRobHB0="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptCredential(tt.input)
			if err == nil {
				t.Errorf("DecryptCredential(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestDifferentPlaintextsProduceDifferentCiphertexts(t *testing.T) {
	testSetup(t)

	enc1, err := EncryptCredential("secret-one")
	if err != nil {
		t.Fatal(err)
	}

	enc2, err := EncryptCredential("secret-two")
	if err != nil {
		t.Fatal(err)
	}

	if enc1 == enc2 {
		t.Error("different plaintexts produced identical ciphertexts")
	}
}

func TestSamePlaintextProducesDifferentCiphertexts(t *testing.T) {
	testSetup(t)

	plaintext := "same-secret"
	enc1, err := EncryptCredential(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	enc2, err := EncryptCredential(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if enc1 == enc2 {
		t.Error("same plaintext encrypted twice produced identical ciphertexts (nonce should differ)")
	}

	// Both should still decrypt to the same value.
	dec1, _ := DecryptCredential(enc1)
	dec2, _ := DecryptCredential(enc2)
	if dec1 != dec2 || dec1 != plaintext {
		t.Errorf("decrypted values differ: %q vs %q, want %q", dec1, dec2, plaintext)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	testSetup(t)

	encrypted, err := EncryptCredential("my-secret")
	if err != nil {
		t.Fatal(err)
	}

	// Switch to a different key.
	wrongKey := sha256.Sum256([]byte("wrong-machine-id"))
	SetKeyForTesting(wrongKey[:])

	_, err = DecryptCredential(encrypted)
	if err == nil {
		t.Error("DecryptCredential with wrong key should fail")
	}
}
