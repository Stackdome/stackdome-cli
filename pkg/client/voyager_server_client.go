package client

import (
	"context"
	"fmt"

	"github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	stackdomeapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	"github.com/ashishmax31/voyager-cli/pkg/api/stackdome"
)

type StackdomeAPIError struct {
	HttpCode int
	err      error
	Message  string
}

func (e *StackdomeAPIError) Error() string {
	return fmt.Sprintf("%s. Received '%d' code from stackdome API server: %s", e.Message, e.HttpCode, e.err.Error())
}

type StackdomeAPIClient interface {
	GetUser(ctx context.Context) (*stackdome.User, error)
	CreateWorskpaceProvisionRequest(ctx context.Context) (*stackdome.WorkspaceProvisionRequest, *StackdomeAPIError)
	GetWorskpaceProvisionRequest(ctx context.Context, id string) (*stackdome.WorkspaceProvisionRequest, *StackdomeAPIError)
	GetCurrentUserWorskpaceProvisionRequest(ctx context.Context) (*stackdome.WorkspaceProvisionRequest, *StackdomeAPIError)
}

type stackdomeClient struct {
	AccessToken string
	URL         string
	Insecure    bool
	client      *stackdomeapi.APIClient
}

func NewStackdomeClient(token string, serverURL string, insecure bool) StackdomeAPIClient {
	cfg := stackdomeapi.Configuration{
		UserAgent: "stackdome-cli",
		Debug:     true,
		Servers: stackdomeapi.ServerConfigurations{
			stackdomeapi.ServerConfiguration{
				URL: serverURL,
			},
		},
	}
	return &stackdomeClient{AccessToken: token, URL: serverURL, Insecure: insecure, client: stackdomeapi.NewAPIClient(&cfg)}
}

func (c *stackdomeClient) GetUser(ctx context.Context) (*stackdome.User, error) {
	// TODO: Talk to voyager server using the url and get the user info
	resp, httpResp, err := c.client.DefaultApi.ApiV1UsersMeGet(c.withAuthenticatedCtx(ctx)).Execute()
	if err != nil {
		return nil, &StackdomeAPIError{HttpCode: httpResp.StatusCode, err: err, Message: "failed to get User information"}
	}
	return &stackdome.User{
		Id:           resp.GetId(),
		Name:         resp.GetName(),
		Username:     resp.GetUsername(),
		Email:        resp.GetEmail(),
		Organisation: resp.GetOrganisation(),
		Role:         resp.GetRole(),
	}, nil
}

func (c *stackdomeClient) CreateWorskpaceProvisionRequest(ctx context.Context) (*stackdome.WorkspaceProvisionRequest, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.
		ApiV1WorkspaceProvisionRequestsPost(c.withAuthenticatedCtx(ctx)).
		WorkspaceProvisionRequest(*stackdomeapi.NewWorkspaceProvisionRequest("test")).
		Execute()
	if err != nil {
		return nil, &StackdomeAPIError{HttpCode: httpResp.StatusCode, err: err, Message: "failed to create workspace provision request"}
	}
	return c.populateProvisionRequest(resp), nil
}

func (c *stackdomeClient) GetWorskpaceProvisionRequest(ctx context.Context, id string) (*stackdome.WorkspaceProvisionRequest, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.
		ApiV1WorkspaceProvisionRequestsIdGet(c.withAuthenticatedCtx(ctx), id).Execute()
	if err != nil {
		return nil, &StackdomeAPIError{HttpCode: httpResp.StatusCode, err: err, Message: "failed to get workspace provision request"}
	}
	return c.populateProvisionRequest(resp), nil
}

func (c *stackdomeClient) GetCurrentUserWorskpaceProvisionRequest(ctx context.Context) (*stackdome.WorkspaceProvisionRequest, *StackdomeAPIError) {
	resp, httpResp, err := c.client.DefaultApi.
		ApiV1WorkspaceProvisionRequestsCurrentGet(c.withAuthenticatedCtx(ctx)).Execute()
	if err != nil {
		return nil, &StackdomeAPIError{HttpCode: httpResp.StatusCode, err: err, Message: "failed to get workspace provision request"}
	}
	return c.populateProvisionRequest(resp), nil
}

func (c *stackdomeClient) withAuthenticatedCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, openapi.ContextAccessToken, c.AccessToken)
}

func (c *stackdomeClient) populateProvisionRequest(apiResp *stackdomeapi.WorkspaceProvisionRequest) *stackdome.WorkspaceProvisionRequest {
	res := &stackdome.WorkspaceProvisionRequest{
		Id:      apiResp.GetId(),
		State:   populateWPRState(apiResp.GetState()),
		Message: apiResp.GetMessage(),
	}

	respStatus, ok := apiResp.GetStatusOk()
	if ok {
		res.Status = &stackdome.WorkspaceProvisionRequestStatus{
			WorkspaceNamespace:           respStatus.GetWorkspaceNamespace(),
			ClusterCaCert:                respStatus.GetClusterCaCert(),
			ClusterUrl:                   respStatus.GetClusterUrl(),
			WorkspaceServiceAccountname:  respStatus.GetWorkspaceServiceAccountname(),
			WorkspaceServiceaccountToken: respStatus.GetWorkspaceServiceaccountToken(),
			Domain:                       respStatus.GetDomain(),
		}
	}

	return res
}

func populateWPRState(in stackdomeapi.WorkspaceProvisionRequestState) stackdome.WorkspaceProvisionRequestState {
	switch in {
	case stackdomeapi.COMPLETED:
		return stackdome.WorkspaceProvisionRequestStateCompleted
	case stackdomeapi.PENDING:
		return stackdome.WorkspaceProvisionRequestStatePending
	case stackdomeapi.ERROR:
		return stackdome.WorkspaceProvisionRequestStateError
	default:
		return stackdome.WorkspaceProvisionRequestStatePending
	}
}
