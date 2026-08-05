package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
	"github.com/stackdome/cli/internal/output"
)

func newStackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage stacks",
	}

	cmd.AddCommand(newStackListCmd())
	cmd.AddCommand(newStackInfoCmd())
	cmd.AddCommand(newStackDeleteCmd())
	return cmd
}

func newStackListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all stacks",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stacks, err := ctx.Client.ListStacks(cmd.Context())
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(stacks)
			}

			if len(stacks) == 0 {
				fmt.Fprintln(os.Stderr, "No stacks found.")
				return nil
			}

			tbl := ctx.Formatter.NewTable("", "NAME", "ID", "STATE")

			for _, s := range stacks {
				marker := " "
				if s.Id != nil && *s.Id == ctx.Config.CurrentStack {
					marker = "*"
				}
				state := "Unknown"
				if rel := output.StackRelease(&s); rel != nil && rel.State != nil {
					state = string(*rel.State)
				}
				id := ""
				if s.Id != nil {
					id = *s.Id
				}
				tbl.AddRow(marker, s.Name, id, output.StateColor(state))
			}
			tbl.Render()
			return nil
		})),
	}
}

func newStackInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show detailed stack info",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stack, err := ctx.Client.FindStackByName(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if stack == nil {
				return clierrors.NotFoundError("Stack", args[0])
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(stack)
			}

			live, err := ctx.Client.GetStackLiveStatus(cmd.Context(), stack)
			if err != nil {
				return err
			}

			output.RenderStackStatus(os.Stdout, stack, live, true)
			return nil
		})),
	}
}

func newStackDeleteCmd() *cobra.Command {
	var flagYes bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a stack by name",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stack, err := ctx.Client.FindStackByName(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if stack == nil {
				return clierrors.NotFoundError("Stack", args[0])
			}

			if !flagYes {
				fmt.Fprintf(os.Stderr, "Delete stack %q? [y/N]: ", stack.Name)
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "y" && confirm != "Y" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			if err := ctx.Client.DeleteStack(cmd.Context(), *stack.Id); err != nil {
				return err
			}

			if ctx.Config.CurrentStack == *stack.Id {
				ctx.Config.CurrentStack = ""
				_ = ctx.Config.Save()
			}

			fmt.Fprintf(os.Stderr, "Stack %q deletion initiated.\n", stack.Name)
			return nil
		})),
	}

	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation")
	return cmd
}
