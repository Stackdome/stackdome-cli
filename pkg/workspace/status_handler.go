package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/config"
)

func (w *workspaceHandler) Status(ctx context.Context, runtime *config.Runtime) error {
	currentWorkspaceName := runtime.Config().CurrentWorkspace
	if currentWorkspaceName == nil {
		return workspaceHandlerErr("current workspace not set")
	}

	currentWorskspaceStorageName, rerr := runtime.CurrentWorkspaceStorageName()
	if rerr != nil {
		return workspaceHandlerErr("failed to get current workspace storage: %w", rerr)
	}

	existingWorkspace, err := w.workspaceService.GetWorkspaceByName(ctx, *currentWorkspaceName)
	if err != nil {
		return workspaceHandlerErr("failed to get workspace: %w", err)
	}

	existingWorkspaceStorage, err := w.workspaceStorageService.GetCurrentWorkspaceStorage(ctx, currentWorskspaceStorageName)
	if err != nil {
		return workspaceHandlerErr("failed to get workspace storage: %w", err)
	}

	if runtime.Args.IsAllResources() {
		fmt.Printf("getting status for all resources in workspace '%s'...\n", existingWorkspace.Name)
		return nil
	}
	// Storage print

	fmt.Printf("getting status for resource '%s' in workspace '%s'...\n", runtime.Args.GetResourceName(), existingWorkspaceStorage.Name)
	return nil
}
