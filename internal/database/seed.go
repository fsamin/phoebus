package database

import (
	"database/sql"
	"log/slog"

	"github.com/fsamin/phoebus/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// SeedAdmin ensures a default admin user exists for bootstrap.
func SeedAdmin(db *sql.DB, cfg *config.Config) error {
	if !cfg.LocalAuth {
		return nil
	}

	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", cfg.AdminUsername).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT INTO users (username, display_name, role, password_hash, auth_provider)
		VALUES ($1, $2, 'admin', $3, 'local')
	`, cfg.AdminUsername, cfg.AdminUsername, string(hash))
	if err != nil {
		return err
	}

	slog.Info("seeded admin user", "username", cfg.AdminUsername)
	return nil
}
