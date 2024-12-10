package validation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/tools"

	"github.com/go-playground/validator"
)

func Validate(voyagerFilePath string) error {
	var workspace v1alpha1.UserStack

	if err := tools.UnmarshalYamlFile(voyagerFilePath, &workspace); err != nil {
		return fmt.Errorf("error reading YAML file: %v", err)
	}

	validate := validator.New()
	definedVolumeNames := []string{}
	for volumeName := range workspace.Volumes {
		definedVolumeNames = append(definedVolumeNames, volumeName)
	}

	for resource, resourceSpec := range workspace.Resources {
		resourceSpec := *resourceSpec
		if err := validate.Struct(resourceSpec); err != nil {
			return fmt.Errorf("error validating YAML file, resource '%s': %v", resource, err)
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
			if !tools.FileExists(envFile) {
				return fmt.Errorf("cant read env file for resource '%s' at: '%s'", resource, envFile)
			}
		}
	}

	return nil
}
