package restart

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var restartArgs struct {
	all bool
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
	return restartCmd
}

func restart(ctx context.Context, args []string) error {
	if len(args) == 0 && !restartArgs.all {
		return fmt.Errorf("atleast one argument is required or pass --all flag")
	}

	// If no resource name is provided, then pass an empty string
	if len(args) == 0 {
		args = append(args, "")
	}

	runtime, err := config.NewRuntime("restart", config.Args{
		AllResources: &restartArgs.all,
		ResourceName: &args[0],
	})
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		return fmt.Errorf("failed to create workspace handler: %w", err)
	}

	return handler.Restart(ctx, runtime)
}
