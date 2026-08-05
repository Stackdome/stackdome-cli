package client

import (
	"context"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
)

func (c *Client) ListPostgresAddons(ctx context.Context) ([]openapi.PostgresAddon, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameAddonsPostgresGet(ctx, c.orgID, c.projectName).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list postgres addons")
	}
	return resp.Items, nil
}

func (c *Client) FindPostgresAddonByName(ctx context.Context, name string) (*openapi.PostgresAddon, error) {
	addons, err := c.ListPostgresAddons(ctx)
	if err != nil {
		return nil, err
	}
	for i := range addons {
		if addons[i].Name == name {
			return &addons[i], nil
		}
	}
	return nil, nil
}
