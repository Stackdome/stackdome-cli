package main

import (
	"context"
	"encoding/json"

	"github.com/Stackdome/stackdome-cli/internal/client"
	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"github.com/spf13/cobra"
)

type logEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

func newLogsCmd() *cobra.Command {
	var (
		flagFollow bool
		flagTail   int32
		flagSince  string
		flagStack  string
	)

	cmd := &cobra.Command{
		Use:   "logs [resource]",
		Short: "Stream logs from a stack or resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			if err := output.ValidateStreamingFormat(ctx.Formatter.Format); err != nil {
				return err
			}
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			resourceName := ""
			if len(args) > 0 {
				resourceName = args[0]
			}

			opts := client.LogOptions{
				Follow: flagFollow,
				Tail:   flagTail,
				Since:  flagSince,
			}

			stream, err := ctx.Client.StreamLogs(cmd.Context(), stackID, resourceName, opts)
			if err != nil {
				if cmd.Context().Err() == context.Canceled {
					return clierrors.ErrUserCanceled
				}
				return err
			}
			defer stream.Close()

			err = client.ParseSSEStream(stream, func(e client.SSEEvent) error {
				if e.Event == "error" {
					return clierrors.New(e.Data)
				}
				if e.IsEnd() {
					return nil
				}
				return printLogEvent(ctx.Formatter, e)
			})
			if cmd.Context().Err() == context.Canceled {
				return clierrors.ErrUserCanceled
			}
			return err
		})),
	}

	cmd.Flags().BoolVarP(&flagFollow, "follow", "f", false, "Follow log output")
	cmd.Flags().Int32Var(&flagTail, "tail", 100, "Number of lines to show")
	cmd.Flags().StringVar(&flagSince, "since", "", "Show logs since duration (e.g. 5m, 1h)")
	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")

	return cmd
}

func printLogEvent(formatter *output.Formatter, event client.SSEEvent) error {
	if formatter.Format != output.FormatJSON {
		formatter.Println(event.Data)
		return nil
	}

	var data any
	if err := json.Unmarshal([]byte(event.Data), &data); err != nil {
		data = event.Data
	}
	return formatter.PrintJSONLine(logEvent{Event: event.Event, Data: data})
}
