package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	openapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	clierrors "github.com/stackdome/cli/internal/errors"
)

func (c *Client) ListVolumes(ctx context.Context) ([]openapi.Volume, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/teams/%s/volumes/current", c.baseURL, c.orgID, c.teamName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, clierrors.Wrap(err, "Failed to create volume list request")
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Accept", "application/json")

	httpClient := c.cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, clierrors.Wrap(err, "Failed to list volumes")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, clierrors.FromHTTP(resp.StatusCode, "Failed to list volumes")
	}

	var list openapi.VolumeList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, clierrors.Wrap(err, "Failed to decode volume list")
	}
	return list.GetItems(), nil
}

func (c *Client) DeleteVolume(ctx context.Context, volumeID string) error {
	httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdTeamsTeamNameVolumesIdDelete(ctx, c.orgID, c.teamName, volumeID).
		Execute()
	if err != nil {
		return WrapError(httpResp, err, "Failed to delete volume")
	}
	return nil
}

func (c *Client) FindVolumeByName(ctx context.Context, name string) (*openapi.Volume, error) {
	volumes, err := c.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}
	for i := range volumes {
		if volumes[i].Name == name {
			return &volumes[i], nil
		}
	}
	return nil, nil
}
