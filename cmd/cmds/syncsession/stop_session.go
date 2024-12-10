package syncsession

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

func newSyncSessionStopCommand() *cobra.Command {
	var syncCmd = &cobra.Command{
		Use:   "stop",
		Short: "stop a sync sync session",
		Long:  `Stop a sync sync session`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := stopSyncSession(); err != nil {
				fmt.Printf("failed to stop sync session: %s\n", err.Error())
				os.Exit(1)
			}
		},
		Args: cobra.NoArgs,
	}
	return syncCmd
}

func stopSyncSession() error {
	runtime, err := config.NewRuntime("sync", config.Args{})
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	handler, err := workspace.NewWorkspaceHandler(runtime)
	if err != nil {
		return fmt.Errorf("failed to create workspace handler: %w", err)
	}

	return handler.StopSyncSession(context.Background())
}
