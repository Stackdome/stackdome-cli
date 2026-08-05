package main

import (
	"fmt"
	"os"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
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
	cmd.AddCommand(newVolumeCreateCmd())
	cmd.AddCommand(newVolumeDeleteCmd())
	return cmd
}

func newVolumeListCmd() *cobra.Command {
	var flagStack string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List volumes of a stack",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			volumes, err := ctx.Client.ListVolumes(cmd.Context(), stackID)
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

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
	return cmd
}

func newVolumeCreateCmd() *cobra.Command {
	var (
		flagSize       string
		flagAccessMode string
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a volume",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			if flagSize == "" {
				return clierrors.ValidationError("--size is required (e.g. 5Gi)")
			}

			volume, err := ctx.Client.CreateVolume(cmd.Context(), args[0], flagSize, flagAccessMode)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(volume)
			}

			fmt.Fprintf(os.Stderr, "Volume %q created.\n", volume.Name)
			return nil
		})),
	}

	cmd.Flags().StringVar(&flagSize, "size", "", "Volume size (e.g. 5Gi)")
	cmd.Flags().StringVar(&flagAccessMode, "access-mode", "ReadWriteOnce", "Access mode (ReadWriteOnce, ReadWriteMany, ReadOnlyMany)")
	return cmd
}

func newVolumeDeleteCmd() *cobra.Command {
	var (
		flagYes   bool
		flagStack string
	)

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a volume",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			volume, err := ctx.Client.FindVolumeByName(cmd.Context(), stackID, args[0])
			if err != nil {
				return err
			}
			if volume == nil {
				return clierrors.NotFoundError("Volume", args[0])
			}

			if _, err := cmdutil.Confirm(ctx.Formatter, fmt.Sprintf("Delete volume %q?", args[0]), flagYes); err != nil {
				return err
			}

			if err := ctx.Client.DeleteVolume(cmd.Context(), *volume.Id); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Volume %q deleted.\n", args[0])
			return nil
		})),
	}

	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation")
	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
	return cmd
}

func volumePhase(v openapi.Volume) string {
	if v.Status == nil || v.Status.Phase == nil {
		return "Unknown"
	}
	return *v.Status.Phase
}
