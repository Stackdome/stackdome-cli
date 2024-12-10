package exec

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

var execArgs struct {
	interactive bool
}

func NewExecCommand() *cobra.Command {
	var execCmd = &cobra.Command{
		Use:   "exec",
		Short: "Execute a command inside a workspace resource.",
		Long:  `Execute a command inside a workspace resource. Pass -i or --i flag for starting an interactive session.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := exec(context.Background(), args); err != nil {
				fmt.Printf("exec error: %s \n", err.Error())
				os.Exit(1)
			}
		},
	}
	execCmd.Flags().BoolVarP(&execArgs.interactive, "i", "i", false, "-i or --i")
	return execCmd
}

func exec(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("missing required arguments.. usage: voyager exec <resource-name> <command>")
	}

	runtime, err := config.NewRuntime("exec", config.Args{
		ResourceName: &args[0],
		Interactive:  &execArgs.interactive,
		ExecuteCmd:   args[1:],
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
	return ignoreCtxCancelledErr(handler.Execute(ctx, runtime))
}

func ignoreCtxCancelledErr(err error) error {
	if err == context.Canceled {
		return nil
	}
	return err
}
