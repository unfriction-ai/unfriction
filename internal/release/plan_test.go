package release

import "testing"

func TestVersionDecisions(t *testing.T) {
	for _, tc := range []struct {
		name, previous string
		commits        []string
		want           string
	}{
		{"first feature", "", []string{"feat: inspect runs"}, "0.1.0"},
		{"first fix", "", []string{"fix: correct output"}, "0.1.0"},
		{"first breaking", "", []string{"feat!: replace input format"}, "0.1.0"},
		{"no initial release", "", []string{"docs: explain usage", "Create README.md"}, ""},
		{"patch", "v0.1.0", []string{"fix(parser): handle blank lines"}, "0.1.1"},
		{"minor", "v0.1.1", []string{"feat: add output", "fix: repair"}, "0.2.0"},
		{"breaking header", "v0.2.1", []string{"feat(cli)!: replace flags"}, "1.0.0"},
		{"breaking footer", "v0.2.1", []string{"fix: update input\n\nBREAKING CHANGE: old format removed"}, "1.0.0"},
		{"breaking hyphen footer", "v1.2.3", []string{"refactor: change API\n\nBREAKING-CHANGE: old API removed"}, "2.0.0"},
		{"no release", "v0.1.0", []string{"chore: update CI", "test: add regression", "docs: usage"}, ""},
		{"rerun", "v0.1.0", nil, ""},
		{"body is not header", "v0.1.0", []string{"docs: usage\n\nfeat: example"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Next(tc.previous, tc.commits)
			if err != nil || got.Version != tc.want {
				t.Fatalf("got %+v, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestRejectInvalidPreviousVersion(t *testing.T) {
	for _, v := range []string{"v01.2.3", "v1.2", "v1.2.3-beta.1", "garbage"} {
		if _, err := Next(v, []string{"fix: bug"}); err == nil {
			t.Fatalf("accepted %q", v)
		}
	}
}
