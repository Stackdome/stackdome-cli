package userworkspace

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/go-playground/validator"
)

func Validate(voyagerFilePath string) error {
	workspace, err := Unmarshal(voyagerFilePath)
	if err != nil {
		return fmt.Errorf("error parsing YAML file: %v\n", err)
	}
	fmt.Printf("workspace: %+v \n", workspace)
	for name, resource := range workspace.Resources {
		fmt.Printf("name: %s, resource: %+v \n", name, *resource)
	}
	validate := validator.New()
	definedVolumeNames := []string{}
	for volumeName := range workspace.Volumes {
		definedVolumeNames = append(definedVolumeNames, volumeName)
	}

	for resource, resourceSpec := range workspace.Resources {
		resourceSpec := *resourceSpec
		if err := validate.Struct(resourceSpec); err != nil {
			return fmt.Errorf("error validating YAML file, resource '%s': %v\n", resource, err)
		}
		// Validate build spec
		if resourceSpec.Build != nil && !slices.Contains(definedVolumeNames, resourceSpec.Build.SourceVolume) {
			return fmt.Errorf("'%s'resource build references a volume which is not defined", resource)
		}
		// Validate volume mounts
		for source := range resourceSpec.VolumeMounts {
			sourcePath := strings.Split(source, "/")
			if len(sourcePath) < 1 {
				return fmt.Errorf("empty volume mount for resource: '%s'", resource)
			}
			// first in this list is the volume name.

			currentVolumeName := sourcePath[0]
			if !slices.Contains(definedVolumeNames, currentVolumeName) {
				return fmt.Errorf("'%s' resource mount references a volume '%s' which is not defined", resource, currentVolumeName)
			}
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
