package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad_RequiresGoogleSheetID(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := `{
  "wordpress_config": {"base_url": "https://example.org", "username": "u", "application_password": "p"},
  "turso_config": {"database_url": "libsql://db", "auth_token": "t"}
}`
	if err := os.WriteFile("configuration.json", []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "google_sheet_id") {
		t.Fatalf("Load() error = %v, want google_sheet_id required", err)
	}
}
