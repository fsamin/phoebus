package config

import (
	"fmt"

	"github.com/ovh/configstore"
	"gopkg.in/yaml.v2"
)

type Config struct {
	HTTP     HTTPConfig     `yaml:"http"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Admin    AdminConfig    `yaml:"admin"`
	Auth     AuthConfig     `yaml:"auth"`
}

type HTTPConfig struct {
	Port int `yaml:"port"`
}

type DatabaseConfig struct {
	URL string `yaml:"url"`
}

type JWTConfig struct {
	Secret string `yaml:"secret"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type AuthConfig struct {
	LocalEnabled bool `yaml:"local_enabled"`
}

func Load() (*Config, error) {
	configstore.InitFromEnvironment()

	cfg := &Config{
		HTTP:     HTTPConfig{Port: 8080},
		Admin:    AdminConfig{Username: "admin", Password: "admin"},
		Auth:     AuthConfig{LocalEnabled: true},
	}

	if err := unmarshalItem("http", &cfg.HTTP); err != nil {
		// optional, keep defaults
	}

	if err := unmarshalItem("database", &cfg.Database); err != nil {
		return nil, fmt.Errorf("missing required config item 'database': %w", err)
	}
	if cfg.Database.URL == "" {
		return nil, fmt.Errorf("database.url is required")
	}

	if err := unmarshalItem("jwt", &cfg.JWT); err != nil {
		return nil, fmt.Errorf("missing required config item 'jwt': %w", err)
	}
	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("jwt.secret is required")
	}

	if err := unmarshalItem("admin", &cfg.Admin); err != nil {
		// optional, keep defaults
	}

	if err := unmarshalItem("auth", &cfg.Auth); err != nil {
		// optional, keep defaults
	}

	return cfg, nil
}

func unmarshalItem(key string, dest interface{}) error {
	v, err := configstore.GetItemValue(key)
	if err != nil {
		return err
	}
	return yaml.Unmarshal([]byte(v), dest)
}
