package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

type Config struct {
	AccessToken        string                 `json:"accessToken,omitempty" doc:"Bearer access token."`
	VoyagerServerUrl   string                 `json:"voyagerServerUrl"`
	Insecure           bool                   `json:"insecure"`
	ProviderConfig     *ComputeProviderConfig `json:"providerConfig"`
	Username           string                 `json:"username,omitempty" doc:"User name."`
	UserPrivateKeyPath string                 `json:"userPrivateKeyPath"`
	SyncDaemonInfo     *SyncDaemonInfo        `json:"SyncDaemonInfo"`
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

func (c *Config) ConfigLocation() (string, error) {
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

func Load() (cfg *Config, err error) {
	file, err := cfg.ConfigLocation()
	if err != nil {
		return
	}
	_, err = os.Stat(file)
	if os.IsNotExist(err) {
		cfg = &Config{}
		err = nil
		return
	}
	if err != nil {
		err = fmt.Errorf("can't check if config file '%s' exists: %v", file, err)
		return
	}

	data, err := os.ReadFile(file)
	if err != nil {
		err = fmt.Errorf("can't read config file '%s': %v", file, err)
		return
	}
	cfg = &Config{}
	if len(data) == 0 {
		return
	}
	err = json.Unmarshal(data, cfg)
	if err != nil {
		err = fmt.Errorf("can't parse config file '%s': %v", file, err)
		return
	}
	return
}

// Save the given configuration to the configuration file.
func Save(cfg *Config) error {
	file, err := cfg.ConfigLocation()
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
