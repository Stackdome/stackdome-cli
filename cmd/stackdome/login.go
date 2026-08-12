package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/Stackdome/stackdome-cli/internal/client"
	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	"github.com/Stackdome/stackdome-cli/internal/config"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	serverapi "github.com/Stackdome/stackdome/pkg/api/openapi"
	"github.com/spf13/cobra"
)

type authenticationResult struct {
	Authenticated  bool   `json:"authenticated" yaml:"authenticated"`
	AccountCreated bool   `json:"account_created,omitempty" yaml:"account_created,omitempty"`
	User           string `json:"user" yaml:"user"`
	OrganizationID string `json:"organization_id" yaml:"organization_id"`
	Project        string `json:"project,omitempty" yaml:"project,omitempty"`
	ServerURL      string `json:"server_url" yaml:"server_url"`
	AuthMethod     string `json:"auth_method" yaml:"auth_method"`
}

func newLoginCmd() *cobra.Command {
	var (
		flagURL      string
		flagToken    string
		flagInsecure bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a Stackdome API token",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdutil.GetContext(cmd)
			setup, insecure, err := resolveAuthSetup(flagURL, flagInsecure, ctx.Config)
			if err != nil {
				return err
			}
			if flagToken == "" {
				return clierrors.ValidationError(setup.loginGuidance())
			}
			return loginWithToken(cmd, ctx.Config, setup.ServerURL, flagToken, insecure)
		},
	}

	cmd.Flags().StringVar(&flagURL, "url", "", "Stackdome server URL (defaults to the selected instance)")
	cmd.Flags().StringVar(&flagToken, "token", "", "Full access API token")
	cmd.Flags().BoolVar(&flagInsecure, "insecure", false, "Allow HTTP or skip HTTPS certificate verification")

	return cmd
}

func normalizeLoginServerURL(raw string, insecure bool) (string, error) {
	serverURL := strings.TrimSpace(raw)
	if !strings.Contains(serverURL, "://") {
		serverURL = "https://" + serverURL
	}

	parsed, err := url.Parse(serverURL)
	if err != nil || parsed.Host == "" {
		return "", clierrors.ValidationError("--url must be a valid Stackdome server URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", clierrors.ValidationError("--url must not include credentials, a query string, or a fragment")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = strings.TrimRight(parsed.RawPath, "/")
	switch parsed.Scheme {
	case "https":
		return parsed.String(), nil
	case "http":
		if !insecure {
			return "", clierrors.ValidationError("refusing insecure HTTP server; use https:// or pass --insecure")
		}
		return parsed.String(), nil
	default:
		return "", clierrors.ValidationError("--url must use https://; pass --insecure to use http://")
	}
}

func loginWithToken(cmd *cobra.Command, cfg *config.Config, serverURL, token string, insecure bool) error {
	c := client.New(serverURL,
		client.WithTokens(token, ""),
		client.WithInsecure(insecure),
	)

	user, err := c.GetCurrentUser(cmd.Context())
	if err != nil {
		return redactLoginError(err, token)
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

	return printAuthenticationResult(cmd, cfg, false, "api_token")
}

func redactLoginError(err error, secrets ...string) error {
	var cliErr *clierrors.CLIError
	if errors.As(err, &cliErr) {
		redacted := *cliErr
		redacted.Message = redactSecrets(redacted.Message, secrets...)
		redacted.Detail = redactSecrets(redacted.Detail, secrets...)
		redacted.Cause = nil
		return &redacted
	}
	return clierrors.New(redactSecrets(err.Error(), secrets...))
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
		safeErr := redactLoginError(err, cfg.AccessToken, cfg.RefreshToken)
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", clierrors.UserMessage(safeErr))
		return nil
	}

	cfg.ProjectName = projectName
	return cfg.Save()
}

func printAuthenticationResult(cmd *cobra.Command, cfg *config.Config, accountCreated bool, authMethod string) error {
	ctx := cmdutil.GetContext(cmd)
	if !ctx.Formatter.IsTable() {
		return ctx.Formatter.PrintStructured(authenticationResult{
			Authenticated:  true,
			AccountCreated: accountCreated,
			User:           cfg.Username,
			OrganizationID: cfg.OrganizationID,
			Project:        cfg.ProjectName,
			ServerURL:      cfg.ServerURL,
			AuthMethod:     authMethod,
		})
	}
	if accountCreated {
		fmt.Fprintf(os.Stderr, "Account created. Logged in as %s\n", cfg.Username)
	} else {
		fmt.Fprintf(os.Stderr, "Logged in as %s\n", cfg.Username)
	}
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
