package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
)

func newDestroyCmd() *cobra.Command {
	var (
		flagYes   bool
		flagStack string
	)

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Delete the current stack",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
			if err != nil {
				return err
			}

			prompt := fmt.Sprintf("This will permanently delete stack %q. Continue?", stack.Name)
			if _, err := cmdutil.Confirm(ctx.Formatter, prompt, flagYes); err != nil {
				return err
			}

			if err := ctx.Client.DeleteStack(cmd.Context(), stackID); err != nil {
				return err
			}

			if ctx.Config.CurrentStack == stackID {
				_ = ctx.Config.SetCurrentStack("")
			}

			fmt.Fprintf(os.Stderr, "Stack %q deletion initiated.\n", stack.Name)
			return nil
		})),
	}

	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation")
	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")

	return cmd
}
