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

func (c *Client) CreatePostgresAddon(ctx context.Context, addon openapi.PostgresAddon) (*openapi.PostgresAddon, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameAddonsPostgresPost(ctx, c.orgID, c.projectName).
		PostgresAddon(addon).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to create postgres addon")
	}
	return resp, nil
}

func (c *Client) GetPostgresAddon(ctx context.Context, addonID string) (*openapi.PostgresAddon, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameAddonsPostgresIdGet(ctx, c.orgID, c.projectName, addonID).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get postgres addon")
	}
	return resp, nil
}

func (c *Client) DeletePostgresAddon(ctx context.Context, addonID string) error {
	_, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameAddonsPostgresIdDelete(ctx, c.orgID, c.projectName, addonID).
		Execute()
	if err != nil {
		return WrapError(httpResp, err, "Failed to delete postgres addon")
	}
	return nil
}

// BackupPostgresAddon triggers an on-demand backup. The server-side backup
// service is still a stub, so failures here are surfaced verbatim.
func (c *Client) BackupPostgresAddon(ctx context.Context, addonID, description string) (*openapi.ApiV1OrganizationsOrgIdProjectsProjectNameAddonsPostgresIdActionsBackupPost202Response, error) {
	body := openapi.ApiV1OrganizationsOrgIdProjectsProjectNameAddonsPostgresIdActionsBackupPostRequest{}
	if description != "" {
		body.Description = &description
	}
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameAddonsPostgresIdActionsBackupPost(ctx, c.orgID, c.projectName, addonID).
		ApiV1OrganizationsOrgIdProjectsProjectNameAddonsPostgresIdActionsBackupPostRequest(body).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to trigger backup")
	}
	return resp, nil
}

func (c *Client) ListPostgresBackups(ctx context.Context, addonID string) ([]openapi.PostgresBackup, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameAddonsPostgresIdBackupsGet(ctx, c.orgID, c.projectName, addonID).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list backups")
	}
	return resp.GetItems(), nil
}

func (c *Client) GetPostgresCredentials(ctx context.Context, addonID, database string, superuser bool) (*openapi.PostgresCredentials, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameAddonsPostgresIdCredentialsDatabaseGet(ctx, c.orgID, c.projectName, addonID, database).
		Superuser(superuser).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get credentials")
	}
	return resp, nil
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
