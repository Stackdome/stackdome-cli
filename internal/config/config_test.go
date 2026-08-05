package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFrom_AdoptsLegacyTeamName(t *testing.T) {
	cfg, err := LoadFrom(writeConfig(t, `{"server_url":"https://x","team_name":"acme"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProjectName != "acme" {
		t.Errorf("expected legacy team_name to become project_name, got %q", cfg.ProjectName)
	}
}

func TestLoadFrom_PrefersProjectName(t *testing.T) {
	cfg, err := LoadFrom(writeConfig(t, `{"server_url":"https://x","team_name":"old","project_name":"new"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProjectName != "new" {
		t.Errorf("expected project_name to win, got %q", cfg.ProjectName)
	}
}
