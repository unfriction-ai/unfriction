package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
		fail bool
	}{
		{"default", []string{"inspect", "../../fixtures/codex-acquisition.jsonl"}, "Acquisition a1", false},
		{"claude", []string{"inspect", "--format", "claude-code", "../../fixtures/claude-acquisition.jsonl"}, "Acquisition a1", false},
		{"unknown", []string{"inspect", "--format", "unknown", "does-not-exist"}, "unsupported format", true},
		{"help", []string{"inspect", "--help"}, "claude-code", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCommand("test")
			var b bytes.Buffer
			cmd.SetOut(&b)
			cmd.SetErr(&b)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if tc.fail {
				if err == nil || !strings.Contains(err.Error(), tc.want) || b.Len() != 0 {
					t.Fatalf("%v %q", err, b.String())
				}
			} else if err != nil || !strings.Contains(b.String(), tc.want) {
				t.Fatalf("%v %q", err, b.String())
			}
		})
	}
}
