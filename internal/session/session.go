// Package session defines vendor-independent selection metadata. Vendor adapters
// own the evidence rules that populate it; it does not interpret recorded tools.
package session

import (
	"fmt"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/source"
)

type Turn struct {
	ID             string
	Prompt         string
	Final          string
	PreviousTurnID string
	PreviousAgent  string
	EarlierUser    string
	Timestamp      string
	Start          int
	End            int
	Completed      bool
	Aborted        bool
}

type Document struct {
	ID        string
	Family    string
	Version   string
	CWD       string
	Branch    string
	Timestamp string
	Turns     []Turn
	Records   []source.Record
}

func (d Document) Find(id string) (Turn, int, error) {
	for i, t := range d.Turns {
		if t.ID == id {
			return t, i, nil
		}
	}
	return Turn{}, 0, fmt.Errorf("turn %q not found; refresh or reselect the recording", id)
}

// Identity rejects a family mismatch rather than retrying another adapter.
func Identity(selector, family string) (string, error) {
	f, id, ok := strings.Cut(selector, ":")
	if !ok || f != family || strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("invalid turn selector; expected %s:<recorded-identity>", family)
	}
	return id, nil
}
