package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
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

func UserWorkspace(voyagerFilePath string) (*userworkspace.Workspace, error) {
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

	if err := userworkspace.Validate(voyagerFilePath); err != nil {
		return nil, fmt.Errorf("invalid voyagerfile: %w", err)
	}
	return userworkspace.Unmarshal(voyagerFilePath)
}
