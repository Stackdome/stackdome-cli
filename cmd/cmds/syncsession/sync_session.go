package syncsession

import (
	"github.com/spf13/cobra"
)

var syncSessionArgs struct {
	voyagerFilePath string
}

func NewSyncSessionCommand() *cobra.Command {
	var syncCmd = &cobra.Command{
		Use:   "sync-session start|stop",
		Short: "Manage a sync sync session",
		Long:  `Manage a sync sync session`,
		Args:  cobra.NoArgs,
	}

	syncCmd.AddCommand(newSyncSessionStartCommand(), newSyncSessionStopCommand())
	return syncCmd
}
