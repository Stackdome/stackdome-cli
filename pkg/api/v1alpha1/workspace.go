package v1alpha1

import "time"

const (
	WorkspaceAvailableCondition         = "Available"
	WorkspaceResourceAvailableCondition = "Available"
)

type Workspace struct {
	ID          string
	Name        string
	Namespace   string
	Labels      []KeyValue
	Annotations []KeyValue
	Version     int32
	Resources   []WorkspaceResource
	Status      *WorkspaceStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (w *Workspace) HasLocalSyncingVolumes() bool {
	for _, resource := range w.Resources {
		for _, volumeMount := range resource.VolumeMounts {
			if volumeMount.SourceVolumeType == LOCAL {
				return true
			}
		}
	}
	return false
}

func (w *Workspace) ResourceHasLocalSyncingVolume(resourceName string) bool {
	resource := w.GetResourceByName(resourceName)
	if resource == nil {
		return false
	}
	for _, volumeMount := range resource.VolumeMounts {
		if volumeMount.SourceVolumeType == LOCAL {
			return true
		}
	}
	return false
}

func (w *Workspace) GetResourceByName(name string) *WorkspaceResource {
	for _, resource := range w.Resources {
		if resource.Name == name {
			return &resource
		}
	}
	return nil
}

func (w *Workspace) IsAvailable() bool {
	return w.Status != nil && w.Status.IsAvailable() && w.Version == w.Status.ObservedVersion
}

func (w *WorkspaceStatus) IsAvailable() bool {
	availableCond := GetCondition(w.Conditions, WorkspaceAvailableCondition)
	return availableCond != nil && availableCond.Status == "True"
}

type WorkspaceStatus struct {
	ObservedVersion int32
	State           string
	Conditions      []Condition
}

type WorkspaceResource struct {
	ID              string
	Name            string
	Labels          []KeyValue
	Annotations     []KeyValue
	Version         int32
	ImageRegistry   *string
	BuildConfig     *BuildConfig
	PreBuiltImage   *PreBuiltImage
	Init            *InitConfig
	ExecutionConfig *ExecutionConfig
	LifecycleConfig *LifecycleConfig
	VolumeMounts    []VolumeMount
	Ports           []Port
	Stateful        bool
	DependsOn       []string
	Status          *WorkspaceResourceStatus
}

func (wr *WorkspaceResource) IsAvailable() bool {
	return wr.Status != nil && wr.Status.IsAvailable() && wr.Version == wr.Status.ObservedVersion
}

func (wr *WorkspaceResourceStatus) IsAvailable() bool {
	availableCond := GetCondition(wr.Conditions, WorkspaceResourceAvailableCondition)
	return availableCond != nil && availableCond.Status == "True"
}

type BuildConfig struct {
	SourceVolumeID string
	ContextPath    string
	DockerFilePath string
	ContextDirHash string
}

type PreBuiltImage struct {
	Image string
}

type InitConfig struct {
	Command []string
	Args    []string
}

type ExecutionConfig struct {
	Command              []string
	Args                 []string
	EnvironmentVariables []KeyValue
}

type LifecycleConfig struct {
	LastRestartRequestTime *time.Time
}

type VolumeMount struct {
	SourceVolumeID   string
	SourceVolumeType VolumeSourceType
	SourceSubPath    *string
	TargetPath       string
}

type WorkspaceResourceStatus struct {
	ObservedVersion     int32
	InternalServiceName *string
	State               string
	Conditions          []Condition
	PublicIngresses     []Ingress
}

type Ingress struct {
	URL        string
	TargetPort int32
}
