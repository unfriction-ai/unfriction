package acceptance_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutomaticFormatSelection(t *testing.T) {
	binary := buildBinary(t)
	for _, fixture := range []string{"codex-acquisition.jsonl", "claude-acquisition.jsonl"} {
		t.Run(fixture, func(t *testing.T) {
			format := "codex"
			if strings.HasPrefix(fixture, "claude") {
				format = "claude-code"
			}
			want := acquisitionReport(t, format, 0)
			output, err := exec.Command(binary, "inspect", filepath.Join("..", "fixtures", fixture)).CombinedOutput()
			if err != nil || !bytes.Equal(output, want) {
				t.Fatalf("%v\n%s", err, output)
			}
		})
	}
}

func TestDetectionFailures(t *testing.T) {
	binary := buildBinary(t)
	codex := `{"type":"event_msg","payload":{"type":"future_event"}}`
	claude := `{"type":"assistant","sessionId":"s","message":{}}`
	for _, tc := range []struct{ name, input, want string }{
		{"unknown", `{"type":"message","text":"Codex Claude Code"}`, "unknown trace format"},
		{"ambiguous", codex + "\n" + claude, "ambiguous trace format"},
		{"recognized but invalid", codex, "unsupported lifecycle event"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trace.jsonl")
			if err := os.WriteFile(path, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "inspect", path)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
			if tc.name != "recognized but invalid" && !strings.Contains(stderr.String(), "--format") {
				t.Fatal(stderr.String())
			}
		})
	}
}

func TestExplicitFormatBypassesExhaustedPrefix(t *testing.T) {
	binary := buildBinary(t)
	for _, tc := range []struct{ format, fixture, metadata string }{
		{"codex", "codex-acquisition.jsonl", `{"type":"session_meta","payload":{}}`},
		{"claude-code", "claude-acquisition.jsonl", `{"type":"file-history-snapshot"}`},
	} {
		t.Run(tc.format, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("..", "fixtures", tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "trace.jsonl")
			if err := os.WriteFile(path, append([]byte(strings.Repeat(tc.metadata+"\n", 64)), input...), 0600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command(binary, "inspect", path).CombinedOutput()
			if err == nil || !strings.Contains(string(output), "prefix limit") || !strings.Contains(string(output), "--format") {
				t.Fatalf("%v %s", err, output)
			}
			output, err = exec.Command(binary, "inspect", "--format", tc.format, path).CombinedOutput()
			want := acquisitionReport(t, tc.format, 64)
			if err != nil || !bytes.Equal(output, want) {
				t.Fatalf("%v %s", err, output)
			}
		})
	}
}
