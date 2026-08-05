package main

import (
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
