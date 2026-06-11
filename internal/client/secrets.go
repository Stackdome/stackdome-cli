package client

import (
	"context"

	openapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
)

func (c *Client) ListSecrets(ctx context.Context) ([]openapi.Secret, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameSecretsGet(ctx, c.orgID, c.teamName).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list secrets")
	}
	return resp.GetItems(), nil
}

func (c *Client) GetSecret(ctx context.Context, secretID string) (*openapi.Secret, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameSecretsIdGet(ctx, c.orgID, c.teamName, secretID).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get secret")
	}
	return resp, nil
}

func (c *Client) CreateSecret(ctx context.Context, secret openapi.Secret) (*openapi.Secret, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameSecretsPost(ctx, c.orgID, c.teamName).
		Secret(secret).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to create secret")
	}
	return resp, nil
}

func (c *Client) UpdateSecret(ctx context.Context, secretID string, secret openapi.Secret) (*openapi.Secret, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameSecretsIdPut(ctx, c.orgID, c.teamName, secretID).
		Secret(secret).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to update secret")
	}
	return resp, nil
}

func (c *Client) DeleteSecret(ctx context.Context, secretID string) error {
	httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameSecretsIdDelete(ctx, c.orgID, c.teamName, secretID).
		Execute()
	if err != nil {
		return WrapError(httpResp, err, "Failed to delete secret")
	}
	return nil
}

func (c *Client) FindSecretByName(ctx context.Context, name string) (*openapi.Secret, error) {
	secrets, err := c.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}
	for i := range secrets {
		if secrets[i].Name == name {
			return &secrets[i], nil
		}
	}
	return nil, nil
}
