package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func FindVoyagerFile(dir string) (string, error) {
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
