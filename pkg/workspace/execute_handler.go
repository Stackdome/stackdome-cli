package workspace

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/provider/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (w *WorkspaceHandler) Execute(ctx context.Context, resourceName string, cmd []string, interactive bool) error {
	workspaceResource, err := w.getWorkspaceResource(ctx, resourceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("workspace not yet deployed. Please run voyager deploy first.")
		}
		return err
	}

	return w.provider.Execute(ctx, k8s.NewServiceTarget(*workspaceResource.Status.InternalAddress), cmd, interactive)
}
