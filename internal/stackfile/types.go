package stackfile

import "gopkg.in/yaml.v3"

type Stackfile struct {
	Name      string               `yaml:"name"`
	Resources map[string]Resource  `yaml:"resources"`
	Volumes   map[string]VolumeDef `yaml:"volumes,omitempty"`
}

type Resource struct {
	Image     string                           `yaml:"image,omitempty"`
	Build     *BuildConfig                     `yaml:"build,omitempty"`
	Ports     []PortDef                        `yaml:"ports,omitempty"`
	Env       map[string]string                `yaml:"env,omitempty"`
	Secrets   map[string]SecretMapping         `yaml:"secrets,omitempty"`
	Addons    map[string]AddonConnectionConfig `yaml:"addons,omitempty"`
	Volumes   []VolumeMountDef                 `yaml:"volumes,omitempty"`
	DependsOn []string                         `yaml:"depends_on,omitempty"`
	Stateful  bool                             `yaml:"stateful,omitempty"`
}

type BuildConfig struct {
	Repo       string `yaml:"repo"`
	Branch     string `yaml:"branch,omitempty"`
	Tag        string `yaml:"tag,omitempty"`
	Commit     string `yaml:"commit,omitempty"`
	Dockerfile string `yaml:"dockerfile,omitempty"`
	Context    string `yaml:"context,omitempty"`
}

type PortDef struct {
	Name      string `yaml:"name"`
	Port      int32  `yaml:"port"`
	Protocol  string `yaml:"protocol,omitempty"`
	Public    bool   `yaml:"public,omitempty"`
	Subdomain string `yaml:"subdomain,omitempty"`
}

type VolumeDef struct {
	Size       string `yaml:"size"`
	AccessMode string `yaml:"access_mode,omitempty"`
}

type VolumeMountDef struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// SecretMapping maps secret keys to env var names.
// Key = env var name, Value = secret key
type SecretMapping map[string]string

type AddonConnectionConfig struct {
	Type     string            `yaml:"type"`
	Env      map[string]string `yaml:"env"`
	Postgres *PostgresAddonConfig `yaml:"-"`

	rawNode yaml.Node `yaml:"-"`
}

type PostgresAddonConfig struct {
	Database  string `yaml:"database,omitempty"`
	Superuser bool   `yaml:"superuser,omitempty"`
}

func (a *AddonConnectionConfig) UnmarshalYAML(node *yaml.Node) error {
	a.rawNode = *node

	var base struct {
		Type string            `yaml:"type"`
		Env  map[string]string `yaml:"env"`
	}
	if err := node.Decode(&base); err != nil {
		return err
	}
	a.Type = base.Type
	a.Env = base.Env

	switch a.Type {
	case "postgres":
		var pg PostgresAddonConfig
		if err := node.Decode(&pg); err != nil {
			return err
		}
		a.Postgres = &pg
	}

	return nil
}
