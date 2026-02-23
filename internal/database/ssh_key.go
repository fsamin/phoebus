package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fsamin/phoebus/internal/crypto"
	"github.com/fsamin/phoebus/internal/sshkey"
	"github.com/jmoiron/sqlx"
)

// SSHKeyPair holds the instance SSH keypair loaded at startup.
type SSHKeyPair struct {
	PrivateKeyPEM []byte
	PublicKey      string
}

// EnsureSSHKey checks if an instance SSH keypair exists in the database.
// If not, it generates one and stores it (private key encrypted).
// Returns the keypair for use by the syncer.
func EnsureSSHKey(db *sqlx.DB, encryptionKey string) (*SSHKeyPair, error) {
	var pubKey string
	err := db.QueryRow("SELECT value FROM instance_settings WHERE key = 'ssh_public_key'").Scan(&pubKey)
	if err == nil {
		// Key exists — load private key
		var encPriv string
		if err := db.QueryRow("SELECT value FROM instance_settings WHERE key = 'ssh_private_key'").Scan(&encPriv); err != nil {
			return nil, fmt.Errorf("load ssh private key: %w", err)
		}
		privPEM, err := crypto.DecryptFromBase64(encPriv, []byte(encryptionKey))
		if err != nil {
			return nil, fmt.Errorf("decrypt ssh private key: %w", err)
		}
		slog.Info("loaded instance SSH keypair from database")
		return &SSHKeyPair{PrivateKeyPEM: privPEM, PublicKey: strings.TrimSpace(pubKey)}, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check ssh key: %w", err)
	}

	// Generate new keypair
	privPEM, pubSSH, err := sshkey.GenerateEd25519()
	if err != nil {
		return nil, fmt.Errorf("generate ssh keypair: %w", err)
	}

	encPriv, err := crypto.EncryptToBase64(privPEM, []byte(encryptionKey))
	if err != nil {
		return nil, fmt.Errorf("encrypt ssh private key: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.Exec(
		"INSERT INTO instance_settings (key, value, encrypted) VALUES ('ssh_private_key', $1, true)",
		encPriv,
	); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("store ssh private key: %w", err)
	}
	if _, err := tx.Exec(
		"INSERT INTO instance_settings (key, value, encrypted) VALUES ('ssh_public_key', $1, false)",
		strings.TrimSpace(pubSSH),
	); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("store ssh public key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit ssh keypair: %w", err)
	}

	slog.Info("generated new instance SSH keypair")
	return &SSHKeyPair{PrivateKeyPEM: privPEM, PublicKey: strings.TrimSpace(pubSSH)}, nil
}
