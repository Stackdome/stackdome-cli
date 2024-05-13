package session

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/client"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

type session struct {
	config              *config.Config
	vogagerServerClient client.VoyagerServerClient
	providerClient      *client.ProviderClient
}

type Session interface {
	CreateResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.CreateOption) error
	GetResourceFromProvider(ctx context.Context, key k8sclient.ObjectKey, obj k8sclient.Object, opts ...k8sclient.GetOption) error
	UpdateResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.UpdateOption) error
	DeleteResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.DeleteOption) error
	GetWorkspaceVolumesFromProvider(ctx context.Context, wstorage *workspacev1alpha1.WorkspaceStorage) ([]workspacev1alpha1.WorkspaceVolume, error)
	ProviderClient() *client.ProviderClient
	Config() *config.Config
}

func NewSession(config *config.Config) (Session, error) {
	if !config.Valid() {
		return nil, fmt.Errorf("voyager configfile is invalid")
	}
	providerClient, err := client.NewProviderClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider client: %w", err)
	}
	return &session{
		config:              config,
		vogagerServerClient: client.NewVoyagerServerClient(config.AccessToken, config.VoyagerServerUrl, true),
		providerClient:      providerClient,
	}, nil
}

func (s *session) ProviderClient() *client.ProviderClient {
	return s.providerClient
}

func (s *session) configValid() error {
	if s.config == nil || len(s.config.AccessToken) == 0 || len(s.config.VoyagerServerUrl) == 0 {
		return fmt.Errorf("access token and voyager server url is mandatory")
	}
	return nil
}

func (s *session) Config() *config.Config {
	return s.config
}

func (s *session) CreateResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.CreateOption) error {
	return s.providerClient.Create(ctx, obj, opts...)
}

func (s *session) GetResourceFromProvider(ctx context.Context, key k8sclient.ObjectKey, obj k8sclient.Object, opts ...k8sclient.GetOption) error {
	return s.providerClient.Get(ctx, key, obj, opts...)
}

func (s *session) UpdateResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.UpdateOption) error {
	return s.providerClient.Update(ctx, obj, opts...)
}

func (s *session) DeleteResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.DeleteOption) error {
	return s.providerClient.Delete(ctx, obj, opts...)
}

func (s *session) GetWorkspaceVolumesFromProvider(
	ctx context.Context, wstorage *workspacev1alpha1.WorkspaceStorage) ([]workspacev1alpha1.WorkspaceVolume, error) {
	workspaceVolumeList := &workspacev1alpha1.WorkspaceVolumeList{}
	if err := s.providerClient.List(
		ctx,
		workspaceVolumeList,
		k8sclient.InNamespace(wstorage.Namespace),
		k8sclient.MatchingLabels{workspacev1alpha1.WorkspaceStorageVolumeLabel: wstorage.Name}); err != nil {
		return nil, err
	}
	return workspaceVolumeList.Items, nil
}
