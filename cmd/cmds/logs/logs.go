package logs

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var logsArgs struct {
	all       bool
	tailLines int64
	follow    bool
}

func NewLogsCommand() *cobra.Command {
	var logsCmd = &cobra.Command{
		Use:   "logs",
		Short: "Get the logs of a resource",
		Long:  `Get the logs of a resource. Pass --all or -a flag to print the logs of all resources.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := logs(context.Background(), args); err != nil {
				fmt.Printf("logs error: %s \n", err.Error())
				os.Exit(1)
			}
		},
	}
	logsCmd.Flags().BoolVarP(&logsArgs.all, "all", "a", false, "-a or --all")
	logsCmd.Flags().BoolVarP(&logsArgs.follow, "follow", "f", false, "-f or --follow")
	logsCmd.Flags().Int64VarP(&logsArgs.tailLines, "tail", "t", 100, "-t=10 or --tail=10")
	return logsCmd
}

func logs(ctx context.Context, args []string) error {
	if len(args) == 0 && !logsArgs.all {
		return fmt.Errorf("atleast one argument is required or pass --all flag")
	}

	if len(args) == 0 {
		args = append(args, "")
	}

	runtime, err := config.NewRuntime("logs", config.Args{
		ResourceName: &args[0],
		AllResources: &logsArgs.all,
		TailLines:    &logsArgs.tailLines,
		Follow:       &logsArgs.follow,
	})

	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		return fmt.Errorf("failed to create workspace handler: %w", err)
	}

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	signalTermination := make(chan os.Signal, 1)
	signal.Notify(signalTermination, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalTermination
		cancelFn()
	}()
	return ignoreCtxCancelledErr(handler.GetLogs(ctx, runtime))
}

func ignoreCtxCancelledErr(err error) error {
	if err == context.Canceled {
		return nil
	}
	return err
}
