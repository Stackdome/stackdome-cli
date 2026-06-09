package main

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
	"github.com/stackdome/cli/internal/output"
)

func newStatusCmd() *cobra.Command {
	var (
		flagWatch      bool
		flagConditions bool
		flagStack      string
	)

	cmd := &cobra.Command{
		Use:   "status [resource]",
		Short: "Show stack and resource status",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID := ""
			if flagStack != "" {
				s, err := ctx.Client.FindStackByName(cmd.Context(), flagStack)
				if err != nil {
					return err
				}
				if s == nil {
					return clierrors.NotFoundError("Stack", flagStack)
				}
				stackID = *s.Id
			} else {
				var err error
				stackID, err = ctx.Config.RequireStack()
				if err != nil {
					return err
				}
			}

			if flagWatch {
				return watchStatus(ctx, cmd, stackID, flagConditions)
			}

			stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(stack)
			}

			output.RenderStackStatus(os.Stdout, stack, flagConditions)
			return nil
		})),
	}

	cmd.Flags().BoolVarP(&flagWatch, "watch", "w", false, "Live refresh")
	cmd.Flags().BoolVar(&flagConditions, "conditions", false, "Show full condition history")
	cmd.Flags().StringVar(&flagStack, "stack", "", "Stack name (overrides current context)")

	return cmd
}

func watchStatus(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID string, showConditions bool) error {
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()

	for {
		stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
		if err != nil {
			return err
		}

		// Clear screen
		os.Stdout.WriteString("\033[2J\033[H")
		output.RenderStackStatus(os.Stdout, stack, showConditions)

		select {
		case <-cmd.Context().Done():
			return nil
		case <-tick.C:
		}
	}
}
