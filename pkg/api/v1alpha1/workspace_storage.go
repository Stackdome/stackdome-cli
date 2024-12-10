package v1alpha1

import (
	"time"

	serverapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
)

const (
	WorkspaceStorageAvailableCondition = "Available"
)

// List of VolumeSourceTypes

type VolumeSourceType string

const (
	LOCAL          VolumeSourceType = VolumeSourceType(serverapi.LOCAL_SYNCED_VOLUME)
	BUILD_ARTIFACT VolumeSourceType = VolumeSourceType(serverapi.BUILD_ARTIFACT_SYNCED_VOLUME)
	EMPTY          VolumeSourceType = VolumeSourceType(serverapi.EMPTY_VOLUME)
)

type WorkspaceStorage struct {
	ID             string
	OrganisationID string
	Name           string
	Namespace      string
	Labels         []KeyValue
	Annotations    []KeyValue
	Version        int32
	WorkspaceName  string
	Volumes        []Volume
	Status         *WorkspaceStorageStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (w *WorkspaceStorage) HasLocalSyncingVolumes() bool {
	for _, volume := range w.Volumes {
		if volume.VolumeSource != nil && volume.VolumeSource.LocalDir != nil {
			return true
		}
	}
	return false
}

type WorkspaceStorageStatus struct {
	ObservedVersion    int32
	Conditions         []Condition
	State              string
	StorageServiceName string
}

type Volume struct {
	Name          string
	Labels        []KeyValue
	Annotations   []KeyValue
	Size          string
	StorageClass  string
	SyncBeforeUse bool
	VolumeSource  *VolumeSource
	Status        *WorkspaceVolumeStatus
}

type WorkspaceVolumeStatus struct {
	Conditions         []Condition
	Phase              string
	BuildArtifactSyncs []BuildArtifactSyncInfo
}

type BuildArtifactSyncInfo struct {
	ResourceName string
	BuildId      string
	Status       string
}

type KeyValue struct {
	Key   string
	Value string
}

func (w *WorkspaceStorage) IsAvailable() bool {
	return w.Status != nil && w.Status.IsAvailable() && w.Version == w.Status.ObservedVersion
}

func (w WorkspaceStorageStatus) IsAvailable() bool {
	cond := GetCondition(w.Conditions, WorkspaceStorageAvailableCondition)
	return cond != nil && cond.Status == "True"
}
