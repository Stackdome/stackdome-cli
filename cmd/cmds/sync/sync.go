package sync

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
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
		RunE:  sync,
		Args:  cobra.NoArgs,
	}
	syncCmd.Flags().StringVar(&syncArgs.voyagerFilePath, "voyagerfile-path", "", "--voyagerfile-path=voyagerfile.yaml")
	return syncCmd
}

func sync(_ *cobra.Command, args []string) error {
	var voyagerFilePath string
	if len(syncArgs.voyagerFilePath) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		voyagerFilePath, err = common.FindVoyagerFile(cwd)
		if err != nil {
			return err
		}
	} else {
		voyagerFilePath = syncArgs.voyagerFilePath
	}
	if len(voyagerFilePath) == 0 {
		return fmt.Errorf("voyager file missing")
	}
	_, err := os.Stat(voyagerFilePath)
	if err != nil {
		return fmt.Errorf("failed to stat voyagerfile at %s: %w", voyagerFilePath, err)
	}

	if err := userworkspace.Validate(voyagerFilePath); err != nil {
		return fmt.Errorf("voyager file not valid: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	currSession, err := session.NewSessionWithProvider(cfg)
	if err != nil {
		return err
	}
	userWorkspace, err := userworkspace.Unmarshal(voyagerFilePath)
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
	return nil
}
