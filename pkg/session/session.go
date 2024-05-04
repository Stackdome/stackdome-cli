package session

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/api/authentication"
	"github.com/ashishmax31/voyager-cli/pkg/api/provider"
	"github.com/ashishmax31/voyager-cli/pkg/client"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type session struct {
	config              *config.Config
	vogagerServerClient client.VoyagerServerClient
	providerClient      *client.ProviderClient
}

type Session interface {
	Authenticate(context.Context) (*authentication.AuthResponse, error)
	InitializeProvider(ctx context.Context) (*provider.KubernetesProviderInfo, error)
	CreateResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.CreateOption) error
	UpsertResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.CreateOption) error
	GetResourceFromProvider(ctx context.Context, key k8sclient.ObjectKey, obj k8sclient.Object, opts ...k8sclient.GetOption) error
	UpdateResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.UpdateOption) error
	DeleteResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.DeleteOption) error
	ProviderClient() *client.ProviderClient
	Config() *config.Config
}

func NewSession(config *config.Config) Session {
	return &session{
		config:              config,
		vogagerServerClient: client.NewClient(config.AccessToken, config.VoyagerServerUrl),
	}
}

func NewSessionWithProvider(config *config.Config) (Session, error) {
	providerClient, err := client.NewProviderClient(config)
	if err != nil {
		return nil, err
	}
	return &session{
		config:              config,
		vogagerServerClient: client.NewClient(config.AccessToken, config.VoyagerServerUrl),
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

func (s *session) Authenticate(ctx context.Context) (*authentication.AuthResponse, error) {
	if err := s.configValid(); err != nil {
		return nil, err
	}
	return s.vogagerServerClient.GetUserInfo(ctx)
}

func (s *session) InitializeProvider(ctx context.Context) (*provider.KubernetesProviderInfo, error) {
	if err := s.configValid(); err != nil {
		return nil, err
	}
	return s.vogagerServerClient.InitializeProvider(ctx)
}

func (s *session) CreateResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.CreateOption) error {
	return s.providerClient.Create(ctx, obj, opts...)
}
func (s *session) UpsertResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.CreateOption) error {
	// return s.providerClient.Create(ctx, obj, opts...)
	var existingObj k8sclient.Object
	if err := s.providerClient.Get(
		ctx,
		types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()},
		existingObj); err != nil {
		if apierrors.IsNotFound(err) {
			return s.CreateResourceInProvider(ctx, obj, opts...)
		}
		return err
	}
	// TODO: Only update if the item is different.
	return s.UpdateResourceInProvider(ctx, obj)
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
