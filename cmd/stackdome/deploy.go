package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Stackdome/stackdome-cli/internal/client"
	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"github.com/Stackdome/stackdome-cli/internal/stackfile"
	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
)

func newDeployCmd() *cobra.Command {
	var (
		flagFile    string
		flagName    string
		flagWait    bool
		flagTimeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a stack from a stackfile or JSON",
		Long: `Deploy a stack from a stackfile or JSON.

With -o json|yaml stdout carries {"stack": ..., "release": ...} — the release id
is the one to follow with ` + "`stackdome get release-events <id> -f`" + `. Deploy
first saves the stack definition, then creates a release. Use ` + "`stackdome apply`" + `
to save without releasing. With --wait the release object is the final one.`,
		Example: "  stackdome deploy -f stackfile.yaml\n  stackdome deploy -f stackfile.yaml --wait --timeout 15m\n  stackdome deploy -f stackfile.yaml -o json",
		Args:    cobra.NoArgs,
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			if ctx.Formatter.IsTable() {
				fmt.Fprintln(os.Stderr, "Applying stack definition...")
			}
			result, err := applyStackDefinition(ctx, cmd, applyOptions{File: flagFile, Name: flagName})
			if err != nil {
				return err
			}

			// Apply only stores the document; the release is what rolls it
			// out. Its id is the one --wait follows — never a pre-existing one.
			release, err := submitRelease(ctx, cmd, result.GetId())
			if err != nil {
				return clierrors.Wrap(err, "Stack was saved, but the release was not created")
			}

			if !flagWait {
				if !ctx.Formatter.IsTable() {
					return ctx.Formatter.PrintStructured(deployResult{Stack: result, Release: release})
				}
				fmt.Fprintf(os.Stderr, "\nRelease #%d for stack %q submitted. Track progress with:\n", release.GetSequence(), result.Name)
				fmt.Fprintf(os.Stderr, "  stackdome get release-events %s -f\n", release.GetId())
				fmt.Fprintf(os.Stderr, "  stackdome status --watch  # live updates\n")
				fmt.Fprintf(os.Stderr, "  stackdome logs            # stream logs\n")
				return nil
			}

			waitCtx, cancel := waitContext(cmd.Context(), flagTimeout)
			defer cancel()
			waitCmd := *cmd
			waitCmd.SetContext(waitCtx)

			final, waitErr := followRelease(ctx, &waitCmd, *result.Id, release.GetId())
			if err := waitCommandError(cmd.Context(), waitCtx, waitErr); err != nil {
				return err
			}
			stack, live, err := fetchDeployObservation(ctx, &waitCmd, *result.Id, release.GetId(), final)
			if err := waitCommandError(cmd.Context(), waitCtx, err); err != nil {
				return err
			}
			if err := waitCommandError(cmd.Context(), waitCtx, nil); err != nil {
				return err
			}
			return printFinalStack(ctx, stack, final, live)
		})),
	}

	cmd.Flags().StringVarP(&flagFile, "file", "f", "stackfile.yaml", "Path to stackfile or stack JSON")
	cmd.Flags().StringVar(&flagName, "name", "", "Override stack name")
	cmd.Flags().BoolVarP(&flagWait, "wait", "w", false, "Wait for the release to finish")
	cmd.Flags().DurationVar(&flagTimeout, "timeout", defaultWaitTimeout, "Maximum time to wait for the release")

	return cmd
}

// apiResolver turns stackfile names into API IDs.
type apiResolver struct{ c *client.Client }

func (r *apiResolver) ResolveSecretByName(ctx context.Context, name string) (string, error) {
	secret, err := r.c.FindSecretByName(ctx, name)
	if err != nil {
		return "", err
	}
	if secret == nil || secret.Id == nil {
		return "", clierrors.Newf("Secret %q not found. Run `stackdome get secrets` to see available secrets.", name)
	}
	return *secret.Id, nil
}

func (r *apiResolver) ResolveAddonByName(ctx context.Context, addonType, name string) (string, error) {
	if addonType != "postgres" {
		return "", clierrors.Newf("Unsupported addon type %q", addonType)
	}
	addon, err := r.c.FindPostgresAddonByName(ctx, name)
	if err != nil {
		return "", err
	}
	if addon == nil || addon.Id == nil {
		return "", clierrors.Newf("Postgres addon %q not found. Check available addons in the dashboard.", name)
	}
	return *addon.Id, nil
}

func loadStack(path, nameOverride string) (*openapi.Stack, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		stack, err := stackfile.LoadJSON(path)
		if err != nil {
			return nil, err
		}
		if nameOverride != "" {
			stack.Name = nameOverride
		}
		return stack, nil

	case ".yaml", ".yml":
		sf, err := stackfile.Load(path)
		if err != nil {
			return nil, err
		}
		if nameOverride != "" {
			sf.Name = nameOverride
		}
		stack, err := sf.ToStack()
		if err != nil {
			return nil, clierrors.Wrap(err, "Failed to convert stackfile")
		}
		return &stack, nil

	default:
		return nil, clierrors.ValidationError("Unsupported file format: " + ext)
	}
}

// deployResult is what `deploy -o json|yaml` prints: scripts need the release
// id to follow events, which a bare stack does not carry.
type deployResult struct {
	Stack      *openapi.Stack             `json:"stack" yaml:"stack"`
	Release    any                        `json:"release" yaml:"release"`
	LiveStatus *openapi.ReleaseLiveStatus `json:"live_status,omitempty" yaml:"live_status,omitempty"`
}

// followRelease streams a release's events to stderr and resolves its outcome,
// returning the final release (which may be non-nil alongside an error). A
// release that did not reach Released is an error, so `deploy --wait` exits
// non-zero on a failed deploy.
func followRelease(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID, releaseID string) (*openapi.StackReleaseDetail, error) {
	var afterSequence int32
	for {
		events, err := ctx.Client.StreamReleaseEvents(cmd.Context(), stackID, releaseID, afterSequence)
		if err != nil {
			return nil, err
		}
		progressed := false
		for e := range events {
			if sequence, ok := streamedReleaseEventSequence(e.Data); ok && sequence > afterSequence {
				afterSequence = sequence
				progressed = true
			}
			if ctx.Formatter.IsTable() {
				printReleaseEventLine(os.Stderr, e)
			}
		}
		if err := releaseWaitContextError(cmd.Context()); err != nil {
			return nil, err
		}

		release, err := ctx.Client.GetRelease(cmd.Context(), stackID, releaseID)
		if err != nil {
			return nil, err
		}
		if release.GetId() != releaseID {
			return release, deployVerificationError("requested release %q, but the server returned %q", releaseID, release.GetId())
		}
		if releaseStateIsTerminal(release.GetState()) {
			return release, releaseOutcomeError(release, ctx.Formatter.IsTable())
		}

		// An explicit stream end can race with a queued release beginning. Resume
		// from the last event instead of treating Pending/InProgress as failure.
		// Empty rounds are delayed so a server repeatedly returning `end` cannot
		// create a hot request loop.
		if !progressed {
			timer := time.NewTimer(time.Second)
			select {
			case <-timer.C:
			case <-cmd.Context().Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return nil, releaseWaitContextError(cmd.Context())
			}
		}
	}
}

func streamedReleaseEventSequence(data string) (int32, bool) {
	var event struct {
		Sequence *int32 `json:"sequence"`
	}
	if json.Unmarshal([]byte(data), &event) != nil || event.Sequence == nil {
		return 0, false
	}
	return *event.Sequence, true
}

func releaseWaitContextError(ctx context.Context) error {
	if ctx.Err() == context.DeadlineExceeded {
		return clierrors.New("Timed out waiting for the release to finish.").WithCode("TIMEOUT")
	}
	if ctx.Err() != nil {
		return clierrors.ErrUserCanceled
	}
	return nil
}

func releaseStateIsTerminal(state openapi.StackReleaseState) bool {
	switch state {
	case openapi.RELEASE_STATE_RELEASED,
		openapi.RELEASE_STATE_FAILED,
		openapi.RELEASE_STATE_CANCELLED,
		openapi.RELEASE_STATE_SUPERSEDED:
		return true
	default:
		return false
	}
}

func releaseOutcomeError(release *openapi.StackReleaseDetail, printSuccess bool) error {
	state := ""
	if release.State != nil {
		state = string(*release.State)
	}

	switch openapi.StackReleaseState(state) {
	case openapi.RELEASE_STATE_RELEASED:
		if printSuccess {
			fmt.Fprintln(os.Stderr, output.Green("Release succeeded."))
		}
		return nil
	case openapi.RELEASE_STATE_FAILED,
		openapi.RELEASE_STATE_CANCELLED,
		openapi.RELEASE_STATE_SUPERSEDED:
		return clierrors.Newf("Release %s: %s", strings.ToLower(state), releaseFailureDetail(release))
	default:
		return clierrors.Newf("Release did not finish (state: %s)", state)
	}
}

func releaseFailureDetail(release *openapi.StackReleaseDetail) string {
	if release.Message != nil && *release.Message != "" {
		return *release.Message
	}
	if len(release.ValidationErrors) > 0 {
		parts := make([]string, 0, len(release.ValidationErrors))
		for _, ve := range release.ValidationErrors {
			parts = append(parts, validationErrorLine(ve))
		}
		return strings.Join(parts, "; ")
	}
	if release.Cause != nil && release.Cause.Detail != nil && *release.Cause.Detail != "" {
		return *release.Cause.Detail
	}
	return "no detail reported"
}

func validationErrorLine(ve openapi.ReleaseValidationError) string {
	var b strings.Builder
	if ve.ResourceName != nil {
		b.WriteString(*ve.ResourceName + ": ")
	}
	if ve.Field != nil {
		b.WriteString(*ve.Field + " ")
	}
	if ve.Message != nil {
		b.WriteString(*ve.Message)
	}
	return strings.TrimSpace(b.String())
}

func fetchDeployObservation(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID, requestedReleaseID string, release *openapi.StackReleaseDetail) (*openapi.Stack, *openapi.ReleaseLiveStatus, error) {
	final, err := ctx.Client.GetStack(cmd.Context(), stackID)
	if err != nil {
		return nil, nil, err
	}
	live, err := ctx.Client.GetStackLiveStatus(cmd.Context(), final)
	if err != nil {
		return nil, nil, err
	}
	if err := verifyDeployObservation(requestedReleaseID, release, final, live); err != nil {
		return nil, nil, err
	}
	return final, live, nil
}

func printFinalStack(ctx *cmdutil.CommandContext, final *openapi.Stack, release *openapi.StackReleaseDetail, live *openapi.ReleaseLiveStatus) error {
	if !ctx.Formatter.IsTable() {
		return ctx.Formatter.PrintStructured(deployResult{Stack: final, Release: release, LiveStatus: live})
	}
	output.RenderStackStatus(os.Stdout, final, live, false)
	return nil
}

func verifyDeployObservation(requestedReleaseID string, release *openapi.StackReleaseDetail, stack *openapi.Stack, live *openapi.ReleaseLiveStatus) error {
	if requestedReleaseID == "" {
		return deployVerificationError("the requested release ID is empty")
	}
	if release == nil {
		return deployVerificationError("release %q has no terminal response", requestedReleaseID)
	}
	if release.GetId() != requestedReleaseID {
		return deployVerificationError("requested release %q, but the server returned %q", requestedReleaseID, release.GetId())
	}
	if release.GetState() != openapi.RELEASE_STATE_RELEASED {
		return deployVerificationError("release %q is %q, not Released", requestedReleaseID, release.GetState())
	}
	if stack == nil || stack.ConvergedRelease == nil {
		return deployVerificationError("stack has no converged release after release %q completed", requestedReleaseID)
	}
	if stack.ConvergedRelease.GetId() != requestedReleaseID {
		return deployVerificationError("release %q completed, but stack converged release is %q", requestedReleaseID, stack.ConvergedRelease.GetId())
	}
	if live == nil || live.GetHealth() != openapi.RELEASE_HEALTH_OK {
		health := openapi.ReleaseHealth("")
		if live != nil {
			health = live.GetHealth()
		}
		return deployVerificationError("release %q runtime health is %q, not ok", requestedReleaseID, health)
	}

	statuses := live.GetResources()
	for _, resource := range stack.Spec.GetStackResources() {
		for _, port := range resource.GetPorts() {
			if !port.GetExposedToPublic() {
				continue
			}
			status, ok := statuses[resource.Name]
			if !ok {
				return deployVerificationError("public resource %q has no live status", resource.Name)
			}
			found := false
			for _, ingress := range status.GetPublicIngress() {
				if ingress.GetTargetPort() == port.GetNumber() && strings.TrimSpace(ingress.GetUrl()) != "" {
					found = true
					break
				}
			}
			if !found {
				return deployVerificationError("public resource %q port %d has no public URL", resource.Name, port.GetNumber())
			}
		}
	}
	return nil
}

func deployVerificationError(format string, args ...any) error {
	return clierrors.Newf("Deployment verification failed: "+format, args...).WithCode("DEPLOY_VERIFICATION_FAILED")
}
