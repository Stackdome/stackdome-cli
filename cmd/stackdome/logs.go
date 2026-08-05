package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/client"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
)

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
				return err
			}
			defer stream.Close()

			return client.ParseSSEStream(stream, func(e client.SSEEvent) error {
				if e.Event == "error" {
					fmt.Fprintf(os.Stderr, "Error: %s\n", e.Data)
					return clierrors.New(e.Data)
				}
				fmt.Println(e.Data)
				return nil
			})
		})),
	}

	cmd.Flags().BoolVarP(&flagFollow, "follow", "f", false, "Follow log output")
	cmd.Flags().Int32Var(&flagTail, "tail", 100, "Number of lines to show")
	cmd.Flags().StringVar(&flagSince, "since", "", "Show logs since duration (e.g. 5m, 1h)")
	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")

	return cmd
}

