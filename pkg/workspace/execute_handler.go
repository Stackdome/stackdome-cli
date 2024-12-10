package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/provider/k8s"
)

func (w *workspaceHandler) Execute(ctx context.Context, runtime *config.Runtime) error {
	currentWorkspaceName := runtime.Config().CurrentWorkspace
	if currentWorkspaceName == nil {
		workspaceHandlerErr("current workspace not set")
	}
	targetResourceName := runtime.Args.GetResourceName()
	if targetResourceName == "" {
		workspaceHandlerErr("resource name not specified")
	}

	executeCmd := runtime.Args.ExecuteCmd
	if len(executeCmd) == 0 {
		workspaceHandlerErr("no command specified")
	}

	currentWorkspace, serr := w.workspaceService.GetWorkspaceByName(ctx, *currentWorkspaceName)
	if serr != nil {
		return workspaceHandlerErr("failed to get current workspace '%s': %w", *currentWorkspaceName, serr)
	}

	targetResource, err := findWorkspaceResource(currentWorkspace, targetResourceName)
	if err != nil {
		return workspaceHandlerErr("failed to find resource '%s' in workspace '%s': %w", targetResourceName, *currentWorkspaceName, err)
	}

	if !targetResource.IsAvailable() {
		return workspaceHandlerErr("resource '%s' is not yet available", targetResourceName)
	}

	if err := w.provider.Execute(ctx,
		k8s.NewServiceTarget(*targetResource.Status.InternalServiceName), runtime.Args.ExecuteCmd, runtime.Args.IsInteractive()); err != nil {
		return workspaceHandlerErr("failed to execute command on resource '%s': %w", targetResourceName, err)
	}
	return nil
}

func findWorkspaceResource(workspace *v1alpha1.Workspace, resourceName string) (*v1alpha1.WorkspaceResource, error) {
	for _, resource := range workspace.Resources {
		if resource.Name == resourceName {
			return &resource, nil
		}
	}
	return nil, fmt.Errorf("resource '%s' not found in workspace '%s'", resourceName, workspace.Name)
}
