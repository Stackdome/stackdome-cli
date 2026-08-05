package client

import (
	"context"
	"fmt"
	"io"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
)

// StreamBuildLogs opens the build's SSE log stream. Same frame shape as
// StreamLogs — feed the reader to ParseSSEStream.
func (c *Client) StreamBuildLogs(ctx context.Context, stackID, buildID string, opts LogOptions) (io.ReadCloser, error) {
	path := fmt.Sprintf("/api/v1/organizations/%s/projects/%s/stacks/%s/builds/%s/logs", c.orgID, c.projectName, stackID, buildID)
	return c.openLogStream(ctx, path, opts)
}

func (c *Client) ListBuilds(ctx context.Context, stackID string) ([]openapi.ImageBuild, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameStacksIdBuildsGet(ctx, c.orgID, c.projectName, stackID).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list builds")
	}
	return resp.GetItems(), nil
}

func (c *Client) ListResourceBuilds(ctx context.Context, stackID, resourceName string) ([]openapi.ImageBuild, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameStacksIdResourcesResourceNameBuildsGet(ctx, c.orgID, c.projectName, stackID, resourceName).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list builds for resource")
	}
	return resp.GetItems(), nil
}

func (c *Client) GetBuild(ctx context.Context, stackID, buildID string) (*openapi.ImageBuild, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameStacksIdBuildsBuildIdGet(ctx, c.orgID, c.projectName, stackID, buildID).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get build")
	}
	return resp, nil
}
