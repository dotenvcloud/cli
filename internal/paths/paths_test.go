package paths

import (
	"testing"
)

func TestParseResourcePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    *ResourcePath
		wantErr bool
	}{
		{
			name: "project only",
			path: "myproject",
			want: &ResourcePath{Project: "myproject"},
		},
		{
			name: "project and target",
			path: "myproject/production",
			want: &ResourcePath{Project: "myproject", Target: "production"},
		},
		{
			name: "full path",
			path: "myproject/production/web",
			want: &ResourcePath{Project: "myproject", Target: "production", Environment: "web"},
		},
		{
			name: "with leading slash",
			path: "/myproject/production",
			want: &ResourcePath{Project: "myproject", Target: "production"},
		},
		{
			name: "with trailing slash",
			path: "myproject/production/",
			want: &ResourcePath{Project: "myproject", Target: "production"},
		},
		{
			name: "with spaces",
			path: " myproject/production ",
			want: &ResourcePath{Project: "myproject", Target: "production"},
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "only slashes",
			path:    "///",
			wantErr: true,
		},
		{
			name:    "too many parts",
			path:    "a/b/c/d",
			wantErr: true,
		},
		{
			name: "leading slash path",
			path: "/target/env",
			want: &ResourcePath{Project: "target", Target: "env"},
		},
		{
			name:    "empty target",
			path:    "project//env",
			wantErr: true,
		},
		{
			name: "empty environment",
			path: "project/target/",
			want: &ResourcePath{Project: "project", Target: "target"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseResourcePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseResourcePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equals(tt.want) {
				t.Errorf("ParseResourcePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatResourcePath(t *testing.T) {
	tests := []struct {
		name        string
		project     string
		target      string
		environment string
		want        string
	}{
		{
			name:    "project only",
			project: "myproject",
			want:    "myproject",
		},
		{
			name:    "project and target",
			project: "myproject",
			target:  "production",
			want:    "myproject/production",
		},
		{
			name:        "full path",
			project:     "myproject",
			target:      "production",
			environment: "web",
			want:        "myproject/production/web",
		},
		{
			name:        "empty components ignored",
			project:     "myproject",
			target:      "",
			environment: "web",
			want:        "myproject/web",
		},
		{
			name: "all empty",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatResourcePath(tt.project, tt.target, tt.environment); got != tt.want {
				t.Errorf("FormatResourcePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourcePath_Level(t *testing.T) {
	tests := []struct {
		name string
		rp   *ResourcePath
		want int
	}{
		{
			name: "nil path",
			rp:   nil,
			want: 0,
		},
		{
			name: "empty path",
			rp:   &ResourcePath{},
			want: 0,
		},
		{
			name: "project only",
			rp:   &ResourcePath{Project: "p"},
			want: 1,
		},
		{
			name: "project and target",
			rp:   &ResourcePath{Project: "p", Target: "t"},
			want: 2,
		},
		{
			name: "full path",
			rp:   &ResourcePath{Project: "p", Target: "t", Environment: "e"},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rp.Level(); got != tt.want {
				t.Errorf("ResourcePath.Level() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourcePath_Parent(t *testing.T) {
	tests := []struct {
		name string
		rp   *ResourcePath
		want *ResourcePath
	}{
		{
			name: "nil path",
			rp:   nil,
			want: nil,
		},
		{
			name: "project only",
			rp:   &ResourcePath{Project: "p"},
			want: nil,
		},
		{
			name: "project and target",
			rp:   &ResourcePath{Project: "p", Target: "t"},
			want: &ResourcePath{Project: "p"},
		},
		{
			name: "full path",
			rp:   &ResourcePath{Project: "p", Target: "t", Environment: "e"},
			want: &ResourcePath{Project: "p", Target: "t"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rp.Parent()
			if (got == nil) != (tt.want == nil) {
				t.Errorf("ResourcePath.Parent() = %v, want %v", got, tt.want)
				return
			}
			if got != nil && !got.Equals(tt.want) {
				t.Errorf("ResourcePath.Parent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourcePath_Contains(t *testing.T) {
	tests := []struct {
		name  string
		rp    *ResourcePath
		other *ResourcePath
		want  bool
	}{
		{
			name:  "nil paths",
			rp:    nil,
			other: nil,
			want:  false,
		},
		{
			name:  "full contains project",
			rp:    &ResourcePath{Project: "p", Target: "t", Environment: "e"},
			other: &ResourcePath{Project: "p"},
			want:  true,
		},
		{
			name:  "full contains project/target",
			rp:    &ResourcePath{Project: "p", Target: "t", Environment: "e"},
			other: &ResourcePath{Project: "p", Target: "t"},
			want:  true,
		},
		{
			name:  "full contains self",
			rp:    &ResourcePath{Project: "p", Target: "t", Environment: "e"},
			other: &ResourcePath{Project: "p", Target: "t", Environment: "e"},
			want:  true,
		},
		{
			name:  "project doesn't contain target",
			rp:    &ResourcePath{Project: "p"},
			other: &ResourcePath{Project: "p", Target: "t"},
			want:  false,
		},
		{
			name:  "different projects",
			rp:    &ResourcePath{Project: "p1", Target: "t", Environment: "e"},
			other: &ResourcePath{Project: "p2"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rp.Contains(tt.other); got != tt.want {
				t.Errorf("ResourcePath.Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourcePath_MatchesPattern(t *testing.T) {
	tests := []struct {
		name    string
		rp      *ResourcePath
		pattern string
		want    bool
	}{
		{
			name:    "nil path",
			rp:      nil,
			pattern: "test",
			want:    false,
		},
		{
			name:    "empty pattern",
			rp:      &ResourcePath{Project: "myproject"},
			pattern: "",
			want:    false,
		},
		{
			name:    "exact match project",
			rp:      &ResourcePath{Project: "myproject"},
			pattern: "myproject",
			want:    true,
		},
		{
			name:    "substring match",
			rp:      &ResourcePath{Project: "myproject"},
			pattern: "proj",
			want:    true,
		},
		{
			name:    "case insensitive",
			rp:      &ResourcePath{Project: "MyProject"},
			pattern: "myproj",
			want:    true,
		},
		{
			name:    "match in target",
			rp:      &ResourcePath{Project: "app", Target: "production"},
			pattern: "prod",
			want:    true,
		},
		{
			name:    "match in environment",
			rp:      &ResourcePath{Project: "app", Target: "staging", Environment: "web"},
			pattern: "web",
			want:    true,
		},
		{
			name:    "match in full path",
			rp:      &ResourcePath{Project: "app", Target: "prod", Environment: "api"},
			pattern: "prod/api",
			want:    true,
		},
		{
			name:    "no match",
			rp:      &ResourcePath{Project: "app", Target: "staging", Environment: "web"},
			pattern: "production",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rp.MatchesPattern(tt.pattern); got != tt.want {
				t.Errorf("ResourcePath.MatchesPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}
