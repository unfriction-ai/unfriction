// Package compatreport constructs offline, editable public issue drafts from
// bounded, allowlisted metadata. It performs no network requests.
package compatreport

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/unfriction-ai/unfriction/internal/diagnostic"
)

const NewIssueURL = "https://github.com/unfriction-ai/unfriction-releases/issues/new"
const MaxURLLength = 2048

var versionPattern = regexp.MustCompile(`^(dev|v?[0-9]{1,6}\.[0-9]{1,6}\.[0-9]{1,6}(-(alpha|beta|rc)(\.[0-9]{1,6})?)?)$`)

func SafeVersion(value string) string {
	if len(value) <= 48 && versionPattern.MatchString(value) {
		return value
	}
	return "unknown"
}

func agentName(family string) string {
	switch family {
	case "codex":
		return "Codex"
	case "claude-code":
		return "Claude Code"
	default:
		return "unknown"
	}
}

// Draft uses no free-text diagnostic data, even when Cause contains secrets.
func Draft(f *diagnostic.Failure, version, family, agentVersion string, explicit bool) string {
	if f == nil {
		return NewIssueURL
	}
	switch f.Category {
	case diagnostic.UnknownFormat, diagnostic.AmbiguousFormat, diagnostic.UndeterminedFormat:
		family = ""
		agentVersion = "unknown"
	case diagnostic.UnsupportedVariant, diagnostic.MalformedInput:
	default:
		return NewIssueURL
	}
	agent := agentName(family)
	selection := "automatic"
	if explicit {
		selection = "explicit override; producer unverified"
	}
	line := "unknown"
	if f.Line > 0 && f.Line <= 1_000_000_000 {
		line = strconv.Itoa(f.Line)
	}
	record, subtype := diagnostic.SafeName(f.Record.Type), diagnostic.SafeName(f.Record.Subtype)
	title := fmt.Sprintf("[trace compatibility] %s: %s", agent, f.Category)
	body := fmt.Sprintf("Unfriction version: %s\nAgent: %s\nAgent version: %s\nSelection: %s\nFailure category: %s\nRecord type: %s\nRecord subtype: %s\nSource line: %s\n\nSanitized diagnostic: %s (type=%s; subtype=%s).\n\nSteps to reproduce:\n[Please describe; remove private information.]\n\nSanitized minimal trace (optional):\n[Review and redact before attaching. No trace content was included automatically.]\n", SafeVersion(version), agent, SafeVersion(agentVersion), selection, f.Category, record, subtype, line, f.Category, record, subtype)
	query := url.Values{"title": {title}, "body": {body}}
	link := NewIssueURL + "?" + query.Encode()
	if len(link) > MaxURLLength {
		return NewIssueURL
	}
	return link
}

func Append(err error, version, family, agentVersion string, explicit bool) error {
	var failure *diagnostic.Failure
	if !errors.As(err, &failure) {
		return err
	}
	if failure.Category == diagnostic.UnknownFormat || failure.Category == diagnostic.AmbiguousFormat || failure.Category == diagnostic.UndeterminedFormat {
		family = ""
		agentVersion = "unknown"
	}
	return fmt.Errorf("%w\n\nUnfriction: %s\nAgent: %s\nAgent version: %s\n\nReport a compatibility issue (review before submitting):\n%s", err, SafeVersion(version), agentName(family), SafeVersion(agentVersion), Draft(failure, version, family, agentVersion, explicit))
}
