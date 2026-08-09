package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
)

type stackListItem struct {
	openapi.Stack
	Current bool `json:"current"`
}

func (item stackListItem) MarshalJSON() ([]byte, error) {
	stackJSON, err := json.Marshal(item.Stack)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(stackJSON, &fields); err != nil {
		return nil, err
	}
	currentJSON, err := json.Marshal(item.Current)
	if err != nil {
		return nil, err
	}
	fields["current"] = currentJSON
	return json.Marshal(fields)
}

func currentStackMatches(stack openapi.Stack, current string) bool {
	return current != "" && (stack.GetId() == current || stack.Name == current)
}

func decorateStackList(stacks []openapi.Stack, current string) []stackListItem {
	items := make([]stackListItem, len(stacks))
	for i := range stacks {
		items[i] = stackListItem{Stack: stacks[i], Current: currentStackMatches(stacks[i], current)}
	}
	return items
}

func newStackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage stacks",
	}

	cmd.AddCommand(newStackListCmd())
	cmd.AddCommand(newStackUseCmd())
	cmd.AddCommand(newStackInfoCmd())
	cmd.AddCommand(newStackDeleteCmd())
	return cmd
}

func newStackUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "use <stack>",
		Aliases: []string{"select"},
		Short:   "Select the current stack by name or ID",
		Args:    cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			return selectStackContext(ctx, cmd, args[0], "use stack")
		}),
	}
}

func selectStackContext(ctx *cmdutil.CommandContext, cmd *cobra.Command, ref, commandName string) error {
	if err := ctx.Config.RequireAuth(); err != nil {
		return err
	}
	if ctx.Config.StackContextFromEnv() {
		return clierrors.ValidationError(commandName + " cannot persist a selection while STACKDOME_URL, STACKDOME_TOKEN, STACKDOME_ORG, or STACKDOME_PROJECT controls the active context; unset the override or pass --stack to the command instead")
	}
	if err := cmdutil.ResolveScope(ctx, cmd); err != nil {
		return err
	}
	id, err := resolveStackRef(ctx, cmd, ref)
	if err != nil {
		return err
	}
	if err := ctx.Config.SetCurrentStack(id); err != nil {
		return err
	}
	return printMutationResult(ctx, mutationResult{
		Status:   "selected",
		Resource: "stack",
		Name:     ref,
		ID:       id,
	}, fmt.Sprintf("Current stack set to %s (%s)", ref, id))
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
				return ctx.Formatter.PrintStructured(decorateStackList(stacks, ctx.Config.CurrentStack))
			}

			if len(stacks) == 0 {
				fmt.Fprintln(os.Stderr, "No stacks found.")
				return nil
			}

			tbl := ctx.Formatter.NewTable("CURRENT", "NAME", "ID", "STATE")

			for _, s := range stacks {
				marker := " "
				if currentStackMatches(s, ctx.Config.CurrentStack) {
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
		Use:   "info <stack>",
		Short: "Show detailed stack info by name or ID",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackRef(ctx, cmd, args[0])
			if err != nil {
				return err
			}
			stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
			if err != nil {
				return err
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
		Use:   "delete <stack>",
		Short: "Delete a stack by name or ID",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackRef(ctx, cmd, args[0])
			if err != nil {
				return err
			}
			stack, err := ctx.Client.GetStack(cmd.Context(), stackID)
			if err != nil {
				return err
			}

			if _, err := cmdutil.Confirm(ctx.Formatter, fmt.Sprintf("Delete stack %q?", stack.Name), flagYes); err != nil {
				return err
			}

			if err := ctx.Client.DeleteStack(cmd.Context(), stackID); err != nil {
				return err
			}

			if ctx.Config.CurrentStack == stackID {
				_ = ctx.Config.SetCurrentStack("")
			}

			return printMutationResult(ctx, mutationResult{
				Status:   "deletion_initiated",
				Resource: "stack",
				Name:     stack.Name,
				ID:       stackID,
			}, fmt.Sprintf("Stack %q deletion initiated.", stack.Name))
		})),
	}

	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation")
	return cmd
}
