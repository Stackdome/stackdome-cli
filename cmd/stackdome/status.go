package main

import (
	"os"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"github.com/spf13/cobra"
)

type statusResult struct {
	Stack      any `json:"stack" yaml:"stack"`
	LiveStatus any `json:"live_status" yaml:"live_status"`
}

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
			if flagWatch {
				if err := output.ValidateStreamingFormat(ctx.Formatter.Format); err != nil {
					return err
				}
			}
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			if flagWatch {
				return watchStatus(ctx, cmd, stackID, flagConditions)
			}

			stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
			if err != nil {
				return err
			}
			live, err := ctx.Client.GetStackLiveStatus(cmd.Context(), stack)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(statusResult{Stack: stack, LiveStatus: live})
			}

			output.RenderStackStatus(os.Stdout, stack, live, flagConditions)
			return nil
		})),
	}

	cmd.Flags().BoolVarP(&flagWatch, "watch", "w", false, "Live refresh")
	cmd.Flags().BoolVar(&flagConditions, "conditions", false, "Show full condition history")
	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")

	return cmd
}

func watchStatus(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID string, showConditions bool) error {
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()

	for {
		if cmd.Context().Err() != nil {
			return clierrors.ErrUserCanceled
		}
		stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
		if err != nil {
			if cmd.Context().Err() != nil {
				return clierrors.ErrUserCanceled
			}
			return err
		}
		live, err := ctx.Client.GetStackLiveStatus(cmd.Context(), stack)
		if err != nil {
			if cmd.Context().Err() != nil {
				return clierrors.ErrUserCanceled
			}
			return err
		}

		// Structured mode emits one object per tick — no redraw, no escape
		// codes, so `status -w -o json` stays parseable as it streams.
		if ctx.Formatter.Format == output.FormatJSON {
			if err := ctx.Formatter.PrintJSONLine(statusResult{Stack: stack, LiveStatus: live}); err != nil {
				return err
			}
		} else if !ctx.Formatter.IsTable() {
			if err := ctx.Formatter.PrintStructured(statusResult{Stack: stack, LiveStatus: live}); err != nil {
				return err
			}
		} else {
			// Clear screen — only meaningful on a terminal; escape codes would
			// otherwise corrupt piped/redirected output.
			if output.IsTTY() {
				os.Stdout.WriteString("\033[2J\033[H")
			}
			output.RenderStackStatus(os.Stdout, stack, live, showConditions)
		}

		select {
		case <-cmd.Context().Done():
			return clierrors.ErrUserCanceled
		case <-tick.C:
		}
	}
}
