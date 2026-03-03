package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("hello phoebus")
	ct, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	pt, err := Decrypt(ct, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(pt, plaintext) {
		t.Fatalf("got %q, want %q", pt, plaintext)
	}
}

func TestEncryptInvalidKeyLength(t *testing.T) {
	_, err := Encrypt([]byte("data"), []byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestDecryptInvalidKeyLength(t *testing.T) {
	_, err := Decrypt([]byte("data"), []byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestDecryptCorruptedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	ct, _ := Encrypt([]byte("secret"), key)
	// Corrupt the ciphertext
	ct[len(ct)-1] ^= 0xff

	_, err := Decrypt(ct, key)
	if err == nil {
		t.Fatal("expected error for corrupted ciphertext")
	}
}

func TestDecryptTooShort(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	_, err := Decrypt([]byte("x"), key)
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func TestEncryptToBase64DecryptFromBase64Roundtrip(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	plaintext := []byte("base64 roundtrip test")
	encoded, err := EncryptToBase64(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptToBase64: %v", err)
	}

	// Verify it's valid base64
	if _, err := base64.StdEncoding.DecodeString(encoded); err != nil {
		t.Fatalf("not valid base64: %v", err)
	}

	pt, err := DecryptFromBase64(encoded, key)
	if err != nil {
		t.Fatalf("DecryptFromBase64: %v", err)
	}

	if !bytes.Equal(pt, plaintext) {
		t.Fatalf("got %q, want %q", pt, plaintext)
	}
}

func TestDecryptFromBase64InvalidBase64(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	_, err := DecryptFromBase64("not-valid-base64!@#$", key)
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}
