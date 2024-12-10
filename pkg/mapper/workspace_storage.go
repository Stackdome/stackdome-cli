package mapper

import (
	serverapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	clientapi "github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
)

type KeyValuer interface {
	GetKey() string
	GetValue() string
}

func WorkspaceStorageFromUserStack(in *clientapi.UserStack) serverapi.WorkspaceStorage {
	return serverapi.WorkspaceStorage{
		Name: in.WorkspaceStorageName(),
		Spec: serverapi.WorkspaceStorageSpec{
			WorkspaceName: in.Name,
			Volumes:       ToServerWorkspaceVolumes(in.Volumes),
		},
	}
}

func ToClientWorkspaceStorages(in []serverapi.WorkspaceStorage) []*clientapi.WorkspaceStorage {
	res := make([]*clientapi.WorkspaceStorage, 0)
	for _, ws := range in {
		res = append(res, ToClientWorkspaceStorage(&ws))
	}
	return res
}

func ToClientWorkspaceStorage(in *serverapi.WorkspaceStorage) *clientapi.WorkspaceStorage {
	return &clientapi.WorkspaceStorage{
		ID:             in.GetId(),
		OrganisationID: in.GetOrganisationId(),
		Name:           in.Name,
		Namespace:      in.GetNamespace(),
		Labels:         ServerLabelsToClientKeyValues(in.GetLabels()),
		Annotations:    ServerAnnotationsToClientKeyValues(in.GetAnnotations()),
		Version:        in.GetVersion(),
		WorkspaceName:  in.Spec.WorkspaceName,
		CreatedAt:      in.GetCreatedAt(),
		UpdatedAt:      in.GetUpdatedAt(),
		Volumes:        ToClientVolumes(in.Spec.Volumes),
		Status:         ToClientWorkspaceStorageStatus(in.GetStatus()),
	}
}

func ToClientWorkspaceStorageStatus(in serverapi.WorkspaceStorageStatus) *clientapi.WorkspaceStorageStatus {
	return &clientapi.WorkspaceStorageStatus{
		ObservedVersion:    in.GetObservedVersion(),
		Conditions:         ToClientAPIConditions(in.GetConditions()),
		State:              string(in.GetState()),
		StorageServiceName: in.GetStorageServerServiceName(),
	}
}

func ToClientVolumes(in []serverapi.Volume) []clientapi.Volume {
	res := make([]clientapi.Volume, 0)
	for _, vol := range in {
		res = append(res, ToClientVolume(&vol))
	}
	return res
}

func ToClientVolume(in *serverapi.Volume) clientapi.Volume {
	return clientapi.Volume{
		Name:          in.Name,
		Labels:        ServerLabelsToClientKeyValues(in.GetLabels()),
		Annotations:   ServerAnnotationsToClientKeyValues(in.GetAnnotations()),
		Size:          in.Spec.GetSize(),
		StorageClass:  in.Spec.GetStorageClass(),
		SyncBeforeUse: in.Spec.GetSyncBeforeUse(),
		VolumeSource:  ToClientVolumeSource(in.Spec.GetSource()),
		Status:        ToClientVolumeStatus(in.GetStatus()),
	}
}

func ToClientVolumeStatus(in serverapi.VolumeStatus) *clientapi.WorkspaceVolumeStatus {
	return &clientapi.WorkspaceVolumeStatus{
		Conditions:         ToClientAPIConditions(in.GetConditions()),
		Phase:              in.GetPhase(),
		BuildArtifactSyncs: ToClientBuildArtifactSyncInfos(in.GetBuildArtifactSyncs()),
	}
}

func ToClientBuildArtifactSyncInfos(in []serverapi.BuildArtifactSyncInfo) []clientapi.BuildArtifactSyncInfo {
	res := make([]clientapi.BuildArtifactSyncInfo, 0)
	for _, syncInfo := range in {
		res = append(res, clientapi.BuildArtifactSyncInfo{
			ResourceName: syncInfo.GetResourceName(),
			BuildId:      syncInfo.GetBuildId(),
			Status:       syncInfo.GetStatus(),
		})
	}
	return res
}

func ToClientVolumeSource(in serverapi.VolumeSource) *clientapi.VolumeSource {
	switch in.SourceType {
	case serverapi.LOCAL:
		return &clientapi.VolumeSource{
			LocalDir: &clientapi.LocalDir{
				Path: in.LocalSource.GetPath(),
				Sync: in.LocalSource.GetSync(),
			},
		}
	case serverapi.BUILD_ARTIFACT:
		return &clientapi.VolumeSource{
			BuildArtifacts: ToClientBuildArtifactSources(in.BuildSource),
		}
	default:
		return &clientapi.VolumeSource{}
	}
}

func ToClientBuildArtifactSources(in []serverapi.BuildArtifact) []*clientapi.BuildArtifactSource {
	res := make([]*clientapi.BuildArtifactSource, 0)
	for _, artifact := range in {
		res = append(res, &clientapi.BuildArtifactSource{
			ResourceName:    artifact.GetResourceRef(),
			SourcePath:      artifact.GetSourcePath(),
			DestinationPath: artifact.GetDestinationPath(),
		})
	}
	return res
}

func ToClientKeyValue[T KeyValuer](in T) clientapi.KeyValue {
	return clientapi.KeyValue{
		Key:   in.GetKey(),
		Value: in.GetValue(),
	}
}

func ServerLabelsToClientKeyValues(in []serverapi.Label) []clientapi.KeyValue {
	res := make([]clientapi.KeyValue, 0)
	for _, kv := range in {
		res = append(res, ToClientKeyValue(&kv))
	}
	return res
}

func ServerAnnotationsToClientKeyValues(in []serverapi.Annotation) []clientapi.KeyValue {
	res := make([]clientapi.KeyValue, 0)
	for _, kv := range in {
		res = append(res, ToClientKeyValue(&kv))
	}
	return res
}
