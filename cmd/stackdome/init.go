package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/stackfile"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type initResult struct {
	Path      string   `json:"path"`
	Source    string   `json:"source"`
	Resources []string `json:"resources"`
	Volumes   []string `json:"volumes"`
	Warnings  []string `json:"warnings"`
	Valid     bool     `json:"valid"`
}

func newInitCmd() *cobra.Command {
	var (
		flagName  string
		flagForce bool
		flagFile  string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new stackfile",
		Long: `Scaffold a new stackfile.yaml for your project.

If a docker-compose.yaml (or compose.yaml) is found in the current directory,
it will be converted to a Stackfile automatically. Use -f to specify a compose
file explicitly. If no compose file is found, a minimal nginx Stackfile is generated.`,
		RunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			name := flagName
			if name == "" {
				dir, err := os.Getwd()
				if err != nil {
					return clierrors.Wrap(err, "Failed to get current directory")
				}
				name = filepath.Base(dir)
			}

			const outPath = "stackfile.yaml"
			if !flagForce {
				if _, err := os.Stat(outPath); err == nil {
					return clierrors.ValidationError(fmt.Sprintf("%s already exists (use --force to overwrite)", outPath))
				}
			}

			composePath := flagFile
			if composePath == "" {
				dir, err := os.Getwd()
				if err != nil {
					return clierrors.Wrap(err, "Failed to get current directory")
				}
				composePath = stackfile.FindComposeFile(dir)
			}

			var (
				sf       *stackfile.Stackfile
				warnings []string
				source   = "template"
				err      error
			)
			if composePath != "" {
				var composeWarnings stackfile.ComposeWarnings
				sf, composeWarnings, err = stackfile.FromCompose(composePath, name)
				if err != nil {
					return clierrors.Wrap(err, "Failed to convert compose file")
				}
				warnings = checkConvertedStackfile(sf, composeWarnings)
				source = "compose"
			} else {
				sf = &stackfile.Stackfile{
					Name: name,
					Resources: map[string]stackfile.Resource{
						"web": {
							Image: "nginx:alpine",
							Ports: []stackfile.PortDef{{Name: "http", Port: 80, Public: true}},
						},
					},
				}
			}

			content, err := yaml.Marshal(sf)
			if err != nil {
				return clierrors.Wrap(err, "Failed to generate Stackfile")
			}
			if err := os.WriteFile(outPath, content, 0o644); err != nil {
				return clierrors.Wrap(err, "Failed to write Stackfile")
			}

			result := initResult{
				Path:      outPath,
				Source:    source,
				Resources: sortedResourceNames(sf),
				Volumes:   sortedVolumeNames(sf),
				Warnings:  warnings,
				Valid:     stackfile.Validate(sf) == nil,
			}
			if !ctx.Formatter.IsTable() {
				return ctx.Formatter.PrintStructured(result)
			}

			fmt.Fprintf(os.Stderr, "Created %s from %s.\n", outPath, source)
			for _, warning := range warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
			}
			if !result.Valid {
				fmt.Fprintln(os.Stderr, "Warning: generated Stackfile needs manual changes before deployment.")
			}
			return nil
		}),
	}

	cmd.Flags().StringVar(&flagName, "name", "", "App name (defaults to current directory name)")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing stackfile.yaml")
	cmd.Flags().StringVarP(&flagFile, "file", "f", "", "Path to docker-compose file to convert")

	return cmd
}

func sortedResourceNames(sf *stackfile.Stackfile) []string {
	names := make([]string, 0, len(sf.Resources))
	for name := range sf.Resources {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedVolumeNames(sf *stackfile.Stackfile) []string {
	names := make([]string, 0, len(sf.Volumes))
	for name := range sf.Volumes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func checkConvertedStackfile(sf *stackfile.Stackfile, composeWarnings stackfile.ComposeWarnings) []string {
	var warnings []string
	if keys := composeWarnings.UnsupportedTopLevelKeys; len(keys) > 0 {
		warnings = append(warnings, fmt.Sprintf("compose used unsupported top-level keys %s — review and recreate this behavior manually in stackfile.yaml", strings.Join(quotedStrings(keys), ", ")))
	}
	for _, name := range sortedVolumeNames(sf) {
		if options := composeWarnings.UnsupportedVolumeOptions[name]; len(options) > 0 {
			warnings = append(warnings, fmt.Sprintf("compose volume %q used unsupported options %s — only the volume name was preserved with a default size; recreate storage settings manually", name, strings.Join(quotedStrings(options), ", ")))
		}
	}
	for _, name := range sortedResourceNames(sf) {
		res := sf.Resources[name]
		if res.Build != nil && res.Build.Repo == "" {
			warnings = append(warnings, fmt.Sprintf("resource %q has a local build (no git repo) — set build.repo to a git URL", name))
		}
		if res.Image == "" && res.Build == nil {
			warnings = append(warnings, fmt.Sprintf("resource %q has no image or build config", name))
		}
		if options := composeWarnings.UnsupportedBuildOptions[name]; len(options) > 0 {
			warnings = append(warnings, fmt.Sprintf("resource %q used unsupported build options %s — only build.context and build.dockerfile were preserved; recreate build settings manually", name, strings.Join(quotedStrings(options), ", ")))
		}
		if forms := composeWarnings.UnsupportedCommandForms[name]; len(forms) > 0 {
			warnings = append(warnings, fmt.Sprintf("resource %q used unsupported or ambiguous Compose command forms for %s — exact argument, shell, or image-default semantics cannot be preserved; the values were omitted, so use explicit non-empty YAML string lists", name, strings.Join(quotedStrings(forms), ", ")))
		}
		if options := composeWarnings.UnsupportedDependsOnOptions[name]; len(options) > 0 {
			warnings = append(warnings, fmt.Sprintf("resource %q used unsupported depends_on options %s — dependency names were preserved; recreate ordering and health requirements manually", name, strings.Join(quotedStrings(options), ", ")))
		}
		if entries := composeWarnings.UnsupportedVolumeMountOptions[name]; len(entries) > 0 {
			warnings = append(warnings, fmt.Sprintf("resource %q used unsupported volume mount entries or options %s — only supported named-volume source and target pairs were preserved; recreate mount behavior manually", name, strings.Join(quotedStrings(entries), ", ")))
		}
		if mappings := composeWarnings.UnsupportedPortMappings[name]; len(mappings) > 0 {
			warnings = append(warnings, fmt.Sprintf("resource %q used port mappings %s whose host IP or published port cannot be represented exactly — container ports were preserved; constrained host-IP bindings remain private, and published host ports require Stackdome routing", name, strings.Join(quotedStrings(mappings), ", ")))
		}
		if entries := composeWarnings.UnsupportedPorts[name]; len(entries) > 0 {
			warnings = append(warnings, fmt.Sprintf("resource %q used unsupported port entries %s — these ports were omitted; replace ranges, invalid syntax, or unsupported protocols with explicit TCP single-port mappings", name, strings.Join(quotedStrings(entries), ", ")))
		}
		if refs := composeWarnings.EnvFiles[name]; len(refs) > 0 {
			quotedRefs := make([]string, len(refs))
			for i, ref := range refs {
				quotedRefs[i] = fmt.Sprintf("%q", ref)
			}
			warnings = append(warnings, fmt.Sprintf("resource %q used env_file entries %s in compose — copy required non-sensitive values into env; Stackfiles reject env_file", name, strings.Join(quotedRefs, ", ")))
		}
		if binds := composeWarnings.UnsupportedBindMounts[name]; len(binds) > 0 {
			warnings = append(warnings, fmt.Sprintf("resource %q used unsupported bind mount sources %s in compose — use a named volume instead", name, strings.Join(binds, ", ")))
		}
		if keys := composeWarnings.UnsupportedServiceKeys[name]; len(keys) > 0 {
			warnings = append(warnings, fmt.Sprintf("resource %q used unsupported compose keys %s — review and recreate this behavior manually in stackfile.yaml", name, strings.Join(quotedStrings(keys), ", ")))
		}
		if variables := composeWarnings.UnresolvedEnvironment[name]; len(variables) > 0 {
			warnings = append(warnings, fmt.Sprintf("resource %q has unresolved environment variables %s — set explicit non-sensitive values in env or connect a Stackdome secret; no values were imported", name, strings.Join(quotedStrings(variables), ", ")))
		}
	}
	return warnings
}

func quotedStrings(values []string) []string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	return quoted
}
