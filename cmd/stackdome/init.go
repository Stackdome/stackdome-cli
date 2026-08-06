package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	clierrors "github.com/stackdome/cli/internal/errors"
	"github.com/stackdome/cli/internal/output"
	"github.com/stackdome/cli/internal/stackfile"
	"gopkg.in/yaml.v3"
)

const stackfileTemplate = `name: {{NAME}}

resources:
  web:
    image: nginx:latest
    ports:
      - name: http
        port: 8080
        public: true
        subdomain: web
    env:
      APP_ENV: "production"
      PUBLIC_URL: "{{ self.public_url }}"
      DB_HOST: "{{ db.host }}"
      DB_URL: "postgres://{{ db.host }}:{{ db.port }}/mydb"
      REDIS_URL: "redis://{{ redis.host }}:6379"
    # secrets:
    #   my-secret:
    #     API_KEY: api_key
    depends_on: [db, redis]

  db:
    image: postgres:16
    ports:
      - name: postgres
        port: 5432
    env:
      POSTGRES_DB: mydb
      POSTGRES_USER: app
      POSTGRES_PASSWORD: changeme
    volumes:
      - name: db-data
        path: /var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - name: redis
        port: 6379

volumes:
  db-data:
    size: 5Gi
`

func newInitCmd() *cobra.Command {
	var (
		flagName     string
		flagForce    bool
		flagFile     string
		flagAgentsMD bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new stackfile",
		Long: `Scaffold a new stackfile.yaml for your project.

If a docker-compose.yaml (or compose.yaml) is found in the current directory,
it will be converted to a stackfile automatically. Use -f to specify a
compose file explicitly.

If no compose file is found, a starter template is generated.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := flagName
			if name == "" {
				dir, err := os.Getwd()
				if err != nil {
					return clierrors.Wrap(err, "Failed to get current directory")
				}
				name = filepath.Base(dir)
			}

			outPath := "stackfile.yaml"
			if !flagForce {
				if _, err := os.Stat(outPath); err == nil {
					return clierrors.ValidationError(fmt.Sprintf("%s already exists (use --force to overwrite)", outPath))
				}
			}

			composePath := flagFile
			if composePath == "" {
				dir, _ := os.Getwd()
				composePath = stackfile.FindComposeFile(dir)
			}

			var content []byte
			if composePath != "" {
				sf, envFiles, err := stackfile.FromCompose(composePath, name)
				if err != nil {
					return clierrors.Wrap(err, "Failed to convert compose file")
				}
				out, err := yaml.Marshal(sf)
				if err != nil {
					return clierrors.Wrap(err, "Failed to generate stackfile")
				}
				content = out

				if err := os.WriteFile(outPath, content, 0644); err != nil {
					return clierrors.Wrap(err, "Failed to write stackfile")
				}

				fmt.Fprintf(os.Stderr, "%s Converted %s → %s\n\n",
					output.Green("✓"), composePath, outPath)

				resources := make([]string, 0, len(sf.Resources))
				for name := range sf.Resources {
					resources = append(resources, name)
				}
				fmt.Fprintf(os.Stderr, "  %s  %s\n", output.Bold("Resources:"), strings.Join(resources, ", "))
				if len(sf.Volumes) > 0 {
					volumes := make([]string, 0, len(sf.Volumes))
					for name := range sf.Volumes {
						volumes = append(volumes, name)
					}
					fmt.Fprintf(os.Stderr, "  %s   %s\n", output.Bold("Volumes:"), strings.Join(volumes, ", "))
				}
				fmt.Fprintln(os.Stderr)

				warnings := checkConvertedStackfile(sf, envFiles)
				if len(warnings) > 0 {
					for _, w := range warnings {
						fmt.Fprintf(os.Stderr, "  %s %s\n", output.Yellow("!"), w)
					}
					fmt.Fprintln(os.Stderr)
				}

				if err := stackfile.Validate(sf); err != nil {
					fmt.Fprintf(os.Stderr, "  %s Validation failed: %s\n\n", output.Red("✗"), err)
				} else {
					fmt.Fprintf(os.Stderr, "  %s Validation passed\n\n", output.Green("✓"))
				}

				fmt.Fprintf(os.Stderr, "  %s\n", output.Dim("Next steps:"))
				fmt.Fprintf(os.Stderr, "  %s\n", output.Dim("  stackdome deploy -f "+outPath))
			} else {
				content = []byte(strings.Replace(stackfileTemplate, "{{NAME}}", name, 1))
				if err := os.WriteFile(outPath, content, 0644); err != nil {
					return clierrors.Wrap(err, "Failed to write stackfile")
				}
				fmt.Fprintf(os.Stderr, "Created %s\n", outPath)
			}

			if flagAgentsMD && shouldWriteAgentsMD() {
				if err := writeAgentsStanza("."); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not update AGENTS.md: %v\n", err)
				} else {
					fmt.Fprintln(os.Stderr, "Updated AGENTS.md with Stackdome instructions")
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&flagName, "name", "", "App name (defaults to current directory name)")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing stackfile.yaml")
	cmd.Flags().StringVarP(&flagFile, "file", "f", "", "Path to docker-compose file to convert")
	cmd.Flags().BoolVar(&flagAgentsMD, "agents-md", true, "Write a Stackdome section to AGENTS.md")

	return cmd
}

func checkConvertedStackfile(sf *stackfile.Stackfile, envFiles map[string]string) []string {
	var warnings []string
	for name, res := range sf.Resources {
		if res.Build != nil && res.Build.Repo == "" {
			warnings = append(warnings, fmt.Sprintf("resource %q has a local build (no git repo) — set build.repo to a git URL", name))
		}
		if res.Image == "" && res.Build == nil {
			warnings = append(warnings, fmt.Sprintf("resource %q has no image or build config", name))
		}
		if ref, ok := envFiles[name]; ok {
			warnings = append(warnings, fmt.Sprintf("resource %q used env_file %q in compose — add `env_file: %s` under it to load those vars at deploy", name, ref, ref))
		}
	}
	return warnings
}
