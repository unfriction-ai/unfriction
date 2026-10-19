package compatreport

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/unfriction-ai/unfriction/internal/diagnostic"
)

func TestDraftContainsOnlySafeMetadata(t *testing.T) {
	secret := "/Users/private/project secret-token session-id prompt output\n&body=injected#fragment"
	failure := &diagnostic.Failure{Category: diagnostic.UnsupportedVariant, Line: 7, Record: diagnostic.Record{Type: "world_state", Subtype: secret}, Cause: errors.New(secret)}
	link := Draft(failure, "1.2.3", "codex", "0.163.0", true)
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.Path != "/unfriction-ai/unfriction-releases/issues/new" || parsed.Fragment != "" || len(parsed.Query()) != 2 {
		t.Fatal(link)
	}
	body := parsed.Query().Get("body")
	for _, want := range []string{"Unfriction version: 1.2.3", "Agent: Codex", "Agent version: 0.163.0", "Source line: 7", "Record type: world_state", "Record subtype: redacted", "producer unverified"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, private := range []string{"/Users/", "secret-token", "session-id", "injected", "fragment"} {
		if strings.Contains(body, private) || strings.Contains(parsed.Query().Get("title"), private) {
			t.Fatal("leaked private input")
		}
	}
	if len(link) > MaxURLLength {
		t.Fatal("oversized URL")
	}
	local := Append(fmt.Errorf("parse trace: %w", failure), "1.2.3", "codex", "0.163.0", true)
	if !strings.HasPrefix(local.Error(), "parse trace: "+secret+"\n\n") || !errors.Is(local, failure.Cause) {
		t.Fatal("changed local diagnostic or lost error chain")
	}
}

func TestUnsafeFieldsAndFallback(t *testing.T) {
	for _, value := range []string{"", "1.2.3-secret-token", "1.2.3+private", "/private/version", "v1\n2.3", strings.Repeat("9", 4096)} {
		if SafeVersion(value) != "unknown" {
			t.Fatalf("unsafe version accepted")
		}
		f := &diagnostic.Failure{Category: diagnostic.MalformedInput, Record: diagnostic.Record{Type: value, Subtype: value}, Line: -1, Cause: errors.New(value)}
		u, _ := url.Parse(Draft(f, value, value, value, false))
		body := u.Query().Get("body")
		if !strings.Contains(body, "Agent: unknown") || !strings.Contains(body, "Source line: unknown") || !strings.Contains(body, "Agent version: unknown") {
			t.Fatal(body)
		}
	}
	for _, value := range []string{"dev", "1.2.3", "v1.2.3", "0.142.0-alpha.1", "1.2.3-rc.2"} {
		if SafeVersion(value) != value {
			t.Fatal(value)
		}
	}
	if Draft(nil, "", "", "", false) != NewIssueURL || Draft(&diagnostic.Failure{Category: diagnostic.Category("private")}, "", "", "", false) != NewIssueURL {
		t.Fatal("unsafe prefill did not fall back")
	}
	for _, category := range []diagnostic.Category{diagnostic.UnknownFormat, diagnostic.AmbiguousFormat, diagnostic.UndeterminedFormat} {
		f := &diagnostic.Failure{Category: category, Cause: errors.New("local")}
		u, _ := url.Parse(Draft(f, "dev", "codex", "1.2.3", false))
		if !strings.Contains(u.Query().Get("body"), "Agent: unknown\nAgent version: unknown") {
			t.Fatal(u)
		}
	}
	err := errors.New("open file failed")
	if Append(err, "dev", "codex", "unknown", false) != err {
		t.Fatal("ordinary error changed")
	}
}
