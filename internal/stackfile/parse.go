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

		if err := validateEnvRefs(name, res.Env, res.Ports, sf.Resources); err != nil {
			return err
		}

		for addonName, addon := range res.Addons {
			if err := validateAddonEnv(name, addonName, addon); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateEnvRefs(resourceName string, env map[string]string, ports []PortDef, allResources map[string]Resource) error {
	for envKey, envVal := range env {
		refs := findRefs(envVal)
		if len(refs) == 0 {
			continue
		}

		// Check that self refs are not mixed with resource refs
		hasSelf := false
		hasResource := false
		for _, ref := range refs {
			if ref.Source == "self" {
				hasSelf = true
			} else {
				hasResource = true
			}
		}
		if hasSelf && hasResource {
			return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': cannot mix self and resource references in the same value")
		}

		// Self refs must be exact (entire value is the ref)
		if hasSelf {
			if !exactRefPattern.MatchString(envVal) {
				return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': self-references must be the only content of the env var (e.g., '{{ self.port.http }}')")
			}
			if err := validateSelfOutput(resourceName, envKey, refs[0].Output, ports); err != nil {
				return err
			}
			continue
		}

		// All resource refs in a single env value must reference the same source
		source := refs[0].Source
		for _, ref := range refs[1:] {
			if ref.Source != source {
				return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': references multiple resources ('" + source + "' and '" + ref.Source + "'). Each env var can only reference one source resource.")
			}
		}

		// Validate each ref's output against the source resource
		for _, ref := range refs {
			targetRes, ok := allResources[ref.Source]
			if !ok {
				return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': references resource '" + ref.Source + "' which is not defined in the stackfile")
			}
			if err := validateResourceOutput(resourceName, envKey, ref.Source, ref.Output, targetRes.Ports); err != nil {
				return err
			}
		}
	}
	return nil
}

var postgresAddonOutputs = map[string]bool{
	"host":           true,
	"port":           true,
	"database":       true,
	"username":       true,
	"password":       true,
	"sslmode":        true,
	"ca_certificate": true,
	"url":            true,
}

func addonOutputsForType(addonType string) map[string]bool {
	switch addonType {
	case "postgres":
		return postgresAddonOutputs
	default:
		return nil
	}
}

func validateAddonEnv(resourceName, addonName string, addon AddonConnectionConfig) error {
	validOutputs := addonOutputsForType(addon.Type)
	if validOutputs == nil {
		return clierrors.ValidationError("Resource '" + resourceName + "' addon '" + addonName + "': unsupported addon type '" + addon.Type + "'")
	}

	for envKey, envVal := range addon.Env {
		refs := findAddonRefs(envVal)
		if len(refs) == 0 {
			return clierrors.ValidationError("Resource '" + resourceName + "' addon '" + addonName + "' env var '" + envKey + "': value must use {{ output }} references (e.g., '{{ host }}'), got bare string '" + envVal + "'")
		}
		for _, ref := range refs {
			if !validOutputs[ref.Output] {
				valid := make([]string, 0, len(validOutputs))
				for k := range validOutputs {
					valid = append(valid, k)
				}
				return clierrors.ValidationError("Resource '" + resourceName + "' addon '" + addonName + "' env var '" + envKey + "': unknown " + addon.Type + " output '" + ref.Output + "'. Valid outputs: " + strings.Join(valid, ", "))
			}
		}
	}
	return nil
}

func validateResourceOutput(resourceName, envKey, sourceResource, output string, sourcePorts []PortDef) error {
	return validateOutputAgainstPorts(resourceName, envKey, sourceResource, output, sourcePorts)
}

func validateSelfOutput(resourceName, envKey, output string, ports []PortDef) error {
	return validateOutputAgainstPorts(resourceName, envKey, "self", output, ports)
}

func validateOutputAgainstPorts(resourceName, envKey, source, output string, ports []PortDef) error {
	if output == "host" {
		return nil
	}

	portNames := make(map[string]PortDef)
	for _, p := range ports {
		portNames[p.Name] = p
	}

	parts := strings.Split(output, ".")
	label := source
	if source == "self" {
		label = "resource '" + resourceName + "'"
	} else {
		label = "resource '" + source + "'"
	}

	switch parts[0] {
	case "port":
		if len(parts) != 2 {
			return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': invalid output '" + output + "'. Expected 'port.<port-name>'")
		}
		if _, ok := portNames[parts[1]]; !ok {
			return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': output '" + output + "' references port '" + parts[1] + "' which is not defined on " + label)
		}

	case "url":
		if len(parts) != 2 {
			return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': invalid output '" + output + "'. Expected 'url.<port-name>'")
		}
		if _, ok := portNames[parts[1]]; !ok {
			return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': output '" + output + "' references port '" + parts[1] + "' which is not defined on " + label)
		}

	case "public":
		if len(parts) != 3 {
			return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': invalid output '" + output + "'. Expected 'public.<port-name>.host' or 'public.<port-name>.url'")
		}
		portName := parts[1]
		suffix := parts[2]
		if suffix != "host" && suffix != "url" {
			return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': invalid output '" + output + "'. Expected 'public.<port-name>.host' or 'public.<port-name>.url'")
		}
		p, ok := portNames[portName]
		if !ok {
			return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': output '" + output + "' references port '" + portName + "' which is not defined on " + label)
		}
		if !p.Public {
			return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': output '" + output + "' requires port '" + portName + "' to have 'public: true'")
		}

	default:
		valid := []string{"host"}
		for _, p := range ports {
			valid = append(valid, "port."+p.Name, "url."+p.Name)
			if p.Public {
				valid = append(valid, "public."+p.Name+".host", "public."+p.Name+".url")
			}
		}
		return clierrors.ValidationError("Resource '" + resourceName + "' env var '" + envKey + "': unknown output '" + output + "' on " + label + ". Valid outputs: " + strings.Join(valid, ", "))
	}

	return nil
}
