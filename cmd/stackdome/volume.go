package main

import (
	"fmt"
	"os"

	openapi "github.com/ashishmax31/stackdome-api-server/pkg/api/openapi"
	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
)

func newVolumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Manage volumes",
	}

	cmd.AddCommand(newVolumeListCmd())
	cmd.AddCommand(newVolumeDeleteCmd())
	return cmd
}

func newVolumeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List volumes",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			volumes, err := ctx.Client.ListVolumes(cmd.Context())
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(volumes)
			}

			if len(volumes) == 0 {
				fmt.Fprintln(os.Stderr, "No volumes found.")
				return nil
			}

			tbl := ctx.Formatter.NewTable("NAME", "SIZE", "ACCESS MODE", "PHASE")
			for _, v := range volumes {
				tbl.AddRow(
					v.Name,
					v.Spec.Size,
					string(v.Spec.AccessMode),
					volumePhase(v),
				)
			}
			tbl.Render()
			return nil
		})),
	}
}

func newVolumeDeleteCmd() *cobra.Command {
	var flagYes bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a volume",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			volume, err := ctx.Client.FindVolumeByName(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if volume == nil {
				return clierrors.NotFoundError("Volume", args[0])
			}

			if !flagYes {
				fmt.Fprintf(os.Stderr, "Delete volume %q? [y/N]: ", args[0])
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "y" && confirm != "Y" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			if err := ctx.Client.DeleteVolume(cmd.Context(), *volume.Id); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Volume %q deleted.\n", args[0])
			return nil
		})),
	}

	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation")
	return cmd
}

func volumePhase(v openapi.Volume) string {
	if v.Status == nil || v.Status.Phase == nil {
		return "Unknown"
	}
	return *v.Status.Phase
}
