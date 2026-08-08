package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
)

type mutationResult struct {
	Status   string `json:"status" yaml:"status"`
	Resource string `json:"resource" yaml:"resource"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	ID       string `json:"id,omitempty" yaml:"id,omitempty"`
}

func printMutationResult(ctx *cmdutil.CommandContext, result mutationResult, tableMessage string) error {
	if !ctx.Formatter.IsTable() {
		return ctx.Formatter.PrintStructured(result)
	}
	fmt.Fprintln(os.Stderr, tableMessage)
	return nil
}

func redactSecrets(message string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	return message
}

func resolveStackID(ctx *cmdutil.CommandContext, cmd *cobra.Command, flagStack string) (string, error) {
	if flagStack != "" {
		return resolveStackRef(ctx, cmd, flagStack)
	}
	current, err := ctx.Config.RequireStack()
	if err != nil {
		return "", err
	}
	// Configs written before set-stack resolved its argument hold a stack name,
	// which every API call rejects. Heal it in place on first use.
	if looksLikeUUID(current) {
		return current, nil
	}
	id, err := resolveStackRef(ctx, cmd, current)
	if err != nil {
		return "", err
	}
	_ = ctx.Config.SetCurrentStack(id)
	return id, nil
}

// looksLikeUUID is a shape check, not validation: stack IDs are UUIDs and stack
// names cannot contain the 8-4-4-4-12 layout, which is all we need to tell them
// apart without a lookup.
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
	}
	return true
}

// resolveStackRef turns whatever the user typed — a stack name, a full ID, or
// the truncated ID the list tables print — into a full stack ID. Everything
// downstream (config current_stack included) needs the ID, so a name stored
// verbatim would brick every later command.
func resolveStackRef(ctx *cmdutil.CommandContext, cmd *cobra.Command, ref string) (string, error) {
	stacks, err := ctx.Client.ListStacks(cmd.Context())
	if err != nil {
		return "", err
	}
	ids := make([]string, 0, len(stacks))
	for i := range stacks {
		if stacks[i].Name == ref {
			return stacks[i].GetId(), nil
		}
		ids = append(ids, stacks[i].GetId())
	}
	return resolveIDPrefix("Stack", ref, ids)
}

// resolveIDPrefix maps a possibly-truncated ID back to a full one, docker-style:
// list tables print IDs through shortID(), so the IDs users copy are prefixes.
// An exact match always wins; otherwise a unique prefix match is used.
func resolveIDPrefix(kind, arg string, ids []string) (string, error) {
	var matches []string
	for _, id := range ids {
		switch {
		case id == arg:
			return id, nil
		case strings.HasPrefix(id, arg):
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", clierrors.NotFoundError(kind, arg)
	default:
		return "", clierrors.ValidationError(
			"Ambiguous " + kind + " ID \"" + arg + "\": matches " + strings.Join(matches, ", "))
	}
}

func resolveBuildID(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID, arg string) (string, error) {
	build, err := resolveBuild(ctx, cmd, stackID, arg)
	if err != nil {
		return "", err
	}
	return build.GetId(), nil
}

func resolveBuild(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID, arg string) (openapi.ImageBuild, error) {
	builds, err := ctx.Client.ListBuilds(cmd.Context(), stackID)
	if err != nil {
		return openapi.ImageBuild{}, err
	}
	ids := make([]string, 0, len(builds))
	buildsByID := make(map[string]openapi.ImageBuild, len(builds))
	refMatches := make([]openapi.ImageBuild, 0, 1)
	for _, b := range builds {
		id := b.GetId()
		if id == arg {
			return b, nil
		}
		ids = append(ids, id)
		buildsByID[id] = b
		if buildReference(b) == arg {
			refMatches = append(refMatches, b)
		}
	}
	switch len(refMatches) {
	case 1:
		return refMatches[0], nil
	case 0:
		id, resolveErr := resolveIDPrefix("Build", arg, ids)
		if resolveErr != nil {
			return openapi.ImageBuild{}, resolveErr
		}
		return buildsByID[id], nil
	default:
		matchingIDs := make([]string, len(refMatches))
		for i := range refMatches {
			matchingIDs[i] = refMatches[i].GetId()
		}
		return openapi.ImageBuild{}, clierrors.ValidationError(
			"Ambiguous Build reference \"" + arg + "\": matches " + strings.Join(matchingIDs, ", "))
	}
}

func resolveReleaseID(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID, arg string) (string, error) {
	// A full ID is already unambiguous and may refer to a historical release
	// outside the first list page. Prefixes still use the list for expansion.
	if looksLikeUUID(arg) {
		return arg, nil
	}
	releases, err := ctx.Client.ListReleases(cmd.Context(), stackID)
	if err != nil {
		return "", err
	}
	ids := make([]string, 0, len(releases))
	for _, r := range releases {
		ids = append(ids, r.GetId())
	}
	return resolveIDPrefix("Release", arg, ids)
}
