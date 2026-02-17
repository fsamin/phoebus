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
	LocalEnabled bool       `yaml:"local_enabled"`
	OIDC         OIDCConfig `yaml:"oidc"`
	LDAP         LDAPConfig `yaml:"ldap"`
}

type OIDCConfig struct {
	Enabled      bool              `yaml:"enabled"`
	IssuerURL    string            `yaml:"issuer_url"`
	ClientID     string            `yaml:"client_id"`
	ClientSecret string            `yaml:"client_secret"`
	RedirectURL  string            `yaml:"redirect_url"`
	Scopes       []string          `yaml:"scopes"`
	ClaimMapping OIDCClaimMapping  `yaml:"claim_mapping"`
}

type OIDCClaimMapping struct {
	DisplayName string `yaml:"display_name"`
	Email       string `yaml:"email"`
	ExternalID  string `yaml:"external_id"`
}

type LDAPConfig struct {
	Enabled          bool              `yaml:"enabled"`
	ServerURL        string            `yaml:"server_url"`
	BaseDN           string            `yaml:"base_dn"`
	UserSearchFilter string            `yaml:"user_search_filter"`
	BindDN           string            `yaml:"bind_dn"`
	BindPassword     string            `yaml:"bind_password"`
	AttributeMapping LDAPAttrMapping   `yaml:"attribute_mapping"`
	GroupToRole       map[string]string `yaml:"group_to_role"`
	GroupSearchBase   string            `yaml:"group_search_base"`
	GroupSearchFilter string            `yaml:"group_search_filter"`
}

type LDAPAttrMapping struct {
	DisplayName string `yaml:"display_name"`
	Email       string `yaml:"email"`
}

func Load() (*Config, error) {
	configstore.InitFromEnvironment()

	cfg := &Config{
		HTTP:     HTTPConfig{Port: 8080},
		Admin:    AdminConfig{Username: "admin", Password: "admin"},
		Auth:     AuthConfig{
			LocalEnabled: true,
			OIDC: OIDCConfig{
				Scopes: []string{"openid", "email", "profile"},
				ClaimMapping: OIDCClaimMapping{
					DisplayName: "name",
					Email:       "email",
					ExternalID:  "sub",
				},
			},
			LDAP: LDAPConfig{
				UserSearchFilter: "(uid={username})",
				AttributeMapping: LDAPAttrMapping{
					DisplayName: "displayName",
					Email:       "mail",
				},
			},
		},
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
