package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/provider/k8s"
	"github.com/ashishmax31/voyager-cli/pkg/sync"
	"github.com/sirupsen/logrus"
)

func (w *workspaceHandler) StartSyncSession(ctx context.Context, userStack *v1alpha1.UserStack) error {
	currentWorkspaceName := w.runtime.Config().CurrentWorkspace
	if currentWorkspaceName == nil {
		return fmt.Errorf("current workspace not set")
	}

	existingWorkspaceStorage, serr := w.workspaceStorageService.GetCurrentWorkspaceStorage(ctx, userStack.WorkspaceStorageName())
	if serr != nil {
		return workspaceHandlerErr("failed to get workspace storage for stack: %w", serr)
	}

	if _, err := w.workspaceStorageService.WaitForCurrentWorkspaceStorageToBeAvailable(ctx, userStack.WorkspaceStorageName()); err != nil {
		return workspaceHandlerErr("workspace storage not available: %w", err)
	}

	initialized, err := w.syncHandler.Initialized(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to check sync status: %w", err)
	}
	if initialized {
		logrus.Info("already initialized")
		return nil
	}

	syncList := make(sync.SourceDestintionList, 0)
	for _, volume := range existingWorkspaceStorage.Volumes {
		if volume.VolumeSource != nil && volume.VolumeSource.LocalDir != nil {
			current := sync.SourceDestintionPair{
				Source: volume.VolumeSource.LocalDir.Path,
				// TODO: Server to expose this field in the status.
				Destination: fmt.Sprintf("/%s/%s", existingWorkspaceStorage.Name, volume.Name),
			}
			syncList = append(syncList, current)
		}
	}

	logrus.Debugf("synclist : %+v \n", syncList)
	// Blocking.
	if err := w.syncHandler.SetupSyncSession(
		ctx,
		syncList,
		k8s.NewServiceTarget(existingWorkspaceStorage.Status.StorageServiceName),
	); err != nil {
		return workspaceHandlerErr("sync session failed: %w", err)
	}
	// Success
	return nil
}

func (w *workspaceHandler) SyncStatus(ctx context.Context) error {
	return w.syncHandler.Status(ctx)
}

func (w *workspaceHandler) StopSyncSession(ctx context.Context) error {
	return w.syncHandler.StopSyncSession(ctx)
}

func (w *workspaceHandler) Sync(ctx context.Context) error {
	userStack, err := w.runtime.UserStack()
	if err != nil {
		return workspaceHandlerErr("failed to get user stack: %w", err)
	}

	currentWorkspaceStorageName := userStack.WorkspaceStorageName()

	initialized, err := w.syncHandler.Initialized(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to check sync session status: %w", err)
	}
	syncRunning, err := w.syncHandler.SyncSessionRunning(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to check sync session status: %w", err)
	}

	switch {
	case !initialized:
		return fmt.Errorf("sync session not initialized! Please run voyager sync init")
	case !syncRunning:
		return fmt.Errorf("sync session not running! Please run voyager sync init and wait for it to complete")
	}

	if err := w.syncHandler.ForceSync(ctx); err != nil {
		return workspaceHandlerErr("failed to force sync: %w", err)
	}
	return w.markAsSynced(ctx, currentWorkspaceStorageName)
}

func (w *workspaceHandler) markAsSynced(ctx context.Context, currentWorkspaceStorageName string) error {
	existingWorkspaceStorage, serr := w.workspaceStorageService.GetCurrentWorkspaceStorage(ctx, currentWorkspaceStorageName)
	if serr != nil {
		return workspaceHandlerErr("failed to get workspace storage for stack: %w", serr)
	}
	for _, volume := range existingWorkspaceStorage.Volumes {
		if volume.VolumeSource != nil && volume.VolumeSource.LocalDir != nil {
			if err := w.workspaceStorageService.MarkCurrentWorkspaceAsSynced(ctx, currentWorkspaceStorageName, volume.Name); err != nil {
				return workspaceHandlerErr("failed to mark as synced: %w", err)
			}
		}
	}
	return nil
}
