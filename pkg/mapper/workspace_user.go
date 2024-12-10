package mapper

import (
	"fmt"

	serverapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	clientapi "github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"k8s.io/utils/ptr"
)

func ToServerWorkspaceStorage(in *clientapi.UserStack) serverapi.WorkspaceStorage {
	return serverapi.WorkspaceStorage{
		Name: in.WorkspaceStorageName(),
		Spec: serverapi.WorkspaceStorageSpec{
			WorkspaceName: in.Name,
			Volumes:       ToServerWorkspaceVolumes(in.Volumes),
		},
	}
}

func ToServerWorkspaceVolumes(in map[string]*clientapi.VolumeSpec) []serverapi.Volume {
	res := make([]serverapi.Volume, 0)
	for name, spec := range in {
		currentVolume := serverapi.Volume{
			Name: name,
			Spec: serverapi.WorkspaceVolumeSpec{
				Size: spec.Size,
			},
		}
		switch {
		case spec.Source != nil && spec.Source.LocalDir != nil:
			currentVolume.Spec.SyncBeforeUse = ptr.To(true)
			currentVolume.Spec.Source = &serverapi.VolumeSource{
				SourceType: serverapi.LOCAL,
				LocalSource: &serverapi.LocalSource{
					Path: spec.Source.LocalDir.Path,
					Sync: true,
				},
			}
		case spec.Source != nil && spec.Source.BuildArtifacts != nil:
			currentVolume.Spec.SyncBeforeUse = ptr.To(true)
			currentVolume.Spec.Source = &serverapi.VolumeSource{
				SourceType:  serverapi.BUILD_ARTIFACT,
				BuildSource: ToServerBuildArtifactVolumeSources(spec.Source.BuildArtifacts),
			}
		}
		res = append(res, currentVolume)
	}
	return res
}

func ToServerBuildArtifactVolumeSources(in []*clientapi.BuildArtifactSource) []serverapi.BuildArtifact {
	res := make([]serverapi.BuildArtifact, 0)
	for _, artifact := range in {
		res = append(res, serverapi.BuildArtifact{
			ResourceRef:     artifact.ResourceName,
			SourcePath:      artifact.SourcePath,
			DestinationPath: artifact.DestinationPath,
		})
	}
	return res
}

func ToClientAPIUser(in *serverapi.User) *clientapi.User {
	return &clientapi.User{
		Id:             in.GetId(),
		Name:           in.GetName(),
		Username:       in.GetUsername(),
		Email:          in.GetEmail(),
		Organisation:   in.GetOrganisation(),
		Role:           in.GetRole(),
		OrganisationID: in.GetOrganisationId(),
	}
}

func ToServerAPIWorkspaceUser(in *clientapi.WorkspaceUser) serverapi.WorkspaceUser {
	return serverapi.WorkspaceUser{
		SshPublicKey: in.SshPublicKey,
		Workspaces:   in.Workspaces,
	}
}

func ToClientAPIWorkspaceUser(in *serverapi.WorkspaceUser) *clientapi.WorkspaceUser {
	return &clientapi.WorkspaceUser{
		ID:           in.GetId(),
		UserID:       in.GetUserId(),
		OrgID:        in.GetOrgId(),
		SshPublicKey: in.SshPublicKey,
		Workspaces:   in.Workspaces,
		Version:      in.GetVersion(),
		State:        string(in.GetState()),
		Message:      in.GetMessage(),
		Status:       ToClientAPIWorkspaceUserStatus(in.Status),
	}
}

func ToClientAPIWorkspaceUserStatus(in *serverapi.WorkspaceUserStatus) *clientapi.WorkspaceUserStatus {
	if in == nil {
		return &clientapi.WorkspaceUserStatus{}
	}
	return &clientapi.WorkspaceUserStatus{
		ObservedVersion:       in.GetObservedVersion(),
		ProvisionedWorkspaces: ToClientAPIProvisionedWorkspaces(in.GetProvisionedNamespaces()),
		ServiceAccountname:    in.GetServiceAccountName(),
		ServiceAccountToken:   in.GetServiceaccountToken(),
		ClusterCaCert:         in.GetClusterCaCert(),
		ClusterUrl:            in.GetClusterUrl(),
		Conditions:            ToClientAPIConditions(in.GetConditions()),
	}
}

func ToClientAPIProvisionedWorkspaces(in []serverapi.WorkspaceUserStatusProvisionedNamespacesInner) []clientapi.ProvisionedWorkspace {
	res := make([]clientapi.ProvisionedWorkspace, 0)
	for _, ns := range in {
		res = append(res, clientapi.ProvisionedWorkspace{
			WorkspaceName: ns.GetWorkspaceName(),
			Namespace:     ns.GetNamespace(),
		})
	}
	return res
}

func ToClientAPIConditions(in []serverapi.Condition) []clientapi.Condition {
	res := make([]clientapi.Condition, 0)
	for _, cond := range in {
		res = append(res, clientapi.Condition{
			Type:               cond.GetType(),
			Status:             cond.GetStatus(),
			LastTransitionTime: cond.GetLastTransitionTime(),
			Reason:             cond.GetReason(),
			Message:            cond.GetMessage(),
		})
	}
	return res
}

func WorkspaceCRName(userName string) string {
	return fmt.Sprintf("%s-workspace", userName)
}

func WorkspaceStorageName(username string) string {
	return fmt.Sprintf("%s-storage", username)
}

// func MapVoyagerFileToWorkspaceCR(in voyagerfile.Workspace, username, namespace, organisation, domain string) *workspacev1alpha1.Workspace {
// 	resourceSpecList := make([]workspacev1alpha1.ResourceSpec, 0)
// 	for resourceName, userSpec := range in.Resources {
// 		currResourceSpec := workspacev1alpha1.ResourceSpec{
// 			Name: resourceName,
// 			Spec: workspacev1alpha1.WorkspaceResourceSpec{
// 				ImageRegistry: userSpec.ImageRegistry,
// 				Command:       userSpec.Command,
// 				Args:          userSpec.Args,
// 				DependsOn:     userSpec.DependsOn,
// 			},
// 		}
// 		if userSpec.Init != nil {
// 			currResourceSpec.Spec.Init = &workspacev1alpha1.WorkspaceResourceInit{
// 				Command: userSpec.Init.Command,
// 				Args:    userSpec.Init.Args,
// 			}
// 		}
// 		if userSpec.Image != nil {
// 			currResourceSpec.Spec.PrebuiltApplicationSpec = &workspacev1alpha1.PrebuiltApplicationSpec{
// 				Image: *userSpec.Image,
// 			}
// 		} else {
// 			currResourceSpec.Spec.ApplicationBuildSpec = &workspacev1alpha1.ApplicationBuildSpec{
// 				Context:    userSpec.Build.BuildContext,
// 				VolumeName: userSpec.Build.SourceVolume,
// 			}
// 			if userSpec.Build.DockerFilePath != nil {
// 				currResourceSpec.Spec.ApplicationBuildSpec.DockerFile = *userSpec.Build.DockerFilePath
// 			}
// 		}
// 		currResourceSpec.Spec.EnvironmentVariables = mapEnvs(userSpec.EnvironmentVariables)
// 		currResourceSpec.Spec.Ports = mapPorts(userSpec.Ports)
// 		currResourceSpec.Spec.VolumeMounts = mapMounts(userSpec.VolumeMounts)
// 		resourceSpecList = append(resourceSpecList, currResourceSpec)
// 	}

// 	ws := workspacev1alpha1.Workspace{
// 		ObjectMeta: v1.ObjectMeta{
// 			Name:      WorkspaceCRName(username),
// 			Namespace: namespace,
// 		},
// 		Spec: workspacev1alpha1.WorkspaceSpec{
// 			Resources:    resourceSpecList,
// 			UserName:     username,
// 			Organisation: organisation,
// 			Domain:       domain,
// 		},
// 	}
// 	return &ws
// }

// func mapEnvs(in map[string]string) []workspacev1alpha1.EnvironmentVariables {
// 	res := make([]workspacev1alpha1.EnvironmentVariables, 0)
// 	for name, value := range in {
// 		res = append(res, workspacev1alpha1.EnvironmentVariables{
// 			Name:  name,
// 			Value: value,
// 		})
// 	}
// 	return res
// }

// func mapMounts(in map[string]string) []workspacev1alpha1.VolumeMount {
// 	res := make([]workspacev1alpha1.VolumeMount, 0)
// 	for src, dst := range in {
// 		res = append(res, workspacev1alpha1.VolumeMount{
// 			Source:      src,
// 			Destination: dst,
// 		})
// 	}
// 	return res
// }

// func mapPorts(in []voyagerfile.Port) []workspacev1alpha1.Port {
// 	res := make([]workspacev1alpha1.Port, 0)
// 	for _, portDefn := range in {
// 		res = append(res, workspacev1alpha1.Port{
// 			Number:         portDefn.Number,
// 			IsHttp:         portDefn.IsHttp,
// 			ExposeToPublic: portDefn.ExposeToPublic,
// 		})
// 	}
// 	return res
// }
