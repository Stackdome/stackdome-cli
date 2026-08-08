package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/client"
	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
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
			if err := output.ValidateStreamingFormat(ctx.Formatter.Format); err != nil {
				return err
			}
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			build, err := resolveBuild(ctx, cmd, stackID, args[0])
			if err != nil {
				return err
			}
			buildID := build.GetId()

			stream, err := ctx.Client.StreamBuildLogs(cmd.Context(), stackID, buildID, client.LogOptions{
				Follow: flagFollow,
				Tail:   flagTail,
				Since:  flagSince,
			})
			if err != nil {
				if cmd.Context().Err() == context.Canceled {
					return clierrors.ErrUserCanceled
				}
				return friendlyBuildLogsError(args[0], build, err)
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
	cmd.Flags().Int32Var(&flagTail, "tail", 200, "Number of lines to show")
	cmd.Flags().StringVar(&flagSince, "since", "", "Show logs since duration (e.g. 5m, 1h)")
	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
	return cmd
}

func friendlyBuildLogsError(buildRef string, build openapi.ImageBuild, err error) error {
	var cliErr *clierrors.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != "NOT_FOUND" {
		return err
	}

	reason := strings.ToLower(cliErr.Detail)
	if !strings.Contains(reason, "logs have been pruned") &&
		!strings.Contains(reason, "no longer exists in the cluster") {
		return err
	}

	message := fmt.Sprintf("Logs for build %q are not available yet; the build has not started.", buildRef)
	if buildIsTerminal(build) {
		message = fmt.Sprintf("Logs for build %q are no longer available; they were pruned after the build completed.", buildRef)
	}

	return &clierrors.CLIError{
		Message:  message,
		Code:     "NOT_FOUND",
		ExitCode: clierrors.ExitNotFound,
		Cause:    err,
	}
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

			tbl := ctx.Formatter.NewTable("BUILD", "RESOURCE", "STATE", "SOURCE", "STARTED", "DURATION")
			for _, b := range builds {
				tbl.AddRow(
					buildReference(b),
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

			renderBuildInfo(ctx.Formatter.Writer, build)
			return nil
		})),
	}

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
	return cmd
}

func renderBuildInfo(w io.Writer, b *openapi.ImageBuild) {
	fmt.Fprintf(w, "Build:     %s\n", buildReference(*b))
	fmt.Fprintf(w, "ID:        %s\n", b.GetId())
	fmt.Fprintf(w, "Resource:  %s\n", b.StackResourceName)
	fmt.Fprintf(w, "State:     %s\n", buildStateColor(*b))
	fmt.Fprintf(w, "Source:    %s\n", buildSource(*b))

	if b.Status != nil && b.Status.ImageUrl != nil && *b.Status.ImageUrl != "" {
		fmt.Fprintf(w, "Image:     %s\n", *b.Status.ImageUrl)
	}

	if start := buildStartTime(*b); start != nil {
		fmt.Fprintf(w, "Started:   %s\n", start.Local().Format(time.RFC3339))
	}

	fmt.Fprintf(w, "Duration:  %s\n", buildDuration(*b))

	if b.Status != nil && b.Status.LastBuildFailureDetail != nil {
		f := b.Status.LastBuildFailureDetail
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Failure:")
		if f.FailureType != nil {
			fmt.Fprintf(w, "  Type:    %s\n", *f.FailureType)
		}
		if f.Reason != nil {
			fmt.Fprintf(w, "  Reason:  %s\n", *f.Reason)
		}
		if f.Message != nil {
			fmt.Fprintf(w, "  Message: %s\n", *f.Message)
		}
		if f.ExitCode != nil {
			fmt.Fprintf(w, "  Exit:    %d\n", *f.ExitCode)
		}
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// buildReference is the short, copyable identifier shown by build list.
// Build IDs begin with the resource name, so the generic first-eight-character
// shortID hides every distinguishing character for builds of one resource.
// The source revision identifies what was built; the ID suffix distinguishes
// repeated builds of the same revision.
func buildReference(b openapi.ImageBuild) string {
	idSuffix := b.GetId()
	if len(idSuffix) > 8 {
		idSuffix = idSuffix[len(idSuffix)-8:]
	}

	if git := b.SourceRevision.GitRepoRevision; git != nil && git.Commit != nil && *git.Commit != "" {
		commit := *git.Commit
		if len(commit) > 7 {
			commit = commit[:7]
		}
		return commit + "-" + idSuffix
	}
	if b.SourceRevision.VolumeSourceRevision != nil {
		return "volume-" + idSuffix
	}
	return idSuffix
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
