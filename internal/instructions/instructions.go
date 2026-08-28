package instructions

import (
	"os"
	"path/filepath"
	"strings"
)

var Names = []string{
	"GOPPI.md",
	"AGENTS.md",
	filepath.Join(".goppi", "instructions.md"),
}

func Load(workdir string) (text string, found []string) {
	var parts []string
	for _, name := range Names {
		path := filepath.Join(workdir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		body := strings.TrimSpace(string(data))
		if body == "" {
			continue
		}
		found = append(found, name)
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n\n"), found
}

const Template = `# GOPPI.md

This file is project instructions for goppi (Upstage Solar coding agent).
Keep it short. The agent reads it on every turn.

## Project

- What this repo is
- How to build and test
- What not to touch

## Commands

` + "```" + `
make build
go test ./...
` + "```" + `
`
