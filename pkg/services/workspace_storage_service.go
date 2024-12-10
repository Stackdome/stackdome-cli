package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"k8s.io/apimachinery/pkg/util/wait"
)

type WorkspaceStorageService interface {
	GetCurrentWorkspaceStorage(ctx context.Context, currentWorkspaceStorageName string) (*v1alpha1.WorkspaceStorage, *ServiceError)
	GetCurrentWorkspaceStorages(ctx context.Context) ([]*v1alpha1.WorkspaceStorage, *ServiceError)
	CreateWorkspaceStorage(ctx context.Context, stack *v1alpha1.UserStack) (*v1alpha1.WorkspaceStorage, *ServiceError)
	UpdateWorkspaceStorage(ctx context.Context, id string, workspace *v1alpha1.UserStack) (*v1alpha1.WorkspaceStorage, *ServiceError)
	DeleteWorkspaceStorage(ctx context.Context, id string) *ServiceError
	WaitForCurrentWorkspaceStorageToBeAvailable(ctx context.Context, currentWorkspaceStorageName string) (*v1alpha1.WorkspaceStorage, *ServiceError)
	MarkCurrentWorkspaceAsSynced(ctx context.Context, currentWorkspaceStorageName string, volumeID string) *ServiceError
}

type workspaceStorageService struct {
	session session.Session
}

type WorkspaceStorageServiceSpec struct {
	Session session.Session
}

func NewWorkspaceStorageService(spec WorkspaceStorageServiceSpec) WorkspaceStorageService {
	return &workspaceStorageService{
		session: spec.Session,
	}
}

func (w *workspaceStorageService) GetCurrentWorkspaceStorage(ctx context.Context, currentWorkspaceStorageName string) (*v1alpha1.WorkspaceStorage, *ServiceError) {
	workspaceStorages, err := w.session.GetCurrentUserWorkspaceStorages(ctx)
	if err != nil {
		return nil, NewServiceError(err)
	}

	for _, ws := range workspaceStorages {
		if ws.Name == currentWorkspaceStorageName {
			return ws, nil
		}
	}
	return nil, NewServiceErrorWithCode(fmt.Errorf("workspace storage not found"), http.StatusNotFound)
}

func (w *workspaceStorageService) GetCurrentWorkspaceStorages(ctx context.Context) ([]*v1alpha1.WorkspaceStorage, *ServiceError) {
	workspaceStorages, err := w.session.GetCurrentUserWorkspaceStorages(ctx)
	if err != nil {
		return nil, NewServiceError(err)
	}
	return workspaceStorages, nil
}

func (w *workspaceStorageService) CreateWorkspaceStorage(ctx context.Context, stack *v1alpha1.UserStack) (*v1alpha1.WorkspaceStorage, *ServiceError) {
	storage, err := w.session.CreateWorkspaceStorage(ctx, stack)
	if err != nil {
		return nil, NewServiceError(err)
	}
	return storage, nil
}

// TODO: Only update if the workspace storage has changed.
func (w *workspaceStorageService) UpdateWorkspaceStorage(ctx context.Context, id string, stack *v1alpha1.UserStack) (*v1alpha1.WorkspaceStorage, *ServiceError) {
	storage, err := w.session.UpdateWorkspaceStorage(ctx, id, stack)
	if err != nil {
		return nil, NewServiceError(err)
	}
	return storage, nil
}

func (w *workspaceStorageService) DeleteWorkspaceStorage(ctx context.Context, id string) *ServiceError {
	if err := w.session.DeleteWorkspaceStorage(ctx, id); err != nil {
		return NewServiceError(err)
	}
	return nil
}

func (w *workspaceStorageService) MarkCurrentWorkspaceAsSynced(ctx context.Context, currentWorkspaceStorageName string, volumeID string) *ServiceError {
	workspaceStorage, serr := w.GetCurrentWorkspaceStorage(ctx, currentWorkspaceStorageName)
	if serr != nil {
		return serr
	}

	if err := w.session.MarkAsSynced(ctx, workspaceStorage.ID, volumeID); err != nil {
		return NewServiceError(err)
	}
	return nil
}

func (w *workspaceStorageService) WaitForCurrentWorkspaceStorageToBeAvailable(ctx context.Context, currentWorkspaceStorageName string) (*v1alpha1.WorkspaceStorage, *ServiceError) {
	var currentWorkspaceStorage *v1alpha1.WorkspaceStorage
	var serr *ServiceError
	pollErr := wait.PollUntilContextTimeout(ctx, time.Second*5, time.Minute*5, true, func(ctx context.Context) (done bool, err error) {
		currentWorkspaceStorage, serr = w.GetCurrentWorkspaceStorage(ctx, currentWorkspaceStorageName)
		if serr != nil {
			return false, fmt.Errorf("failed to get workspace storage: %w", serr)
		}
		if currentWorkspaceStorage.IsAvailable() {
			return true, nil
		}
		fmt.Println("Waiting for workspace storage to be ready...")
		return false, nil
	})
	if pollErr != nil {
		return nil, NewServiceError(pollErr)
	}
	return currentWorkspaceStorage, nil
}
