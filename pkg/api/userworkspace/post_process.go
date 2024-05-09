package userworkspace

import (
	"os"

	"github.com/hashicorp/go-envparse"
)

func (w Workspace) Process() error {
	for _, spec := range w.Resources {
		for _, envFile := range spec.EnvFiles {
			file, err := os.Open(envFile)
			if err != nil {
				return err
			}
			envVarsFromFile, err := envparse.Parse(file)
			if err != nil {
				return err
			}
			if spec.EnvironmentVariables == nil {
				spec.EnvironmentVariables = map[string]string{}
			}
			for key, value := range envVarsFromFile {
				spec.EnvironmentVariables[key] = value
			}
		}
	}
	return nil
}
