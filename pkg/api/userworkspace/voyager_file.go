package userworkspace

import (
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/pkg/tools"
	"gopkg.in/yaml.v2"
)

type Workspace map[string]*WorkspaceResourceSpec

type WorkspaceResourceSpec struct {
	StorageSize          *string                `yaml:"storageSize"`
	ImageRegistry        *string                `yaml:"imageRegistry"`
	Command              []string               `yaml:"command"`
	Args                 []string               `yaml:"args"`
	Mounts               map[string]string      `yaml:"mounts"`
	EnvironmentVariables map[string]string      `yaml:"environmentVariables"`
	EnvFiles             []string               `yaml:"envFiles"`
	DependsOn            []string               `yaml:"dependsOn"`
	Ports                []Port                 `yaml:"ports"`
	Source               *ApplicationSourceSpec `yaml:"source" validate:"required_without=Image"`
	Image                *string                `yaml:"image" validate:"required_without=Source"`
	// Internal
	NeedsSync bool
}

type Port struct {
	Number         int32 `yaml:"number" validate:"required"`
	ExposeToPublic bool  `yaml:"exposeToPublic"`
	IsHttp         bool  `yaml:"isHttp"`
}

type ResourceMounts struct {
	Source      string `yaml:"source" validate:"required"`
	Destination string `yaml:"destination" validate:"required"`
}

type ApplicationSourceSpec struct {
	SourceDir      string `yaml:"sourceDir" validate:"required"`
	BuildContext   string `yaml:"buildContext" validate:"required"`
	DockerFilePath string `yaml:"dockerFilePath" validate:"required"`
	// Internal
	DirHash string
}

type PrebuiltApplicationSpec struct {
	Image string `yaml:"image" validate:"required"`
}

func Unmarshal(voyagerFilePath string) (Workspace, error) {
	yamlFile, err := os.Open(voyagerFilePath)
	if err != nil {
		return nil, fmt.Errorf("error opening YAML file: %v\n", err)
	}
	defer yamlFile.Close()

	// Parse the YAML file
	var workspace Workspace
	err = yaml.NewDecoder(yamlFile).Decode(&workspace)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML file: %v\n", err)
	}
	return workspace, nil
}

func (a *ApplicationSourceSpec) LocalPath() string {
	return a.SourceDir
}

func (r *WorkspaceResourceSpec) SetAsReady() {
	r.NeedsSync = false
}

func (w *Workspace) SetDirHashForAllResources() {
	for _, resource := range *w {
		if resource.Source != nil {
			resource.Source.DirHash = tools.ComputeDirHash(resource.Source.SourceDir)
		}
	}
}

func (r *WorkspaceResourceSpec) SetDirHash() {
	if r.Source != nil {
		r.Source.DirHash = tools.ComputeDirHash(r.Source.SourceDir)
	}
}

func (w *Workspace) SetAsReady() {
	for _, resource := range *w {
		resource.NeedsSync = false
	}
}
