package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/client"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
	"github.com/stackdome/cli/internal/output"
	"github.com/stackdome/cli/internal/stackfile"
)

func newDeployCmd() *cobra.Command {
	var (
		flagFile string
		flagName string
		flagWait bool
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a stack from a stackfile or JSON",
		Long: `Deploy a stack from a stackfile or JSON.

With -o json|yaml stdout carries {"stack": ..., "release": ...} — the release id
is the one to follow with ` + "`stackdome release events <id> -f`" + `. With --wait
the release object is the final one.`,
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stack, err := loadStack(flagFile, flagName)
			if err != nil {
				return err
			}

			// Names in `secrets:`/`addons:` become IDs before the document is
			// sent — the server only speaks IDs.
			if err := stackfile.ResolveStack(cmd.Context(), stack, &apiResolver{c: ctx.Client}); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Applying stack %q...\n", stack.Name)
			result, err := ctx.Client.ApplyStack(cmd.Context(), *stack)
			if err != nil {
				return err
			}

			if err := ctx.Config.SetCurrentStack(*result.Id); err != nil {
				return err
			}

			// Apply only stores the document; the release is what rolls it
			// out. Its id is the one --wait follows — never a pre-existing one.
			release, err := ctx.Client.CreateRelease(cmd.Context(), *result.Id)
			if err != nil {
				return err
			}

			if !flagWait {
				if !ctx.Formatter.IsTable() {
					return ctx.Formatter.PrintStructured(deployResult{Stack: result, Release: release})
				}
				fmt.Fprintf(os.Stderr, "\nRelease #%d for stack %q submitted. Track progress with:\n", release.GetSequence(), result.Name)
				fmt.Fprintf(os.Stderr, "  stackdome release events %s -f\n", release.GetId())
				fmt.Fprintf(os.Stderr, "  stackdome status --watch  # live updates\n")
				fmt.Fprintf(os.Stderr, "  stackdome logs            # stream logs\n")
				return nil
			}

			final, waitErr := followRelease(ctx, cmd, *result.Id, release.GetId())

			if err := printFinalStack(ctx, cmd, *result.Id, final); err != nil && waitErr == nil {
				return err
			}
			return waitErr
		})),
	}

	cmd.Flags().StringVarP(&flagFile, "file", "f", "stackfile.yaml", "Path to stackfile or stack JSON")
	cmd.Flags().StringVar(&flagName, "name", "", "Override stack name")
	cmd.Flags().BoolVarP(&flagWait, "wait", "w", false, "Wait for the release to finish")

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
		return "", clierrors.Newf("Secret %q not found. Run `stackdome secret list` to see available secrets.", name)
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
		if err := stackfile.ResolveEnvFiles(sf, filepath.Dir(path)); err != nil {
			return nil, err
		}
		if nameOverride != "" {
			sf.Name = nameOverride
		}
		stack := sf.ToStack()
		return &stack, nil

	default:
		return nil, clierrors.ValidationError("Unsupported file format: " + ext)
	}
}

// deployResult is what `deploy -o json|yaml` prints: scripts need the release
// id to follow events, which a bare stack does not carry.
type deployResult struct {
	Stack   *openapi.Stack `json:"stack" yaml:"stack"`
	Release any            `json:"release" yaml:"release"`
}

// followRelease streams a release's events to stderr and resolves its outcome,
// returning the final release (which may be non-nil alongside an error). A
// release that did not reach Released is an error, so `deploy --wait` exits
// non-zero on a failed deploy.
func followRelease(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID, releaseID string) (*openapi.StackReleaseDetail, error) {
	events, err := ctx.Client.StreamReleaseEvents(cmd.Context(), stackID, releaseID, 0)
	if err != nil {
		return nil, err
	}
	for e := range events {
		printReleaseEventLine(os.Stderr, e)
	}
	if cmd.Context().Err() != nil {
		return nil, clierrors.ErrUserCanceled
	}

	release, err := ctx.Client.GetRelease(cmd.Context(), stackID, releaseID)
	if err != nil {
		return nil, err
	}
	return release, releaseOutcomeError(release)
}

func releaseOutcomeError(release *openapi.StackReleaseDetail) error {
	state := ""
	if release.State != nil {
		state = string(*release.State)
	}

	switch openapi.StackReleaseState(state) {
	case openapi.RELEASE_STATE_RELEASED:
		fmt.Fprintln(os.Stderr, output.Green("Release succeeded."))
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

func printFinalStack(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID string, release *openapi.StackReleaseDetail) error {
	final, err := ctx.Client.GetStack(cmd.Context(), stackID)
	if err != nil {
		return err
	}
	if !ctx.Formatter.IsTable() {
		return ctx.Formatter.PrintStructured(deployResult{Stack: final, Release: release})
	}
	live, err := ctx.Client.GetStackLiveStatus(cmd.Context(), final)
	if err != nil {
		return err
	}
	output.RenderStackStatus(os.Stdout, final, live, false)
	return nil
}
