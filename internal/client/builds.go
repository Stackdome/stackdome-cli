package client

import (
	"context"

	openapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
)

func (c *Client) ListBuilds(ctx context.Context, stackID string) ([]openapi.ImageBuild, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameStacksIdBuildsGet(ctx, c.orgID, c.teamName, stackID).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list builds")
	}
	return resp.GetItems(), nil
}

func (c *Client) ListResourceBuilds(ctx context.Context, stackID, resourceName string) ([]openapi.ImageBuild, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameStacksIdResourcesResourceNameBuildsGet(ctx, c.orgID, c.teamName, stackID, resourceName).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list builds for resource")
	}
	return resp.GetItems(), nil
}

func (c *Client) GetBuild(ctx context.Context, stackID, buildID string) (*openapi.ImageBuild, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameStacksIdBuildsBuildIdGet(ctx, c.orgID, c.teamName, stackID, buildID).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get build")
	}
	return resp, nil
}
