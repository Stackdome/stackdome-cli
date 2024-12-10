package tools

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Read the content of a file and return it as a string.

// ReadFile reads the content of a file and returns it as a string.
func ReadFile(filePath string) (string, error) {
	res, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func UnmarshalYamlFile[T any](filePath string, toType *T) error {
	res, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}
	err = yaml.Unmarshal(res, toType)
	if err != nil {
		return fmt.Errorf("error unmarshalling yaml: %v", err)
	}
	return nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
