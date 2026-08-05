package client

import (
	"context"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
)

func (c *Client) CreateStack(ctx context.Context, stack openapi.Stack) (*openapi.Stack, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameStacksPost(ctx, c.orgID, c.projectName).
		Stack(stack).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to create stack")
	}
	return resp, nil
}

func (c *Client) GetStack(ctx context.Context, stackID string) (*openapi.Stack, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameStacksIdGet(ctx, c.orgID, c.projectName, stackID).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get stack")
	}
	return resp, nil
}

func (c *Client) UpdateStack(ctx context.Context, stackID string, stack openapi.Stack) (*openapi.Stack, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameStacksIdPut(ctx, c.orgID, c.projectName, stackID).
		Stack(stack).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to update stack")
	}
	return resp, nil
}

func (c *Client) DeleteStack(ctx context.Context, stackID string) error {
	_, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameStacksIdDelete(ctx, c.orgID, c.projectName, stackID).Execute()
	if err != nil {
		return WrapError(httpResp, err, "Failed to delete stack")
	}
	return nil
}

func (c *Client) ListStacks(ctx context.Context) ([]openapi.Stack, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameStacksGet(ctx, c.orgID, c.projectName).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list stacks")
	}
	return resp.Items, nil
}

func (c *Client) GetStackResources(ctx context.Context, stackID string) ([]openapi.StackResource, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameStacksIdResourcesGet(ctx, c.orgID, c.projectName, stackID).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get stack resources")
	}
	return resp.Items, nil
}

func (c *Client) RestartResource(ctx context.Context, stackID, resourceName string) (*openapi.StackResource, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameStacksIdResourcesResourceNameActionsRestartPost(ctx, c.orgID, c.projectName, stackID, resourceName).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to restart resource")
	}
	return resp, nil
}

// GetStackLiveStatus returns the runtime status of a stack's resources, which
// now lives on the stack's release rather than on the stack itself. Returns nil
// when the stack has no release yet.
//
// It reports on the converged release — what is actually serving — falling back
// to the latest release only when nothing has converged yet.
func (c *Client) GetStackLiveStatus(ctx context.Context, stack *openapi.Stack) (*openapi.ReleaseLiveStatus, error) {
	rel := stack.ConvergedRelease
	if rel == nil {
		rel = stack.LatestRelease
	}
	if rel == nil || rel.Id == nil || stack.Id == nil {
		return nil, nil
	}

	resp, httpResp, err := c.apiClient.ReleasesApi.
		GetRelease(ctx, c.orgID, c.projectName, *stack.Id, *rel.Id).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get release status")
	}
	return resp.LiveStatus, nil
}

func (c *Client) FindStackByName(ctx context.Context, name string) (*openapi.Stack, error) {
	stacks, err := c.ListStacks(ctx)
	if err != nil {
		return nil, err
	}
	for i := range stacks {
		if stacks[i].Name == name {
			return &stacks[i], nil
		}
	}
	return nil, nil
}
