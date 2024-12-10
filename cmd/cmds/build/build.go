package build

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var buildArgs struct {
	stackFilePath string
	all           bool
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
	buildCmd.Flags().StringVar(&buildArgs.stackFilePath, common.VoyagerFilePathFlag, "./voyagerfile.yaml", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return buildCmd
}

func build(ctx context.Context, args []string) error {
	if len(args) == 0 && !buildArgs.all {
		return fmt.Errorf("atleast one argument is required or pass --all flag")
	}

	if len(args) == 0 {
		args = append(args, "")
	}

	runtime, err := config.NewRuntime("build", config.Args{
		StackFilePath: &buildArgs.stackFilePath,
		AllResources:  &buildArgs.all,
		ResourceName:  &args[0],
	})

	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		return fmt.Errorf("failed to create workspace handler: %w", err)
	}
	return handler.Build(ctx, runtime)
}
