package main

import (
	"fmt"
	"os"
	"time"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/client"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
	"github.com/stackdome/cli/internal/output"
)

func newBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "View build history",
	}

	cmd.AddCommand(newBuildListCmd())
	cmd.AddCommand(newBuildInfoCmd())
	cmd.AddCommand(newBuildLogsCmd())
	return cmd
}

func newBuildLogsCmd() *cobra.Command {
	var (
		flagFollow bool
		flagTail   int32
		flagSince  string
		flagStack  string
	)

	cmd := &cobra.Command{
		Use:   "logs <build-id>",
		Short: "Stream logs for a build",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			buildID, err := resolveBuildID(ctx, cmd, stackID, args[0])
			if err != nil {
				return err
			}

			stream, err := ctx.Client.StreamBuildLogs(cmd.Context(), stackID, buildID, client.LogOptions{
				Follow: flagFollow,
				Tail:   flagTail,
				Since:  flagSince,
			})
			if err != nil {
				return err
			}
			defer stream.Close()

			return client.ParseSSEStream(stream, func(e client.SSEEvent) error {
				if e.Event == "error" {
					fmt.Fprintf(os.Stderr, "Error: %s\n", e.Data)
					return clierrors.New(e.Data)
				}
				if e.IsEnd() {
					return nil
				}
				fmt.Println(e.Data)
				return nil
			})
		})),
	}

	cmd.Flags().BoolVarP(&flagFollow, "follow", "f", false, "Follow log output")
	cmd.Flags().Int32Var(&flagTail, "tail", 200, "Number of lines to show")
	cmd.Flags().StringVar(&flagSince, "since", "", "Show logs since duration (e.g. 5m, 1h)")
	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
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

			buildID, err := resolveBuildID(ctx, cmd, stackID, args[0])
			if err != nil {
				return err
			}

			build, err := ctx.Client.GetBuild(cmd.Context(), stackID, buildID)
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

	if start := buildStartTime(*b); start != nil {
		fmt.Printf("Started:   %s\n", start.Local().Format(time.RFC3339))
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
	start := buildStartTime(b)
	if start == nil {
		return "-"
	}
	return output.TimeAgo(*start)
}

func buildDuration(b openapi.ImageBuild) string {
	if b.CreatedAt != nil && b.UpdatedAt != nil {
		return fmtDuration(b.UpdatedAt.Sub(*b.CreatedAt))
	}
	start, end := buildStartTime(b), buildEndTime(b)
	if start == nil {
		return "-"
	}
	// end.After(start) guards the single-condition case, where the start and
	// end fallbacks resolve to the same condition.
	if end != nil && end.After(*start) && buildIsTerminal(b) {
		return fmtDuration(end.Sub(*start))
	}
	if !buildIsTerminal(b) {
		return fmtDuration(time.Since(*start))
	}
	return "-"
}

func fmtDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	return d.Round(time.Second).String()
}

func buildIsTerminal(b openapi.ImageBuild) bool {
	if b.Status == nil || b.Status.State == nil {
		return false
	}
	return *b.Status.State == "Success" || *b.Status.State == "Failed"
}

// buildStartTime prefers the model timestamp; the API omits it today
// (hub #10), so fall back to the BuildJobCreated condition or the earliest one.
func buildStartTime(b openapi.ImageBuild) *time.Time {
	if b.CreatedAt != nil {
		return b.CreatedAt
	}
	return conditionTime(b, "BuildJobCreated", false)
}

// buildEndTime mirrors buildStartTime: model timestamp, else the Available
// condition (build completion) or the latest condition.
func buildEndTime(b openapi.ImageBuild) *time.Time {
	if b.UpdatedAt != nil {
		return b.UpdatedAt
	}
	return conditionTime(b, "Available", true)
}

// conditionTime returns the transition time of the named condition, else the
// latest (or earliest) transition time across all conditions.
func conditionTime(b openapi.ImageBuild, want string, latest bool) *time.Time {
	if b.Status == nil {
		return nil
	}
	var fallback *time.Time
	for _, c := range b.Status.Conditions {
		if c.LastTransitionTime == nil {
			continue
		}
		if c.Type != nil && *c.Type == want {
			return c.LastTransitionTime
		}
		if fallback == nil || (latest && c.LastTransitionTime.After(*fallback)) || (!latest && c.LastTransitionTime.Before(*fallback)) {
			fallback = c.LastTransitionTime
		}
	}
	return fallback
}
