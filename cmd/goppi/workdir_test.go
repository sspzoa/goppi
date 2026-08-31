package main

import "testing"

func TestPeelWorkdirArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		wd   string
		rest []string
	}{
		{
			name: "none",
			args: []string{"--fix"},
			rest: []string{"--fix"},
		},
		{
			name: "leading cwd",
			args: []string{"-C", "/tmp/wd", "--fix"},
			wd:   "/tmp/wd",
			rest: []string{"--fix"},
		},
		{
			name: "leading double cwd",
			args: []string{"--cwd", "/a", "-C", "/b", "init"},
			wd:   "/b",
			rest: []string{"init"},
		},
		{
			name: "doctor flags only",
			args: []string{"--online", "--fix"},
			rest: []string{"--online", "--fix"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd, rest := peelWorkdirArgs(tc.args)
			if wd != tc.wd {
				t.Fatalf("wd=%q want %q", wd, tc.wd)
			}
			if len(rest) != len(tc.rest) {
				t.Fatalf("rest=%v want %v", rest, tc.rest)
			}
			for i := range tc.rest {
				if rest[i] != tc.rest[i] {
					t.Fatalf("rest[%d]=%q want %q", i, rest[i], tc.rest[i])
				}
			}
		})
	}
}
