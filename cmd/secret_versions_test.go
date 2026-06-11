package cmd

import "testing"

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
