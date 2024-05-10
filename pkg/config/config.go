package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

type Config struct {
	AccessToken        string                 `json:"accessToken,omitempty" doc:"Bearer access token."`
	VoyagerServerUrl   string                 `json:"voyagerServerUrl"`
	Insecure           bool                   `json:"insecure"`
	ProviderConfig     *ComputeProviderConfig `json:"providerConfig"`
	Username           string                 `json:"username,omitempty" doc:"User name."`
	Organisation       string                 `json:"organisation"`
	TokenValidity      time.Time
	UserPrivateKeyPath string          `json:"userPrivateKeyPath"`
	SyncDaemonInfo     *SyncDaemonInfo `json:"SyncDaemonInfo"`
}

type SyncDaemonInfo struct {
	PortForwardDaemonPID int `json:"portForwardDaemonPID"`
	MutagenDaemonPID     int `json:"mutagenDaemonPID"`
}

type ComputeProviderConfig struct {
	ServiceAccountName string `json:"serviceAccountName"`
	Namespace          string `json:"namespace"`
	Token              string `json:"token"`
	CaCert             []byte `json:"caCert"`
	ServerUrl          string `json:"serverUrl"`
	SSHUserName        string `json:"SSHUserName"`
}

func notNull(attr any) bool {
	return attr != nil
}

func notEmpty(attr string) bool {
	return len(attr) != 0
}

func (c *Config) Valid() bool {
	validations := []bool{
		notEmpty(c.AccessToken),
		notEmpty(c.VoyagerServerUrl),
		notNull(c.ProviderConfig),
		notEmpty(c.ProviderConfig.Namespace),
		len(c.ProviderConfig.CaCert) != 0,
		notEmpty(c.ProviderConfig.Token),
		notEmpty(c.ProviderConfig.ServerUrl),
	}
	for _, valid := range validations {
		if !valid {
			return false
		}
	}
	return true
}

func ConfigLocation() (string, error) {
	if ocmconfig := os.Getenv("VOYAGER_CONFIG"); ocmconfig != "" {
		return ocmconfig, nil
	}
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	// Get the user's home directory
	configDir := currentUser.HomeDir
	path := filepath.Join(configDir, "/.voyager/config.json")
	return path, nil
}

func (c *Config) ConfigDir() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	// Get the user's home directory
	configDir := currentUser.HomeDir
	path := filepath.Join(configDir, "/.voyager")
	return path, nil
}

func Load() (*Config, error) {
	filePath, err := ConfigLocation()
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cant find voyager config file at: %s", filePath)
		}
		return nil, fmt.Errorf("can't check if config file '%s' exists: %w", filePath, err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't read config file '%s': %v", filePath, err)
	}
	cfg := &Config{}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty config file")
	}
	err = json.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("can't parse config file '%s': %v", filePath, err)
	}
	return cfg, nil
}

func New() *Config {
	return &Config{}
}

// Save the given configuration to the configuration file.
func Save(cfg *Config) error {
	file, err := ConfigLocation()
	if err != nil {
		return err
	}
	dir := filepath.Dir(file)
	err = os.MkdirAll(dir, os.FileMode(0755))
	if err != nil {
		return fmt.Errorf("can't create directory %s: %v", dir, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("can't marshal config: %v", err)
	}
	err = os.WriteFile(file, data, 0600)
	if err != nil {
		return fmt.Errorf("can't write file '%s': %v", file, err)
	}
	return nil
}
