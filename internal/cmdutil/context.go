package cmdutil

import (
	"log/slog"
	"os"

	"github.com/stackdome/cli/internal/client"
	"github.com/stackdome/cli/internal/config"
	"github.com/stackdome/cli/internal/output"
)

type CommandContext struct {
	Config    *config.Config
	Client    *client.Client
	Formatter *output.Formatter
	Logger    *slog.Logger
}

func NewCommandContext(cfg *config.Config, format output.Format, logLevel slog.Level) *CommandContext {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	var c *client.Client
	if cfg.IsLoggedIn() {
		c = client.New(cfg.ServerURL,
			client.WithTokens(cfg.AccessToken, cfg.RefreshToken),
			client.WithOrgAndProject(cfg.OrganizationID, cfg.ProjectName),
			client.WithInsecure(cfg.Insecure),
			client.WithTokenRefreshCallback(func(accessToken, refreshToken string) error {
				cfg.AccessToken = accessToken
				cfg.RefreshToken = refreshToken
				return cfg.Save()
			}),
		)
	}

	return &CommandContext{
		Config:    cfg,
		Client:    c,
		Formatter: output.NewFormatter(format),
		Logger:    logger,
	}
}
