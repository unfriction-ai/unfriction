package codex

import "testing"

func TestStructuredUnifiedDiffGrammar(t *testing.T) {
	for _, tc := range []struct {
		name, diff string
		valid      bool
	}{
		{"replacement", "@@ -1 +1 @@\n-old\n+new\n", true},
		{"context and changed lengths", "@@ -4,2 +4,3 @@\n keep\n-old\n+new\n+extra\n", true},
		{"insert", "@@ -0,0 +1,2 @@\n+one\n+two\n", true},
		{"remove", "@@ -1,2 +0,0 @@\n-one\n-two\n", true},
		{"multiple hunks", "@@ -1 +1 @@\n-old\n+new\n@@ -5 +5 @@\n-before\n+after", true},
		{"empty", "", false},
		{"not a diff", "changed", false},
		{"context only", "@@ -1 +1 @@\n unchanged\n", false},
		{"truncated", "@@ -1 +1 @@\n-old\n", false},
		{"excess lines", "@@ -1 +1 @@\n-old\n+new\n+extra\n", false},
		{"truncated first hunk", "@@ -1 +1 @@\n-old\n@@ -5 +5 @@\n-before\n+after\n", false},
		{"named header", "@@ -1 +1 @@ function\n-old\n+new\n", false},
		{"zero start with lines", "@@ -0 +1 @@\n-old\n+new\n", false},
		{"overflow", "@@ -1,9999999999999999999999 +1 @@\n-old\n+new\n", false},
		{"empty hunk", "@@ -0,0 +0,0 @@\n", false},
		{"unprefixed line", "@@ -1 +1 @@\nold\n+new\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validUnifiedDiff(tc.diff); got != tc.valid {
				t.Fatalf("valid=%t, want %t", got, tc.valid)
			}
		})
	}
}
