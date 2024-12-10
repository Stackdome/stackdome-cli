package session

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type config interface {
	GetServerURL() string
	GetAccessToken() string
	GetInsecure() bool
	GetOrganisationID() string
	ProviderCACert() string
	ProviderServerURL() string
	ProviderToken() string
	Valid() bool
	SSHUser() string
}

type SessionError struct {
	HttpCode int
	err      error
	Message  string
}

func (e *SessionError) Error() string {
	return fmt.Sprintf("session error: %s", e.err.Error())
}

func ToSessionErr(in *client.StackdomeAPIError) *SessionError {
	if in == nil {
		return nil
	}
	return &SessionError{
		HttpCode: in.HttpCode,
		err:      in,
		Message:  in.Message,
	}
}

func NewSessionError(err error) *SessionError {
	return &SessionError{
		err: err,
	}
}

type session struct {
	stackdomeClient client.StackdomeAPIClient
	providerClient  *client.ProviderClient
}

type Session interface {
	// GetWorkspaceVolumesFromProvider(ctx context.Context, wstorage *workspacev1alpha1.WorkspaceStorage) ([]workspacev1alpha1.WorkspaceVolume, error)
	GetCurrentUserWorkspaceStorages(ctx context.Context) ([]*v1alpha1.WorkspaceStorage, *SessionError)
	GetWorkspaceStorage(ctx context.Context, id string) (*v1alpha1.WorkspaceStorage, *SessionError)
	UpdateWorkspaceStorage(ctx context.Context, ID string, userStack *v1alpha1.UserStack) (*v1alpha1.WorkspaceStorage, *SessionError)
	DeleteWorkspaceStorage(ctx context.Context, ID string) *SessionError
	CreateWorkspaceStorage(ctx context.Context, userStack *v1alpha1.UserStack) (*v1alpha1.WorkspaceStorage, *SessionError)
	MarkAsSynced(ctx context.Context, storageID string, volumeID string) *SessionError
	GetCurrentWorskpaceUser(ctx context.Context) (*v1alpha1.WorkspaceUser, *SessionError)
	CreateWorkspaceUser(ctx context.Context, desiredWorkspaceUser *v1alpha1.WorkspaceUser) (*v1alpha1.WorkspaceUser, *SessionError)
	UpdateWorkspaceUser(ctx context.Context, ID string, workspaceUser *v1alpha1.WorkspaceUser) (*v1alpha1.WorkspaceUser, *SessionError)

	CreateWorkspace(ctx context.Context, workspace *v1alpha1.Workspace) (*v1alpha1.Workspace, *SessionError)
	GetWorkspace(ctx context.Context, id string) (*v1alpha1.Workspace, *SessionError)
	GetWorkspaceResources(ctx context.Context, id string) ([]v1alpha1.WorkspaceResource, *SessionError)
	UpdateWorkspace(ctx context.Context, ID string, workspace *v1alpha1.Workspace) (*v1alpha1.Workspace, *SessionError)
	DeleteWorkspace(ctx context.Context, ID string) *SessionError
	GetCurrentWorkspaces(ctx context.Context) ([]*v1alpha1.Workspace, *SessionError)

	ProviderClient() *client.ProviderClient
}

func NewSession(config config, withProvider bool) (Session, error) {
	if config.GetOrganisationID() == "" {
		return nil, fmt.Errorf("organisation id missing, run 'stackdome login' ")
	}
	if withProvider {
		if !config.Valid() {
			return nil, fmt.Errorf("stackdome configfile is invalid! Create a workspace environment first by running 'voyager create-workspace ...'")
		}
		providerClient, err := client.NewProviderClient(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider client: %w", err)
		}
		return &session{
			stackdomeClient: client.NewStackdomeClient(config),
			providerClient:  providerClient,
		}, nil
	}
	return &session{
		stackdomeClient: client.NewStackdomeClient(config),
	}, nil
}

func (s *session) ProviderClient() *client.ProviderClient {
	return s.providerClient
}

func (s *session) CreateWorkspace(ctx context.Context, workspace *v1alpha1.Workspace) (*v1alpha1.Workspace, *SessionError) {
	createdWorkspace, err := s.stackdomeClient.CreateWorkspace(ctx, workspace)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return createdWorkspace, nil
}

func (s *session) GetWorkspace(ctx context.Context, id string) (*v1alpha1.Workspace, *SessionError) {
	workspace, err := s.stackdomeClient.GetWorkspace(ctx, id)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return workspace, nil
}

func (s *session) GetWorkspaceResources(ctx context.Context, id string) ([]v1alpha1.WorkspaceResource, *SessionError) {
	workspaceResources, serr := s.stackdomeClient.GetWorkspaceResources(ctx, id)
	if serr != nil {
		return nil, ToSessionErr(serr)
	}
	return workspaceResources, nil
}

func (s *session) UpdateWorkspace(ctx context.Context, ID string, workspace *v1alpha1.Workspace) (*v1alpha1.Workspace, *SessionError) {
	updatedWorkspace, err := s.stackdomeClient.UpdateWorkspace(ctx, ID, workspace)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return updatedWorkspace, nil
}

func (s *session) DeleteWorkspace(ctx context.Context, ID string) *SessionError {
	err := s.stackdomeClient.DeleteWorkspace(ctx, ID)
	if err != nil {
		return ToSessionErr(err)
	}
	return nil
}

func (s *session) GetCurrentWorkspaces(ctx context.Context) ([]*v1alpha1.Workspace, *SessionError) {
	workspaces, err := s.stackdomeClient.GetCurrentWorkspaces(ctx)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return workspaces, nil
}

func (s *session) GetCurrentWorskpaceUser(ctx context.Context) (*v1alpha1.WorkspaceUser, *SessionError) {
	user, err := s.stackdomeClient.GetCurrentUserWorskpaceUser(ctx)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return user, nil
}

func (s *session) GetWorkspaceStorage(ctx context.Context, id string) (*v1alpha1.WorkspaceStorage, *SessionError) {
	storage, err := s.stackdomeClient.GetWorkspaceStorage(ctx, id)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return storage, nil
}

func (s *session) UpdateWorkspaceStorage(ctx context.Context, ID string, userStack *v1alpha1.UserStack) (*v1alpha1.WorkspaceStorage, *SessionError) {
	storage, err := s.stackdomeClient.UpdateWorkspaceStorage(ctx, ID, userStack)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return storage, nil
}

func (s *session) DeleteWorkspaceStorage(ctx context.Context, ID string) *SessionError {
	err := s.stackdomeClient.DeleteWorkspaceStorage(ctx, ID)
	if err != nil {
		return ToSessionErr(err)
	}
	return nil
}

func (s *session) MarkAsSynced(ctx context.Context, storageID string, volumeID string) *SessionError {
	err := s.stackdomeClient.MarkVolumeAsSynced(ctx, storageID, volumeID)
	if err != nil {
		return ToSessionErr(err)
	}
	return nil
}

func (s *session) CreateWorkspaceUser(ctx context.Context, desiredWorkspaceUser *v1alpha1.WorkspaceUser) (*v1alpha1.WorkspaceUser, *SessionError) {
	user, err := s.stackdomeClient.CreateWorkspaceUser(ctx, desiredWorkspaceUser)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return user, nil
}

func (s *session) UpdateWorkspaceUser(ctx context.Context, ID string, workspaceUser *v1alpha1.WorkspaceUser) (*v1alpha1.WorkspaceUser, *SessionError) {
	user, err := s.stackdomeClient.UpdateWorkspaceUser(ctx, ID, workspaceUser)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return user, nil
}

func (s *session) GetCurrentUserWorkspaceStorages(ctx context.Context) ([]*v1alpha1.WorkspaceStorage, *SessionError) {
	storages, err := s.stackdomeClient.GetCurrentUserWorkspaceStorages(ctx)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return storages, nil
}

func (s *session) CreateWorkspaceStorage(ctx context.Context, userStack *v1alpha1.UserStack) (*v1alpha1.WorkspaceStorage, *SessionError) {
	storage, err := s.stackdomeClient.CreateWorkspaceStorage(ctx, userStack)
	if err != nil {
		return nil, ToSessionErr(err)
	}
	return storage, nil
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
