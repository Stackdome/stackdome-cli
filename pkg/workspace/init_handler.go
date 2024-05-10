package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/mapper"
	"github.com/ashishmax31/voyager-cli/pkg/process"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

func (w *WorkspaceHandler) Init(ctx context.Context) error {
	workspaceStorage := mapper.MapVoyagerFileToWorkspaceStorage(
		w.userdefinedWorkspace,
		w.session.Config().Username,
		w.session.Config().ProviderConfig.Namespace,
	)
	if err := w.ensureWorkspaceStorage(ctx, &workspaceStorage); err != nil {
		return err
	}
	ctx, cancelFn := context.WithTimeout(ctx, time.Minute)
	defer cancelFn()
	existingWS := &workspacev1alpha1.WorkspaceStorage{}
	err := wait.PollUntilContextCancel(ctx, time.Second*10, true, func(ctx context.Context) (done bool, err error) {
		fmt.Println("Waiting for workspace storage to be ready...")
		getErr := w.session.GetResourceFromProvider(
			ctx,
			types.NamespacedName{Name: workspaceStorage.Name, Namespace: workspaceStorage.Namespace},
			existingWS,
		)
		if getErr != nil {
			return false, err
		}
		availableCond := meta.FindStatusCondition(existingWS.Status.Conditions, string(workspacev1alpha1.WorkspaceStorageAvailable))
		return availableCond != nil && (availableCond.Status == metav1.ConditionTrue), nil
	})
	if err != nil {
		return workspaceHandlerErr("timedout waiting for workspace storage to be ready")
	}
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}

	cmdArgs := []string{"voyager", "sync-session", "start"}
	voyagerFileFlagValue := process.GetCurrentProcessFlag(common.VoyagerFilePathFlag)
	if voyagerFileFlagValue != nil {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", common.VoyagerFilePathFlag, *voyagerFileFlagValue))
	}
	syncProcess := &exec.Cmd{
		Path:   executablePath,
		Args:   cmdArgs,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	// Wait for the forked process to complete.
	if err := syncProcess.Run(); err != nil {
		return workspaceHandlerErr("error when starting voyager init subcommand: %w", err)
	}
	return nil
}

// TODO: Use provider interface for errors from provider and equality checks.
func (w *WorkspaceHandler) ensureWorkspaceStorage(ctx context.Context, desiredWS *workspacev1alpha1.WorkspaceStorage) error {
	existingWS := &workspacev1alpha1.WorkspaceStorage{}
	if err := w.session.GetResourceFromProvider(
		ctx,
		types.NamespacedName{Name: desiredWS.Name, Namespace: desiredWS.Namespace},
		existingWS); err != nil {
		if apierrors.IsNotFound(err) {
			return w.session.CreateResourceInProvider(ctx, desiredWS)
		}
		return err
	}
	if !equality.Semantic.DeepDerivative(desiredWS.Spec, existingWS.Spec) {
		existingWS.Spec = desiredWS.Spec
		return w.session.UpdateResourceInProvider(ctx, existingWS)
	}
	return nil
}
