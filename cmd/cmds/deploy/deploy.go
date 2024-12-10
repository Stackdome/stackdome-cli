package deploy

import (
	"context"
	"fmt"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/config"
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
	deployCmd.Flags().StringVar(&deployArgs.voyagerFilePath, common.VoyagerFilePathFlag, "./voyagerfile.yaml", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return deployCmd
}

func deploy(ctx context.Context) error {
	runtime, err := config.NewRuntime("deploy", config.Args{
		StackFilePath: &deployArgs.voyagerFilePath,
	})
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		return fmt.Errorf("failed to create workspace handler: %w", err)
	}

	return handler.Deploy(ctx, runtime)
}
