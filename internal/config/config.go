package config

import (
	"fmt"

	"github.com/ovh/configstore"
)

type Config struct {
	Port          int
	DatabaseURL   string
	JWTSecret     string
	AdminUsername  string
	AdminPassword string
	LocalAuth     bool
}

func Load() (*Config, error) {
	configstore.InitFromEnvironment()

	cfg := &Config{
		Port:          8080,
		LocalAuth:     true,
		AdminUsername:  "admin",
		AdminPassword: "admin",
	}

	if v, err := configstore.GetItemValue("port"); err == nil {
		fmt.Sscanf(v, "%d", &cfg.Port)
	}

	v, err := configstore.GetItemValue("database-url")
	if err != nil {
		return nil, fmt.Errorf("missing required config: database_url: %w", err)
	}
	cfg.DatabaseURL = v

	v, err = configstore.GetItemValue("jwt-secret")
	if err != nil {
		return nil, fmt.Errorf("missing required config: jwt_secret: %w", err)
	}
	cfg.JWTSecret = v

	if v, err := configstore.GetItemValue("admin-username"); err == nil {
		cfg.AdminUsername = v
	}
	if v, err := configstore.GetItemValue("admin-password"); err == nil {
		cfg.AdminPassword = v
	}
	if v, err := configstore.GetItemValue("local-auth-enabled"); err == nil {
		cfg.LocalAuth = v != "false"
	}

	return cfg, nil
}
