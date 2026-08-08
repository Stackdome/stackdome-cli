package stackfile

import (
	"encoding/json"
	"os"

	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
)

func LoadJSON(path string) (*openapi.Stack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, clierrors.Newf("Stack file not found: %s", path)
		}
		return nil, clierrors.Wrapf(err, "Failed to read stack file: %s", path)
	}

	var stack openapi.Stack
	if err := json.Unmarshal(data, &stack); err != nil {
		return nil, clierrors.Wrapf(err, "Failed to parse stack JSON: %s", path)
	}

	if stack.Name == "" {
		return nil, clierrors.ValidationError("Stack JSON missing required field: name")
	}

	return &stack, nil
}
