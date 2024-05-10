package syncsession

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
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
				fmt.Printf("sync session failed: %s", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.NoArgs,
	}
	syncCmd.Flags().StringVar(&syncSessionStartArgs.voyagerFilePath, "voyagerfile-path", "", "--voyagerfile-path=voyagerfile.yaml")
	return syncCmd
}

func startSyncSession() error {
	userWorkspace, err := common.UserWorkspace(syncSessionArgs.voyagerFilePath)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	currSession, err := session.NewSession(cfg)
	if err != nil {
		return err
	}

	if err := userWorkspace.Process(); err != nil {
		return err
	}

	syncHandler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	signalTermination := make(chan os.Signal, 1)
	signal.Notify(signalTermination, syscall.SIGINT, syscall.SIGTERM)
	exitedChan := make(chan struct{})
	go func() {
		if err := syncHandler.StartSyncSession(ctx); err != nil {
			fmt.Printf("sync session stopped with err: %s \n", err.Error())
		}
		close(exitedChan)
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
