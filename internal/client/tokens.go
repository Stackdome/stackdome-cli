package client

import (
	"context"
	"time"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
)

// CreateAPIToken mints a personal access token. The raw `sdm_` value comes back
// exactly once — the server keeps only its hash.
func (c *Client) CreateAPIToken(ctx context.Context, name string, scopes []string, resourceIDs []string, expiresAt *time.Time) (*openapi.APITokenCreateResponse, error) {
	req := openapi.APITokenCreateRequest{
		Name:        name,
		Scopes:      scopes,
		ResourceIds: resourceIDs,
		ExpiresAt:   expiresAt,
	}

	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1ApiTokensPost(ctx).
		APITokenCreateRequest(req).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to create API token")
	}
	return resp, nil
}

func (c *Client) ListAPITokens(ctx context.Context) ([]openapi.APIToken, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1ApiTokensGet(ctx).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list API tokens")
	}
	return resp.GetItems(), nil
}

func (c *Client) DeleteAPIToken(ctx context.Context, tokenID string) error {
	httpResp, err := c.apiClient.DefaultApi.
		ApiV1ApiTokensIdDelete(ctx, tokenID).
		Execute()
	if err != nil {
		return WrapError(httpResp, err, "Failed to delete API token")
	}
	return nil
}

func (c *Client) ListTokenScopes(ctx context.Context) (*openapi.ScopeList, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1ApiTokensScopesGet(ctx).
		Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to list token scopes")
	}
	return resp, nil
}
