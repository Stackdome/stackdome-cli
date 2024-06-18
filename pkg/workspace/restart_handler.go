package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/ashishmax31/voyager-cli/pkg/mapper"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

func (w *WorkspaceHandler) Restart(ctx context.Context, resourceName string) error {
	initialized, err := w.syncHandler.Initialized(ctx)
	if err != nil {
		return workspaceHandlerErr("failed to check sync session status: %w", err)
	}
	if initialized {
		if err := w.Sync(ctx); err != nil {
			return err
		}
		if resourceName == "all" {
			fmt.Println("Restarting all resources...")
		} else {
			fmt.Printf("Restarting '%s'...\n", resourceName)
		}
		desiredWS := mapper.MapVoyagerFileToWorkspaceCR(
			w.userdefinedWorkspace,
			w.session.Config().Username,
			w.session.Config().ProviderConfig.Namespace,
			w.session.Config().Organisation,
			w.session.Config().ProviderConfig.WorkspaceDomain,
		)
		existingWS, present, err := w.getWorkspace(ctx, desiredWS)
		if err != nil {
			return err
		}
		if present {
			if resourceName == "all" {
				w.markResourceForRestart(existingWS, resourceName, true)
			} else {
				w.markResourceForRestart(existingWS, resourceName, false)
			}
			logrus.Debugf("restart requested WS: %+v\n", existingWS.Spec.Resources)
			return w.session.UpdateResourceInProvider(ctx, existingWS)
		}
		return fmt.Errorf("workspace not yet deployed. Please run voyager deploy first.")
	}
	return fmt.Errorf("sync session not running! Please run voyager sync init")
}

func (w *WorkspaceHandler) markResourceForRestart(ws *workspacev1alpha1.Workspace, resourceName string, restartAll bool) {
	for i := range ws.Spec.Resources {
		currResource := &ws.Spec.Resources[i]
		if currResource.Name == resourceName || restartAll {
			currResourceSpecPtr := &currResource.Spec
			currResourceSpecPtr.RestartRequest = ptr.To(metav1.NewTime(time.Now().UTC()))
		}
	}
}
