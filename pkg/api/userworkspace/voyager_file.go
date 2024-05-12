package userworkspace

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Workspace struct {
	Resources map[string]*WorkspaceResourceSpec `yaml:",inline"`
	Volumes   map[string]*VolumeSpec            `yaml:"volumes"`
}

type VolumeSpec struct {
	Size   *string       `yaml:"size"`
	Source *VolumeSource `yaml:"source"`
}

type LocalDir struct {
	Path string `yaml:"path"`
	Sync bool   `yaml:"sync"`
}

type VolumeSource struct {
	LocalDir *LocalDir `yaml:"localDir"`
	// Git?
	// URL?
	// S3?
}
type WorkspaceResourceSpec struct {
	ImageRegistry        *string               `yaml:"imageRegistry"`
	Command              []string              `yaml:"command"`
	Args                 []string              `yaml:"args"`
	VolumeMounts         map[string]string     `yaml:"volumeMounts"`
	EnvironmentVariables map[string]string     `yaml:"environmentVariables"`
	EnvFiles             []string              `yaml:"envFiles"`
	DependsOn            []string              `yaml:"dependsOn"`
	Ports                []Port                `yaml:"ports"`
	Build                *ApplicationBuildSpec `yaml:"build" validate:"required_without=Image"`
	Image                *string               `yaml:"image" validate:"required_without=Build"`
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

type ApplicationBuildSpec struct {
	// Volume name where the applications source code is present
	SourceVolume string `yaml:"sourceVolume" validate:"required"`
	// Build context within the source volume.
	BuildContext string `yaml:"buildContext" validate:"required"`
	// Path within the volume where the dockerfile can be found.
	DockerFilePath *string `yaml:"dockerFilePath" validate:"required"`
	// Internal
	DirHash string
}

type PrebuiltApplicationSpec struct {
	Image string `yaml:"image" validate:"required"`
}

func Unmarshal(voyagerFilePath string) (*Workspace, error) {
	yamlFile, err := os.Open(voyagerFilePath)
	if err != nil {
		return nil, fmt.Errorf("error opening YAML file: %v\n", err)
	}
	defer yamlFile.Close()

	// Parse the YAML file
	var workspace Workspace
	decoder := yaml.NewDecoder(yamlFile)
	decoder.SetStrict(true)
	err = decoder.Decode(&workspace)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML file: %v\n", err)
	}
	return &workspace, nil
}

func (w *Workspace) SetDirHashForAllResources(hash string) {
	for _, resource := range w.Resources {
		if resource.Build != nil {
			resource.Build.DirHash = hash
		}
	}
}

func (r *WorkspaceResourceSpec) SetDirHash(hash string) {
	if r.Build != nil {
		r.Build.DirHash = hash
	}
}
