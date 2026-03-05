package config

import (
	"testing"
)

func TestBuildDatabaseURL_Standard(t *testing.T) {
	adv := AdvancedDatabaseConfig{
		User:     "dbdemo-rw",
		Password: "dbdemo-rw-pwd",
		Database: "dbdemo",
		Writers: []AdvancedDatabaseEndpoint{
			{Host: "db-host", Port: 5432},
		},
		Type: "postgresql",
		SSL:  "off",
	}

	got, err := buildDatabaseURL(adv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "postgres://dbdemo-rw:dbdemo-rw-pwd@db-host:5432/dbdemo?sslmode=disable"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestBuildDatabaseURL_SSLMappings(t *testing.T) {
	tests := []struct {
		ssl      string
		wantMode string
	}{
		{"off", "disable"},
		{"preferred", "prefer"},
		{"required", "require"},
		{"strict", "verify-full"},
		{"", "disable"},
	}

	for _, tt := range tests {
		t.Run("ssl="+tt.ssl, func(t *testing.T) {
			adv := AdvancedDatabaseConfig{
				User:     "u",
				Password: "p",
				Database: "d",
				Writers:  []AdvancedDatabaseEndpoint{{Host: "h", Port: 5432}},
				Type:     "postgresql",
				SSL:      tt.ssl,
			}
			got, err := buildDatabaseURL(adv)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			wantSuffix := "sslmode=" + tt.wantMode
			if !contains(got, wantSuffix) {
				t.Errorf("URL %q does not contain %q", got, wantSuffix)
			}
		})
	}
}

func TestBuildDatabaseURL_UnsupportedType(t *testing.T) {
	adv := AdvancedDatabaseConfig{
		User:     "u",
		Password: "p",
		Database: "d",
		Writers:  []AdvancedDatabaseEndpoint{{Host: "h", Port: 5432}},
		Type:     "mysql",
		SSL:      "off",
	}

	_, err := buildDatabaseURL(adv)
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
	if !contains(err.Error(), "unsupported database type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBuildDatabaseURL_NoWriters(t *testing.T) {
	adv := AdvancedDatabaseConfig{
		User:     "u",
		Password: "p",
		Database: "d",
		Writers:  nil,
		Type:     "postgresql",
		SSL:      "off",
	}

	_, err := buildDatabaseURL(adv)
	if err == nil {
		t.Fatal("expected error for no writers, got nil")
	}
	if !contains(err.Error(), "no writer endpoints") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBuildDatabaseURL_SpecialChars(t *testing.T) {
	adv := AdvancedDatabaseConfig{
		User:     "user@domain",
		Password: "p:ss/w@rd",
		Database: "my-db",
		Writers:  []AdvancedDatabaseEndpoint{{Host: "host", Port: 5432}},
		Type:     "postgresql",
		SSL:      "off",
	}

	got, err := buildDatabaseURL(adv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// url.PathEscape encodes @ as %40, : as %3A, / as %2F
	if !contains(got, "user%40domain") {
		t.Errorf("user not properly escaped in URL: %s", got)
	}
	if !contains(got, "p%3Ass%2Fw%40rd") {
		t.Errorf("password not properly escaped in URL: %s", got)
	}
}

func TestBuildDatabaseURL_UnsupportedSSL(t *testing.T) {
	adv := AdvancedDatabaseConfig{
		User:     "u",
		Password: "p",
		Database: "d",
		Writers:  []AdvancedDatabaseEndpoint{{Host: "h", Port: 5432}},
		Type:     "postgresql",
		SSL:      "invalid",
	}

	_, err := buildDatabaseURL(adv)
	if err == nil {
		t.Fatal("expected error for unsupported ssl, got nil")
	}
	if !contains(err.Error(), "unsupported ssl value") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBuildDatabaseURL_MultipleWriters(t *testing.T) {
	adv := AdvancedDatabaseConfig{
		User:     "u",
		Password: "p",
		Database: "d",
		Writers: []AdvancedDatabaseEndpoint{
			{Host: "primary", Port: 5432},
			{Host: "secondary", Port: 5433},
		},
		Type: "postgresql",
		SSL:  "off",
	}

	got, err := buildDatabaseURL(adv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use first writer only
	if !contains(got, "primary:5432") {
		t.Errorf("expected first writer host in URL, got: %s", got)
	}
	if contains(got, "secondary") {
		t.Errorf("should not contain secondary writer, got: %s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
