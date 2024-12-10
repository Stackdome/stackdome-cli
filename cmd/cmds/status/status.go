package status

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"

	"github.com/spf13/cobra"
)

var statusArgs struct {
	all bool
}

func NewStatusCommand() *cobra.Command {
	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Get the status of a resource/all resources",
		Long:  `Get the status of a resource/all resources. Pass --all or -a flag to print the status of all resources.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := status(context.Background(), args); err != nil {
				fmt.Printf("status error: %s \n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.RangeArgs(0, 1),
	}
	statusCmd.Flags().BoolVarP(&statusArgs.all, common.AllResourcesFlag, "a", false, fmt.Sprintf("--%s", common.AllResourcesFlag))
	return statusCmd
}

func status(ctx context.Context, args []string) error {
	if !statusArgs.all && len(args) == 0 {
		return fmt.Errorf("no resources specified. Run status <resourceName> to get the status of the resource")
	}

	if len(args) == 0 {
		args = append(args, "")
	}

	runtime, err := config.NewRuntime("status", config.Args{
		AllResources: &statusArgs.all,
		ResourceName: &args[0],
	})
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)

	if err != nil {
		return fmt.Errorf("failed to create workspace handler: %w", err)
	}

	return handler.Status(ctx, runtime)
}
