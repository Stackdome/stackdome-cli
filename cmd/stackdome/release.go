package main

import (
	"encoding/json"
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

func newReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Inspect and cancel stack releases",
	}

	cmd.AddCommand(newReleaseListCmd())
	cmd.AddCommand(newReleaseInfoCmd())
	cmd.AddCommand(newReleaseCancelCmd())
	cmd.AddCommand(newReleaseRollbackCmd())
	cmd.AddCommand(newReleaseEventsCmd())
	return cmd
}

func newReleaseListCmd() *cobra.Command {
	var flagStack string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List releases for the current stack",
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			releases, err := ctx.Client.ListReleases(cmd.Context(), stackID)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(releases)
			}
			if len(releases) == 0 {
				fmt.Fprintln(os.Stderr, "No releases found.")
				return nil
			}

			tbl := ctx.Formatter.NewTable("ID", "SEQ", "STATE", "CAUSE", "CREATED", "DURATION")
			for _, r := range releases {
				tbl.AddRow(
					shortID(r.GetId()),
					fmt.Sprintf("%d", r.GetSequence()),
					releaseStateColor(r.State),
					releaseCause(r.Cause),
					releaseAge(r.CreatedAt),
					releaseDuration(r.CreatedAt, r.CompletedAt),
				)
			}
			tbl.Render()
			return nil
		})),
	}

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
	return cmd
}

func newReleaseInfoCmd() *cobra.Command {
	var flagStack string

	cmd := &cobra.Command{
		Use:   "info <release-id>",
		Short: "Show release details",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			releaseID, err := resolveReleaseID(ctx, cmd, stackID, args[0])
			if err != nil {
				return err
			}

			release, err := ctx.Client.GetRelease(cmd.Context(), stackID, releaseID)
			if err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(release)
			}
			renderReleaseInfo(release)
			return nil
		})),
	}

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
	return cmd
}

func newReleaseCancelCmd() *cobra.Command {
	var flagStack string

	cmd := &cobra.Command{
		Use:   "cancel <release-id>",
		Short: "Cancel a pending release",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			releaseID, err := resolveReleaseID(ctx, cmd, stackID, args[0])
			if err != nil {
				return err
			}

			if err := ctx.Client.CancelRelease(cmd.Context(), stackID, releaseID); err != nil {
				return err
			}
			return printMutationResult(ctx, mutationResult{
				Status:   "cancelled",
				Resource: "release",
				ID:       releaseID,
			}, fmt.Sprintf("Release %s cancelled.", releaseID))
		})),
	}

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
	return cmd
}

// rollbackResult keeps the created release available to automation and adds
// live status only when --wait has observed its terminal state.
type rollbackResult struct {
	Release    any                        `json:"release" yaml:"release"`
	LiveStatus *openapi.ReleaseLiveStatus `json:"live_status,omitempty" yaml:"live_status,omitempty"`
}

func newReleaseRollbackCmd() *cobra.Command {
	var (
		flagStack   string
		flagWait    bool
		flagTimeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "rollback <release-id>",
		Short: "Create a release from a historical release",
		Long:  "Create a new release from a historical release. --wait follows it for up to 10 minutes by default.",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}
			fromReleaseID, err := resolveReleaseID(ctx, cmd, stackID, args[0])
			if err != nil {
				return err
			}

			release, err := ctx.Client.RollbackRelease(cmd.Context(), stackID, fromReleaseID)
			if err != nil {
				return err
			}
			if !flagWait {
				if !ctx.Formatter.IsTable() {
					return ctx.Formatter.PrintStructured(rollbackResult{Release: release})
				}
				fmt.Fprintf(os.Stderr, "Rollback release #%d submitted. Track progress with:\n", release.GetSequence())
				fmt.Fprintf(os.Stderr, "  stackdome release events %s -f\n", release.GetId())
				return nil
			}

			waitCtx, cancel := waitContext(cmd.Context(), flagTimeout)
			defer cancel()
			waitCmd := *cmd
			waitCmd.SetContext(waitCtx)
			final, waitErr := followRelease(ctx, &waitCmd, stackID, release.GetId())
			if err := waitCommandError(cmd.Context(), waitCtx, waitErr); err != nil {
				return err
			}
			stack, live, err := fetchDeployObservation(ctx, &waitCmd, stackID, release.GetId(), final)
			if err := waitCommandError(cmd.Context(), waitCtx, err); err != nil {
				return err
			}
			if err := waitCommandError(cmd.Context(), waitCtx, nil); err != nil {
				return err
			}

			if !ctx.Formatter.IsTable() {
				if err := ctx.Formatter.PrintStructured(rollbackResult{Release: final, LiveStatus: live}); err != nil {
					return err
				}
			} else {
				output.RenderStackStatus(os.Stdout, stack, live, false)
			}
			return nil
		})),
	}

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
	cmd.Flags().BoolVarP(&flagWait, "wait", "w", false, "Wait for the rollback release to finish")
	cmd.Flags().DurationVar(&flagTimeout, "timeout", defaultWaitTimeout, "Maximum time to wait for the rollback release")
	return cmd
}

func newReleaseEventsCmd() *cobra.Command {
	var (
		flagStack  string
		flagFollow bool
	)

	cmd := &cobra.Command{
		Use:   "events <release-id>",
		Short: "Show release events",
		Args:  cobra.ExactArgs(1),
		PreRunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, _ *cobra.Command, _ []string) error {
			if flagFollow && ctx.Formatter.Format == output.FormatYAML {
				return clierrors.ValidationError("release events --follow does not support YAML output; use table or JSON")
			}
			return nil
		}),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			releaseID, err := resolveReleaseID(ctx, cmd, stackID, args[0])
			if err != nil {
				return err
			}

			if flagFollow {
				events, err := ctx.Client.StreamReleaseEvents(cmd.Context(), stackID, releaseID, 0)
				if err != nil {
					if cmd.Context().Err() != nil {
						return clierrors.ErrUserCanceled
					}
					return err
				}
				var streamErr error
				for e := range events {
					if e.Event == "error" {
						// Surfaced as the command error; printing it here too
						// would duplicate it (and pollute -o json stdout).
						streamErr = clierrors.New(e.Data)
						continue
					}
					// Structured mode streams newline-delimited JSON envelopes;
					// the human formatter would put prose and ANSI colour on
					// stdout.
					if ctx.Formatter.IsTable() {
						printReleaseEventLine(os.Stdout, e)
					} else {
						fmt.Fprintf(os.Stdout, "{\"event\":%q,\"data\":%s}\n", e.Event, eventDataJSON(e.Data))
					}
				}
				if cmd.Context().Err() != nil {
					return clierrors.ErrUserCanceled
				}
				return streamErr
			}

			events, err := ctx.Client.ListReleaseEvents(cmd.Context(), stackID, releaseID, 0)
			if err != nil {
				return err
			}
			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(events)
			}
			for _, ev := range events {
				fmt.Fprintln(os.Stdout, formatReleaseEvent(ev))
			}
			return nil
		})),
	}

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")
	cmd.Flags().BoolVarP(&flagFollow, "follow", "f", false, "Follow the event stream")
	return cmd
}

func renderReleaseInfo(r *openapi.StackReleaseDetail) {
	fmt.Printf("Release:   %s\n", r.GetId())
	fmt.Printf("Sequence:  #%d\n", r.GetSequence())
	fmt.Printf("State:     %s\n", releaseStateColor(r.State))
	fmt.Printf("Cause:     %s\n", releaseCause(r.Cause))
	if r.Message != nil && *r.Message != "" {
		fmt.Printf("Message:   %s\n", *r.Message)
	}
	if r.CreatedAt != nil {
		fmt.Printf("Created:   %s\n", r.CreatedAt.Local().Format(time.RFC3339))
	}
	fmt.Printf("Duration:  %s\n", releaseDuration(r.CreatedAt, r.CompletedAt))

	if len(r.ValidationErrors) > 0 {
		fmt.Println()
		fmt.Println("Validation errors:")
		for _, ve := range r.ValidationErrors {
			fmt.Printf("  - %s\n", validationErrorLine(ve))
		}
	}
}

// printReleaseEventLine renders one streamed SSE frame. The payload is a
// ReleaseEvent JSON document; anything unparseable is printed raw so nothing is
// silently swallowed.
// eventDataJSON embeds an SSE payload as-is when it is already JSON, and as a
// JSON string when it is not, so every streamed line stays valid JSON.
func eventDataJSON(data string) string {
	if json.Valid([]byte(data)) {
		return data
	}
	b, err := json.Marshal(data)
	if err != nil {
		return `""`
	}
	return string(b)
}

func printReleaseEventLine(w io.Writer, e client.SSEEvent) {
	if e.Event == "error" {
		fmt.Fprintf(w, "%s %s\n", output.Red("error"), e.Data)
		return
	}
	var ev openapi.ReleaseEvent
	if json.Unmarshal([]byte(e.Data), &ev) != nil {
		fmt.Fprintln(w, e.Data)
		return
	}
	fmt.Fprintln(w, formatReleaseEvent(ev))
}

func formatReleaseEvent(ev openapi.ReleaseEvent) string {
	var b strings.Builder
	if ev.OccurredAt != nil {
		b.WriteString(output.Dim(ev.OccurredAt.Local().Format(time.TimeOnly)) + "  ")
	}
	if ev.ResourceName != nil && *ev.ResourceName != "" {
		b.WriteString(output.Cyan(*ev.ResourceName) + "  ")
	}
	msg := ev.GetMessage()
	if msg == "" {
		msg = ev.GetType()
	}
	if ev.Level != nil && strings.EqualFold(*ev.Level, "error") {
		msg = output.Red(msg)
	}
	b.WriteString(msg)
	return b.String()
}

func releaseStateColor(state *openapi.StackReleaseState) string {
	if state == nil {
		return "Unknown"
	}
	s := string(*state)
	switch *state {
	case openapi.RELEASE_STATE_RELEASED:
		return output.Green(s)
	case openapi.RELEASE_STATE_FAILED, openapi.RELEASE_STATE_CANCELLED:
		return output.Red(s)
	case openapi.RELEASE_STATE_PENDING, openapi.RELEASE_STATE_IN_PROGRESS:
		return output.Yellow(s)
	default:
		return s
	}
}

func releaseCause(cause *openapi.ReleaseCause) string {
	if cause == nil || cause.Kind == nil {
		return "-"
	}
	return string(*cause.Kind)
}

func releaseAge(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return output.TimeAgo(*t)
}

func releaseDuration(start, end *time.Time) string {
	if start == nil || end == nil {
		return "-"
	}
	d := end.Sub(*start)
	if d < time.Second {
		return "<1s"
	}
	return d.Round(time.Second).String()
}
