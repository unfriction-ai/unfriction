package acceptance_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecognizedContextRecords(t *testing.T) {
	binary := buildBinary(t)
	for _, tc := range []struct{ fixture, notice string }{
		{"world-state", "Environment state recorded; supplied context is not reconstructed."},
		{"compaction", "Opaque context compaction recorded; retained context is unknown."},
		{"last-prompt", ""},
		{"session-hook", "Session-start hook recorded; output may supply context and is not interpreted."},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			output, err := exec.Command(binary, "inspect", filepath.Join("..", "fixtures", "context-records", tc.fixture+".jsonl")).CombinedOutput()
			format := "codex"
			if tc.fixture == "last-prompt" || tc.fixture == "session-hook" {
				format = "claude-code"
			}
			want := string(acquisitionReport(t, format, 1))
			if tc.notice != "" {
				want = strings.Replace(want, "Final response:", "Context coverage:\n- Source line 1: "+tc.notice+"\n\nFinal response:", 1)
			}
			if err != nil || !bytes.Equal(output, []byte(want)) {
				t.Fatalf("%v\ngot:\n%s\nwant:\n%s", err, output, want)
			}
			if strings.Contains(string(output), "redacted") || strings.Count(string(output), "Acquisition a1") != 1 || strings.Count(string(output), "Observation for a1") != 1 {
				t.Fatal("context content leaked or was misattributed")
			}
		})
	}
}

func TestUnsupportedContextVariantHasNoReport(t *testing.T) {
	binary := buildBinary(t)
	data, err := os.ReadFile("../fixtures/context-records/session-hook.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	input := strings.Replace(string(data), `"hookEvent": "SessionStart"`, `"hookEvent": "private-sentinel"`, 1)
	path := filepath.Join(t.TempDir(), "unsupported.jsonl")
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"inspect", path}, {"inspect", "--format", "claude-code", path}} {
		cmd := exec.Command(binary, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported attachment variant") || !strings.Contains(stderr.String(), "line 1") || strings.Contains(stderr.String(), "private-sentinel") {
			t.Fatalf("%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
		}
	}
}
