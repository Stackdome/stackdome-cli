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
	all             bool
}

func NewRestartCommand() *cobra.Command {
	var restartCmd = &cobra.Command{
		Use:   "restart",
		Short: "Restart a resource.",
		Long:  `Restart a resource. Pass --all or -a flag to restart all resources`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := restart(context.Background(), args); err != nil {
				fmt.Printf("restart error: %s \n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.RangeArgs(0, 1),
	}
	restartCmd.Flags().BoolVarP(&restartArgs.all, "all", "a", false, "-a or --all")
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
	currSession, err := session.NewSession(cfg, true)
	if err != nil {
		return err
	}

	if err := userWorkspace.Process(); err != nil {
		return err
	}

	if err := common.ValidateResourceNameRef(args, userWorkspace, restartArgs.all); err != nil {
		return err
	}
	handler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}
	if restartArgs.all {
		args = []string{"all"}
	}
	return handler.Restart(ctx, args[0])
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
