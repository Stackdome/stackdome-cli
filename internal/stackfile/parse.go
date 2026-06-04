package stackfile

import (
	"os"
	"path/filepath"
	"strings"

	clierrors "github.com/stackdome/cli/internal/errors"
	"gopkg.in/yaml.v3"
)

func Load(path string) (*Stackfile, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return loadYAML(path)
	default:
		return nil, clierrors.ValidationError("Unsupported file format: " + ext + " (expected .yaml or .yml)")
	}
}

func loadYAML(path string) (*Stackfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, clierrors.Newf("Stackfile not found: %s", path)
		}
		return nil, clierrors.Wrapf(err, "Failed to read stackfile: %s", path)
	}

	var sf Stackfile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, clierrors.Wrapf(err, "Failed to parse stackfile: %s", path)
	}

	if err := validate(&sf); err != nil {
		return nil, err
	}

	return &sf, nil
}

func validate(sf *Stackfile) error {
	if sf.Name == "" {
		return clierrors.ValidationError("Stackfile missing required field: name")
	}
	if len(sf.Resources) == 0 {
		return clierrors.ValidationError("Stackfile must define at least one resource")
	}
	for name, res := range sf.Resources {
		if res.Image == "" && res.Build == nil {
			return clierrors.ValidationError("Resource '" + name + "' must have either 'image' or 'build'")
		}
		if res.Image != "" && res.Build != nil {
			return clierrors.ValidationError("Resource '" + name + "' cannot have both 'image' and 'build'")
		}
		if res.Build != nil {
			if res.Build.Repo == "" {
				return clierrors.ValidationError("Resource '" + name + "' build config missing 'repo'")
			}
			set := 0
			if res.Build.Branch != "" {
				set++
			}
			if res.Build.Tag != "" {
				set++
			}
			if res.Build.Commit != "" {
				set++
			}
			if set > 1 {
				return clierrors.ValidationError("Resource '" + name + "' build config: only one of 'branch', 'tag', or 'commit' can be set")
			}
			if res.Build.Dockerfile != "" && !strings.HasSuffix(res.Build.Dockerfile, "Dockerfile") && !strings.Contains(res.Build.Dockerfile, "Dockerfile.") && !strings.Contains(res.Build.Dockerfile, "dockerfile") {
				return clierrors.ValidationError("Resource '" + name + "' build config: 'dockerfile' should be a path to a Dockerfile")
			}
		}
		for _, p := range res.Ports {
			if p.Name == "" {
				return clierrors.ValidationError("Resource '" + name + "' has a port without a name")
			}
			if p.Port <= 0 {
				return clierrors.ValidationError("Resource '" + name + "' port '" + p.Name + "' has invalid port number")
			}
		}
		for _, vm := range res.Volumes {
			if _, ok := sf.Volumes[vm.Name]; !ok {
				return clierrors.ValidationError("Resource '" + name + "' references undefined volume '" + vm.Name + "'")
			}
		}
	}
	return nil
}
