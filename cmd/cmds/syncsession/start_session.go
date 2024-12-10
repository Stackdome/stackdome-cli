package syncsession

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var syncSessionStartArgs struct {
	voyagerFilePath string
}

func newSyncSessionStartCommand() *cobra.Command {
	var syncCmd = &cobra.Command{
		Use:   "start",
		Short: "start a sync sync session",
		Long:  `start a sync sync session which is responsible for syncing local directories mentioned in the voyagerfile against remote volumes`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := startSyncSession(); err != nil {
				fmt.Printf("sync session failed: %s\n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.NoArgs,
	}
	syncCmd.Flags().StringVar(&syncSessionStartArgs.voyagerFilePath, common.VoyagerFilePathFlag, "", fmt.Sprintf("--%s=voyagerfile.yaml", common.VoyagerFilePathFlag))
	return syncCmd
}

func startSyncSession() error {
	runtime, err := config.NewRuntime("start-sync", config.Args{
		StackFilePath: &syncSessionStartArgs.voyagerFilePath,
	})
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	syncHandler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		return fmt.Errorf("failed to create workspace handler: %w", err)
	}

	userstack, err := runtime.UserStack()
	if err != nil {
		return fmt.Errorf("failed to get user stack: %w", err)
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	signalTermination := make(chan os.Signal, 1)
	signal.Notify(signalTermination, syscall.SIGINT, syscall.SIGTERM)
	exitedChan := make(chan struct{})
	go func() {
		defer close(exitedChan)
		if err := syncHandler.StartSyncSession(ctx, userstack); err != nil {
			fmt.Printf("sync session stopped with err: %s \n", err.Error())
		}
	}()
	select {
	case <-exitedChan:
		return nil
	case <-signalTermination:
		cancelFn()
		<-exitedChan
	}
	fmt.Printf("Successfully synced volumes..")
	return nil
}
