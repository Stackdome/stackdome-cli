package mapper

import (
	"strings"

	serverapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	clientapi "github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"k8s.io/utils/ptr"
)

func ToClientAPIWorkspaces(in []serverapi.Workspace) []*clientapi.Workspace {
	res := make([]*clientapi.Workspace, 0)
	for _, workspace := range in {
		res = append(res, ToClientAPIWorkspace(&workspace))
	}
	return res
}

func ToClientAPIWorkspace(in *serverapi.Workspace) *clientapi.Workspace {
	return &clientapi.Workspace{
		ID:          in.GetId(),
		Name:        in.Name,
		Namespace:   in.GetNamespace(),
		Labels:      ServerLabelsToClientKeyValues(in.Labels),
		Annotations: ServerAnnotationsToClientKeyValues(in.Annotations),
		Resources:   ToClientWorkspaceResources(in.Spec.Resources),
		Status:      toClientWorkspaceStatus(in.Status),
	}
}

func toClientWorkspaceStatus(in *serverapi.WorkspaceStatus) *clientapi.WorkspaceStatus {
	if in == nil {
		return nil
	}
	return &clientapi.WorkspaceStatus{
		Conditions: ToClientAPIConditions(in.Conditions),
		State:      in.GetState(),
	}
}

func ToClientWorkspaceResources(in []serverapi.WorkspaceResource) []clientapi.WorkspaceResource {
	res := make([]clientapi.WorkspaceResource, 0)
	for _, resource := range in {
		currentResource := clientapi.WorkspaceResource{
			ID:              *resource.Id,
			Name:            resource.Name,
			ImageRegistry:   resource.ImageRegistry,
			DependsOn:       resource.DependsOn,
			Labels:          ServerLabelsToClientKeyValues(resource.Labels),
			Annotations:     ServerAnnotationsToClientKeyValues(resource.Annotations),
			BuildConfig:     toClientBuildSpec(resource.Build),
			PreBuiltImage:   toClientPrebuiltImage(resource.Prebuilt),
			Init:            toClientInitConfig(resource.Init),
			ExecutionConfig: toClientExecutionConfig(resource.ExecutionConfig),
			LifecycleConfig: toClientLifecycleConfig(resource.LifecycleConfig),
			Ports:           toClientPorts(resource.Ports),
			Stateful:        nilValue(resource.Stateful),
			VolumeMounts:    toClientVolumeMounts(resource.VolumeMounts),
			Status:          toClientWorkspaceResourceStatus(resource.Status),
		}
		res = append(res, currentResource)
	}
	return res
}

func ToClientWorkspaceResource(in *serverapi.WorkspaceResource) *clientapi.WorkspaceResource {
	if in == nil {
		return nil
	}
	return &clientapi.WorkspaceResource{
		ID:              *in.Id,
		Name:            in.Name,
		ImageRegistry:   in.ImageRegistry,
		DependsOn:       in.DependsOn,
		Labels:          ServerLabelsToClientKeyValues(in.Labels),
		Annotations:     ServerAnnotationsToClientKeyValues(in.Annotations),
		BuildConfig:     toClientBuildSpec(in.Build),
		PreBuiltImage:   toClientPrebuiltImage(in.Prebuilt),
		Init:            toClientInitConfig(in.Init),
		ExecutionConfig: toClientExecutionConfig(in.ExecutionConfig),
		LifecycleConfig: toClientLifecycleConfig(in.LifecycleConfig),
		Ports:           toClientPorts(in.Ports),
		Stateful:        nilValue(in.Stateful),
		VolumeMounts:    toClientVolumeMounts(in.VolumeMounts),
		Status:          toClientWorkspaceResourceStatus(in.Status),
	}
}

func toClientInitConfig(in *serverapi.InitConfig) *clientapi.InitConfig {
	if in == nil {
		return nil
	}
	return &clientapi.InitConfig{
		Command: in.Command,
		Args:    in.Args,
	}
}

func toClientPrebuiltImage(in *serverapi.PrebuiltConfig) *clientapi.PreBuiltImage {
	if in == nil {
		return nil
	}
	return &clientapi.PreBuiltImage{
		Image: in.Image,
	}
}

func toClientExecutionConfig(in *serverapi.ExecutionConfig) *clientapi.ExecutionConfig {
	if in == nil {
		return nil
	}
	return &clientapi.ExecutionConfig{
		Command:              in.Command,
		Args:                 in.Args,
		EnvironmentVariables: toClientEnvironmentVariables(in.EnvironmentVariables),
	}
}

func toClientEnvironmentVariables(in []serverapi.EnvVar) []clientapi.KeyValue {
	res := make([]clientapi.KeyValue, 0)
	for _, envVar := range in {
		res = append(res, clientapi.KeyValue{
			Key:   envVar.Name,
			Value: envVar.Value,
		})
	}
	return res
}

func toClientBuildSpec(in *serverapi.BuildConfig) *clientapi.BuildConfig {
	if in == nil {
		return nil
	}
	return &clientapi.BuildConfig{
		SourceVolumeID: in.SourceVolumeId,
		ContextPath:    in.ContextPath,
		DockerFilePath: in.GetDockerfilePath(),
		ContextDirHash: in.SourceHash,
	}
}

func toClientVolumeMounts(in []serverapi.VolumeMount) []clientapi.VolumeMount {
	res := make([]clientapi.VolumeMount, 0)
	for _, mount := range in {
		res = append(res, clientapi.VolumeMount{
			SourceVolumeID: mount.SourceVolumeId,
			SourceSubPath:  mount.SourceSubPath,
			TargetPath:     mount.TargetPath,
		})
	}
	return res
}

func toClientLifecycleConfig(in *serverapi.LifecycleConfig) *clientapi.LifecycleConfig {
	if in == nil {
		return nil
	}
	return &clientapi.LifecycleConfig{
		LastRestartRequestTime: in.LastRestartRequestTime,
	}
}

func toClientPorts(in []serverapi.Port) []clientapi.Port {
	res := make([]clientapi.Port, 0)
	for _, port := range in {
		res = append(res, clientapi.Port{
			Number:         port.Number,
			ExposeToPublic: port.GetExposedToPublic(),
		})
	}
	return res
}

func toClientWorkspaceResourceStatus(in *serverapi.ResourceStatus) *clientapi.WorkspaceResourceStatus {
	if in == nil {
		return nil
	}
	return &clientapi.WorkspaceResourceStatus{
		ObservedVersion:     in.GetObservedVersion(),
		State:               in.GetState(),
		InternalServiceName: in.InternalServiceName,
		PublicIngresses:     toClientPublicIngress(in.PublicIngress),
		Conditions:          ToClientAPIConditions(in.Conditions),
	}
}

func toClientPublicIngress(in []serverapi.Ingress) []clientapi.Ingress {
	res := make([]clientapi.Ingress, 0)
	for _, ingress := range in {
		res = append(res, clientapi.Ingress{
			URL:        ingress.GetUrl(),
			TargetPort: ingress.GetTargetPort(),
		})
	}
	return res
}

func ToServerAPIWorkpace(in *clientapi.Workspace) serverapi.Workspace {
	return serverapi.Workspace{
		Name:        in.Name,
		Labels:      clientLabelsToServerKeyValues(in.Labels),
		Annotations: clientAnnotationsToServerKeyValues(in.Annotations),
		Spec:        toServerWorkspaceSpec(in.Resources),
	}
}

func toServerWorkspaceSpec(in []clientapi.WorkspaceResource) serverapi.WorkspaceSpec {
	res := make([]serverapi.WorkspaceResource, 0)
	for _, resource := range in {
		currentResource := serverapi.WorkspaceResource{
			Name:            resource.Name,
			ImageRegistry:   resource.ImageRegistry,
			DependsOn:       resource.DependsOn,
			Labels:          clientLabelsToServerKeyValues(resource.Labels),
			Annotations:     clientAnnotationsToServerKeyValues(resource.Annotations),
			Build:           toServerBuildSpec(resource.BuildConfig),
			Prebuilt:        toServerPrebuiltImage(resource.PreBuiltImage),
			Init:            toServerInitConfig(resource.Init),
			VolumeMounts:    toServerVolumeMounts(resource.VolumeMounts),
			ExecutionConfig: toServerExecutionConfig(resource.ExecutionConfig),
			LifecycleConfig: toServerLifecycleConfig(resource.LifecycleConfig),
			Ports:           toServerPorts(resource.Ports),
			Stateful:        &resource.Stateful,
		}
		res = append(res, currentResource)
	}
	return serverapi.WorkspaceSpec{
		Resources: res,
	}
}

// TODO: Allow users to set subdomain for a resource ingress.
func toServerPorts(in []clientapi.Port) []serverapi.Port {
	res := make([]serverapi.Port, 0)
	for _, port := range in {
		res = append(res, serverapi.Port{
			Number:          port.Number,
			ExposedToPublic: &port.ExposeToPublic,
		})
	}
	return res

}

func toServerLifecycleConfig(in *clientapi.LifecycleConfig) *serverapi.LifecycleConfig {
	if in == nil {
		return nil
	}
	return &serverapi.LifecycleConfig{
		LastRestartRequestTime: in.LastRestartRequestTime,
	}
}

func toServerExecutionConfig(in *clientapi.ExecutionConfig) *serverapi.ExecutionConfig {
	if in == nil {
		return nil
	}
	return &serverapi.ExecutionConfig{
		Command:              in.Command,
		Args:                 in.Args,
		EnvironmentVariables: toServerEnvironmentVariables(in.EnvironmentVariables),
	}
}

func toServerEnvironmentVariables(in []clientapi.KeyValue) []serverapi.EnvVar {
	res := make([]serverapi.EnvVar, 0)
	for _, kv := range in {
		res = append(res, serverapi.EnvVar{
			Name:  kv.Key,
			Value: kv.Value,
		})
	}
	return res
}

func toServerVolumeMounts(in []clientapi.VolumeMount) []serverapi.VolumeMount {
	res := make([]serverapi.VolumeMount, 0)
	for _, mount := range in {
		res = append(res, serverapi.VolumeMount{
			SourceVolumeId: mount.SourceVolumeID,
			SourceSubPath:  mount.SourceSubPath,
			TargetPath:     mount.TargetPath,
		})
	}
	return res
}

func toServerInitConfig(in *clientapi.InitConfig) *serverapi.InitConfig {
	if in == nil {
		return nil
	}
	return &serverapi.InitConfig{
		Command: in.Command,
		Args:    in.Args,
	}
}

func toServerPrebuiltImage(in *clientapi.PreBuiltImage) *serverapi.PrebuiltConfig {
	if in == nil {
		return nil
	}
	return &serverapi.PrebuiltConfig{
		Image: in.Image,
	}
}

func toServerBuildSpec(in *clientapi.BuildConfig) *serverapi.BuildConfig {
	if in == nil {
		return nil
	}
	return &serverapi.BuildConfig{
		SourceVolumeId: in.SourceVolumeID,
		ContextPath:    in.ContextPath,
		DockerfilePath: in.DockerFilePath,
		SourceHash:     in.ContextDirHash,
	}
}

func clientLabelsToServerKeyValues(in []clientapi.KeyValue) []serverapi.Label {
	res := make([]serverapi.Label, 0)
	for _, kv := range in {
		res = append(res, serverapi.Label{
			Key:   kv.Key,
			Value: kv.Value,
		})
	}
	return res
}

func clientAnnotationsToServerKeyValues(in []clientapi.KeyValue) []serverapi.Annotation {
	res := make([]serverapi.Annotation, 0)
	for _, kv := range in {
		res = append(res, serverapi.Annotation{
			Key:   kv.Key,
			Value: kv.Value,
		})
	}
	return res
}

func WorkspaceFromUserStack(in *clientapi.UserStack) clientapi.Workspace {
	return clientapi.Workspace{
		Name:      in.Name,
		Resources: workspaceResources(in.Resources),
	}
}

func workspaceResources(in map[string]*clientapi.WorkspaceResourceSpec) []clientapi.WorkspaceResource {
	res := make([]clientapi.WorkspaceResource, 0)
	for name, spec := range in {
		currentResource := clientapi.WorkspaceResource{
			Name:          name,
			ImageRegistry: spec.ImageRegistry,
			BuildConfig:   buildConfig(spec.Build),
			PreBuiltImage: prebuiltImage(spec.Image),
			Init:          initConfig(spec.Init),
			ExecutionConfig: &clientapi.ExecutionConfig{
				Command:              spec.Command,
				Args:                 spec.Args,
				EnvironmentVariables: environmentVariables(spec.EnvironmentVariables),
			},
			LifecycleConfig: &clientapi.LifecycleConfig{},
			VolumeMounts:    volumeMounts(spec.VolumeMounts),
			Ports:           spec.Ports,
			Stateful:        false,
			DependsOn:       spec.DependsOn,
			Status:          &clientapi.WorkspaceResourceStatus{},
		}
		res = append(res, currentResource)
	}
	return res
}

func environmentVariables(in map[string]string) []clientapi.KeyValue {
	res := make([]clientapi.KeyValue, 0)
	for k, v := range in {
		res = append(res, clientapi.KeyValue{
			Key:   k,
			Value: v,
		})
	}
	return res
}

func volumeMounts(in map[string]string) []clientapi.VolumeMount {
	res := make([]clientapi.VolumeMount, 0)
	for src, dst := range in {
		// Ex : src = "sourceVolumeID/subPath", dst = "targetPath"
		// Ex: deps/node_modules:/app/node_modules
		// deps is the sourceVolumeID and node_modules is the subPath
		//TODO: Use OS specific path separator?
		curr := strings.Split(src, "/")
		sourceVolumeID := curr[0]
		var subPath *string
		if strings.Join(curr[1:], "/") != "" {
			subPath = ptr.To(strings.Join(curr[1:], "/"))
		}
		res = append(res, clientapi.VolumeMount{
			SourceVolumeID: sourceVolumeID,
			SourceSubPath:  subPath,
			TargetPath:     dst,
		})
	}
	return res
}

func initConfig(in *clientapi.InitCommand) *clientapi.InitConfig {
	if in == nil {
		return nil
	}
	return &clientapi.InitConfig{
		Command: in.Command,
		Args:    in.Args,
	}
}

func prebuiltImage(image *string) *clientapi.PreBuiltImage {
	if image == nil {
		return nil
	}
	return &clientapi.PreBuiltImage{
		Image: *image,
	}
}

func buildConfig(in *clientapi.ApplicationBuildSpec) *clientapi.BuildConfig {
	if in == nil {
		return nil
	}
	return &clientapi.BuildConfig{
		SourceVolumeID: in.SourceVolume,
		ContextPath:    in.BuildContext,
		DockerFilePath: nilValue(in.DockerFilePath),
		ContextDirHash: in.DirHash,
	}
}

func nilValue[T any](in *T) T {
	if in == nil {
		var zero T
		return zero
	}
	return *in
}
