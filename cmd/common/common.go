package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/tools"
	"github.com/ashishmax31/voyager-cli/pkg/validation"
)

const (
	VoyagerFilePathFlag    = "voyagerfile-path"
	AllResourcesFlag       = "all"
	InteractiveSessionFlag = "interactive"
)

func findVoyagerFile(dir string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if !file.IsDir() {
			currfileName := strings.ToLower(file.Name())
			if currfileName == "voyagerfile.yaml" || currfileName == "voyagerfile.yml" {
				return filepath.Join(dir, file.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("cant locate voyagerfile in directory: %s", dir)
}

func CurrentWorkspaceDefinition(voyagerFilePath string) (*v1alpha1.Workspace, error) {
	if len(voyagerFilePath) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		voyagerFilePath, err = findVoyagerFile(cwd)
		if err != nil {
			return nil, err
		}
	}
	if len(voyagerFilePath) == 0 {
		return nil, fmt.Errorf("voyager file missing")
	}
	_, err := os.Stat(voyagerFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat voyagerfile at %s: %w", voyagerFilePath, err)
	}

	if err := validation.Validate(voyagerFilePath); err != nil {
		return nil, fmt.Errorf("invalid voyagerfile: %w", err)
	}

	res := &v1alpha1.Workspace{}
	if err := tools.UnmarshalYamlFile(voyagerFilePath, res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal voyagerfile: %w", err)
	}
	return res, nil
}
