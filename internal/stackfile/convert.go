package stackfile

import (
	"regexp"
	"strings"

	openapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	"k8s.io/utils/ptr"
)

var envRefPattern = regexp.MustCompile(`\{\{\s*(\w+)\.(\S+)\s*\}\}`)

func (sf *Stackfile) ToStack() openapi.Stack {
	spec := openapi.StackSpec{
		StackResources: sf.buildResources(),
		Volumes:        sf.buildVolumes(),
		Connections:    sf.buildConnections(),
	}
	return openapi.Stack{
		Name: sf.Name,
		Spec: spec,
	}
}

func (sf *Stackfile) buildResources() []openapi.StackResource {
	resources := make([]openapi.StackResource, 0, len(sf.Resources))
	for name, res := range sf.Resources {
		sr := openapi.StackResource{
			Name:      name,
			DependsOn: res.DependsOn,
		}

		if res.Image != "" {
			sr.ImageSpec = &openapi.ImageSpec{Image: res.Image}
		}

		if res.Build != nil {
			sr.BuildSpec = buildSpec(res.Build)
		}

		if res.Stateful {
			sr.Stateful = ptr.To(true)
		}

		sr.Ports = buildPorts(res.Ports)
		sr.ExecutionConfig = buildExecutionConfig(res.Env)
		sr.VolumeMounts = buildVolumeMounts(res.Volumes)

		resources = append(resources, sr)
	}
	return resources
}

func buildSpec(b *BuildConfig) *openapi.StackResourceBuildSpec {
	spec := &openapi.StackResourceBuildSpec{
		ContextPathWithinSource: ".",
		DockerfilePath:          "Dockerfile",
		ImageRepository: openapi.ImageRepository{
			UseInternalRegistry: ptr.To(true),
		},
	}

	if b.Context != "" {
		spec.ContextPathWithinSource = b.Context
	}
	if b.Dockerfile != "" {
		spec.DockerfilePath = b.Dockerfile
	}

	spec.SourceContext = openapi.BuildSourceContext{
		GitRepo: &openapi.BuildSourceContextGitRepo{
			RepoUrl: b.Repo,
		},
	}

	revision := openapi.BuildSourceRevision{}
	if b.Branch != "" {
		revision.GitRepoRevision = &openapi.GitRepoRevision{
			Branch: &openapi.GitRepoRevisionBranch{
				Name: ptr.To(b.Branch),
			},
		}
	} else if b.Tag != "" {
		revision.GitRepoRevision = &openapi.GitRepoRevision{
			Tag: ptr.To(b.Tag),
		}
	} else if b.Commit != "" {
		revision.GitRepoRevision = &openapi.GitRepoRevision{
			Commit: ptr.To(b.Commit),
		}
	} else {
		revision.GitRepoRevision = &openapi.GitRepoRevision{
			Branch: &openapi.GitRepoRevisionBranch{
				Name: ptr.To("main"),
			},
		}
	}
	spec.SourceRevision = revision

	return spec
}

func buildPorts(ports []PortDef) []openapi.Port {
	if len(ports) == 0 {
		return nil
	}
	out := make([]openapi.Port, len(ports))
	for i, p := range ports {
		port := openapi.Port{
			Name:            p.Name,
			Number:          p.Port,
			ExposedToPublic: p.Public,
		}
		if p.Protocol != "" {
			port.Protocol = ptr.To(p.Protocol)
		}
		if p.Subdomain != "" {
			port.SubdomainPrefix = ptr.To(p.Subdomain)
		}
		out[i] = port
	}
	return out
}

func buildExecutionConfig(env map[string]string) *openapi.ExecutionConfig {
	if len(env) == 0 {
		return nil
	}

	var envVars []openapi.EnvVar
	for name, value := range env {
		ev := openapi.EnvVar{Name: name}

		if strings.HasPrefix(value, "{{ self.") && strings.HasSuffix(value, " }}") {
			output := strings.TrimPrefix(value, "{{ self.")
			output = strings.TrimSuffix(output, " }}")
			output = strings.TrimSpace(output)
			ev.SelfOutput = ptr.To(output)
		} else if !envRefPattern.MatchString(value) {
			ev.Value = ptr.To(value)
		}
		// {{ resource.output }} refs are handled via connections, skip here

		if ev.Value != nil || ev.SelfOutput != nil {
			envVars = append(envVars, ev)
		}
	}

	if len(envVars) == 0 {
		return nil
	}
	return &openapi.ExecutionConfig{
		EnvironmentVariables: envVars,
	}
}

func buildVolumeMounts(mounts []VolumeMountDef) []openapi.VolumeMount {
	if len(mounts) == 0 {
		return nil
	}
	out := make([]openapi.VolumeMount, len(mounts))
	for i, m := range mounts {
		out[i] = openapi.VolumeMount{
			SourceVolumeName: m.Name,
			TargetPath:       m.Path,
		}
	}
	return out
}

func (sf *Stackfile) buildVolumes() []openapi.Volume {
	if len(sf.Volumes) == 0 {
		return nil
	}
	volumes := make([]openapi.Volume, 0, len(sf.Volumes))
	for name, v := range sf.Volumes {
		accessMode := openapi.VolumeAccessMode("ReadWriteOnce")
		if v.AccessMode != "" {
			accessMode = openapi.VolumeAccessMode(v.AccessMode)
		}
		vol := openapi.Volume{
			Name: name,
			Spec: openapi.VolumeSpec{
				Size:               v.Size,
				AccessMode:         accessMode,
				NeedsSyncBeforeUse: false,
			},
		}
		volumes = append(volumes, vol)
	}
	return volumes
}

func (sf *Stackfile) buildConnections() []openapi.StackConnection {
	var connections []openapi.StackConnection

	for resourceName, res := range sf.Resources {
		connections = append(connections, buildEnvRefConnections(resourceName, res.Env)...)
		connections = append(connections, buildSecretConnections(resourceName, res.Secrets)...)
		connections = append(connections, buildAddonConnections(resourceName, res.Addons)...)
		connections = append(connections, buildVolumeMountConnections(resourceName, res.Volumes)...)
	}

	return connections
}

func buildEnvRefConnections(targetResource string, env map[string]string) []openapi.StackConnection {
	grouped := make(map[string][]openapi.ConnectionMapping)

	for envName, value := range env {
		matches := envRefPattern.FindStringSubmatch(value)
		if matches == nil || matches[1] == "self" {
			continue
		}

		source := matches[1]
		output := matches[2]

		mapping := openapi.ConnectionMapping{
			Target: openapi.ConnectionTarget{
				Type: "env",
				Name: ptr.To(envName),
			},
			Value: openapi.ValueRef{
				Output: ptr.To(output),
			},
		}
		grouped[source] = append(grouped[source], mapping)
	}

	var connections []openapi.StackConnection
	for source, mappings := range grouped {
		conn := openapi.StackConnection{
			Kind: "env",
			From: openapi.TopologyNodeRef{
				Type: "stack_resource",
				Name: ptr.To(source),
			},
			To: openapi.TopologyNodeRef{
				Type: "stack_resource",
				Name: ptr.To(targetResource),
			},
			Mappings: mappings,
		}
		connections = append(connections, conn)
	}
	return connections
}

func buildSecretConnections(targetResource string, secrets map[string]SecretMapping) []openapi.StackConnection {
	var connections []openapi.StackConnection

	for secretID, mapping := range secrets {
		var mappings []openapi.ConnectionMapping
		for envName, secretKey := range mapping {
			mappings = append(mappings, openapi.ConnectionMapping{
				Target: openapi.ConnectionTarget{
					Type: "env",
					Name: ptr.To(envName),
				},
				Value: openapi.ValueRef{
					Output: ptr.To(secretKey),
				},
			})
		}

		conn := openapi.StackConnection{
			Kind: "env",
			From: openapi.TopologyNodeRef{
				Type: "secret",
				Id:   ptr.To(secretID),
			},
			To: openapi.TopologyNodeRef{
				Type: "stack_resource",
				Name: ptr.To(targetResource),
			},
			Mappings: mappings,
		}
		connections = append(connections, conn)
	}
	return connections
}

func buildAddonConnections(targetResource string, addons map[string]AddonConnectionConfig) []openapi.StackConnection {
	var connections []openapi.StackConnection

	for addonID, addon := range addons {
		var mappings []openapi.ConnectionMapping
		for envName, tmpl := range addon.Env {
			vr := openapi.ValueRef{}
			if strings.Contains(tmpl, "{{") {
				vr.Template = ptr.To(tmpl)
				values := extractTemplateVars(tmpl)
				if len(values) > 0 {
					vr.Values = &values
				}
			} else {
				vr.Output = ptr.To(tmpl)
			}

			mappings = append(mappings, openapi.ConnectionMapping{
				Target: openapi.ConnectionTarget{
					Type: "env",
					Name: ptr.To(envName),
				},
				Value: vr,
			})
		}

		fromType := "addon/" + addon.Type
		conn := openapi.StackConnection{
			Kind: "env",
			From: openapi.TopologyNodeRef{
				Type: fromType,
				Id:   ptr.To(addonID),
			},
			To: openapi.TopologyNodeRef{
				Type: "stack_resource",
				Name: ptr.To(targetResource),
			},
			Mappings: mappings,
		}

		if addon.Postgres != nil {
			pg := addon.Postgres
			if pg.Database != "" || pg.Superuser {
				pgConfig := &openapi.PostgresEnvConfig{}
				if pg.Database != "" {
					pgConfig.Database = ptr.To(pg.Database)
				}
				if pg.Superuser {
					pgConfig.Superuser = ptr.To(true)
				}
				conn.Config = &openapi.StackConnectionConfig{
					PostgresEnvConfig: pgConfig,
				}
			}
		}

		connections = append(connections, conn)
	}
	return connections
}

var templateVarPattern = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

func extractTemplateVars(tmpl string) map[string]openapi.OutputValueRef {
	matches := templateVarPattern.FindAllStringSubmatch(tmpl, -1)
	if len(matches) == 0 {
		return nil
	}
	values := make(map[string]openapi.OutputValueRef)
	for _, m := range matches {
		varName := m[1]
		values[varName] = openapi.OutputValueRef{Output: varName}
	}
	return values
}

func buildVolumeMountConnections(targetResource string, mounts []VolumeMountDef) []openapi.StackConnection {
	var connections []openapi.StackConnection
	for _, m := range mounts {
		conn := openapi.StackConnection{
			Kind: "volume_mount",
			From: openapi.TopologyNodeRef{
				Type: "volume",
				Name: ptr.To(m.Name),
			},
			To: openapi.TopologyNodeRef{
				Type: "stack_resource",
				Name: ptr.To(targetResource),
			},
			Config: &openapi.StackConnectionConfig{
				VolumeMountConfig: &openapi.VolumeMountConfig{
					MountPath: m.Path,
				},
			},
		}
		connections = append(connections, conn)
	}
	return connections
}
