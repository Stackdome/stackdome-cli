package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
			stackID := flagStack
			if stackID == "" {
				var err error
				stackID, err = ctx.Config.RequireStack()
				if err != nil {
					return err
				}
			}

			stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
			if err != nil {
				return err
			}

			if !flagYes {
				fmt.Fprintf(os.Stderr, "This will permanently delete stack %q. Type the stack name to confirm: ", stack.Name)
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				if strings.TrimSpace(scanner.Text()) != stack.Name {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			if err := ctx.Client.DeleteStack(cmd.Context(), stackID); err != nil {
				return err
			}

			if ctx.Config.CurrentStack == stackID {
				ctx.Config.CurrentStack = ""
				_ = ctx.Config.Save()
			}

			fmt.Fprintf(os.Stderr, "Stack %q deletion initiated.\n", stack.Name)
			return nil
		})),
	}

	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation")
	cmd.Flags().StringVar(&flagStack, "stack", "", "Stack ID (overrides current context)")

	return cmd
}
