package mapper

import (
	"fmt"

	voyagerfile "github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

func WorkspaceCRName(userName string) string {
	return fmt.Sprintf("%s-workspace", userName)
}

const DefaultSize = "2Gi"

func MapVoyagerFileToWorkspaceStorage(in voyagerfile.Workspace, user string, namespace string) workspacev1alpha1.WorkspaceStorage {
	resourceStorageSpecList := make([]workspacev1alpha1.ResourceStorageSpec, 0)
	for volumeName, spec := range in.Volumes {
		currSpec := workspacev1alpha1.ResourceStorageSpec{
			VolumeName: volumeName,
		}
		if spec.Size != nil {
			currSpec.Size = *spec.Size
		} else {
			currSpec.Size = DefaultSize
		}
		if spec.Source != nil {
			currSpec.Type = workspacev1alpha1.SyncingStorageType
			if spec.Source.LocalDir.Synced {
				currSpec.NeedsSync = false
			} else {
				currSpec.NeedsSync = true
			}
		} else {
			currSpec.Type = workspacev1alpha1.EmptyStorageType
			currSpec.NeedsSync = false
			currSpec.DontAllowSync = true
		}
		resourceStorageSpecList = append(resourceStorageSpecList, currSpec)
	}

	ws := workspacev1alpha1.WorkspaceStorage{
		ObjectMeta: v1.ObjectMeta{
			Name:      workspacev1alpha1.WorkspaceStorageName(user),
			Namespace: namespace,
		},
		Spec: workspacev1alpha1.WorkspaceStorageSpec{
			ResourceStorageSpecs: resourceStorageSpecList,
		},
	}
	return ws
}

func MapVoyagerFileToWorkspaceCR(in voyagerfile.Workspace, username string, namespace string) *workspacev1alpha1.Workspace {
	resourceSpecList := make([]workspacev1alpha1.ResourceSpec, 0)
	for resourceName, userSpec := range in.Resources {
		currResourceSpec := workspacev1alpha1.ResourceSpec{
			Name: resourceName,
			Spec: workspacev1alpha1.WorkspaceResourceSpec{
				ImageRegistry: userSpec.ImageRegistry,
				Command:       userSpec.Command,
				Args:          userSpec.Args,
				DependsOn:     userSpec.DependsOn,
			},
		}
		if userSpec.Image != nil {
			currResourceSpec.Spec.PrebuiltApplicationSpec = &workspacev1alpha1.PrebuiltApplicationSpec{
				Image: *userSpec.Image,
			}
		} else {
			currResourceSpec.Spec.ApplicationBuildSpec = &workspacev1alpha1.ApplicationBuildSpec{
				Context:         userSpec.Build.BuildContext,
				BuildSourceHash: userSpec.Build.DirHash,
				VolumeName:      userSpec.Build.SourceVolume,
			}
			if userSpec.Build.DockerFilePath != nil {
				currResourceSpec.Spec.ApplicationBuildSpec.DockerFile = *userSpec.Build.DockerFilePath
			}
		}
		currResourceSpec.Spec.EnvironmentVariables = mapEnvs(userSpec.EnvironmentVariables)
		currResourceSpec.Spec.Ports = mapPorts(userSpec.Ports)
		currResourceSpec.Spec.VolumeMounts = mapMounts(userSpec.VolumeMounts)
		resourceSpecList = append(resourceSpecList, currResourceSpec)
	}

	ws := workspacev1alpha1.Workspace{
		ObjectMeta: v1.ObjectMeta{
			Name:      WorkspaceCRName(username),
			Namespace: namespace,
		},
		Spec: workspacev1alpha1.WorkspaceSpec{
			Resources:    resourceSpecList,
			UserName:     username,
			Organisation: "",
		},
	}
	return &ws
}

func mapEnvs(in map[string]string) []workspacev1alpha1.EnvironmentVariables {
	res := make([]workspacev1alpha1.EnvironmentVariables, 0)
	for name, value := range in {
		res = append(res, workspacev1alpha1.EnvironmentVariables{
			Name:  name,
			Value: value,
		})
	}
	return res
}

func mapMounts(in map[string]string) []workspacev1alpha1.VolumeMount {
	res := make([]workspacev1alpha1.VolumeMount, 0)
	for src, dst := range in {
		res = append(res, workspacev1alpha1.VolumeMount{
			Source:      src,
			Destination: dst,
		})
	}
	return res
}

func mapPorts(in []voyagerfile.Port) []workspacev1alpha1.Port {
	res := make([]workspacev1alpha1.Port, 0)
	for _, portDefn := range in {
		res = append(res, workspacev1alpha1.Port{
			Number:         portDefn.Number,
			IsHttp:         portDefn.IsHttp,
			ExposeToPublic: portDefn.ExposeToPublic,
		})
	}
	return res
}
