package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func randomKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := randomKey(t)
	original := []byte(`{"title":"hello","description":"world"}`)

	encoded, err := Encrypt(key, original)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	decoded, err := Decrypt(key, encoded)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}

	if !bytes.Equal(original, decoded) {
		t.Fatalf("round-trip mismatch: got %q, want %q", decoded, original)
	}
}

func TestDecryptFailsWithWrongKey(t *testing.T) {
	key := randomKey(t)
	wrongKey := randomKey(t)

	encoded, err := Encrypt(key, []byte("secret payload"))
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	if _, err = Decrypt(wrongKey, encoded); err == nil {
		t.Fatal("expected decryption error with wrong key, got nil")
	}
}

func TestDecryptFailsWithTamperedCiphertext(t *testing.T) {
	key := randomKey(t)

	encoded, err := Encrypt(key, []byte("secret payload"))
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	// Flip the last base64 character to corrupt the GCM tag
	tampered := []byte(encoded)
	tampered[len(tampered)-1] ^= 0x01
	// Re-encode safely: just append a character change
	if tampered[len(tampered)-1] == encoded[len(encoded)-1] {
		tampered[len(tampered)-1]++
	}

	if _, err = Decrypt(key, string(tampered)); err == nil {
		t.Fatal("expected decryption error with tampered ciphertext, got nil")
	}
}

func TestNonceIsUnique(t *testing.T) {
	key := randomKey(t)
	plaintext := []byte("same plaintext")

	enc1, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt (1) error: %v", err)
	}
	enc2, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt (2) error: %v", err)
	}

	if enc1 == enc2 {
		t.Fatal("two encryptions of the same plaintext produced identical output (nonce reuse)")
	}
}
