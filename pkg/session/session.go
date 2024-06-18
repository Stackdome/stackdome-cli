package session

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ashishmax31/voyager-cli/pkg/api/stackdome"
	"github.com/ashishmax31/voyager-cli/pkg/client"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"k8s.io/apimachinery/pkg/util/wait"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

type session struct {
	config          *config.Config
	stackdomeClient client.StackdomeAPIClient
	providerClient  *client.ProviderClient
}

type Session interface {
	CreateResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.CreateOption) error
	GetResourceFromProvider(ctx context.Context, key k8sclient.ObjectKey, obj k8sclient.Object, opts ...k8sclient.GetOption) error
	UpdateResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.UpdateOption) error
	DeleteResourceInProvider(ctx context.Context, obj k8sclient.Object, opts ...k8sclient.DeleteOption) error
	GetWorkspaceVolumesFromProvider(ctx context.Context, wstorage *workspacev1alpha1.WorkspaceStorage) ([]workspacev1alpha1.WorkspaceVolume, error)
	InitializeProvider(context.Context) error
	ProviderClient() *client.ProviderClient
	Config() *config.Config
}

func NewSession(config *config.Config, withProvider bool) (Session, error) {
	if withProvider {
		if !config.Valid() {
			return nil, fmt.Errorf("stackdome configfile is invalid! Run init first")
		}
		providerClient, err := client.NewProviderClient(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider client: %w", err)
		}
		return &session{
			config:          config,
			stackdomeClient: client.NewStackdomeClient(config.AccessToken, config.VoyagerServerUrl, false),
			providerClient:  providerClient,
		}, nil
	}
	return &session{
		config:          config,
		stackdomeClient: client.NewStackdomeClient(config.AccessToken, config.VoyagerServerUrl, false),
	}, nil
}

func (s *session) ProviderClient() *client.ProviderClient {
	return s.providerClient
}

func (s *session) InitializeProvider(ctx context.Context) error {
	err := s.ensureProvisionRequestExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize provider: %w", err)
	}
	provisionRequest, pErr := s.waitForWorkspaceProvisionRequestCompletion(ctx)
	if pErr != nil {
		return fmt.Errorf("failed to initialize provider: %w", pErr)
	}

	if err := s.populateAndSaveProviderConfig(provisionRequest); err != nil {
		return fmt.Errorf("failed to initialize provider: %w", pErr)
	}

	providerClient, err := client.NewProviderClient(s.config)
	if err != nil {
		return fmt.Errorf("failed to initialize provider client: %w", err)
	}
	s.providerClient = providerClient
	return nil
}

func (s *session) ensureProvisionRequestExists(ctx context.Context) error {
	_, err := s.stackdomeClient.GetCurrentUserWorskpaceProvisionRequest(ctx)
	if err != nil {
		if err.HttpCode == http.StatusNotFound {
			return s.handleProvisionRequestCreation(ctx)
		}
		return err
	}
	return nil
}

func (s *session) waitForWorkspaceProvisionRequestCompletion(ctx context.Context) (*stackdome.WorkspaceProvisionRequest, error) {
	var currentRequest *stackdome.WorkspaceProvisionRequest
	var perr *client.StackdomeAPIError

	pollErr := wait.PollUntilContextTimeout(ctx, time.Second*5, time.Minute*1, true, func(ctx context.Context) (done bool, err error) {
		currentRequest, perr = s.stackdomeClient.GetCurrentUserWorskpaceProvisionRequest(ctx)
		if perr != nil {
			return false, fmt.Errorf("failed to get current user's workspace provision request. Error: %w", perr)
		}
		if currentRequest.State == stackdome.WorkspaceProvisionRequestStateCompleted {
			return true, nil
		}
		return false, nil
	})

	if pollErr != nil {
		return nil, fmt.Errorf("failed to wait for workspace provision request completion. Error: %w", pollErr)
	}

	return currentRequest, nil
}

func (s *session) handleProvisionRequestCreation(ctx context.Context) error {
	_, err := s.stackdomeClient.CreateWorskpaceProvisionRequest(ctx)
	if err != nil {
		return err
	}
	return nil
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

func (s *session) populateAndSaveProviderConfig(input *stackdome.WorkspaceProvisionRequest) error {
	if s.config.ProviderConfig == nil {
		s.config.ProviderConfig = &config.ComputeProviderConfig{}
	}
	s.config.ProviderConfig.CaCert = input.Status.ClusterCaCert
	s.config.ProviderConfig.ServerUrl = input.Status.ClusterUrl
	s.config.ProviderConfig.Namespace = input.Status.WorkspaceNamespace
	s.config.ProviderConfig.ServiceAccountName = input.Status.WorkspaceServiceAccountname
	s.config.ProviderConfig.Token = input.Status.WorkspaceServiceaccountToken
	s.config.ProviderConfig.WorkspaceDomain = input.Status.Domain
	if err := config.Save(s.config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}
