package config

import (
	"fmt"

	"github.com/ovh/configstore"
	"gopkg.in/yaml.v2"
)

type Config struct {
	HTTP          HTTPConfig     `yaml:"http"`
	Database      DatabaseConfig `yaml:"database"`
	JWT           JWTConfig      `yaml:"jwt"`
	Admin         AdminConfig    `yaml:"admin"`
	Auth          AuthConfig     `yaml:"auth"`
	Assets        AssetsConfig   `yaml:"assets"`
	Log           LogConfig      `yaml:"log"`
	EncryptionKey string         `yaml:"encryption_key"`
}

type LogConfig struct {
	Format          string `yaml:"format"`           // "json" (default), "text", "gelf"
	Level           string `yaml:"level"`             // "debug", "info" (default), "warn", "error"
	RequestIDHeader string `yaml:"request_id_header"` // header name for upstream request ID (e.g. "X-Request-Id")
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

type AssetsConfig struct {
	Backend     string               `yaml:"backend"` // "filesystem" (default) or "s3"
	MaxFileSize int64                `yaml:"max_file_size"`
	Filesystem  FilesystemStoreConfig `yaml:"filesystem"`
	S3          S3StoreConfig        `yaml:"s3"`
}

type FilesystemStoreConfig struct {
	DataDir string `yaml:"data_dir"`
}

type S3StoreConfig struct {
	Bucket         string `yaml:"bucket"`
	Region         string `yaml:"region"`
	Endpoint       string `yaml:"endpoint"`
	Prefix         string `yaml:"prefix"`
	AccessKey      string `yaml:"access_key"`
	SecretKey      string `yaml:"secret_key"`
	ForcePathStyle bool   `yaml:"force_path_style"`
}

type AuthConfig struct {
	LocalEnabled bool            `yaml:"local_enabled"`
	OIDC         OIDCConfig      `yaml:"oidc"`
	LDAP         LDAPConfig      `yaml:"ldap"`
	ProxyAuth    ProxyAuthConfig `yaml:"proxy_auth"`
}

type ProxyAuthConfig struct {
	Enabled           bool              `yaml:"enabled"`
	HeaderUser        string            `yaml:"header_user"`
	HeaderGroups      string            `yaml:"header_groups"`
	HeaderEmail       string            `yaml:"header_email"`
	HeaderDisplayName string            `yaml:"header_display_name"`
	DefaultRole       string            `yaml:"default_role"`
	GroupToRole       map[string]string `yaml:"group_to_role"`
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
		Log:      LogConfig{Format: "json", Level: "info"},
		Assets: AssetsConfig{
			Backend:     "filesystem",
			MaxFileSize: 50 * 1024 * 1024, // 50 MB
			Filesystem:  FilesystemStoreConfig{DataDir: "./data/assets"},
			S3:          S3StoreConfig{Prefix: "assets"},
		},
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

	var encCfg struct {
		Key string `yaml:"key"`
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

func unmarshalItem(key string, dest interface{}) error {
	v, err := configstore.GetItemValue(key)
	if err != nil {
		return err
	}
	return yaml.Unmarshal([]byte(v), dest)
}
