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

const DefaultSize = "3Gi"

func MapVoyagerFileToWorkspaceStorage(in voyagerfile.Workspace, user string, namespace string) workspacev1alpha1.WorkspaceStorage {

	resourceStorageSpecList := make([]workspacev1alpha1.ResourceStorageSpec, 0)

	for resourceName, spec := range in {
		currSpec := workspacev1alpha1.ResourceStorageSpec{
			Name: resourceName,
			// TODO, find dir hash.
			Hash: "init",
		}
		if spec.Image != nil {
			currSpec.Type = workspacev1alpha1.PreBuiltApplicationStateStorage
			if spec.StorageSize != nil {
				currSpec.Size = *spec.StorageSize
			} else {
				currSpec.Size = DefaultSize
			}
			currSpec.DontAllowSync = true
			currSpec.NeedsSync = false
		} else {
			currSpec.Type = workspacev1alpha1.ApplicationSourceStorage
			if spec.StorageSize != nil {
				currSpec.Size = *spec.StorageSize
			} else {
				currSpec.Size = DefaultSize
			}
			currSpec.NeedsSync = true
			currSpec.DontAllowSync = false
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
	for resourceName, userSpec := range in {
		currResourceSpec := workspacev1alpha1.ResourceSpec{
			Name:         resourceName,
			StorageSize:  *userSpec.StorageSize,
			SyncRequired: userSpec.NeedsSync,
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
			currResourceSpec.Spec.ApplicationSourceSpec = &workspacev1alpha1.ApplicationSourceSpec{
				Context:         userSpec.Source.BuildContext,
				DockerFile:      userSpec.Source.DockerFilePath,
				BuildSourceHash: userSpec.Source.DirHash,
			}
		}
		currResourceSpec.Spec.EnvironmentVariables = mapEnvs(userSpec.EnvironmentVariables)
		currResourceSpec.Spec.Ports = mapPorts(userSpec.Ports)
		currResourceSpec.Spec.Mounts = mapMounts(userSpec.Mounts)
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

func mapMounts(in map[string]string) []workspacev1alpha1.ResourceMounts {
	res := make([]workspacev1alpha1.ResourceMounts, 0)
	for src, dst := range in {
		res = append(res, workspacev1alpha1.ResourceMounts{
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
