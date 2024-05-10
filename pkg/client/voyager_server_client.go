package client

import (
	"context"
	"os"
	"time"

	"github.com/ashishmax31/voyager-cli/pkg/api/authentication"
	"github.com/ashishmax31/voyager-cli/pkg/api/provider"
)

type VoyagerServerClient interface {
	GetUserInfo(ctx context.Context) (*authentication.AuthResponse, error)
	InitializeProvider(ctx context.Context) (*provider.KubernetesProviderInfo, error)
}

type vclient struct {
	AccessToken string
	URL         string
	Insecure    bool
}

func NewVoyagerServerClient(token string, serverURL string, insecure bool) VoyagerServerClient {
	return &vclient{AccessToken: token, URL: serverURL, Insecure: insecure}
}

func (c *vclient) GetUserInfo(ctx context.Context) (*authentication.AuthResponse, error) {
	// TODO: Talk to voyager server using the url and get the user info
	return &authentication.AuthResponse{
		Username:     "ashish",
		Organisation: "voyager-labs-dev",
		// 1 year
		TokenValidTill: time.Now().Add(time.Hour * 24 * 365),
	}, nil
}

func (c *vclient) InitializeProvider(ctx context.Context) (*provider.KubernetesProviderInfo, error) {
	// TODO: Talk to voyager server using the url and get the user info
	caCert, err := os.ReadFile("/Users/ashishanand/projects/skysync/voyager-cli/ca.crt")
	if err != nil {
		return nil, err
	}
	return &provider.KubernetesProviderInfo{
		Namespace: "ashish-workspace",
		Cacrt:     caCert,
		Token:     `eyJhbGciOiJSUzI1NiIsImtpZCI6IldFX2ZDMVNrVzVCeTUzUGkwMHNGbGF6d3c0b2hNZWtWb3FkNWdEb0hMTUUifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJhc2hpc2gtd29ya3NwYWNlIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImFzaGlzaC1zZWNyZXQiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoiYXNoaXNoIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQudWlkIjoiOTJlNmZlOTItNTY2MS00YjkxLThmZDYtZDBlMTQyZDEyNjI3Iiwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmFzaGlzaC13b3Jrc3BhY2U6YXNoaXNoIn0.izVkkrDnaE1VMMRSPttHsOnecqQly2wkDJzAuRPes34AItBQiXQNJU-NPOkpXnL4DymeID78qitOQmy5uNA8NOolpFXmC2u2YTeBPDjNf0qVy5L0-ORjxkOU2_WfigvxwH1NBmiLdBxP5-BmRbBNmP-Hk1xgzmnkOoQyAR1jC4tYj7-dXQsvoyqzRqGfID2kJjuGHWZXJOFfyGZiNrjWdRCCfFJfFiDWfsyzlIfOf98jF6hu29DxAPeDwiqWb3qV3PilMrVPpUX75FwN19k_wv-qkTDgGV4QMNuYzX2I5FdJCzm9bfzXEkQeBry6ZlSe7koAUciXKFe1cibylqNFRA`,
		ServerUrl: "0.0.0.0:53266",
		SSHUser:   "root",
	}, nil
}
