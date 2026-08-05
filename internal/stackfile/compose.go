package stackfile

import (
	"fmt"
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
}

type composeService struct {
	Image       string            `yaml:"image"`
	Build       any               `yaml:"build"`
	Command     any               `yaml:"command"`
	Entrypoint  any               `yaml:"entrypoint"`
	Ports       []string          `yaml:"ports"`
	Environment any               `yaml:"environment"`
	EnvFile     any               `yaml:"env_file"`
	Volumes     []string          `yaml:"volumes"`
	DependsOn   any               `yaml:"depends_on"`
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
// return value maps resource name -> the compose `env_file` it referenced:
// yaml.Marshal of a Stackfile cannot emit that key, so the caller reports them
// instead of dropping them silently.
func FromCompose(path, appName string) (*Stackfile, map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if len(compose.Services) == 0 {
		return nil, nil, fmt.Errorf("no services found in %s", path)
	}

	sf := &Stackfile{
		Name:      appName,
		Resources: make(map[string]Resource),
		Volumes:   make(map[string]VolumeDef),
	}
	envFiles := make(map[string]string)

	for name, svc := range compose.Services {
		sf.Resources[name] = convertService(svc)
		if ref := parseEnvFileRef(svc.EnvFile); ref != "" {
			envFiles[name] = ref
		}
	}

	for volName := range compose.Volumes {
		sf.Volumes[volName] = VolumeDef{Size: "1Gi"}
	}

	collectNamedVolumes(sf)

	return sf, envFiles, nil
}

func convertService(svc composeService) Resource {
	var res Resource

	res.Image = svc.Image
	res.Build = parseBuild(svc.Build)
	res.Command, res.Args = parseCommandArgs(svc.Entrypoint, svc.Command)
	res.Ports = parsePorts(svc.Ports)
	res.Env = parseEnvironment(svc.Environment)
	res.Volumes = parseVolumeMounts(svc.Volumes)
	res.DependsOn = parseDependsOn(svc.DependsOn)

	return res
}

func parseCommandArgs(entrypoint, command any) (cmd []string, args []string) {
	ep := parseStringOrList(entrypoint)
	c := parseStringOrList(command)

	if len(ep) > 0 {
		// Both set: entrypoint → command, command → args
		cmd = ep
		args = c
	} else {
		// Only command set: maps to command (overrides the container's default)
		cmd = c
	}
	return
}

func parseStringOrList(raw any) []string {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case string:
		fields := strings.Fields(v)
		if len(fields) == 0 {
			return nil
		}
		return fields
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func parseEnvFileRef(raw any) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

func parseBuild(raw any) *BuildConfig {
	if raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case string:
		return &BuildConfig{Context: v}
	case map[string]any:
		bc := &BuildConfig{}
		if ctx, ok := v["context"].(string); ok {
			bc.Context = ctx
		}
		if df, ok := v["dockerfile"].(string); ok {
			bc.Dockerfile = df
		}
		return bc
	}
	return nil
}

func parsePorts(ports []string) []PortDef {
	if len(ports) == 0 {
		return nil
	}

	var defs []PortDef
	for _, p := range ports {
		def := parsePort(p)
		if def != nil {
			defs = append(defs, *def)
		}
	}
	return defs
}

func parsePort(s string) *PortDef {
	s = strings.TrimSpace(s)

	protocol := ""
	if idx := strings.Index(s, "/"); idx != -1 {
		protocol = strings.ToUpper(s[idx+1:])
		s = s[:idx]
	}

	parts := strings.Split(s, ":")
	var containerPort string
	hostMapped := false

	switch len(parts) {
	case 1:
		containerPort = parts[0]
	case 2:
		containerPort = parts[1]
		hostMapped = true
	case 3:
		containerPort = parts[2]
		hostMapped = true
	default:
		return nil
	}

	portRange := strings.Split(containerPort, "-")
	port, err := strconv.ParseInt(portRange[0], 10, 32)
	if err != nil {
		return nil
	}

	def := &PortDef{
		Port: int32(port),
	}

	if protocol != "" && protocol != "TCP" {
		def.Protocol = protocol
	}

	def.Name = portName(int32(port), protocol)

	if hostMapped && !isBackingServicePort(int32(port)) {
		def.Public = true
	}

	return def
}

func isBackingServicePort(port int32) bool {
	switch port {
	case 5432:  // PostgreSQL
		return true
	case 3306:  // MySQL / MariaDB
		return true
	case 6379:  // Redis
		return true
	case 27017: // MongoDB
		return true
	case 9092:  // Kafka
		return true
	case 4222:  // NATS
		return true
	case 2181:  // ZooKeeper
		return true
	case 9200:  // Elasticsearch / OpenSearch
		return true
	case 5672:  // RabbitMQ (AMQP)
		return true
	case 11211: // Memcached
		return true
	case 8123:  // ClickHouse (HTTP)
		return true
	case 9000:  // ClickHouse (native)
		return true
	case 6650:  // Apache Pulsar
		return true
	case 7687:  // Neo4j (Bolt)
		return true
	case 8529:  // ArangoDB
		return true
	case 9042:  // Cassandra (CQL)
		return true
	case 7000:  // Cassandra (inter-node)
		return true
	case 6380:  // KeyDB / Valkey
		return true
	case 26257: // CockroachDB
		return true
	case 28015: // RethinkDB
		return true
	case 8086:  // InfluxDB
		return true
	case 1433:  // SQL Server
		return true
	case 1521:  // Oracle DB
		return true
	case 6363:  // Milvus (vector DB)
		return true
	case 19530: // Milvus (gRPC)
		return true
	case 6333:  // Qdrant (vector DB)
		return true
	case 8484:  // Weaviate (vector DB)
		return true
	}
	return false
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

func parseEnvironment(raw any) map[string]string {
	if raw == nil {
		return nil
	}

	env := make(map[string]string)

	switch v := raw.(type) {
	case map[string]any:
		for key, val := range v {
			env[key] = fmt.Sprintf("%v", val)
		}
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			key, val, _ := strings.Cut(s, "=")
			env[key] = val
		}
	}

	if len(env) == 0 {
		return nil
	}
	return env
}

func parseVolumeMounts(volumes []string) []VolumeMountDef {
	if len(volumes) == 0 {
		return nil
	}

	var mounts []VolumeMountDef
	for _, v := range volumes {
		parts := strings.SplitN(v, ":", 3)
		if len(parts) < 2 {
			continue
		}
		source := parts[0]
		target := parts[1]

		if strings.HasPrefix(source, "/") || strings.HasPrefix(source, ".") {
			continue
		}

		mounts = append(mounts, VolumeMountDef{
			Name: source,
			Path: target,
		})
	}

	if len(mounts) == 0 {
		return nil
	}
	return mounts
}

func parseDependsOn(raw any) []string {
	if raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case []any:
		var deps []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				deps = append(deps, s)
			}
		}
		return deps
	case map[string]any:
		var deps []string
		for name := range v {
			deps = append(deps, name)
		}
		sort.Strings(deps)
		return deps
	}
	return nil
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
