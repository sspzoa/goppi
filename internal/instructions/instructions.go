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

고삐가 매 턴 읽는 프로젝트 지시입니다. 짧게 쓰세요.

## 이 레포

- 무엇인지
- 빌드·테스트 방법
- 건드리지 말 것

## 명령

` + "```" + `
make build
go test ./...
` + "```" + `
`
