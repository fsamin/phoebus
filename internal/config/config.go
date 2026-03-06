package config

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/ovh/configstore"
	"sigs.k8s.io/yaml"
)

type Config struct {
	HTTP          HTTPConfig     `json:"http"`
	Database      DatabaseConfig `json:"database"`
	JWT           JWTConfig      `json:"jwt"`
	Admin         AdminConfig    `json:"admin"`
	Auth          AuthConfig     `json:"auth"`
	Assets        AssetsConfig   `json:"assets"`
	Log           LogConfig      `json:"log"`
	EncryptionKey string         `json:"encryption_key"`
}

type LogConfig struct {
	Format          string `json:"format"`            // "json" (default), "text", "gelf"
	Level           string `json:"level"`             // "debug", "info" (default), "warn", "error"
	RequestIDHeader string `json:"request_id_header"` // header name for upstream request ID (e.g. "X-Request-Id")
}

type HTTPConfig struct {
	Port int `json:"port"`
}

type DatabaseConfig struct {
	URL string `json:"url"`
}

type JWTConfig struct {
	Secret string `json:"secret"`
}

type AdminConfig struct {
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	ForcedAdmins []string `json:"forced_admins"`
}

// IsForcedAdmin returns true if the given username is in the forced_admins list.
func (c *Config) IsForcedAdmin(username string) bool {
	for _, u := range c.Admin.ForcedAdmins {
		if u == username {
			return true
		}
	}
	return false
}

type AssetsConfig struct {
	Backend     string                `json:"backend"` // "filesystem" (default) or "s3"
	MaxFileSize int64                 `json:"max_file_size"`
	Filesystem  FilesystemStoreConfig `json:"filesystem"`
	S3          S3StoreConfig         `json:"s3"`
}

type FilesystemStoreConfig struct {
	DataDir string `json:"data_dir"`
}

type S3StoreConfig struct {
	Bucket         string `json:"bucket"`
	Region         string `json:"region"`
	Endpoint       string `json:"endpoint"`
	Prefix         string `json:"prefix"`
	AccessKey      string `json:"access_key"`
	SecretKey      string `json:"secret_key"`
	ForcePathStyle bool   `json:"force_path_style"`
}

type AuthConfig struct {
	LocalEnabled bool            `json:"local_enabled"`
	OIDC         OIDCConfig      `json:"oidc"`
	LDAP         LDAPConfig      `json:"ldap"`
	ProxyAuth    ProxyAuthConfig `json:"proxy_auth"`
}

type ProxyAuthConfig struct {
	Enabled           bool              `json:"enabled"`
	HeaderUser        string            `json:"header_user"`
	HeaderGroups      string            `json:"header_groups"`
	HeaderEmail       string            `json:"header_email"`
	HeaderDisplayName string            `json:"header_display_name"`
	DefaultRole       string            `json:"default_role"`
	GroupToRole       map[string]string `json:"group_to_role"`
}

type OIDCConfig struct {
	Enabled      bool             `json:"enabled"`
	IssuerURL    string           `json:"issuer_url"`
	ClientID     string           `json:"client_id"`
	ClientSecret string           `json:"client_secret"`
	RedirectURL  string           `json:"redirect_url"`
	Scopes       []string         `json:"scopes"`
	ClaimMapping OIDCClaimMapping `json:"claim_mapping"`
}

type OIDCClaimMapping struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	ExternalID  string `json:"external_id"`
}

type LDAPConfig struct {
	Enabled           bool              `json:"enabled"`
	ServerURL         string            `json:"server_url"`
	BaseDN            string            `json:"base_dn"`
	UserSearchFilter  string            `json:"user_search_filter"`
	BindDN            string            `json:"bind_dn"`
	BindPassword      string            `json:"bind_password"`
	AttributeMapping  LDAPAttrMapping   `json:"attribute_mapping"`
	GroupToRole       map[string]string `json:"group_to_role"`
	GroupSearchBase   string            `json:"group_search_base"`
	GroupSearchFilter string            `json:"group_search_filter"`
}

type LDAPAttrMapping struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

// AdvancedDatabaseConfig represents an alternative database configuration format
// commonly used in advanced environments.
type AdvancedDatabaseConfig struct {
	User      string                 `json:"user"`
	Password  string                 `json:"password"`
	Database  string                 `json:"database"`
	Writers   []AdvancedDatabaseEndpoint `json:"writers"`
	Readers   []AdvancedDatabaseEndpoint `json:"readers"`
	Analytics []AdvancedDatabaseEndpoint `json:"analytics"`
	Type      string                 `json:"type"`
	SSL       string                 `json:"ssl"`
}

// AdvancedDatabaseEndpoint represents a host:port pair in advanced database config.
type AdvancedDatabaseEndpoint struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// buildDatabaseURL constructs a PostgreSQL DSN from a advanced database configuration.
// It uses the first writer endpoint and maps the SSL field to PostgreSQL sslmode values.
func buildDatabaseURL(adv AdvancedDatabaseConfig) (string, error) {
	if adv.Type != "postgresql" {
		return "", fmt.Errorf("unsupported database type %q, only \"postgresql\" is supported", adv.Type)
	}
	if len(adv.Writers) == 0 {
		return "", fmt.Errorf("database config has no writer endpoints")
	}

	sslMode, err := mapSSLMode(adv.SSL)
	if err != nil {
		return "", err
	}

	w := adv.Writers[0]
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(adv.User, adv.Password),
		Host:     fmt.Sprintf("%s:%d", w.Host, w.Port),
		Path:     adv.Database,
		RawQuery: "sslmode=" + sslMode,
	}
	return u.String(), nil
}

func mapSSLMode(ssl string) (string, error) {
	switch ssl {
	case "off":
		return "disable", nil
	case "preferred":
		return "prefer", nil
	case "required":
		return "require", nil
	case "strict":
		return "verify-full", nil
	case "":
		return "disable", nil
	default:
		return "", fmt.Errorf("unsupported ssl value %q, expected off|preferred|required|strict", ssl)
	}
}

func Load() (*Config, error) {
	configstore.InitFromEnvironment()

	cfg := &Config{
		HTTP:  HTTPConfig{Port: 8080},
		Admin: AdminConfig{Username: "admin", Password: "admin"},
		Log:   LogConfig{Format: "json", Level: "info"},
		Assets: AssetsConfig{
			Backend:     "filesystem",
			MaxFileSize: 50 * 1024 * 1024, // 50 MB
			Filesystem:  FilesystemStoreConfig{DataDir: "./data/assets"},
			S3:          S3StoreConfig{Prefix: "assets"},
		},
		Auth: AuthConfig{
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
			ProxyAuth: ProxyAuthConfig{
				HeaderUser:   "X-Remote-User",
				HeaderGroups: "X-Remote-Groups",
				DefaultRole:  "learner",
			},
		},
	}

	if err := unmarshalItem("http", &cfg.HTTP); err != nil {
		// optional, keep defaults
	}

	if err := loadDatabaseConfig(cfg); err != nil {
		return nil, err
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

	var encCfg struct {
		Key string `json:"key"`
	}
	if err := unmarshalItem("encryption", &encCfg); err == nil {
		cfg.EncryptionKey = encCfg.Key
	}

	if err := unmarshalItem("assets", &cfg.Assets); err != nil {
		// optional, keep defaults
	}

	if err := unmarshalItem("log", &cfg.Log); err != nil {
		// optional, keep defaults
	}

	return cfg, nil
}

// loadDatabaseConfig tries to load the database config from a advanced-style JSON
// format first (auto-detected by the presence of a "type" field), then falls
// back to the standard YAML format (url: ...).
func loadDatabaseConfig(cfg *Config) error {
	raw, err := configstore.GetItemValue("database")
	if err != nil {
		return fmt.Errorf("missing required config item 'database': %w", err)
	}

	// Try advanced JSON format
	var adv AdvancedDatabaseConfig
	if jsonErr := json.Unmarshal([]byte(raw), &adv); jsonErr == nil && adv.Type != "" {
		dsn, err := buildDatabaseURL(adv)
		if err != nil {
			return fmt.Errorf("invalid advanced database config: %w", err)
		}
		cfg.Database.URL = dsn
		return nil
	}

	// Fallback: standard YAML format
	if yamlErr := yaml.Unmarshal([]byte(raw), &cfg.Database); yamlErr != nil {
		return fmt.Errorf("invalid database config: %w", yamlErr)
	}
	if cfg.Database.URL == "" {
		return fmt.Errorf("database.url is required")
	}
	return nil
}

func unmarshalItem(key string, dest interface{}) error {
	v, err := configstore.GetItemValue(key)
	if err != nil {
		return err
	}
	return yaml.Unmarshal([]byte(v), dest)
}
