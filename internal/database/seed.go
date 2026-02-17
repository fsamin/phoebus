package database

import (
	"log/slog"

	"github.com/fsamin/phoebus/internal/config"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// SeedAdmin ensures a default admin user exists for bootstrap.
func SeedAdmin(db *sqlx.DB, cfg *config.Config) error {
	if !cfg.Auth.LocalEnabled {
		return nil
	}

	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", cfg.Admin.Username).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Admin.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT INTO users (username, display_name, role, password_hash, auth_provider)
		VALUES ($1, $2, 'admin', $3, 'local')
	`, cfg.Admin.Username, cfg.Admin.Username, string(hash))
	if err != nil {
		return err
	}

	slog.Info("seeded admin user", "username", cfg.Admin.Username)
	return nil
}
