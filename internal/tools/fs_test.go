package tools

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "main.ts", false},
		{"**/*.go", "internal/tools/fs.go", true},
		{"**/*.go", "fs.go", true},
		{"internal/**/*.go", "internal/tools/fs.go", true},
		{"internal/**/*.go", "cmd/goppi/main.go", false},
		{"**/*", "a/b/c", true},
	}
	for _, tc := range cases {
		got, err := matchGlob(tc.pattern, tc.name)
		if err != nil {
			t.Fatalf("%s: %v", tc.pattern, err)
		}
		if got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}
