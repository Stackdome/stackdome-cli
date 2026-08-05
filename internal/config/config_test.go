package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("STACKDOME_TOKEN", "sdm_abc")
	t.Setenv("STACKDOME_URL", "https://hub.example")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AccessToken != "sdm_abc" {
		t.Errorf("AccessToken = %q, want sdm_abc", cfg.AccessToken)
	}
	if cfg.ServerURL != "https://hub.example" {
		t.Errorf("ServerURL = %q, want https://hub.example", cfg.ServerURL)
	}
	if err := cfg.RequireAuth(); err != nil {
		t.Errorf("RequireAuth with env credentials and no config file: %v", err)
	}
}

// Env credentials are ephemeral: saving unrelated state (e.g. current stack)
// must not leak the env token or URL into the config file.
func TestEnvVarsNotPersisted(t *testing.T) {
	path := writeConfig(t, `{"server_url":"https://file","access_token":"file_tok"}`)
	t.Setenv("STACKDOME_CONFIG", path)
	t.Setenv("STACKDOME_TOKEN", "sdm_abc")
	t.Setenv("STACKDOME_URL", "https://hub.example")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.CurrentStack = "web"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sdm_abc") || strings.Contains(string(data), "hub.example") {
		t.Errorf("env values persisted to disk: %s", data)
	}
	if !strings.Contains(string(data), "file_tok") {
		t.Errorf("file token was clobbered: %s", data)
	}
}

// A login performed while STACKDOME_TOKEN is set must persist the credential
// it just obtained, not the stale one the file already held.
func TestLoginWhileEnvTokenSetPersistsNewToken(t *testing.T) {
	path := writeConfig(t, `{"server_url":"https://file","access_token":"old_tok"}`)
	t.Setenv("STACKDOME_CONFIG", path)
	t.Setenv("STACKDOME_TOKEN", "sdm_abc")
	t.Setenv("STACKDOME_URL", "https://hub.example")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ServerURL = "https://new"
	cfg.AccessToken = "new_tok"
	cfg.RefreshToken = "new_refresh"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"new_tok", "new_refresh", "https://new"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in saved config: %s", want, data)
		}
	}
	if strings.Contains(string(data), "old_tok") {
		t.Errorf("stale token persisted over the new one: %s", data)
	}

	// `STACKDOME_URL=X stackdome login --url X --token $STACKDOME_TOKEN` logs in
	// with exactly the env values. Save's latch would read that as "still the
	// ephemeral env value" and write an empty config, so login clears the latch.
	t.Run("values identical to the environment", func(t *testing.T) {
		path := writeConfig(t, `{}`)
		t.Setenv("STACKDOME_CONFIG", path)
		t.Setenv("STACKDOME_TOKEN", "sdm_abc")
		t.Setenv("STACKDOME_URL", "https://hub.example")

		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		cfg.ServerURL = "https://hub.example"
		cfg.AccessToken = "sdm_abc"
		cfg.AdoptEnvValues()
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"sdm_abc", "https://hub.example"} {
			if !strings.Contains(string(data), want) {
				t.Errorf("expected %q in saved config: %s", want, data)
			}
		}
	})
}

// An env-token session must not create or depend on a config file.
func TestSetCurrentStackWithEnvTokenDoesNotWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	t.Setenv("STACKDOME_CONFIG", path)
	t.Setenv("STACKDOME_TOKEN", "sdm_abc")
	t.Setenv("STACKDOME_URL", "https://hub.example")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetCurrentStack("web"); err != nil {
		t.Fatalf("SetCurrentStack: %v", err)
	}
	if cfg.CurrentStack != "web" {
		t.Errorf("CurrentStack = %q, want web (in memory)", cfg.CurrentStack)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("config file was created at %s", path)
	}
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

// STACKDOME_ORG / STACKDOME_PROJECT let a scoped API token supply the scope the
// CLI would otherwise have to discover through the API.
func TestOrgAndProjectFromEnv(t *testing.T) {
	t.Setenv("STACKDOME_CONFIG", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("STACKDOME_TOKEN", "sdm_abc")
	t.Setenv("STACKDOME_URL", "https://hub.example")
	t.Setenv("STACKDOME_ORG", "org-123")
	t.Setenv("STACKDOME_PROJECT", "default")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OrganizationID != "org-123" {
		t.Errorf("OrganizationID = %q, want org-123", cfg.OrganizationID)
	}
	if cfg.ProjectName != "default" {
		t.Errorf("ProjectName = %q, want default", cfg.ProjectName)
	}
}

func TestOrgAndProjectFromEnvNotPersisted(t *testing.T) {
	path := writeConfig(t, `{"server_url":"https://file","access_token":"file_tok","organization_id":"file-org","project_name":"file-proj"}`)
	t.Setenv("STACKDOME_CONFIG", path)
	t.Setenv("STACKDOME_ORG", "env-org")
	t.Setenv("STACKDOME_PROJECT", "env-proj")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.CurrentStack = "web"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "env-org") || strings.Contains(string(data), "env-proj") {
		t.Errorf("env scope persisted to disk: %s", data)
	}
	if !strings.Contains(string(data), "file-org") || !strings.Contains(string(data), "file-proj") {
		t.Errorf("file scope was clobbered: %s", data)
	}
}
