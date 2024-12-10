package workspace

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/tools"
	"github.com/fsnotify/fsnotify"
)

func (w *workspaceHandler) Deploy(ctx context.Context, runtime *config.Runtime) error {
	stack, err := runtime.UserStack()
	if err != nil {
		return fmt.Errorf("failed to get user stack: %w", err)
	}

	currentWorkspaceName := w.runtime.Config().CurrentWorkspace
	if currentWorkspaceName == nil {
		return fmt.Errorf("current workspace not set")
	}

	if stack.HasVolumes() {
		currentWorkspaceStorage, err := w.reconcileWorskpaceStorage(ctx, stack.WorkspaceStorageName(), stack)
		if err != nil {
			return workspaceHandlerErr("failed to reconcile workspace storage: %w", err)
		}
		if _, waitErr := w.workspaceStorageService.WaitForCurrentWorkspaceStorageToBeAvailable(ctx, currentWorkspaceStorage.Name); waitErr != nil {
			return workspaceHandlerErr("workspace storage not available: %w", waitErr)
		}
	}

	if stack.HasSyncingVolumes() {
		// ensure syncing process is running, if not start a new sync process.
		running, err := w.syncHandler.SyncSessionRunning(ctx)
		if err != nil {
			return workspaceHandlerErr("failed to check sync session status: %w", err)
		}
		if !running {
			fmt.Println("stack has local syncing type volumes..initializing sync session...")
			if err := w.initializeSyncSession(ctx, runtime); err != nil {
				return err
			}
		}
	}

	if err := w.Sync(ctx); err != nil {
		return err
	}

	fmt.Println("Deploying stack...")

	if err := stack.ReadEnvFiles(); err != nil {
		return workspaceHandlerErr("failed to read env files: %w", err)
	}

	existingWorkspace, serr := w.workspaceService.GetWorkspaceByName(ctx, *currentWorkspaceName)
	if serr != nil {
		if serr.Code == http.StatusNotFound {
			fmt.Println("No existing workspace found. Creating a new one...")
			_, err := w.workspaceService.CreateWorkspace(ctx, stack)
			if err != nil {
				return err
			}
			fmt.Println("Workspace deployed successfully..")
			return nil
		}
		return serr
	}
	_, updateErr := w.workspaceService.UpdateWorkspace(ctx, existingWorkspace.ID, stack)
	if updateErr != nil {
		return fmt.Errorf("failed to update workspace: %w", updateErr)
	}
	fmt.Println("Workspace deployed successfully..")
	return nil
}

func (w *workspaceHandler) reconcileWorskpaceStorage(ctx context.Context, currentWorkspaceStorageName string, stack *v1alpha1.UserStack) (*v1alpha1.WorkspaceStorage, error) {
	existingWorkspaceStorage, serr := w.workspaceStorageService.GetCurrentWorkspaceStorage(ctx, currentWorkspaceStorageName)
	if serr != nil {
		if serr.Code == http.StatusNotFound {
			fmt.Println("No existing workspace storage found. Creating a new one...")
			createdWorkspaceStorage, err := w.workspaceStorageService.CreateWorkspaceStorage(ctx, stack)
			if err != nil {
				return nil, err
			}
			return createdWorkspaceStorage, nil
		}
		return nil, serr
	}
	existingWorkspaceStorage, err := w.workspaceStorageService.UpdateWorkspaceStorage(ctx, existingWorkspaceStorage.ID, stack)
	if err != nil {
		return nil, fmt.Errorf("failed to update workspace storage: %w", err)
	}

	return existingWorkspaceStorage, nil
}

func (w *workspaceHandler) initializeSyncSession(ctx context.Context, runtime *config.Runtime) error {
	configDir := w.runtime.ConfigDir

	executablePath, err := os.Executable()
	if err != nil {
		return err
	}

	// TODO: Move this to runtime.
	cmdArgs := []string{"voyager", "sync-session", "start"}
	voyagerFileFlagValue := runtime.Args.GetStackFilePath()
	cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", common.VoyagerFilePathFlag, voyagerFileFlagValue))

	logFile, err := w.runtime.CreateLogFile("sync-session-logs")
	if err != nil {
		return workspaceHandlerErr("error creating/opening sync-session-logs file: %w", err)
	}
	defer logFile.Close()

	syncSessionStartedWatcher := tools.NewFileSystemWatcher(
		configDir,
		tools.WithOperationFilter(fsnotify.Create),
		tools.WithFileWatches(w.syncHandler.SyncSessionRunningFlagPath()),
	)
	if err := syncSessionStartedWatcher.StartWatch(); err != nil {
		return err
	}

	syncProcess := &exec.Cmd{
		Path:   executablePath,
		Args:   cmdArgs,
		Stdout: logFile,
		Stderr: logFile,
	}
	// Wait for the forked process to complete.
	if err := syncProcess.Start(); err != nil {
		return workspaceHandlerErr("error when starting voyager init subcommand: %w", err)
	}

	if started, _ := w.syncHandler.SyncSessionRunning(ctx); started {
		fmt.Println("Sync session already runnning.")
		return nil
	}

	syncSessionProcessExitChan := make(chan error, 1)
	defer syncSessionStartedWatcher.Stop()
	go func() {
		syncSessionProcessExitChan <- syncProcess.Wait()
	}()
	select {
	case err := <-syncSessionProcessExitChan:
		return err
	case <-syncSessionStartedWatcher.NotifyChan():
		fmt.Println("Sync session started.")
	}
	return nil
}
