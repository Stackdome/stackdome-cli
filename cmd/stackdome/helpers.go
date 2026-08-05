package main

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
)

func resolveStackID(ctx *cmdutil.CommandContext, cmd *cobra.Command, flagStack string) (string, error) {
	if flagStack != "" {
		s, err := ctx.Client.FindStackByName(cmd.Context(), flagStack)
		if err != nil {
			return "", err
		}
		if s == nil {
			return "", clierrors.NotFoundError("Stack", flagStack)
		}
		return *s.Id, nil
	}
	return ctx.Config.RequireStack()
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
	builds, err := ctx.Client.ListBuilds(cmd.Context(), stackID)
	if err != nil {
		return "", err
	}
	ids := make([]string, 0, len(builds))
	for _, b := range builds {
		ids = append(ids, b.GetId())
	}
	return resolveIDPrefix("Build", arg, ids)
}

func resolveReleaseID(ctx *cmdutil.CommandContext, cmd *cobra.Command, stackID, arg string) (string, error) {
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
