package client

import (
	"context"

	openapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
)

func (c *Client) CreateStack(ctx context.Context, stack openapi.Stack) (*openapi.Stack, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameStacksPost(ctx, c.orgID, c.teamName).
		Stack(stack).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to create stack")
	}
	return resp, nil
}

func (c *Client) GetStack(ctx context.Context, stackID string) (*openapi.Stack, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameStacksIdGet(ctx, c.orgID, c.teamName, stackID).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get stack")
	}
	return resp, nil
}

func (c *Client) UpdateStack(ctx context.Context, stackID string, stack openapi.Stack) (*openapi.Stack, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameStacksIdPut(ctx, c.orgID, c.teamName, stackID).
		Stack(stack).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to update stack")
	}
	return resp, nil
}

func (c *Client) DeleteStack(ctx context.Context, stackID string) error {
	_, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameStacksIdDelete(ctx, c.orgID, c.teamName, stackID).Execute()
	if err != nil {
		return WrapError(httpResp, err, "Failed to delete stack")
	}
	return nil
}

func (c *Client) ListStacks(ctx context.Context) ([]openapi.Stack, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameStacksGet(ctx, c.orgID, c.teamName).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list stacks")
	}
	return resp.Items, nil
}

func (c *Client) GetStackResources(ctx context.Context, stackID string) ([]openapi.StackResource, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameStacksIdResourcesGet(ctx, c.orgID, c.teamName, stackID).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get stack resources")
	}
	return resp.Items, nil
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
