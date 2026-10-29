// Package source retains physical recording locations across session selection.
package source

import (
	"bufio"
	"io"
)

type Record struct {
	Line int
	Data []byte
}

// Cursor is shared by whole-file and selected-record normalization. Selection
// passes original records; it never rewrites JSON or invents physical lines.
type Cursor struct {
	scanner *bufio.Scanner
	records []Record
	index   int
	current Record
}

func Reader(r io.Reader) *Cursor {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 4*1024*1024)
	return &Cursor{scanner: s}
}

func Records(records []Record) *Cursor { return &Cursor{records: records} }
func (c *Cursor) Scan() bool {
	if c.scanner != nil {
		if !c.scanner.Scan() {
			return false
		}
		c.current = Record{Line: c.current.Line + 1, Data: c.scanner.Bytes()}
		return true
	}
	if c.index >= len(c.records) {
		return false
	}
	c.current = c.records[c.index]
	c.index++
	return true
}
func (c *Cursor) Bytes() []byte { return c.current.Data }
func (c *Cursor) Text() string  { return string(c.current.Data) }
func (c *Cursor) Line() int     { return c.current.Line }
func (c *Cursor) Err() error {
	if c.scanner != nil {
		return c.scanner.Err()
	}
	return nil
}

func ReadAll(r io.Reader) ([]Record, error) {
	c := Reader(r)
	var records []Record
	for c.Scan() {
		records = append(records, Record{Line: c.Line(), Data: append([]byte(nil), c.Bytes()...)})
	}
	return records, c.Err()
}
