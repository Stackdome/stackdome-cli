package main

import (
	"fmt"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
	var flagStack string

	cmd := &cobra.Command{
		Use:   "restart <resource>",
		Short: "Restart a stack resource",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			resourceName := args[0]

			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			_, err = ctx.Client.RestartResource(cmd.Context(), stackID, resourceName)
			if err != nil {
				return err
			}

			return printMutationResult(ctx, mutationResult{
				Status:   "restart_initiated",
				Resource: "stack_resource",
				Name:     resourceName,
			}, fmt.Sprintf("Restart initiated for resource %q", resourceName))
		})),
	}

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")

	return cmd
}
