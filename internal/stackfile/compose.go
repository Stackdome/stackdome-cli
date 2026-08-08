package stackfile

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Volumes  map[string]any            `yaml:"volumes"`
	Version  any                       `yaml:"version"`
	Name     string                    `yaml:"name"`
	Extra    map[string]any            `yaml:",inline"`
}

type composeService struct {
	Image       string         `yaml:"image"`
	Build       any            `yaml:"build"`
	Command     any            `yaml:"command"`
	Entrypoint  any            `yaml:"entrypoint"`
	Ports       []string       `yaml:"ports"`
	Environment any            `yaml:"environment"`
	EnvFile     any            `yaml:"env_file"`
	Volumes     []string       `yaml:"volumes"`
	DependsOn   any            `yaml:"depends_on"`
	Extra       map[string]any `yaml:",inline"`
}

type ComposeWarnings struct {
	EnvFiles                      map[string][]string
	UnsupportedBindMounts         map[string][]string
	UnsupportedTopLevelKeys       []string
	UnsupportedServiceKeys        map[string][]string
	UnresolvedEnvironment         map[string][]string
	UnsupportedBuildOptions       map[string][]string
	UnsupportedDependsOnOptions   map[string][]string
	UnsupportedVolumeOptions      map[string][]string
	UnsupportedVolumeMountOptions map[string][]string
	UnsupportedPorts              map[string][]string
	UnsupportedPortMappings       map[string][]string
	UnsupportedCommandForms       map[string][]string
}

type composeServiceWarnings struct {
	unsupportedBindMounts         []string
	unresolvedEnvironment         []string
	unsupportedBuildOptions       []string
	unsupportedDependsOnOptions   []string
	unsupportedVolumeMountOptions []string
	unsupportedPorts              []string
	unsupportedPortMappings       []string
	unsupportedCommandForms       []string
}

func FindComposeFile(dir string) string {
	candidates := []string{
		"docker-compose.yaml",
		"docker-compose.yml",
		"compose.yaml",
		"compose.yml",
	}
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// FromCompose converts a docker-compose file into a stackfile. The second
// return value records Compose features that Stackfiles cannot express, so the
// caller can warn rather than dropping them silently.
func FromCompose(path, appName string) (*Stackfile, ComposeWarnings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ComposeWarnings{}, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, ComposeWarnings{}, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if len(compose.Services) == 0 {
		return nil, ComposeWarnings{}, fmt.Errorf("no services found in %s", path)
	}

	sf := &Stackfile{
		Name:      appName,
		Resources: make(map[string]Resource),
		Volumes:   make(map[string]VolumeDef),
	}
	warnings := ComposeWarnings{
		EnvFiles:                      make(map[string][]string),
		UnsupportedBindMounts:         make(map[string][]string),
		UnsupportedTopLevelKeys:       unsupportedComposeKeys(compose.Extra),
		UnsupportedServiceKeys:        make(map[string][]string),
		UnresolvedEnvironment:         make(map[string][]string),
		UnsupportedBuildOptions:       make(map[string][]string),
		UnsupportedDependsOnOptions:   make(map[string][]string),
		UnsupportedVolumeOptions:      make(map[string][]string),
		UnsupportedVolumeMountOptions: make(map[string][]string),
		UnsupportedPorts:              make(map[string][]string),
		UnsupportedPortMappings:       make(map[string][]string),
		UnsupportedCommandForms:       make(map[string][]string),
	}

	for name, svc := range compose.Services {
		res, serviceWarnings := convertService(svc)
		sf.Resources[name] = res
		if keys := unsupportedComposeKeys(svc.Extra); len(keys) > 0 {
			warnings.UnsupportedServiceKeys[name] = keys
		}
		if len(serviceWarnings.unresolvedEnvironment) > 0 {
			warnings.UnresolvedEnvironment[name] = serviceWarnings.unresolvedEnvironment
		}
		if refs := parseEnvFileRefs(svc.EnvFile); len(refs) > 0 {
			warnings.EnvFiles[name] = refs
		}
		if len(serviceWarnings.unsupportedBindMounts) > 0 {
			warnings.UnsupportedBindMounts[name] = serviceWarnings.unsupportedBindMounts
		}
		if len(serviceWarnings.unsupportedBuildOptions) > 0 {
			warnings.UnsupportedBuildOptions[name] = serviceWarnings.unsupportedBuildOptions
		}
		if len(serviceWarnings.unsupportedDependsOnOptions) > 0 {
			warnings.UnsupportedDependsOnOptions[name] = serviceWarnings.unsupportedDependsOnOptions
		}
		if len(serviceWarnings.unsupportedVolumeMountOptions) > 0 {
			warnings.UnsupportedVolumeMountOptions[name] = serviceWarnings.unsupportedVolumeMountOptions
		}
		if len(serviceWarnings.unsupportedPorts) > 0 {
			warnings.UnsupportedPorts[name] = serviceWarnings.unsupportedPorts
		}
		if len(serviceWarnings.unsupportedPortMappings) > 0 {
			warnings.UnsupportedPortMappings[name] = serviceWarnings.unsupportedPortMappings
		}
		if len(serviceWarnings.unsupportedCommandForms) > 0 {
			warnings.UnsupportedCommandForms[name] = serviceWarnings.unsupportedCommandForms
		}
	}

	for volName, definition := range compose.Volumes {
		sf.Volumes[volName] = VolumeDef{Size: "1Gi"}
		if options := unsupportedVolumeDefinitionOptions(definition); len(options) > 0 {
			warnings.UnsupportedVolumeOptions[volName] = options
		}
	}

	collectNamedVolumes(sf)

	return sf, warnings, nil
}

func convertService(svc composeService) (Resource, composeServiceWarnings) {
	var res Resource
	var warnings composeServiceWarnings

	res.Image = svc.Image
	res.Build, warnings.unsupportedBuildOptions = parseBuild(svc.Build)
	res.Command, res.Args, warnings.unsupportedCommandForms = parseCommandArgs(svc.Entrypoint, svc.Command)
	res.Ports, warnings.unsupportedPorts, warnings.unsupportedPortMappings = parsePorts(svc.Ports)
	res.Env, warnings.unresolvedEnvironment = parseEnvironment(svc.Environment)
	res.Volumes, warnings.unsupportedBindMounts, warnings.unsupportedVolumeMountOptions = parseVolumeMounts(svc.Volumes)
	res.DependsOn, warnings.unsupportedDependsOnOptions = parseDependsOn(svc.DependsOn)

	return res, warnings
}

func unsupportedComposeKeys(extra map[string]any) []string {
	keys := make([]string, 0, len(extra))
	for key := range extra {
		// Compose extension fields are inert definitions; any values merged from
		// them have already materialized under ordinary service keys.
		if strings.HasPrefix(key, "x-") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func unsupportedVolumeDefinitionOptions(raw any) []string {
	definition, ok := raw.(map[string]any)
	if !ok {
		if raw == nil {
			return nil
		}
		return []string{"<definition>"}
	}
	return unsupportedComposeKeys(definition)
}

func parseCommandArgs(entrypoint, command any) (cmd []string, args []string, unsupported []string) {
	var ambiguous bool
	cmd, ambiguous = parseStringList(entrypoint)
	if ambiguous {
		unsupported = append(unsupported, "entrypoint")
	}
	args, ambiguous = parseStringList(command)
	if ambiguous {
		unsupported = append(unsupported, "command")
	}
	sort.Strings(unsupported)
	return cmd, args, unsupported
}

func parseStringList(raw any) ([]string, bool) {
	if raw == nil {
		return nil, false
	}
	switch v := raw.(type) {
	case string:
		// Compose string forms have quoting and shell semantics that a
		// Stackfile argv cannot preserve safely. Require an explicit list.
		return nil, true
	case []any:
		if len(v) == 0 {
			return nil, true
		}
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, true
			}
			out = append(out, s)
		}
		return out, false
	}
	return nil, true
}

func parseEnvFileRefs(raw any) []string {
	switch v := raw.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []any:
		refs := make([]string, 0, len(v))
		for _, item := range v {
			switch entry := item.(type) {
			case string:
				if entry != "" {
					refs = append(refs, entry)
				}
			case map[string]any:
				if path, ok := entry["path"].(string); ok && path != "" {
					refs = append(refs, path)
				}
			}
		}
		return refs
	case map[string]any:
		if path, ok := v["path"].(string); ok && path != "" {
			return []string{path}
		}
	}
	return nil
}

func parseBuild(raw any) (*BuildConfig, []string) {
	if raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case string:
		return &BuildConfig{Context: v}, nil
	case map[string]any:
		bc := &BuildConfig{}
		var unsupported []string
		for key, value := range v {
			switch key {
			case "context":
				if context, ok := value.(string); ok {
					bc.Context = context
				} else {
					unsupported = append(unsupported, key)
				}
			case "dockerfile":
				if dockerfile, ok := value.(string); ok {
					bc.Dockerfile = dockerfile
				} else {
					unsupported = append(unsupported, key)
				}
			default:
				if !strings.HasPrefix(key, "x-") {
					unsupported = append(unsupported, key)
				}
			}
		}
		sort.Strings(unsupported)
		return bc, unsupported
	}
	return nil, []string{"<value>"}
}

func parsePorts(ports []string) ([]PortDef, []string, []string) {
	if len(ports) == 0 {
		return nil, nil, nil
	}

	var defs []PortDef
	var unsupported []string
	var unsupportedMappings []string
	for _, p := range ports {
		def, mappingWarning := parsePort(p)
		if def != nil {
			defs = append(defs, *def)
			if mappingWarning {
				unsupportedMappings = append(unsupportedMappings, p)
			}
		} else {
			unsupported = append(unsupported, p)
		}
	}
	sort.Strings(unsupported)
	sort.Strings(unsupportedMappings)
	return defs, unsupported, unsupportedMappings
}

func parsePort(s string) (*PortDef, bool) {
	s = strings.TrimSpace(s)

	protocol := "TCP"
	if idx := strings.Index(s, "/"); idx != -1 {
		protocol = strings.ToUpper(s[idx+1:])
		s = s[:idx]
		if protocol != "TCP" {
			return nil, false
		}
	}

	hostIP, publishedPort, containerPort, hostMapped, ok := splitPortMapping(s)
	if !ok {
		return nil, false
	}
	port, err := strconv.ParseInt(containerPort, 10, 32)
	if err != nil || port <= 0 || port > 65535 {
		return nil, false
	}

	published := port
	if hostMapped {
		published, err = strconv.ParseInt(publishedPort, 10, 32)
		if err != nil || published <= 0 || published > 65535 {
			return nil, false
		}
	}

	def := &PortDef{
		Port:     int32(port),
		Protocol: protocol,
	}

	def.Name = portName(int32(port), protocol)

	mappingWarning := false
	if hostMapped {
		def.Public = true
		if published != port {
			mappingWarning = true
		}
		if hostIP != "" {
			ip := net.ParseIP(hostIP)
			if ip == nil {
				return nil, false
			}
			if !ip.IsUnspecified() {
				def.Public = false
				mappingWarning = true
			}
		}
	}

	return def, mappingWarning
}

func splitPortMapping(value string) (hostIP, published, container string, mapped, ok bool) {
	lastSeparator := strings.LastIndex(value, ":")
	if lastSeparator < 0 {
		if value == "" || strings.Contains(value, "-") {
			return "", "", "", false, false
		}
		return "", "", value, false, true
	}

	container = value[lastSeparator+1:]
	prefix := value[:lastSeparator]
	secondSeparator := strings.LastIndex(prefix, ":")
	if secondSeparator < 0 {
		if prefix == "" || container == "" || strings.Contains(prefix, "-") || strings.Contains(container, "-") {
			return "", "", "", false, false
		}
		return "", prefix, container, true, true
	}

	hostIP = prefix[:secondSeparator]
	published = prefix[secondSeparator+1:]
	if strings.HasPrefix(hostIP, "[") && strings.HasSuffix(hostIP, "]") {
		hostIP = strings.TrimSuffix(strings.TrimPrefix(hostIP, "["), "]")
	} else if strings.Contains(hostIP, ":") {
		return "", "", "", false, false
	}
	if hostIP == "" || published == "" || container == "" || strings.Contains(published, "-") || strings.Contains(container, "-") {
		return "", "", "", false, false
	}
	return hostIP, published, container, true, true
}

func portName(port int32, _ string) string {
	switch port {
	case 80, 8080, 8000, 3000:
		return "http"
	case 443:
		return "https"
	case 5432:
		return "postgres"
	case 3306:
		return "mysql"
	case 6379:
		return "redis"
	case 27017:
		return "mongo"
	case 9092:
		return "kafka"
	case 4222:
		return "nats"
	default:
		return fmt.Sprintf("port-%d", port)
	}
}

func parseEnvironment(raw any) (map[string]string, []string) {
	if raw == nil {
		return nil, nil
	}

	env := make(map[string]string)
	var unresolved []string

	switch v := raw.(type) {
	case map[string]any:
		for key, val := range v {
			if val == nil {
				unresolved = append(unresolved, key)
				continue
			}
			switch val.(type) {
			case map[string]any, []any:
				unresolved = append(unresolved, key)
				continue
			}
			env[key] = fmt.Sprintf("%v", val)
		}
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			key, val, found := strings.Cut(s, "=")
			if !found {
				if key != "" {
					unresolved = append(unresolved, key)
				}
				continue
			}
			env[key] = val
		}
	}

	sort.Strings(unresolved)
	if len(env) == 0 {
		env = nil
	}
	return env, unresolved
}

func parseVolumeMounts(volumes []string) ([]VolumeMountDef, []string, []string) {
	if len(volumes) == 0 {
		return nil, nil, nil
	}

	var (
		mounts           []VolumeMountDef
		unsupportedBinds []string
		unsupported      []string
	)
	for _, v := range volumes {
		source, target, mode, ok := splitVolumeMount(v)
		if !ok {
			unsupported = append(unsupported, v)
			continue
		}

		if isBindMountSource(source) {
			unsupportedBinds = append(unsupportedBinds, source)
			continue
		}

		mounts = append(mounts, VolumeMountDef{
			Name: source,
			Path: target,
		})
		if mode != "" {
			unsupported = append(unsupported, v)
		}
	}

	if len(mounts) == 0 {
		mounts = nil
	}
	sort.Strings(unsupportedBinds)
	sort.Strings(unsupported)
	return mounts, unsupportedBinds, unsupported
}

func splitVolumeMount(value string) (string, string, string, bool) {
	separator := strings.Index(value, ":")
	if isWindowsDrivePath(value) {
		rest := value[2:]
		next := strings.Index(rest, ":")
		if next < 0 {
			return "", "", "", false
		}
		separator = next + 2
	}
	if separator <= 0 || separator == len(value)-1 {
		return "", "", "", false
	}
	target := value[separator+1:]
	mode := ""
	if modeSeparator := strings.Index(target, ":"); modeSeparator >= 0 {
		mode = target[modeSeparator+1:]
		target = target[:modeSeparator]
	}
	if target == "" {
		return "", "", "", false
	}
	return value[:separator], target, mode, true
}

func isWindowsDrivePath(value string) bool {
	if len(value) < 3 || value[1] != ':' || (value[2] != '\\' && value[2] != '/') {
		return false
	}
	drive := value[0]
	return drive >= 'A' && drive <= 'Z' || drive >= 'a' && drive <= 'z'
}

func isBindMountSource(source string) bool {
	return strings.HasPrefix(source, "/") || strings.HasPrefix(source, ".") || isWindowsDrivePath(source)
}

func parseDependsOn(raw any) ([]string, []string) {
	if raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case []any:
		var deps []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				deps = append(deps, s)
			}
		}
		return deps, nil
	case map[string]any:
		var deps []string
		var unsupported []string
		for name, value := range v {
			deps = append(deps, name)
			switch options := value.(type) {
			case nil:
			case map[string]any:
				for key := range options {
					if !strings.HasPrefix(key, "x-") {
						unsupported = append(unsupported, name+"."+key)
					}
				}
			default:
				unsupported = append(unsupported, name)
			}
		}
		sort.Strings(deps)
		sort.Strings(unsupported)
		return deps, unsupported
	}
	return nil, []string{"<value>"}
}

func collectNamedVolumes(sf *Stackfile) {
	for _, res := range sf.Resources {
		for _, m := range res.Volumes {
			if _, exists := sf.Volumes[m.Name]; !exists {
				sf.Volumes[m.Name] = VolumeDef{Size: "1Gi"}
			}
		}
	}
	if len(sf.Volumes) == 0 {
		sf.Volumes = nil
	}
}
