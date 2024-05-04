package userworkspace

import (
	"fmt"
	"os"

	"github.com/go-playground/validator"
	"gopkg.in/yaml.v2"
)

func Validate(voyagerFilePath string) error {
	yamlFile, err := os.Open(voyagerFilePath)
	if err != nil {
		return fmt.Errorf("error opening YAML file: %v\n", err)
	}
	defer yamlFile.Close()

	// Parse the YAML file
	var workspace Workspace
	err = yaml.NewDecoder(yamlFile).Decode(&workspace)
	if err != nil {
		return fmt.Errorf("error parsing YAML file: %v\n", err)
	}
	validate := validator.New()
	for resource, resourceSpec := range workspace {
		if err := validate.Struct(resourceSpec); err != nil {
			return fmt.Errorf("error validating YAML file, resource '%s': %v\n", resource, err)
		}
		for _, envFile := range resourceSpec.EnvFiles {
			if !fileExists(envFile) {
				return fmt.Errorf("cant read env file for resource '%s' at: '%s'", resource, envFile)
			}
		}

	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
