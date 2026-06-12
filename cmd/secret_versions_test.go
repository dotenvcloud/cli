package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dotenvcloud/cli/internal/ui"
)

func TestParseSecretPath(t *testing.T) {
	cases := []struct {
		name                             string
		arg                              string
		wantProject, wantTarget, wantEnv string
		wantErr                          bool
	}{
		{name: "project only", arg: "myapp", wantProject: "myapp"},
		{name: "project and target", arg: "myapp/prod", wantProject: "myapp", wantTarget: "prod"},
		{name: "full path", arg: "myapp/prod/web", wantProject: "myapp", wantTarget: "prod", wantEnv: "web"},
		{name: "too many segments", arg: "a/b/c/d", wantErr: true},
		{name: "empty segment", arg: "myapp//web", wantErr: true},
		{name: "trailing slash", arg: "myapp/", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project, target, env, err := parseSecretPath(tc.arg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got none", tc.arg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if project != tc.wantProject || target != tc.wantTarget || env != tc.wantEnv {
				t.Errorf("got (%q,%q,%q), want (%q,%q,%q)", project, target, env, tc.wantProject, tc.wantTarget, tc.wantEnv)
			}
		})
	}
}

// TestPrintSecretDiff pins the corrected from->to direction: additions belong to
// the `to` (newer) side, removals to the `from` (older) side. Under the
// prior-state version model the `to` side is the LIVE current secret, so an
// added key must render as `+`, not `-`.
func TestPrintSecretDiff(t *testing.T) {
	capture := func(showValues bool, from, to map[string]string) string {
		orig := ui.Stdout
		var buf bytes.Buffer
		ui.Stdout = &buf
		defer func() { ui.Stdout = orig }()
		printSecretDiff("v1", "current", from, to, showValues)
		return buf.String()
	}

	t.Run("added key shows as + with the to value", func(t *testing.T) {
		out := capture(true,
			map[string]string{"A": "1"},
			map[string]string{"A": "1", "BLAH": "121"},
		)
		if !strings.Contains(out, "+ BLAH=121") {
			t.Errorf("expected '+ BLAH=121' in:\n%s", out)
		}
		if strings.Contains(out, "- ") {
			t.Errorf("did not expect a removal in:\n%s", out)
		}
	})

	t.Run("removed key shows as - with the from value", func(t *testing.T) {
		out := capture(true,
			map[string]string{"A": "1", "GONE": "9"},
			map[string]string{"A": "1"},
		)
		if !strings.Contains(out, "- GONE=9") {
			t.Errorf("expected '- GONE=9' in:\n%s", out)
		}
	})

	t.Run("changed value shows from -> to", func(t *testing.T) {
		out := capture(true,
			map[string]string{"A": "old"},
			map[string]string{"A": "new"},
		)
		if !strings.Contains(out, "~ A: old -> new") {
			t.Errorf("expected '~ A: old -> new' in:\n%s", out)
		}
	})

	t.Run("identical maps report no differences", func(t *testing.T) {
		out := capture(true,
			map[string]string{"A": "1"},
			map[string]string{"A": "1"},
		)
		if !strings.Contains(out, "No differences") {
			t.Errorf("expected 'No differences' in:\n%s", out)
		}
	})

	t.Run("empty to side (no live content) shows every key as removed", func(t *testing.T) {
		out := capture(true,
			map[string]string{"A": "1", "B": "2"},
			map[string]string{},
		)
		if !strings.Contains(out, "- A=1") || !strings.Contains(out, "- B=2") {
			t.Errorf("expected '- A=1' and '- B=2' in:\n%s", out)
		}
		if strings.Contains(out, "+ ") {
			t.Errorf("did not expect additions in:\n%s", out)
		}
	})

	t.Run("masks values unless show-values is set", func(t *testing.T) {
		out := capture(false,
			map[string]string{},
			map[string]string{"SECRET": "topsecret"},
		)
		if strings.Contains(out, "topsecret") {
			t.Errorf("masked diff leaked the value:\n%s", out)
		}
		if !strings.Contains(out, "+ SECRET=********") {
			t.Errorf("expected masked '+ SECRET=********' in:\n%s", out)
		}
	})
}
