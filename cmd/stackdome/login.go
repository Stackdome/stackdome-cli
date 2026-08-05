package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	serverapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
	"github.com/stackdome/cli/internal/client"
	"github.com/stackdome/cli/internal/config"
	clierrors "github.com/stackdome/cli/internal/errors"
	"golang.org/x/term"
)

func newLoginCmd() *cobra.Command {
	var (
		flagURL      string
		flagEmail    string
		flagPassword string
		flagToken    string
		flagInsecure bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a Stackdome server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagURL == "" {
				return clierrors.ValidationError("--url is required")
			}

			// Refuse to block on a prompt nobody can answer.
			if flagToken == "" && (flagEmail == "" || flagPassword == "") && !term.IsTerminal(int(os.Stdin.Fd())) {
				return clierrors.ValidationError("non-interactive login requires --token, or both --email and --password")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if flagToken != "" {
				return loginWithToken(cmd, cfg, flagURL, flagToken, flagInsecure)
			}
			return loginWithCredentials(cmd, cfg, flagURL, flagEmail, flagPassword, flagInsecure)
		},
	}

	cmd.Flags().StringVar(&flagURL, "url", "", "Stackdome server URL (required)")
	cmd.Flags().StringVar(&flagEmail, "email", "", "Email address")
	cmd.Flags().StringVar(&flagPassword, "password", "", "Password")
	cmd.Flags().StringVar(&flagToken, "token", "", "API token (skips email/password)")
	cmd.Flags().BoolVar(&flagInsecure, "insecure", false, "Allow insecure HTTPS")

	return cmd
}

func loginWithToken(cmd *cobra.Command, cfg *config.Config, serverURL, token string, insecure bool) error {
	c := client.New(serverURL,
		client.WithTokens(token, ""),
		client.WithInsecure(insecure),
	)

	user, err := c.GetCurrentUser(cmd.Context())
	if err != nil {
		return err
	}

	cfg.ServerURL = serverURL
	cfg.AccessToken = token
	cfg.RefreshToken = ""
	cfg.OrganizationID = user.GetOrganisationId()
	cfg.Username = userDisplayName(user)
	cfg.Insecure = insecure

	if err := persistLogin(cmd, c, cfg); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Logged in as %s\n", cfg.Username)
	return nil
}

// persistLogin writes the credential before resolving the project: a project
// lookup that fails must not throw away a token the user just obtained. Without
// a project, commands that need one fail later with a clear message — but the
// login itself stands.
func persistLogin(cmd *cobra.Command, c *client.Client, cfg *config.Config) error {
	// An explicit login always persists in full, even when the values happen to
	// equal STACKDOME_URL / STACKDOME_TOKEN.
	cfg.AdoptEnvValues()
	if err := cfg.Save(); err != nil {
		return err
	}

	projectName, err := c.ResolveDefaultProject(cmd.Context(), cfg.OrganizationID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", clierrors.UserMessage(err))
		return nil
	}

	cfg.ProjectName = projectName
	return cfg.Save()
}

func loginWithCredentials(cmd *cobra.Command, cfg *config.Config, serverURL, email, password string, insecure bool) error {
	if email == "" {
		email = promptInput("Email: ")
	}
	if password == "" {
		password = promptPassword("Password: ")
	}

	c := client.New(serverURL, client.WithInsecure(insecure))

	result, err := c.Login(cmd.Context(), email, password)
	if err != nil {
		return err
	}

	cfg.ServerURL = serverURL
	cfg.AccessToken = result.AccessToken
	cfg.RefreshToken = result.RefreshToken
	cfg.OrganizationID = result.User.GetOrganisationId()
	cfg.Username = userDisplayName(result.User)
	cfg.Insecure = insecure

	if err := persistLogin(cmd, c, cfg); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Logged in as %s\n", cfg.Username)
	return nil
}

func userDisplayName(u *serverapi.User) string {
	if u == nil {
		return ""
	}
	if name := u.GetUsername(); name != "" {
		return name
	}
	if name := u.GetName(); name != "" {
		return name
	}
	return u.GetEmail()
}

func promptInput(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func promptPassword(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return ""
	}
	return string(b)
}
