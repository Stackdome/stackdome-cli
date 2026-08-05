package client

import (
	"context"

	serverapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	clierrors "github.com/stackdome/cli/internal/errors"
)

type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         *serverapi.User
}

type SignupResult struct {
	AccessToken  string
	RefreshToken string
	User         *serverapi.User
}

func (c *Client) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	loginReq := serverapi.NewLoginRequest(email, password)

	resp, httpResp, err := c.apiClient.DefaultApi.ApiV1AuthLoginPost(ctx).
		LoginRequest(*loginReq).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 401 {
			return nil, clierrors.AuthError("Invalid email or password.")
		}
		return nil, WrapError(httpResp, err, "Login failed")
	}

	c.SetTokens(resp.GetToken(), resp.GetRefreshToken())

	return &LoginResult{
		AccessToken:  resp.GetToken(),
		RefreshToken: resp.GetRefreshToken(),
		User:         resp.User,
	}, nil
}

func (c *Client) Signup(ctx context.Context, name, email, password, orgName string) (*SignupResult, error) {
	signupReq := serverapi.NewUserSignupRequest(name, email, password)
	org := serverapi.NewOrganisation()
	org.SetName(orgName)
	signupReq.SetOrganisation(*org)

	resp, httpResp, err := c.apiClient.DefaultApi.ApiV1UserSignupPost(ctx).
		UserSignupRequest(*signupReq).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Signup failed")
	}

	c.SetTokens(resp.GetJwtToken(), resp.GetRefreshToken())

	return &SignupResult{
		AccessToken:  resp.GetJwtToken(),
		RefreshToken: resp.GetRefreshToken(),
		User:         resp.User,
	}, nil
}

func (c *Client) GetCurrentUser(ctx context.Context) (*serverapi.User, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.ApiV1UsersCurrentGet(ctx).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get current user")
	}
	return resp, nil
}

func (c *Client) GetOrgTeams(ctx context.Context, orgID string) ([]serverapi.Team, error) {
	resp, httpResp, err := c.apiClient.DefaultApi.ApiV1OrganizationsOrgIdTeamsGet(ctx, orgID).Execute()
	if err != nil {
		return nil, WrapError(httpResp, err, "Failed to get teams")
	}
	return resp.Items, nil
}

func (c *Client) ResolveDefaultTeam(ctx context.Context, orgID string) (string, error) {
	teams, err := c.GetOrgTeams(ctx, orgID)
	if err != nil {
		return "", err
	}
	for _, t := range teams {
		if t.DefaultTeam != nil && *t.DefaultTeam {
			return t.Name, nil
		}
	}
	if len(teams) > 0 {
		return teams[0].Name, nil
	}
	return "", clierrors.New("No teams found for your account.")
}
