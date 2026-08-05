package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	clierrors "github.com/stackdome/cli/internal/errors"
)

const (
	configDirName  = ".stackdome"
	configFileName = "config.json"
	configFileMode = 0600
	configDirMode  = 0700
)

type Config struct {
	ServerURL      string `json:"server_url"`
	AccessToken    string `json:"access_token,omitempty"`
	RefreshToken   string `json:"refresh_token,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
	ProjectName    string `json:"project_name,omitempty"`
	Username       string `json:"username,omitempty"`
	CurrentStack   string `json:"current_stack,omitempty"`
	Insecure       bool   `json:"insecure,omitempty"`

	path string `json:"-"`

	// Values read from the config file, kept so Save never persists the
	// ephemeral STACKDOME_URL / STACKDOME_TOKEN overrides.
	fileServerURL    string `json:"-"`
	fileAccessToken  string `json:"-"`
	fileRefreshToken string `json:"-"`
	urlFromEnv       bool   `json:"-"`
	tokenFromEnv     bool   `json:"-"`
}

// TokenFromEnv reports whether the access token came from STACKDOME_TOKEN.
// Env tokens are not refreshable — there is no refresh token to pair them with.
func (c *Config) TokenFromEnv() bool { return c.tokenFromEnv }

func DefaultPath() (string, error) {
	if p := os.Getenv("STACKDOME_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", clierrors.Wrap(err, "Cannot determine home directory")
	}
	return filepath.Join(home, configDirName, configFileName), nil
}

func Load() (*Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		return nil, err
	}
	cfg.applyEnv()
	return cfg, nil
}

// applyEnv overlays STACKDOME_URL / STACKDOME_TOKEN on top of the file config.
// A token from the environment stands alone: no refresh token comes with it.
func (c *Config) applyEnv() {
	c.fileServerURL, c.fileAccessToken, c.fileRefreshToken = c.ServerURL, c.AccessToken, c.RefreshToken

	if v := os.Getenv("STACKDOME_URL"); v != "" {
		c.ServerURL = v
		c.urlFromEnv = true
	}
	if v := os.Getenv("STACKDOME_TOKEN"); v != "" {
		c.AccessToken = v
		c.RefreshToken = ""
		c.tokenFromEnv = true
	}
}

func LoadFrom(path string) (*Config, error) {
	cfg := &Config{path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, clierrors.Wrapf(err, "Failed to read config from %s", path)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, clierrors.Wrapf(err, "Failed to parse config from %s", path)
	}

	// Teams were renamed to projects; adopt the legacy key from older configs.
	if cfg.ProjectName == "" {
		var legacy struct {
			TeamName string `json:"team_name"`
		}
		if json.Unmarshal(data, &legacy) == nil {
			cfg.ProjectName = legacy.TeamName
		}
	}

	cfg.path = path
	return cfg, nil
}

func (c *Config) Save() error {
	if c.path == "" {
		p, err := DefaultPath()
		if err != nil {
			return err
		}
		c.path = p
	}

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, configDirMode); err != nil {
		return clierrors.Wrapf(err, "Failed to create config directory %s", dir)
	}

	// Env overrides are ephemeral — write back what the file had.
	out := *c
	if c.urlFromEnv {
		out.ServerURL = c.fileServerURL
	}
	if c.tokenFromEnv {
		out.AccessToken, out.RefreshToken = c.fileAccessToken, c.fileRefreshToken
	}

	data, err := json.MarshalIndent(&out, "", "  ")
	if err != nil {
		return clierrors.Wrap(err, "Failed to serialize config")
	}

	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, configFileMode); err != nil {
		return clierrors.Wrapf(err, "Failed to write config to %s", tmp)
	}
	if err := os.Rename(tmp, c.path); err != nil {
		os.Remove(tmp)
		return clierrors.Wrapf(err, "Failed to save config to %s", c.path)
	}
	return nil
}

func (c *Config) Clear() error {
	*c = Config{path: c.path}
	return c.Save()
}

func (c *Config) IsLoggedIn() bool {
	return c.AccessToken != "" && c.ServerURL != ""
}

func (c *Config) RequireAuth() error {
	if !c.IsLoggedIn() {
		return clierrors.AuthError("Not logged in. Run `stackdome login` first.")
	}
	return nil
}

func (c *Config) RequireStack() (string, error) {
	if err := c.RequireAuth(); err != nil {
		return "", err
	}
	if c.CurrentStack == "" {
		return "", clierrors.New("No stack selected. Run `stackdome deploy` or use `--stack <name>`.")
	}
	return c.CurrentStack, nil
}

func (c *Config) SetCurrentStack(name string) error {
	c.CurrentStack = name
	return c.Save()
}

func (c *Config) Path() string {
	return c.path
}

func (c *Config) Summary() string {
	if !c.IsLoggedIn() {
		return "Not logged in"
	}
	s := fmt.Sprintf("Server:   %s\nUser:     %s\nOrg:      %s", c.ServerURL, c.Username, c.OrganizationID)
	if c.CurrentStack != "" {
		s += fmt.Sprintf("\nStack:    %s", c.CurrentStack)
	}
	return s
}
