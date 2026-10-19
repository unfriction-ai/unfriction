package compatreport

import (
	"io"
	"strings"
	"testing"
)

func TestVersionDiscovery(t *testing.T) {
	codex := `{"type":"session_meta","payload":{"cli_version":"0.163.0"}}`
	claude := `{"type":"assistant","sessionId":"private","message":{},"version":"2.1.302"}`
	for _, tc := range []struct {
		name, family, input, want string
		line                      int
	}{
		{"codex", "codex", codex, "0.163.0", 1},
		{"claude", "claude-code", claude, "2.1.302", 1},
		{"wrong family", "codex", claude, "unknown", 1},
		{"untrusted record", "claude-code", `{"type":"other","sessionId":"s","version":"1.2.3"}`, "unknown", 1},
		{"metadata after failure", "codex", "{}\n" + codex, "unknown", 1},
		{"conflicting", "codex", codex + "\n" + strings.Replace(codex, "0.163.0", "0.164.0", 1), "unknown", 2},
		{"malicious version", "codex", strings.Replace(codex, "0.163.0", "1.2.3-secret", 1), "unknown", 1},
		{"null version", "codex", `{"type":"session_meta","payload":{"cli_version":null}}`, "unknown", 1},
		{"record budget", "codex", strings.Repeat("{}\n", MaxMetadataRecords) + codex, "unknown", 0},
		{"byte budget", "codex", strings.Repeat(" ", MaxMetadataBytes) + codex, "unknown", 0},
		{"truncated metadata", "codex", strings.Repeat(" ", MaxMetadataBytes-10) + codex, "unknown", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := NewCapture(strings.NewReader(tc.input))
			replay, err := io.ReadAll(capture)
			if err != nil || string(replay) != tc.input {
				t.Fatal("changed parser input")
			}
			if len(capture.prefix) > MaxMetadataBytes {
				t.Fatal("unbounded capture")
			}
			if got := capture.AgentVersion(tc.family, tc.line); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

type countingReader struct {
	io.Reader
	reads int
}

func (r *countingReader) Read(p []byte) (int, error) { r.reads++; return r.Reader.Read(p) }
func TestVersionLookupNeverReadsInput(t *testing.T) {
	reader := &countingReader{Reader: strings.NewReader(`{"type":"session_meta","payload":{"cli_version":"1.2.3"}}`)}
	capture := NewCapture(reader)
	if capture.AgentVersion("codex", 1) != "unknown" || reader.reads != 0 {
		t.Fatal("lookup read ahead")
	}
}
