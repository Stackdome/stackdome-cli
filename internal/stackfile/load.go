// Package stackfile is the CLI's thin layer over the hub's canonical stackfile
// package: file loading with CLI-shaped errors, docker-compose conversion, and
// raw stack JSON. Schema, validation and conversion live in the hub package —
// never fork them.
package stackfile

import (
	"os"
	"path/filepath"
	"strings"

	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	hub "github.com/Stackdome/stackdome/pkg/stackfile"
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
	FromStack    = hub.FromStack
	SchemaJSON   = hub.SchemaJSON
)

// Load reads, validates and returns the stackfile at path.
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

	return sf, nil
}
