package sshkey

import (
	"strings"
	"testing"
)

func TestGenerateEd25519(t *testing.T) {
	privPEM, pubSSH, err := GenerateEd25519()
	if err != nil {
		t.Fatalf("GenerateEd25519: %v", err)
	}

	if !strings.Contains(string(privPEM), "OPENSSH PRIVATE KEY") {
		t.Error("private key should be in OpenSSH PEM format")
	}

	if !strings.HasPrefix(pubSSH, "ssh-ed25519 ") {
		t.Error("public key should start with ssh-ed25519")
	}

	// Generate a second keypair — should be different
	privPEM2, pubSSH2, err := GenerateEd25519()
	if err != nil {
		t.Fatalf("GenerateEd25519 (2nd): %v", err)
	}
	if string(privPEM) == string(privPEM2) {
		t.Error("two generated keypairs should not be identical")
	}
	if pubSSH == pubSSH2 {
		t.Error("two generated public keys should not be identical")
	}
}
