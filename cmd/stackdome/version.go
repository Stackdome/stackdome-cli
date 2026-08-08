package main

import (
	"runtime"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		RunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			info := struct {
				Version string `json:"version"`
				Commit  string `json:"commit"`
				Built   string `json:"built"`
				Go      string `json:"go"`
				OSArch  string `json:"os_arch"`
			}{Version, GitCommit, BuildDate, runtime.Version(), runtime.GOOS + "/" + runtime.GOARCH}

			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(info)
			}

			f := ctx.Formatter
			f.Printf("stackdome %s\n", info.Version)
			f.Printf("  commit:  %s\n", info.Commit)
			f.Printf("  built:   %s\n", info.Built)
			f.Printf("  go:      %s\n", info.Go)
			f.Printf("  os/arch: %s\n", info.OSArch)
			return nil
		}),
	}
}
