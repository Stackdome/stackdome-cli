package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/tools"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
)

type WorkspaceInitializationService interface {
	InitializeWorkspace(ctx context.Context, workspaceName string) *ServiceError
}

type workspaceInitializationService struct {
	runtime *config.Runtime
}

func NewWorkspaceInitializationService(runtime *config.Runtime) WorkspaceInitializationService {
	return &workspaceInitializationService{
		runtime: runtime,
	}
}

func (s *workspaceInitializationService) InitializeWorkspace(ctx context.Context, workspaceName string) *ServiceError {
	publicKeyPath, privateKeyPath, err := tools.EnsureSSHKeyPair(s.runtime.ConfigDir)
	if err != nil {
		return NewServiceError(err)
	}

	s.runtime.Config().SetUserPrivateKeyPublicKeyPath(privateKeyPath, publicKeyPath)
	if err := s.runtime.SaveConfig(); err != nil {
		return NewServiceError(err)
	}

	userPublicKey, err := tools.ReadFile(publicKeyPath)
	if err != nil {
		return NewServiceError(err)
	}

	// We need to merge the current workspace name with the ones already present in the config file.
	desiredWorkspaceUser := &v1alpha1.WorkspaceUser{
		SshPublicKey: userPublicKey,
		Workspaces:   []string{workspaceName},
	}

	err = s.ensureWorkspaceUser(ctx, desiredWorkspaceUser, workspaceName)
	if err != nil {
		return NewServiceErrorWithMessage(err, "failed to initialize workspace")
	}

	workspaceUser, pErr := s.waitForWorkspaceUserToBeAvailable(ctx)
	if pErr != nil {
		return NewServiceErrorWithMessage(pErr, "failed to initialize workspace")
	}

	if err := s.runtime.Config().PersistProviderConfig(workspaceUser.Status); err != nil {
		return NewServiceErrorWithMessage(err, "failed to initialize workspace")
	}

	if err := s.runtime.Config().PersistCurrentWorkspace(workspaceName); err != nil {
		return NewServiceErrorWithMessage(err, "failed to initialize workspace")
	}

	return nil
}

func (s *workspaceInitializationService) ensureWorkspaceUser(ctx context.Context, workspaceUser *v1alpha1.WorkspaceUser, currentWorkspaceName string) error {
	currentWorkspaceUser, err := s.runtime.Session.GetCurrentWorskpaceUser(ctx)
	if err != nil {
		if err.HttpCode == http.StatusNotFound {
			return s.handleWorkspaceUserCreation(ctx, workspaceUser, currentWorkspaceName)
		}
		return fmt.Errorf("failed to get current user's workspace user. Error: %w", err)
	}

	if currentWorkspaceUser.Status.ContainsWorkspace(currentWorkspaceName) &&
		currentWorkspaceUser.IsAvailable() && currentWorkspaceUser.SshPublicKey == workspaceUser.SshPublicKey {
		return nil
	}

	return s.handleWorkspaceUserUpdate(ctx, workspaceUser, currentWorkspaceUser)
}

func (s *workspaceInitializationService) handleWorkspaceUserCreation(ctx context.Context, desiredWorkspaceUser *v1alpha1.WorkspaceUser, currentWorkspace string) error {
	var workspacesInConfig []string
	for _, workspace := range s.runtime.Config().Workspaces {
		if workspace.WorkspaceName != currentWorkspace {
			workspacesInConfig = append(workspacesInConfig, workspace.WorkspaceName)
		}
	}
	desiredWorkspaceUser.Workspaces = append(desiredWorkspaceUser.Workspaces, workspacesInConfig...)
	_, err := s.runtime.Session.CreateWorkspaceUser(ctx, desiredWorkspaceUser)
	if err != nil {
		return fmt.Errorf("failed to create workspace user. Error: %w", err)
	}
	return nil
}

func (s *workspaceInitializationService) handleWorkspaceUserUpdate(ctx context.Context, desiredWorkspaceUser *v1alpha1.WorkspaceUser, currentWorkspaceUser *v1alpha1.WorkspaceUser) error {
	allWorkspaces := make(map[string]struct{})

	for _, workspace := range s.runtime.Config().Workspaces {
		allWorkspaces[workspace.WorkspaceName] = struct{}{}
	}

	for _, workspace := range currentWorkspaceUser.Workspaces {
		allWorkspaces[workspace] = struct{}{}
	}

	for _, workspace := range desiredWorkspaceUser.Workspaces {
		allWorkspaces[workspace] = struct{}{}
	}

	desiredWorkspaces := make([]string, 0)
	for workspace := range allWorkspaces {
		desiredWorkspaces = append(desiredWorkspaces, workspace)
	}
	desiredWorkspaceUser.Workspaces = desiredWorkspaces

	_, err := s.runtime.Session.UpdateWorkspaceUser(ctx, currentWorkspaceUser.ID, desiredWorkspaceUser)
	if err != nil {
		return fmt.Errorf("failed to update workspace user. Error: %w", err)
	}
	return nil
}

func (s *workspaceInitializationService) waitForWorkspaceUserToBeAvailable(ctx context.Context) (*v1alpha1.WorkspaceUser, error) {
	var currentUser *v1alpha1.WorkspaceUser
	var perr *session.SessionError

	pollErr := wait.PollUntilContextTimeout(ctx, time.Second*5, time.Minute*2, true, func(ctx context.Context) (done bool, err error) {
		currentUser, perr = s.runtime.Session.GetCurrentWorskpaceUser(ctx)
		if perr != nil {
			return false, fmt.Errorf("failed to get current user's workspaceuser. Error: %w", perr)
		}
		if currentUser.Status.IsAvailable() {
			return true, nil
		}
		return false, nil
	})

	if pollErr != nil {
		return nil, fmt.Errorf("error when waiting for workspaceuser to become ready: %w", pollErr)
	}
	return currentUser, nil
}
