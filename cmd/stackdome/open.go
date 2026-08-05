package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/cmdutil"
	clierrors "github.com/stackdome/cli/internal/errors"
)

func newOpenCmd() *cobra.Command {
	var flagStack string

	cmd := &cobra.Command{
		Use:   "open [resource]",
		Short: "Open a resource's public URL in the browser",
		Args:  cobra.MaximumNArgs(1),
		RunE: cmdutil.WithContext(cmdutil.RequireAuth(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			stackID, err := resolveStackID(ctx, cmd, flagStack)
			if err != nil {
				return err
			}

			resources, err := ctx.Client.GetStackResources(cmd.Context(), stackID)
			if err != nil {
				return err
			}

			resourceFilter := ""
			if len(args) > 0 {
				resourceFilter = args[0]
			}

			type publicURL struct {
				Resource string
				URL      string
			}

			var urls []publicURL
			for _, res := range resources {
				if resourceFilter != "" && res.Name != resourceFilter {
					continue
				}
				if res.Status == nil {
					continue
				}
				for _, ing := range res.Status.PublicIngress {
					if ing.Url != nil && *ing.Url != "" {
						urls = append(urls, publicURL{Resource: res.Name, URL: *ing.Url})
					}
				}
			}

			if resourceFilter != "" && len(urls) == 0 {
				found := false
				for _, res := range resources {
					if res.Name == resourceFilter {
						found = true
						break
					}
				}
				if !found {
					return clierrors.NotFoundError("Resource", resourceFilter)
				}
				return clierrors.Newf("No public URLs found for resource %q", resourceFilter)
			}

			if len(urls) == 0 {
				return clierrors.New("No public URLs found in this stack")
			}

			if len(urls) > 1 {
				tbl := ctx.Formatter.NewTable("RESOURCE", "URL")
				for _, u := range urls {
					tbl.AddRow(u.Resource, u.URL)
				}
				tbl.Render()
				fmt.Fprintln(os.Stderr)
			}

			target := urls[0].URL
			fmt.Fprintf(os.Stderr, "Opening %s...\n", target)
			return openBrowser(target)
		})),
	}

	cmd.Flags().StringVarP(&flagStack, "stack", "s", "", "Stack name (overrides current context)")

	return cmd
}

func openBrowser(url string) error {
	if !strings.Contains(url, "://") {
		url = "http://" + url
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return clierrors.Newf("unsupported platform %q — open %s manually", runtime.GOOS, url)
	}
	return cmd.Start()
}
