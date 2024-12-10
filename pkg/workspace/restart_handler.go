package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/config"
)

func (w *workspaceHandler) Restart(ctx context.Context, runtime *config.Runtime) error {
	currentWorkspaceName := runtime.Config().CurrentWorkspace
	if currentWorkspaceName == nil {
		return workspaceHandlerErr("current workspace not set")
	}

	if runtime.Args.IsAllResources() {
		return w.restartAllResources(ctx, runtime)
	}

	resourceName := runtime.Args.GetResourceName()
	if resourceName == "" {
		return workspaceHandlerErr("resource name not specified")
	}

	return w.restartResource(ctx, resourceName)
}

func (w *workspaceHandler) restartAllResources(ctx context.Context, runtime *config.Runtime) error {
	currentWorkspace, err := w.workspaceService.GetWorkspaceByName(ctx, *runtime.Config().CurrentWorkspace)
	if err != nil {
		return workspaceHandlerErr("failed to get current workspace '%s': %w", *runtime.Config().CurrentWorkspace, err)
	}

	if currentWorkspace.HasLocalSyncingVolumes() {
		if err := w.handleStorageSync(ctx); err != nil {
			return workspaceHandlerErr("failed to sync: %w", err)
		}
	}

	if err := w.workspaceService.RestartAllResources(ctx, currentWorkspace); err != nil {
		return workspaceHandlerErr("failed to restart all resources: %w", err)
	}
	fmt.Printf("workspace '%s' marked for restart\n", currentWorkspace.Name)
	return nil
}

func (w *workspaceHandler) restartResource(ctx context.Context, resourceName string) error {
	currentWorkspace, err := w.workspaceService.GetWorkspaceByName(ctx, *w.runtime.Config().CurrentWorkspace)
	if err != nil {
		return workspaceHandlerErr("failed to get current workspace '%s': %w", *w.runtime.Config().CurrentWorkspace, err)
	}

	resource := currentWorkspace.GetResourceByName(resourceName)
	if resource == nil {
		return workspaceHandlerErr("resource '%s' not found", resourceName)
	}

	if currentWorkspace.ResourceHasLocalSyncingVolume(resourceName) {
		if err := w.handleStorageSync(ctx); err != nil {
			return workspaceHandlerErr("failed to sync: %w", err)
		}
	}

	if err := w.workspaceService.RestartResource(ctx, currentWorkspace, resourceName); err != nil {
		return workspaceHandlerErr("failed to restart resource '%s': %w", resourceName, err)
	}
	fmt.Printf("resource '%s' marked for restart\n", resourceName)
	return nil
}

func (w *workspaceHandler) handleStorageSync(ctx context.Context) error {
	initialized, werr := w.syncHandler.Initialized(ctx)
	if werr != nil {
		return workspaceHandlerErr("failed to check sync session status: %w", werr)
	}
	if !initialized {
		return workspaceHandlerErr("sync session not running! Please run voyager sync init")
	}
	if err := w.syncHandler.ForceSync(ctx); err != nil {
		return workspaceHandlerErr("failed to force sync: %w", err)
	}
	return nil
}
