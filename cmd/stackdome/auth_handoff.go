package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Stackdome/stackdome-cli/internal/config"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
)

type authSetup struct {
	ServerURL   string `json:"server_url" yaml:"server_url"`
	SignInURL   string `json:"sign_in_url" yaml:"sign_in_url"`
	SignupURL   string `json:"signup_url" yaml:"signup_url"`
	APITokenURL string `json:"api_token_url" yaml:"api_token_url"`
	Login       string `json:"login_command" yaml:"login_command"`
}

func resolveAuthSetup(flagURL string, flagInsecure bool, cfg *config.Config) (*authSetup, bool, error) {
	rawURL := strings.TrimSpace(flagURL)
	insecure := flagInsecure
	if rawURL == "" {
		rawURL = cfg.ServerURL
		insecure = flagInsecure || cfg.Insecure
	}
	if rawURL == "" {
		return nil, false, clierrors.ValidationError("--url is required when no Stackdome instance is selected")
	}

	serverURL, err := normalizeLoginServerURL(rawURL, insecure)
	if err != nil {
		return nil, false, err
	}
	loginCommand := fmt.Sprintf("stackdome login --url %s", quoteShellArgument(serverURL))
	if insecure {
		loginCommand += " --insecure"
	}
	loginCommand += " --token <token>"

	return &authSetup{
		ServerURL:   serverURL,
		SignInURL:   authPageURL(serverURL, "/sign-in"),
		SignupURL:   authPageURL(serverURL, "/sign-up"),
		APITokenURL: authPageURL(serverURL, "/settings/api-tokens"),
		Login:       loginCommand,
	}, insecure, nil
}

func quoteShellArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func authPageURL(serverURL, pagePath string) string {
	parsed, _ := url.Parse(serverURL)
	parsed.Path = strings.TrimRight(parsed.Path, "/") + pagePath
	parsed.RawPath = ""
	return parsed.String()
}

func (s *authSetup) signupGuidance() string {
	return fmt.Sprintf(
		"Sign up for Stackdome at %s\nAfter signing in, create a Full access API token at %s\nThen log in to the CLI with:\n  %s",
		s.SignupURL,
		s.APITokenURL,
		s.Login,
	)
}

func (s *authSetup) loginGuidance() string {
	return fmt.Sprintf(
		"Stackdome CLI login requires an API token.\nSign in at %s (or sign up at %s), then create a Full access API token at %s.\nRun:\n  %s",
		s.SignInURL,
		s.SignupURL,
		s.APITokenURL,
		s.Login,
	)
}
