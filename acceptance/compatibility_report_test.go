package acceptance_test

import (
	"bytes"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicCompatibilityDraft(t *testing.T) {
	binary := buildBinary(t, "-ldflags", "-X main.version=1.2.3")
	for _, tc := range []struct{ name, format, input, diagnostic, agent, version, record string }{
		{"codex", "codex", `{"type":"session_meta","payload":{"cli_version":"0.163.0"}}` + "\n" + `{"type":"world_state","payload":{"full":true,"state":null}}`, "unsupported world_state shape", "Codex", "0.163.0", "world_state"},
		{"codex auto", "", `{"type":"session_meta","payload":{"cli_version":"0.163.0"}}` + "\n" + `{"type":"event_msg","payload":{"type":"task_started","turn_id":"private-turn"}}` + "\n" + `{"type":"world_state","payload":{"full":true,"state":null}}`, "unsupported world_state shape", "Codex", "0.163.0", "world_state"},
		{"claude", "claude-code", `{"type":"attachment","sessionId":"private-session","uuid":"private-id","version":"2.1.302","attachment":{"type":"future_hook"}}`, "unsupported attachment shape", "Claude Code", "2.1.302", "attachment"},
		{"unknown", "", `{"type":"private-secret","prompt":"private-prompt","path":"/Users/private/home","token":"secret-token"}`, "unknown trace format", "unknown", "unknown", "unavailable"},
		{"missing version", "codex", `{"type":"response_item","payload":{"type":"private-subtype"}}` + "\n" + `{"type":"session_meta","payload":{"cli_version":"9.9.9"}}`, "unsupported event", "Codex", "unknown", "response_item"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trace.jsonl")
			if err := os.WriteFile(path, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"inspect", path}
			if tc.format != "" {
				args = append(args, "--format", tc.format)
			}
			cmd := exec.Command(binary, args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.diagnostic) {
				t.Fatalf("%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
			var link string
			for _, line := range strings.Split(stderr.String(), "\n") {
				if strings.HasPrefix(line, "https://github.com/") {
					link = line
				}
			}
			parsed, err := url.Parse(link)
			if err != nil || parsed.Host != "github.com" || parsed.Path != "/unfriction-ai/unfriction-releases/issues/new" {
				t.Fatalf("missing public draft URL: %q (%v)", link, err)
			}
			body := parsed.Query().Get("body")
			for _, want := range []string{"Unfriction version: 1.2.3", "Agent: " + tc.agent, "Agent version: " + tc.version, tc.record, "Sanitized minimal trace"} {
				if !strings.Contains(body, want) {
					t.Fatalf("missing %q in %s", want, body)
				}
			}
			if tc.format != "" && !strings.Contains(body, "producer unverified") {
				t.Fatal("explicit override presented as producer evidence")
			}
			if tc.name == "codex auto" && (!strings.Contains(body, "Source line: 3") || !strings.Contains(body, "Selection: automatic")) {
				t.Fatal(body)
			}
			if tc.name == "codex" && !strings.Contains(body, "Source line: 2") {
				t.Fatal(body)
			}
			decoded := parsed.Query().Get("title") + body
			for _, private := range []string{"private-session", "private-id", "private-secret", "private-prompt", "/Users/private/home", "secret-token", "private-subtype", path, "Closes #"} {
				if strings.Contains(decoded, private) {
					t.Fatalf("private value in public draft: %q", private)
				}
			}
			if len(link) > 2048 {
				t.Fatalf("oversized URL: %d", len(link))
			}
		})
	}
}

func TestNoDraftForSuccessOrFileAccessError(t *testing.T) {
	binary := buildBinary(t)
	for _, path := range []string{"../fixtures/first-run.jsonl", filepath.Join(t.TempDir(), "missing.jsonl"), t.TempDir()} {
		output, _ := exec.Command(binary, "inspect", path).CombinedOutput()
		if strings.Contains(string(output), "issues/new") {
			t.Fatalf("unexpected draft for %s: %s", path, output)
		}
	}
}
