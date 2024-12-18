package workspace

import (
	"context"
	"net/http"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/sirupsen/logrus"
)

func (w *workspaceHandler) Delete(ctx context.Context, runtime *config.Runtime) error {
	if runtime.Args.IsAllWorkspaces() {
		return w.deleteAllWorkspaces(ctx, runtime)
	}

	var workspaceName string
	if runtime.Args.IsCurrentWorkspace() {
		workspaceNamePtr := runtime.Config().CurrentWorkspace
		if workspaceNamePtr == nil {
			return workspaceHandlerErr("current workspace not set")
		}
		workspaceName = *workspaceNamePtr
	} else {
		workspaceName = runtime.Args.GetWorkspaceName()
	}
	return w.deleteWorkspace(ctx, runtime, workspaceName)
}

func (w *workspaceHandler) deleteAllWorkspaces(ctx context.Context, runtime *config.Runtime) error {
	workspaces, err := w.workspaceService.GetCurrentWorkspaces(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to get workspaces: %w", err)
	}

	for _, workspace := range workspaces {
		if err := w.workspaceService.DeleteWorkspace(ctx, workspace.ID); err != nil {
			if err.Code == http.StatusNotFound {
				logrus.Infof("workspace '%s' not found", workspace.Name)
				continue
			}
			return workspaceHandlerErr("failed to delete workspace '%s': %w", workspace.Name, err)
		}
	}

	if err := w.syncHandler.StopSyncSession(ctx); err != nil {
		return workspaceHandlerErr("failed to stop sync session: %w", err)
	}

	if runtime.Args.IsRemoveStorage() {
		workspaceStorages, err := w.workspaceStorageService.GetCurrentWorkspaceStorages(ctx)
		if err != nil {
			return workspaceHandlerErr("failed to get workspace storages: %w", err)
		}
		for _, workspaceStorage := range workspaceStorages {
			if err := w.workspaceStorageService.DeleteWorkspaceStorage(ctx, workspaceStorage.ID); err != nil {
				if err.Code == http.StatusNotFound {
					logrus.Infof("workspace storage '%s' not found", workspaceStorage.Name)
					continue
				}
				return workspaceHandlerErr("failed to delete workspace storage '%s': %w", workspaceStorage.Name, err)
			}
		}
	}
	return nil
}

func (w *workspaceHandler) deleteWorkspace(ctx context.Context, runtime *config.Runtime, workspaceName string) error {
	var alreadyDeleted bool
	workspace, err := w.workspaceService.GetWorkspaceByName(ctx, workspaceName)
	if err != nil {
		if err.Code == http.StatusNotFound {
			logrus.Infof("workspace '%s' not found. It may have already been deleted", workspaceName)
			alreadyDeleted = true
		} else {
			return workspaceHandlerErr("failed to get workspace '%s': %w", workspaceName, err)
		}
	}

	if !alreadyDeleted {
		if err := w.workspaceService.DeleteWorkspace(ctx, workspace.ID); err != nil {
			return workspaceHandlerErr("failed to delete workspace '%s': %w", workspaceName, err)
		}
	}

	if err := w.syncHandler.StopSyncSession(ctx); err != nil {
		return workspaceHandlerErr("failed to stop sync session: %w", err)
	}

	if runtime.Args.IsRemoveStorage() {
		workspaceStorages, err := w.workspaceStorageService.GetCurrentWorkspaceStorages(ctx)
		if err != nil {
			return workspaceHandlerErr("failed to get workspace storage '%s': %w", workspaceName, err)
		}

		for _, workspaceStorage := range workspaceStorages {
			if workspaceStorage.WorkspaceName == workspace.Name {
				if err := w.workspaceStorageService.DeleteWorkspaceStorage(ctx, workspaceStorage.ID); err != nil {
					if err.Code == http.StatusNotFound {
						logrus.Infof("workspace storage for workspace '%s' not found", workspaceName)
						return nil
					}
					return workspaceHandlerErr("failed to delete workspace storage '%s': %w", workspaceName, err)
				}
				return nil
			}
		}
		return nil
	}
	return nil
}
