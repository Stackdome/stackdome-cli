/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var syncArgs struct {
	voyagerFilePath string
}

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: sync,
	Args: cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().StringVar(&syncArgs.voyagerFilePath, "voyagerfile-path", "", "--voyagerfile-path=voyagerfile.yaml")
}

func sync(cmd *cobra.Command, args []string) error {
	var voyagerFilePath string
	if len(syncArgs.voyagerFilePath) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		voyagerFilePath, err = findVoyagerFile(cwd)
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
	syncHandler, err := workspace.NewWorkspaceStorageHandler(currSession, userWorkspace)
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
