package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	clierrors "github.com/stackdome/cli/internal/errors"
)

// ListVolumes lists the volumes of a stack.
//
// Hand-rolled on purpose: `GET .../stacks/{id}/volumes` exists in the router but
// is missing from the OpenAPI spec (config/openapi/stackdome_api.yaml), so the
// generated client has no method for it. The generated client only exposes
// project-level volume POST/GET-by-id/DELETE. Replace this once the spec gains
// the list operation.
func (c *Client) ListVolumes(ctx context.Context, stackID string) ([]openapi.Volume, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/projects/%s/stacks/%s/volumes", c.baseURL, c.orgID, c.projectName, stackID)

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

func (c *Client) CreateVolume(ctx context.Context, name, size, accessMode string) (*openapi.Volume, error) {
	volume := openapi.Volume{
		Name: name,
		Spec: openapi.VolumeSpec{
			Size:       size,
			AccessMode: openapi.VolumeAccessMode(accessMode),
		},
	}
	resp, httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameVolumesPost(ctx, c.orgID, c.projectName).
		Volume(volume).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to create volume")
	}
	return resp, nil
}

func (c *Client) DeleteVolume(ctx context.Context, volumeID string) error {
	httpResp, err := c.apiClient.DefaultApi.
		ApiV1OrganizationsOrgIdProjectsProjectNameVolumesIdDelete(ctx, c.orgID, c.projectName, volumeID).
		Execute()
	if err != nil {
		return WrapError(httpResp, err, "Failed to delete volume")
	}
	return nil
}

func (c *Client) FindVolumeByName(ctx context.Context, stackID, name string) (*openapi.Volume, error) {
	volumes, err := c.ListVolumes(ctx, stackID)
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
