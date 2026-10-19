package codex

import (
	"os"
	"strings"
	"testing"
)

func TestPatchAction(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/codex-action.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 4 || r.Events[2].Target != "README.md" || r.Events[3].ActionID != r.Events[2].ActionID {
		t.Fatalf("%+v", r)
	}
}

func TestPatchGrammar(t *testing.T) {
	good := "*** Begin Patch\n*** Update File: README.md\n@@\n-Old\n+New\n*** End Patch"
	for _, tc := range []struct{ name, input string }{
		{"multi file", strings.Replace(good, "*** End Patch", "*** Update File: other.md\n@@\n-a\n+b\n*** End Patch", 1)},
		{"rename", strings.Replace(good, "@@", "*** Move to: other.md\n@@", 1)},
		{"add", strings.Replace(good, "Update File", "Add File", 1)},
		{"delete", strings.Replace(good, "Update File", "Delete File", 1)},
		{"no edits", strings.Replace(good, "-Old\n+New", " context", 1)},
		{"missing hunk", strings.Replace(good, "@@\n", "", 1)},
		{"malformed hunk", strings.Replace(good, "+New", "bad", 1)},
		{"missing end", strings.TrimSuffix(good, "*** End Patch")},
		{"empty target", strings.Replace(good, "README.md", "", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := patchTarget(tc.input); err == nil {
				t.Fatal("accepted unsupported patch")
			}
		})
	}
	if target, create, err := patchTarget(good); err != nil || create || target != "README.md" {
		t.Fatalf("%s %v", target, err)
	}
	for _, cmd := range []string{"sed -n '1,220p' README.md; pwd", "sed -i '1,220p' README.md", "sed -n '1,220p' README.md other.md", "sed -n '1,220p' $(pwd)", "sed -n '1,221p' README.md"} {
		if acquisitionTarget(cmd) != "" {
			t.Fatalf("accepted %s", cmd)
		}
	}
}

func TestCreationAndOneSidedHunks(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		create      bool
	}{
		{"create", "*** Begin Patch\n*** Add File: CREATED.md\n+new\n*** End Patch", true},
		{"delete lines", "*** Begin Patch\n*** Update File: README.md\n@@\n-old\n*** End Patch", false},
		{"insert lines", "*** Begin Patch\n*** Update File: README.md\n@@\n+new\n*** End Patch", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, create, err := patchTarget(tc.input)
			if err != nil || create != tc.create {
				t.Fatalf("create=%v err=%v", create, err)
			}
		})
	}
	for _, input := range []string{
		"*** Begin Patch\n*** Add File: CREATED.md\n*** End Patch",
		"*** Begin Patch\n*** Add File: CREATED.md\n-old\n*** End Patch",
		"*** Begin Patch\n*** Add File: CREATED.md\n+new\n*** Add File: OTHER.md\n+new\n*** End Patch",
		"*** Begin Patch\n*** Update File: README.md\n@@\n context\n@@\n+new\n*** End Patch",
	} {
		if _, _, err := patchTarget(input); err == nil {
			t.Fatalf("accepted unsupported patch: %s", input)
		}
	}
	if _, _, err := patchOutput("patch rejected by user\nunknown trailing record"); err == nil {
		t.Fatal("accepted unknown result envelope")
	}
}
