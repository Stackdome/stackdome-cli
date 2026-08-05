package main

import (
	"fmt"
	"os"
	"time"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
	"github.com/stackdome/cli/internal/output"
)

func newBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "View build history",
	}

	cmd.AddCommand(newBuildListCmd())
	cmd.AddCommand(newBuildInfoCmd())
	return cmd
}

func newBuildListCmd() *cobra.Command {
	var (
		flagResource string
		flagStack    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List builds for the current stack",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			var builds []openapi.ImageBuild
			if flagResource != "" {
				builds, err = ctx.Client.ListResourceBuilds(cmd.Context(), stackID, flagResource)
			} else {
				builds, err = ctx.Client.ListBuilds(cmd.Context(), stackID)
			}
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(builds)
			}

			if len(builds) == 0 {
				fmt.Fprintln(os.Stderr, "No builds found.")
				return nil
			}

			tbl := ctx.Formatter.NewTable("ID", "RESOURCE", "STATE", "SOURCE", "STARTED", "DURATION")
			for _, b := range builds {
				tbl.AddRow(
					shortID(b.GetId()),
					b.StackResourceName,
					buildStateColor(b),
					buildSource(b),
					buildStarted(b),
					buildDuration(b),
				)
			}
			tbl.Render()
			return nil
		})),
	}

	cmd.Flags().StringVarP(&flagResource, "resource", "r", "", "Filter to specific resource")
	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")

	return cmd
}

func newBuildInfoCmd() *cobra.Command {
	var flagStack string

	cmd := &cobra.Command{
		Use:   "info <build-id>",
		Short: "Show build details",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			build, err := ctx.Client.GetBuild(cmd.Context(), stackID, args[0])
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(build)
			}

			renderBuildInfo(build)
			return nil
		})),
	}

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
	return cmd
}

func renderBuildInfo(b *openapi.ImageBuild) {
	fmt.Printf("Build:     %s\n", b.GetId())
	fmt.Printf("Resource:  %s\n", b.StackResourceName)
	fmt.Printf("State:     %s\n", buildStateColor(*b))
	fmt.Printf("Source:    %s\n", buildSource(*b))

	if b.Status != nil && b.Status.ImageUrl != nil && *b.Status.ImageUrl != "" {
		fmt.Printf("Image:     %s\n", *b.Status.ImageUrl)
	}

	if b.CreatedAt != nil {
		fmt.Printf("Started:   %s\n", b.CreatedAt.Format(time.DateTime))
	}

	fmt.Printf("Duration:  %s\n", buildDuration(*b))

	if b.Status != nil && b.Status.LastBuildFailureDetail != nil {
		f := b.Status.LastBuildFailureDetail
		fmt.Println()
		fmt.Println("Failure:")
		if f.FailureType != nil {
			fmt.Printf("  Type:    %s\n", *f.FailureType)
		}
		if f.Reason != nil {
			fmt.Printf("  Reason:  %s\n", *f.Reason)
		}
		if f.Message != nil {
			fmt.Printf("  Message: %s\n", *f.Message)
		}
		if f.ExitCode != nil {
			fmt.Printf("  Exit:    %d\n", *f.ExitCode)
		}
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func buildStateColor(b openapi.ImageBuild) string {
	if b.Status == nil || b.Status.State == nil {
		return "Unknown"
	}
	state := *b.Status.State
	switch state {
	case "Success":
		return output.Green(state)
	case "Failed":
		return output.Red(state)
	case "Pending", "Building":
		return output.Yellow(state)
	default:
		return state
	}
}

func buildSource(b openapi.ImageBuild) string {
	rev := b.SourceRevision
	if rev.GitRepoRevision != nil {
		git := rev.GitRepoRevision
		if git.Branch != nil {
			name := *git.Branch
			if git.Commit != nil && len(*git.Commit) >= 7 {
				return name + "@" + (*git.Commit)[:7]
			}
			return name
		}
		if git.Tag != nil {
			return "tag:" + *git.Tag
		}
		if git.Commit != nil && len(*git.Commit) >= 7 {
			return (*git.Commit)[:7]
		}
	}
	if rev.VolumeSourceRevision != nil {
		return "volume"
	}
	return "-"
}

func buildStarted(b openapi.ImageBuild) string {
	if b.CreatedAt == nil {
		return "-"
	}
	return output.TimeAgo(*b.CreatedAt)
}

func buildDuration(b openapi.ImageBuild) string {
	if b.CreatedAt == nil || b.UpdatedAt == nil {
		return "-"
	}
	d := b.UpdatedAt.Sub(*b.CreatedAt)
	if d < time.Second {
		return "<1s"
	}
	return d.Round(time.Second).String()
}
