package cmdutil

import (
	"log/slog"
	"os"

	"github.com/stackdome/cli/internal/config"
	"github.com/stackdome/cli/internal/output"
)

type CommandContext struct {
	Config    *config.Config
	Formatter *output.Formatter
	Logger    *slog.Logger
}

func NewCommandContext(cfg *config.Config, format output.Format, logLevel slog.Level) *CommandContext {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	return &CommandContext{
		Config:    cfg,
		Formatter: output.NewFormatter(format),
		Logger:    logger,
	}
}
