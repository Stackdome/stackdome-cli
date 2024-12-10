package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	serverapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	internalapi "github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/mapper"
	"github.com/davecgh/go-spew/spew"
)

type StackdomeAPIError struct {
	HttpCode int
	err      error
	Message  string
}

func (e *StackdomeAPIError) Error() string {
	return fmt.Sprintf("%s: received '%d' code from stackdome API server: %s", e.Message, e.HttpCode, e.err.Error())
}

type StackdomeAPIClient interface {
	GetUser(ctx context.Context) (*internalapi.User, error)
	CreateWorkspaceUser(ctx context.Context, workspaceUser *internalapi.WorkspaceUser) (*internalapi.WorkspaceUser, *StackdomeAPIError)
	UpdateWorkspaceUser(ctx context.Context, ID string, workspaceUser *internalapi.WorkspaceUser) (*internalapi.WorkspaceUser, *StackdomeAPIError)
	GetWorskpaceUser(ctx context.Context, workspaceUserID string) (*internalapi.WorkspaceUser, *StackdomeAPIError)
	GetCurrentUserWorskpaceUser(ctx context.Context) (*internalapi.WorkspaceUser, *StackdomeAPIError)
	GetCurrentUserWorkspaceStorages(ctx context.Context) ([]*internalapi.WorkspaceStorage, *StackdomeAPIError)
	GetWorkspaceStorage(ctx context.Context, id string) (*internalapi.WorkspaceStorage, *StackdomeAPIError)
	UpdateWorkspaceStorage(ctx context.Context, ID string, workspace *internalapi.UserStack) (*internalapi.WorkspaceStorage, *StackdomeAPIError)
	CreateWorkspaceStorage(ctx context.Context, workspace *internalapi.UserStack) (*internalapi.WorkspaceStorage, *StackdomeAPIError)
	DeleteWorkspaceStorage(ctx context.Context, ID string) *StackdomeAPIError

	CreateWorkspace(ctx context.Context, workspace *internalapi.Workspace) (*internalapi.Workspace, *StackdomeAPIError)
	GetWorkspace(ctx context.Context, id string) (*internalapi.Workspace, *StackdomeAPIError)
	GetWorkspaceResources(ctx context.Context, id string) ([]internalapi.WorkspaceResource, *StackdomeAPIError)
	UpdateWorkspace(ctx context.Context, ID string, workspace *internalapi.Workspace) (*internalapi.Workspace, *StackdomeAPIError)
	DeleteWorkspace(ctx context.Context, ID string) *StackdomeAPIError
	GetCurrentWorkspaces(ctx context.Context) ([]*internalapi.Workspace, *StackdomeAPIError)

	MarkVolumeAsSynced(ctx context.Context, workspaceStorageID string, volumeID string) *StackdomeAPIError
}

type stackdomeClient struct {
	AccessToken    string
	URL            string
	Insecure       bool
	OrganisationID string
	client         *serverapi.APIClient
}

type clientConfig interface {
	Valid() bool
	GetServerURL() string
	GetAccessToken() string
	GetInsecure() bool
	GetOrganisationID() string
}

func NewStackdomeClient(in clientConfig) StackdomeAPIClient {
	cfg := serverapi.Configuration{
		UserAgent: "stackdome-cli",
		Debug:     false,
		Servers: serverapi.ServerConfigurations{
			serverapi.ServerConfiguration{
				URL: in.GetServerURL(),
			},
		},
		HTTPClient: &http.Client{
			Timeout: time.Second * 10,
		},
	}
	return &stackdomeClient{
		AccessToken:    in.GetAccessToken(),
		URL:            in.GetServerURL(),
		Insecure:       in.GetInsecure(),
		OrganisationID: in.GetOrganisationID(),
		client:         serverapi.NewAPIClient(&cfg),
	}
}

func (c *stackdomeClient) GetUser(ctx context.Context) (*internalapi.User, error) {
	spew.Dump(*c)
	resp, httpResp, err := c.client.DefaultApi.ApiV1UsersMeGet(c.withAuthenticatedCtx(ctx)).Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to get User information")
	}
	return mapper.ToClientAPIUser(resp), nil
}

// ----------------------------------------------------------------------------
// Workspaces

func (c *stackdomeClient) CreateWorkspace(ctx context.Context, workspace *internalapi.Workspace) (*internalapi.Workspace, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.ApiV1OrganizationsOrgIdWorkspacesPost(c.withAuthenticatedCtx(ctx), c.OrganisationID).
		Workspace(mapper.ToServerAPIWorkpace(workspace)).Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to create workspace")
	}
	return mapper.ToClientAPIWorkspace(resp), nil
}

func (c *stackdomeClient) GetWorkspace(ctx context.Context, id string) (*internalapi.Workspace, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.ApiV1OrganizationsOrgIdWorkspacesIdGet(c.withAuthenticatedCtx(ctx), c.OrganisationID, id).Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to get workspace")
	}
	return mapper.ToClientAPIWorkspace(resp), nil
}

func (c *stackdomeClient) GetWorkspaceResources(ctx context.Context, id string) ([]internalapi.WorkspaceResource, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.ApiV1OrganizationsOrgIdWorkspacesWorkspaceIdResourcesGet(c.withAuthenticatedCtx(ctx), c.OrganisationID, id).Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to get workspace")
	}
	return mapper.ToClientWorkspaceResources(resp.Items), nil
}

func (c *stackdomeClient) UpdateWorkspace(ctx context.Context, ID string, workspace *internalapi.Workspace) (*internalapi.Workspace, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.ApiV1OrganizationsOrgIdWorkspacesIdPut(c.withAuthenticatedCtx(ctx), c.OrganisationID, ID).
		Workspace(mapper.ToServerAPIWorkpace(workspace)).Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to update workspace")
	}
	return mapper.ToClientAPIWorkspace(resp), nil
}

func (c *stackdomeClient) DeleteWorkspace(ctx context.Context, ID string) *StackdomeAPIError {
	httpResp, err := c.client.DefaultApi.ApiV1OrganizationsOrgIdWorkspacesIdDelete(c.withAuthenticatedCtx(ctx), c.OrganisationID, ID).Execute()
	if err != nil {
		return handleError(httpResp, err, "failed to delete workspace")
	}
	return nil
}

func (c *stackdomeClient) GetCurrentWorkspaces(ctx context.Context) ([]*internalapi.Workspace, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.ApiV1OrganizationsOrgIdWorkspacesCurrentGet(c.withAuthenticatedCtx(ctx), c.OrganisationID).Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to get workspaces")
	}

	return mapper.ToClientAPIWorkspaces(resp.Items), nil
}

//
// Workspaces end
// ----------------------------------------------------------------------------

func (c *stackdomeClient) GetCurrentUserWorkspaceStorages(ctx context.Context) ([]*internalapi.WorkspaceStorage, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.
		ApiV1OrganizationsOrgIdWorkspaceStoragesCurrentGet(c.withAuthenticatedCtx(ctx), c.OrganisationID).Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to get workspace storages")
	}
	return mapper.ToClientWorkspaceStorages(resp.Items), nil
}

func (c *stackdomeClient) GetWorkspaceStorage(ctx context.Context, id string) (*internalapi.WorkspaceStorage, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.
		ApiV1OrganizationsOrgIdWorkspaceStoragesIdGet(c.withAuthenticatedCtx(ctx), c.OrganisationID, id).Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to get workspace storage")
	}
	return mapper.ToClientWorkspaceStorage(resp), nil
}

func (c *stackdomeClient) CreateWorkspaceStorage(ctx context.Context, workspace *internalapi.UserStack) (*internalapi.WorkspaceStorage, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.
		ApiV1OrganizationsIdWorkspaceStoragesPost(c.withAuthenticatedCtx(ctx), c.OrganisationID).
		WorkspaceStorage(mapper.ToServerWorkspaceStorage(workspace)).
		Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to create workspace storage")
	}
	return mapper.ToClientWorkspaceStorage(resp), nil
}

func (c *stackdomeClient) UpdateWorkspaceStorage(ctx context.Context, ID string, workspace *internalapi.UserStack) (*internalapi.WorkspaceStorage, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.
		ApiV1OrganizationsOrgIdWorkspaceStoragesIdPut(c.withAuthenticatedCtx(ctx), c.OrganisationID, ID).
		WorkspaceStorage(mapper.ToServerWorkspaceStorage(workspace)).
		Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to update workspace storage")
	}
	return mapper.ToClientWorkspaceStorage(resp), nil
}

func (c *stackdomeClient) DeleteWorkspaceStorage(ctx context.Context, ID string) *StackdomeAPIError {
	httpResp, err := c.client.DefaultApi.
		ApiV1OrganizationsOrgIdWorkspaceStoragesIdDelete(c.withAuthenticatedCtx(ctx), c.OrganisationID, ID).
		Execute()
	if err != nil {
		return handleError(httpResp, err, "failed to delete workspace storage")
	}
	return nil
}

func (c *stackdomeClient) MarkVolumeAsSynced(ctx context.Context, workspaceStorageID string, volumeID string) *StackdomeAPIError {
	httpResp, err := c.client.DefaultApi.
		ApiV1OrganizationsOrgIdWorkspaceStoragesIdVolumesVolumeIdMarkAsSyncedPost(c.withAuthenticatedCtx(ctx), c.OrganisationID, workspaceStorageID, volumeID).
		Execute()
	if err != nil {
		return handleError(httpResp, err, "failed to mark volume as synced")
	}
	return nil
}

func (c *stackdomeClient) CreateWorkspaceUser(ctx context.Context, workspaceUser *internalapi.WorkspaceUser) (*internalapi.WorkspaceUser, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.
		ApiV1WorkspaceUsersPost(c.withAuthenticatedCtx(ctx)).
		WorkspaceUser(mapper.ToServerAPIWorkspaceUser(workspaceUser)).
		Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to create workspace provision request")
	}
	return mapper.ToClientAPIWorkspaceUser(resp), nil
}

func (c *stackdomeClient) UpdateWorkspaceUser(ctx context.Context, ID string, workspaceUser *internalapi.WorkspaceUser) (*internalapi.WorkspaceUser, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.
		ApiV1WorkspaceUsersIdPut(c.withAuthenticatedCtx(ctx), ID).
		WorkspaceUser(mapper.ToServerAPIWorkspaceUser(workspaceUser)).
		Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to update workspace provision request")
	}
	return mapper.ToClientAPIWorkspaceUser(resp), nil
}

func (c *stackdomeClient) GetWorskpaceUser(ctx context.Context, workspaceUserID string) (*internalapi.WorkspaceUser, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.
		ApiV1WorkspaceUsersIdGet(c.withAuthenticatedCtx(ctx), workspaceUserID).Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to get workspace provision request")
	}
	return mapper.ToClientAPIWorkspaceUser(resp), nil
}

func (c *stackdomeClient) GetCurrentUserWorskpaceUser(ctx context.Context) (*internalapi.WorkspaceUser, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.ApiV1WorkspaceUsersCurrentGet(c.withAuthenticatedCtx(ctx)).Execute()
	if err != nil {
		return nil, handleError(httpResp, err, "failed to get workspace provision request")
	}
	return mapper.ToClientAPIWorkspaceUser(resp), nil
}

func (c *stackdomeClient) withAuthenticatedCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, serverapi.ContextAccessToken, c.AccessToken)
}

func handleError(httpResp *http.Response, err error, message string) *StackdomeAPIError {
	if httpResp != nil {
		return &StackdomeAPIError{HttpCode: httpResp.StatusCode, err: err, Message: message}
	}
	if isTimeoutError(err) {
		return &StackdomeAPIError{HttpCode: http.StatusRequestTimeout, err: err, Message: message}
	}
	// Unknown error
	return &StackdomeAPIError{HttpCode: 0, err: err, Message: message}
}

func isTimeoutError(err error) bool {
	if urlErr, ok := err.(*url.Error); ok && urlErr.Timeout() {
		return true
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	if err == context.DeadlineExceeded {
		return true
	}
	return false
}
