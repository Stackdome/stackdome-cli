package mapper

import (
	serverapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	clientapi "github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
)

func ToClientResourceBuilds(in []serverapi.WorkspaceResourceBuild) []clientapi.ResourceBuild {
	res := make([]clientapi.ResourceBuild, 0)
	for _, rb := range in {
		res = append(res, ToClientResourceBuild(&rb))
	}
	return res
}

func ToClientResourceBuild(in *serverapi.WorkspaceResourceBuild) clientapi.ResourceBuild {
	return clientapi.ResourceBuild{
		ID:                    in.GetId(),
		WorkspaceID:           in.GetWorkspaceId(),
		WorkspaceResourceID:   in.GetWorkspaceResourceId(),
		WorkspaceResourceName: in.GetWorkspaceResourceName(),
		SourceHash:            in.GetSourceHash(),
		ImageRegistry:         in.GetImageRegistry(),
		Status:                ToClientResourceBuildStatus(in.Status),
	}
}

func ToClientResourceBuildStatus(in *serverapi.ResourceBuildStatus) *clientapi.ResourceBuildStatus {
	if in == nil {
		return nil
	}
	return &clientapi.ResourceBuildStatus{
		State:      in.GetState(),
		Conditions: ToClientAPIConditions(in.GetConditions()),
		ImageURL:   in.GetImageUrl(),
		SourceHash: in.GetBuildSourceHash(),
	}
}
