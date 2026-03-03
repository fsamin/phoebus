package sshkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// GenerateEd25519 generates an Ed25519 SSH keypair.
// Returns the PEM-encoded private key and the OpenSSH-formatted public key.
func GenerateEd25519() (privateKeyPEM []byte, publicKeySSH string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", fmt.Errorf("generate ed25519 key: %w", err)
	}

	// Marshal private key to OpenSSH PEM
	privBlock, err := ssh.MarshalPrivateKey(priv, "phoebus-instance")
	if err != nil {
		return nil, "", fmt.Errorf("marshal private key: %w", err)
	}
	privateKeyPEM = pem.EncodeToMemory(privBlock)

	// Marshal public key to OpenSSH authorized_keys format
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, "", fmt.Errorf("marshal public key: %w", err)
	}
	publicKeySSH = string(ssh.MarshalAuthorizedKey(sshPub))

	return privateKeyPEM, publicKeySSH, nil
}
