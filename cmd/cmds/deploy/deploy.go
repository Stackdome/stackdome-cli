package deploy

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var deployArgs struct {
	voyagerFilePath string
}

func NewDeployCommand() *cobra.Command {
	var deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the resources specified in the voyagerfile",
		Long:  `Deploy the resources specified in the voyagerfile`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := deploy(context.Background()); err != nil {
				fmt.Printf("deploy error: %s \n", err.Error())
			}
		},
		Args: cobra.NoArgs,
	}
	deployCmd.Flags().StringVar(&deployArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return deployCmd
}

func deploy(ctx context.Context) error {
	userWorkspace, err := common.UserWorkspace(deployArgs.voyagerFilePath)
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

	handler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}

	return handler.Deploy(ctx)
}
