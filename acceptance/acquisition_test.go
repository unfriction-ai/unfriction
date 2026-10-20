package acceptance_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportCodexAcquisition(t *testing.T) {
	checkAcquisition(t, "codex", "codex-acquisition.jsonl")
}

func checkAcquisition(t *testing.T, format, fixture string) {
	t.Helper()
	binary := buildBinary(t)
	want := acquisitionReport(t, format, 0)
	for range 2 {
		output, err := exec.Command(binary, "inspect", "--format", format, filepath.Join("..", "fixtures", fixture)).CombinedOutput()
		if err != nil || string(output) != string(want) {
			t.Fatalf("inspect %s: %v\ngot:\n%s\nwant:\n%s", format, err, output, want)
		}
	}
}

func TestImportClaudeAcquisition(t *testing.T) {
	checkAcquisition(t, "claude-code", "claude-acquisition.jsonl")
}

func TestInvalidAcquisitionProducesDiagnosticWithoutReport(t *testing.T) {
	binary := buildBinary(t)
	for _, format := range []string{"codex", "claude-code"} {
		t.Run(format, func(t *testing.T) {
			fixture := "codex-acquisition.jsonl"
			if format == "claude-code" {
				fixture = "claude-acquisition.jsonl"
			}
			input, err := os.ReadFile(filepath.Join("..", "fixtures", fixture))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(input)), "\n")
			requestLine := 2
			if format == "claude-code" {
				requestLine = 1
			}
			input = []byte(strings.Join(append(lines[:requestLine], lines[requestLine+1:]...), "\n"))
			path := filepath.Join(t.TempDir(), "orphan.jsonl")
			if err := os.WriteFile(path, input, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "inspect", "--format", format, path)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unmatched observation") || !strings.Contains(stderr.String(), "line ") {
				t.Fatalf("error=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
		})
	}
}

// Prefix metadata changes provenance line numbers without changing acquisitions.
func acquisitionReport(t *testing.T, format string, prefixLines int) []byte {
	t.Helper()
	fixture, line := "acquisition.txt", 4
	if format == "claude-code" {
		fixture, line = "claude-acquisition.txt", 3
	}
	data, err := os.ReadFile(filepath.Join("..", "fixtures", fixture))
	if err != nil {
		t.Fatal(err)
	}
	return []byte(strings.ReplaceAll(string(data), fmt.Sprintf("(source line %d)", line), fmt.Sprintf("(source line %d)", line+prefixLines)))
}
