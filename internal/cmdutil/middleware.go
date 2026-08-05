package cmdutil

import (
	"github.com/spf13/cobra"
	clierrors "github.com/stackdome/cli/internal/errors"
)

type contextKey struct{}

func SetContext(cmd *cobra.Command, ctx *CommandContext) {
	cmd.SetContext(withValue(cmd.Context(), contextKey{}, ctx))
}

func GetContext(cmd *cobra.Command) *CommandContext {
	return cmd.Context().Value(contextKey{}).(*CommandContext)
}

type RunEWithContext func(ctx *CommandContext, cmd *cobra.Command, args []string) error

func WithContext(fn RunEWithContext) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := GetContext(cmd)
		return fn(ctx, cmd, args)
	}
}

func RequireAuth(fn RunEWithContext) RunEWithContext {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
		if err := ctx.Config.RequireAuth(); err != nil {
			return err
		}
		if err := resolveScope(ctx, cmd); err != nil {
			return err
		}
		return fn(ctx, cmd, args)
	}
}

// resolveScope fills in the org/project the client scopes its calls to. With
// STACKDOME_TOKEN and no config file there is nothing on disk to read them
// from, so resolve them from the API once, in memory only — unless
// STACKDOME_ORG and STACKDOME_PROJECT already supplied both, in which case no
// discovery call is made at all (a scoped API token may not be allowed one).
func resolveScope(ctx *CommandContext, cmd *cobra.Command) error {
	if ctx.Config.OrganizationID != "" && ctx.Config.ProjectName != "" {
		return nil
	}

	if ctx.Config.OrganizationID == "" {
		user, err := ctx.Client.GetCurrentUser(cmd.Context())
		if err != nil {
			return scopeError(ctx, err)
		}
		ctx.Config.OrganizationID = user.GetOrganisationId()
	}
	if ctx.Config.ProjectName == "" {
		name, err := ctx.Client.ResolveDefaultProject(cmd.Context(), ctx.Config.OrganizationID)
		if err != nil {
			return scopeError(ctx, err)
		}
		ctx.Config.ProjectName = name
	}

	ctx.Client.SetOrgAndProject(ctx.Config.OrganizationID, ctx.Config.ProjectName)
	return nil
}

// scopeError rewrites a discovery failure for token auth: "run stackdome login"
// is meaningless for an API token, which most likely just lacks the scope to
// read projects.
func scopeError(ctx *CommandContext, err error) error {
	if !ctx.Config.TokenFromEnv() {
		return err
	}
	return clierrors.Wrap(err,
		"Could not determine your organisation and project: the API token may lack project read scope. "+
			"Set STACKDOME_ORG and STACKDOME_PROJECT to skip discovery.").
		WithCode("SCOPE_UNRESOLVED").
		WithExitCode(clierrors.ExitAuth)
}

func RequireStack(fn func(ctx *CommandContext, cmd *cobra.Command, args []string, stackName string) error) RunEWithContext {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
		stackName, _ := cmd.Flags().GetString("stack")
		if stackName == "" {
			var err error
			stackName, err = ctx.Config.RequireStack()
			if err != nil {
				return err
			}
		}
		return fn(ctx, cmd, args, stackName)
	}
}
