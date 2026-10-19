package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

func editTarget(raw json.RawMessage) (string, error) {
	var input struct {
		Path       string  `json:"file_path"`
		Old        *string `json:"old_string"`
		New        *string `json:"new_string"`
		ReplaceAll *bool   `json:"replace_all"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return "", fmt.Errorf("unsupported Edit input; expected file_path, old_string, new_string, and replace_all=false")
	}
	if strings.TrimSpace(input.Path) == "" || input.Old == nil || *input.Old == "" || input.New == nil || *input.Old == *input.New || input.ReplaceAll == nil || *input.ReplaceAll {
		return "", fmt.Errorf("unsupported Edit; expected a nonempty target and changed replacement with replace_all=false")
	}
	return input.Path, nil
}
