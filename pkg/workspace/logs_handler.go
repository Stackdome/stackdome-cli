package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/provider"
	"github.com/ashishmax31/voyager-cli/pkg/provider/k8s"
)

func (w *workspaceHandler) GetLogs(ctx context.Context, runtime *config.Runtime) error {
	currentWorkspaceName := runtime.Config().CurrentWorkspace
	if currentWorkspaceName == nil {
		return workspaceHandlerErr("current workspace is not set")
	}

	currentWorkspace, serr := w.workspaceService.GetWorkspaceByName(
		ctx,
		*currentWorkspaceName,
	)
	if serr != nil {
		return workspaceHandlerErr("failed to fetch current workspace '%s': %w", *currentWorkspaceName, serr)
	}

	logTargets := make([]provider.Target, 0)
	if runtime.Args.IsAllResources() {
		for _, resource := range currentWorkspace.Resources {
			if resource.Status.IsAvailable() {
				logTargets = append(logTargets, k8s.NewServiceTarget(*resource.Status.InternalServiceName))
			} else {
				fmt.Printf("skipping resource '%s' as its not available", resource.Name)
			}
		}
		if len(logTargets) == 0 {
			return workspaceHandlerErr("no resources available in workspace")
		}
	} else {
		for _, resource := range currentWorkspace.Resources {
			if resource.Name == runtime.Args.GetResourceName() {
				if resource.Status.IsAvailable() {
					logTargets = append(logTargets, k8s.NewServiceTarget(*resource.Status.InternalServiceName))
				} else {
					return workspaceHandlerErr("resource '%s' is not available", runtime.Args.GetResourceName())
				}
			}
		}
		if len(logTargets) == 0 {
			return workspaceHandlerErr("resource '%s' not found in workspace", runtime.Args.GetResourceName())
		}
	}

	return w.provider.StreamLogs(ctx, logTargets, provider.LogOptions{
		Follow:    runtime.Args.IsFollow(),
		TailLines: runtime.Args.GetTailLines(),
	})
}
