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
