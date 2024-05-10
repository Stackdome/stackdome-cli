package sync

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

var syncArgs struct {
	voyagerFilePath string
}

func NewSyncCommand() *cobra.Command {
	var syncCmd = &cobra.Command{
		Use:   "sync",
		Short: "sync local directories mentioned in the voyagerfile against remote volumes",
		Long:  `sync local directories mentioned in the voyagerfile against remote volumes`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := sync(); err != nil {
				fmt.Printf("failed to sync local directories against remote volumes: %s", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.NoArgs,
	}
	syncCmd.Flags().StringVar(&syncArgs.voyagerFilePath, "voyagerfile-path", "", "--voyagerfile-path=voyagerfile.yaml")
	return syncCmd
}

func sync() error {
	userWorkspace, err := common.UserWorkspace(syncArgs.voyagerFilePath)
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
