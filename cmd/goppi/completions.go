package main

import (
	"fmt"
)

func cmdCompletions(args []string) error {
	shell := "zsh"
	if len(args) > 0 {
		shell = args[0]
	}
	switch shell {
	case "zsh":
		fmt.Print(zshComp)
	case "bash":
		fmt.Print(bashComp)
	default:
		return fmt.Errorf("usage: goppi completions zsh|bash")
	}
	return nil
}

const zshComp = `#compdef goppi

_goppi() {
  local -a cmds
  cmds=(
    'login:Save an Upstage API key'
    'logout:Remove the saved key'
    'models:List Solar chat models'
    'doctor:Check local setup'
    'init:Write GOPPI.md'
    'inspect:Show resolved config'
    'sessions:List and manage sessions'
    'export:Export a session as Markdown'
    'version:Print version'
    'completions:Shell completion scripts'
    'help:Show help'
  )
  if (( CURRENT == 2 )); then
    _describe 'command' cmds
    return
  fi
  case $words[2] in
    sessions) _values 'sessions' list delete ;;
    completions) _values 'shell' zsh bash ;;
  esac
}

_goppi
`

const bashComp = `_goppi() {
  local cur
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "login logout models doctor init inspect sessions export version completions help" -- "$cur") )
  fi
}
complete -F _goppi goppi
`
