package workspace

import (
	"context"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/config"
)

func (h *workspaceHandler) ListWorkspaces(ctx context.Context, runtime *config.Runtime) ([]*v1alpha1.Workspace, error) {
	workspaces, err := h.workspaceService.GetCurrentWorkspaces(ctx)
	if err != nil {
		return nil, workspaceHandlerErr("failed to list workspaces: %w", err)
	}
	return workspaces, nil
}

func (h *workspaceHandler) ListWorkspaceStorages(ctx context.Context, runtime *config.Runtime) ([]*v1alpha1.WorkspaceStorage, error) {
	workspaces, err := h.workspaceStorageService.GetCurrentWorkspaceStorages(ctx)
	if err != nil {
		return nil, workspaceHandlerErr("failed to list workspace storages: %w", err)
	}
	return workspaces, nil
}

func (h *workspaceHandler) ListWorkspaceBuilds(ctx context.Context, runtime *config.Runtime) ([]v1alpha1.ResourceBuild, error) {
	currentWorkspaceName := runtime.Config().CurrentWorkspace
	if currentWorkspaceName == nil {
		return nil, workspaceHandlerErr("current workspace not set")
	}
	currentWorkspace, err := h.workspaceService.GetWorkspaceByName(ctx, *currentWorkspaceName)
	if err != nil {
		return nil, workspaceHandlerErr("failed to get current workspace '%s': %w", *currentWorkspaceName, err)
	}

	if runtime.Args.IsAllResources() {
		builds, err := h.workspaceService.ListWorkspaceBuilds(ctx, currentWorkspace)
		if err != nil {
			return nil, workspaceHandlerErr("failed to list workspace builds: %w", err)
		}
		return builds, nil
	}

	resourceName := runtime.Args.GetResourceName()
	if resourceName == "" {
		return nil, workspaceHandlerErr("resource name not specified")
	}
	resource := currentWorkspace.GetResourceByName(resourceName)
	if resource == nil {
		return nil, workspaceHandlerErr("resource '%s' not found in the workspace", resourceName)
	}

	if resource.BuildConfig == nil {
		return nil, workspaceHandlerErr("resource '%s' does not have a build configuration", resourceName)
	}

	builds, err := h.workspaceService.ListWorkspaceResourceBuilds(ctx, currentWorkspace, resource)
	if err != nil {
		return nil, workspaceHandlerErr("failed to list resource builds: %w", err)
	}

	return builds, nil
}
