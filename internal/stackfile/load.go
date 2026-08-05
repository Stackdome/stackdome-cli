// Package stackfile is the CLI's thin layer over the hub's canonical stackfile
// package: file loading with CLI-shaped errors, the `env_file` convenience the
// hub schema does not carry, the docker-compose converter, and raw stack JSON.
// Schema, validation and conversion live in the hub package — never fork them.
package stackfile

import (
	"os"
	"path/filepath"
	"strings"

	hub "github.com/Stackdome/stackdome/pkg/stackfile"
	"github.com/joho/godotenv"
	clierrors "github.com/stackdome/cli/internal/errors"
	"gopkg.in/yaml.v3"
)

type (
	Stackfile             = hub.Stackfile
	Resource              = hub.Resource
	BuildConfig           = hub.BuildConfig
	PortDef               = hub.PortDef
	VolumeDef             = hub.VolumeDef
	VolumeMountDef        = hub.VolumeMountDef
	SecretMapping         = hub.SecretMapping
	AddonConnectionConfig = hub.AddonConnectionConfig
	Resolver              = hub.Resolver
)

var (
	Validate     = hub.Validate
	ResolveStack = hub.ResolveStack
)

// Load reads, validates and returns the stackfile at path. Any `env_file`
// references are resolved relative to the stackfile's directory.
func Load(path string) (*Stackfile, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yaml" && ext != ".yml" {
		return nil, clierrors.ValidationError("Unsupported file format: " + ext + " (expected .yaml or .yml)")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, clierrors.Newf("Stackfile not found: %s", path)
		}
		return nil, clierrors.Wrapf(err, "Failed to read stackfile: %s", path)
	}

	sf, err := hub.Load(data)
	if err != nil {
		return nil, clierrors.ValidationError(err.Error())
	}

	if err := applyEnvFiles(sf, data, filepath.Dir(path)); err != nil {
		return nil, err
	}

	return sf, nil
}

// applyEnvFiles merges each resource's `env_file` into its env map. The key is
// a CLI-only extension, so it is read off the raw YAML rather than the hub's
// Resource type. Values already set in `env:` win.
func applyEnvFiles(sf *Stackfile, data []byte, baseDir string) error {
	var raw struct {
		Resources map[string]struct {
			EnvFile string `yaml:"env_file"`
		} `yaml:"resources"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return clierrors.Wrap(err, "Failed to parse stackfile")
	}

	for name, r := range raw.Resources {
		if r.EnvFile == "" {
			continue
		}
		envPath := r.EnvFile
		if !filepath.IsAbs(envPath) {
			envPath = filepath.Join(baseDir, envPath)
		}
		fileEnv, err := godotenv.Read(envPath)
		if err != nil {
			return clierrors.Wrapf(err, "Failed to read env_file for resource %q", name)
		}

		res, ok := sf.Resources[name]
		if !ok {
			continue
		}
		if res.Env == nil {
			res.Env = make(map[string]string, len(fileEnv))
		}
		for k, v := range fileEnv {
			if v == "" {
				continue
			}
			if _, exists := res.Env[k]; !exists {
				res.Env[k] = v
			}
		}
		sf.Resources[name] = res
	}

	return nil
}
