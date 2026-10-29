package inspection

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotRejectsChangedRecording(t *testing.T) {
	for _, kind := range []string{"unchanged", "append", "same-size edit", "truncate", "replace", "remove"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.jsonl")
			original := []byte("original\n")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "append":
				err = os.WriteFile(path, append(append([]byte{}, original...), []byte("new\n")...), 0600)
			case "same-size edit":
				err = os.WriteFile(path, []byte("modified\n"), 0600)
			case "truncate":
				err = os.WriteFile(path, nil, 0600)
			case "replace":
				other := path + ".new"
				if err = os.WriteFile(other, original, 0600); err == nil {
					err = os.Rename(other, path)
				}
			case "remove":
				err = os.Remove(path)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = verifySnapshot(path, before, original)
			if (err == nil) != (kind == "unchanged") {
				t.Fatalf("%s: %v", kind, err)
			}
		})
	}
}
