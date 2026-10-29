// Package inspection owns shared format selection, validation, and diagnostics.
package inspection

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/unfriction-ai/unfriction/internal/claude"
	"github.com/unfriction-ai/unfriction/internal/codex"
	"github.com/unfriction-ai/unfriction/internal/compatreport"
	"github.com/unfriction-ai/unfriction/internal/diagnostic"
	"github.com/unfriction-ai/unfriction/internal/run"
	"github.com/unfriction-ai/unfriction/internal/session"
	"github.com/unfriction-ai/unfriction/internal/traceformat"
)

func Read(path, format, selector, version string) (run.Run, error) {
	if format != "auto" && format != "codex" && format != "claude-code" {
		return run.Run{}, fmt.Errorf("unsupported format %q; expected auto, codex, or claude-code", format)
	}
	file, err := os.Open(path)
	if err != nil {
		return run.Run{}, fmt.Errorf("open trace: %w", err)
	}
	defer file.Close()
	var reader io.Reader = file
	var snapshot []byte
	var before os.FileInfo
	if selector != "" {
		before, err = file.Stat()
		if err != nil {
			return run.Run{}, err
		}
		snapshot, err = io.ReadAll(file)
		if err != nil {
			return run.Run{}, err
		}
		reader = bytes.NewReader(snapshot)
	}
	capture := compatreport.NewCapture(reader)
	selected := format
	var input io.Reader = capture
	if selected == "auto" {
		selected, input, err = traceformat.Detect(capture)
		if err != nil {
			return run.Run{}, compatreport.Append(err, version, "", "unknown", false)
		}
	}
	var r run.Run
	if selector == "" {
		if selected == "claude-code" {
			r, err = claude.Parse(input)
		} else {
			r, err = codex.Parse(input)
		}
	} else {
		var id string
		id, err = session.Identity(selector, selected)
		if err != nil {
			return run.Run{}, err
		}
		var d session.Document
		var index int
		if selected == "claude-code" {
			r, d, index, err = claude.ParseTurn(input, id)
		} else {
			r, d, index, err = codex.ParseTurn(input, id)
		}
		if err == nil {
			if err := verifySnapshot(path, before, snapshot); err != nil {
				return run.Run{}, err
			}
			t := d.Turns[index]
			r.Selection = &run.Selection{Path: path, Session: d.ID, Turn: t.ID, Position: index + 1, Total: len(d.Turns), PreviousAgent: t.PreviousAgent, EarlierUser: t.EarlierUser}
		}
	}
	if err != nil {
		agentVersion := "unknown"
		var failure *diagnostic.Failure
		if errors.As(err, &failure) {
			agentVersion = capture.AgentVersion(selected, failure.Line)
		}
		return run.Run{}, compatreport.Append(fmt.Errorf("parse trace: %w", err), version, selected, agentVersion, format != "auto")
	}
	return r, nil
}

// verifySnapshot checks identity as well as content: timestamps and size alone
// cannot detect a replacement or an in-place edit of equal length.
func verifySnapshot(path string, before os.FileInfo, snapshot []byte) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("recording removed or unreadable; refresh and reselect")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !os.SameFile(before, info) {
		return fmt.Errorf("recording replaced; refresh and reselect")
	}
	current, err := io.ReadAll(f)
	if err != nil || !bytes.Equal(snapshot, current) {
		return fmt.Errorf("recording changed during inspection; refresh and reselect")
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(info, after) {
		return fmt.Errorf("recording replaced during inspection; refresh and reselect")
	}
	return nil
}
