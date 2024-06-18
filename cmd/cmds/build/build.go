package build

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var buildArgs struct {
	voyagerFilePath string
	all             bool
}

func NewBuildCommand() *cobra.Command {
	var buildCmd = &cobra.Command{
		Use:   "build",
		Short: "Trigger a new build for a resource/all resources.",
		Long:  `Trigger a new build for a resource/all resources. Pass --all or -a flag to trigger a new build of all resources.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := build(context.Background(), args); err != nil {
				fmt.Printf("build error: %s \n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.RangeArgs(0, 1),
	}
	buildCmd.Flags().BoolVarP(&buildArgs.all, "all", "a", false, "-a or --all")
	buildCmd.Flags().StringVar(&buildArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return buildCmd
}

func build(ctx context.Context, args []string) error {
	userWorkspace, err := common.UserWorkspace(buildArgs.voyagerFilePath)
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

	if err := common.ValidateResourceNameRef(args, userWorkspace, buildArgs.all); err != nil {
		return err
	}
	handler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}
	if buildArgs.all {
		args = []string{"all"}
	}
	return handler.Build(ctx, args[0])
}
