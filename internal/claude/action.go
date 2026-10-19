package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func editTarget(raw json.RawMessage) (string, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return "", fmt.Errorf("invalid Edit input")
	}
	if _, ok := fields["offset"]; ok {
		return offsetEditTarget(raw)
	}
	if _, ok := fields["new_text"]; ok {
		return offsetEditTarget(raw)
	}
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

// The recording establishes an attempted edit with this input shape, followed by
// a tool-disabled error. It does not establish offset units or successful editing.
func offsetEditTarget(raw json.RawMessage) (string, error) {
	var input struct {
		Path    string  `json:"file_path"`
		Offset  *string `json:"offset"`
		NewText *string `json:"new_text"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return "", fmt.Errorf("unsupported offset Edit input; expected file_path, decimal-string offset, and new_text")
	}
	if strings.TrimSpace(input.Path) == "" || input.Offset == nil || input.NewText == nil || *input.NewText == "" {
		return "", fmt.Errorf("incomplete offset Edit input")
	}
	if _, err := strconv.ParseUint(*input.Offset, 10, 64); err != nil {
		return "", fmt.Errorf("offset Edit requires a nonnegative decimal-string offset")
	}
	return input.Path, nil
}
