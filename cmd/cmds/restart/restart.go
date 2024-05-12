package restart

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var restartArgs struct {
	voyagerFilePath string
}

func NewRestartCommand() *cobra.Command {
	var restartCmd = &cobra.Command{
		Use:   "restart",
		Short: "Restart a resource/all resources.",
		Long:  `Restart a resource/all resources.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := restart(context.Background(), args); err != nil {
				fmt.Printf("restart error: %s \n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.ExactArgs(1),
	}
	restartCmd.Flags().StringVar(&restartArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return restartCmd
}

func restart(ctx context.Context, args []string) error {
	userWorkspace, err := common.UserWorkspace(restartArgs.voyagerFilePath)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// Provider initialized.
	currSession, err := session.NewSession(cfg)
	if err != nil {
		return err
	}

	if err := userWorkspace.Process(); err != nil {
		return err
	}

	resourceToRestart := args[0]
	if err := validateResourceNameRef(resourceToRestart, userWorkspace); err != nil {
		return err
	}
	handler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}
	return handler.Restart(ctx, resourceToRestart)
}

func validateResourceNameRef(resourceName string, ws *userworkspace.Workspace) error {
	if resourceName == "all" {
		return nil
	}
	if _, found := ws.Resources[resourceName]; !found {
		return fmt.Errorf("resource '%s' not found in voyagerfile.[Please enter a valid resource defined in the voyagerfile or 'all']", resourceName)
	}
	return nil
}
