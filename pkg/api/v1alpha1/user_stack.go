package v1alpha1

import (
	"fmt"
	"os"

	"github.com/hashicorp/go-envparse"
	"gopkg.in/yaml.v2"
)

type UserStack struct {
	Name      string
	Resources map[string]*WorkspaceResourceSpec `yaml:",inline"`
	Volumes   map[string]*VolumeSpec            `yaml:"volumes"`
}

func (w *UserStack) WorkspaceStorageName() string {
	return fmt.Sprintf("%s-%s", w.Name, "storage")
}

func (w *UserStack) HasSyncingVolumes() bool {
	for _, volume := range w.Volumes {
		if volume.Source != nil && volume.Source.LocalDir != nil {
			return true
		}
	}
	return false
}

func (w *UserStack) HasVolumes() bool {
	return len(w.Volumes) > 0
}

type VolumeSpec struct {
	Size   string        `yaml:"size"`
	Source *VolumeSource `yaml:"source"`
}

type LocalDir struct {
	Path string `yaml:"path"`
	Sync bool   `yaml:"sync"`
}

type VolumeSource struct {
	LocalDir       *LocalDir              `yaml:"localDir"`
	BuildArtifacts []*BuildArtifactSource `yaml:"buildArtifacts,omitempty"`
	// URL?
	// S3?
}
type BuildArtifactSource struct {
	ResourceName    string `yaml:"resourceName"`
	SourcePath      string `yaml:"sourcePath"`
	DestinationPath string `yaml:"destinationPath"`
}

type WorkspaceResourceSpec struct {
	ImageRegistry        *string               `yaml:"imageRegistry"`
	Command              []string              `yaml:"command"`
	Args                 []string              `yaml:"args"`
	Init                 *InitCommand          `yaml:"init"`
	VolumeMounts         map[string]string     `yaml:"volumeMounts"`
	EnvironmentVariables map[string]string     `yaml:"environmentVariables"`
	EnvFiles             []string              `yaml:"envFiles"`
	DependsOn            []string              `yaml:"dependsOn"`
	Ports                []Port                `yaml:"ports"`
	Build                *ApplicationBuildSpec `yaml:"build" validate:"required_without=Image"`
	Image                *string               `yaml:"image" validate:"required_without=Build"`
}

type InitCommand struct {
	Command []string `yaml:"command" validate:"required"`
	Args    []string `yaml:"args"`
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

type ResourceStatus struct {
	ResourceName string
	Available    bool
	Reason       string
	Message      string
	Addresses    []Address
	BuildStatus  *BuildStatus
}

type BuildStatus struct {
	BuildName  string
	SourceHash string
	Completed  bool
	Reason     string
	Message    string
}

type VolumeStatus struct {
	VolumeName   string
	LocalPath    *string
	LastSyncedAt *string
	Available    bool
}

type Address struct {
	Port int
	Url  string
}

type WorkspaceAvailablityStatus struct {
	Available bool
	Reason    string
	Message   string
}

func Unmarshal(voyagerFilePath string) (*UserStack, error) {
	yamlFile, err := os.Open(voyagerFilePath)
	if err != nil {
		return nil, fmt.Errorf("error opening YAML file: %v\n", err)
	}
	defer yamlFile.Close()

	// Parse the YAML file
	var workspace UserStack
	decoder := yaml.NewDecoder(yamlFile)
	decoder.SetStrict(true)
	err = decoder.Decode(&workspace)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML file: %v\n", err)
	}
	return &workspace, nil
}

func (w *UserStack) SetDirHashForAllResources(hash string) {
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

func (w *UserStack) ReadEnvFiles() error {
	for _, spec := range w.Resources {
		for _, envFile := range spec.EnvFiles {
			file, err := os.Open(envFile)
			if err != nil {
				return err
			}
			envVarsFromFile, err := envparse.Parse(file)
			if err != nil {
				return err
			}
			if spec.EnvironmentVariables == nil {
				spec.EnvironmentVariables = map[string]string{}
			}
			for key, value := range envVarsFromFile {
				spec.EnvironmentVariables[key] = value
			}
		}
	}
	return nil
}
