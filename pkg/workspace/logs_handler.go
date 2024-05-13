package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/mapper"
	"github.com/ashishmax31/voyager-cli/pkg/provider"
	"github.com/ashishmax31/voyager-cli/pkg/provider/k8s"
)

func (w *WorkspaceHandler) GetLogs(ctx context.Context, resourceRef string, follow bool, tailLines int64) error {
	desiredWS := mapper.MapVoyagerFileToWorkspaceCR(
		w.userdefinedWorkspace,
		w.session.Config().Username,
		w.session.Config().ProviderConfig.Namespace,
	)
	existingWS, WSpresent, WsErr := w.getWorkspace(ctx, desiredWS)
	if WsErr != nil {
		return WsErr
	}
	if !WSpresent {
		return fmt.Errorf("workspace not yet deployed. Please run voyager deploy first.")
	}

	targets := make([]provider.Target, 0)
	for _, resourceSpec := range existingWS.Spec.Resources {
		if resourceRef == resourceSpec.Name || resourceRef == "all" {
			WSresource, err := w.getWorkspaceResource(ctx, resourceSpec.Name)
			if err != nil {
				return err
			}
			targets = append(targets, k8s.NewServiceTarget(*WSresource.Status.InternalAddress))
		}
	}

	logOptions := provider.LogOptions{}
	if follow {
		logOptions.Follow = true
	} else {
		logOptions.TailLines = tailLines
	}

	return w.provider.StreamLogs(ctx, targets, logOptions)
}
