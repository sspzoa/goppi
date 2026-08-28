package complete

import (
	"fmt"
	"strings"
)

func Script(shell string) (string, error) {
	switch shell {
	case "zsh":
		return zshScript(), nil
	case "bash":
		return bashScript(), nil
	case "fish":
		return fishScript(), nil
	default:
		return "", fmt.Errorf("usage: goppi completions zsh|bash|fish")
	}
}

func zshScript() string {
	var cmds []string
	for _, c := range CLICommands() {
		cmds = append(cmds, fmt.Sprintf("    '%s:%s'", c.Name, escapeZsh(c.Summary)))
	}
	return fmt.Sprintf(`#compdef goppi

_goppi() {
  local -a cmds models efforts formats
  cmds=(
%s
  )
  models=(%s)
  efforts=(%s)
  formats=(%s)

  local cur="${words[CURRENT]}"
  if (( CURRENT == 2 )); then
    if [[ $cur == -* ]]; then
      _values 'flag' %s
      return
    fi
    _describe 'command' cmds
    return
  fi

  case ${words[CURRENT-1]} in
    -m|--model) _values 'model' $models; return ;;
    --effort) _values 'effort' $efforts; return ;;
    --output-format) _values 'format' $formats; return ;;
    -C|--cwd) _files -/ ; return ;;
    -r|--resume) _goppi_sessions; return ;;
  esac

  case $words[2] in
    sessions)
      if (( CURRENT == 3 )); then
        _values 'sessions' list delete
      elif [[ $words[3] == delete || $words[3] == rm ]]; then
        _goppi_sessions
      fi
      ;;
    export) _goppi_sessions ;;
    completions) _values 'shell' %s ;;
    inspect) _values 'flag' --json ;;
    login) _values 'flag' --stdin ;;
    complete) _values 'kind' commands models efforts sessions formats shells flags slash ;;
  esac
}

_goppi_sessions() {
  local -a ids
  ids=(${(f)"$(goppi complete sessions 2>/dev/null)"})
  (( $#ids )) && _describe 'session' ids
}

_goppi
`, strings.Join(cmds, "\n"), joinNames(Models()), joinNames(Efforts()), joinNames(Formats()), flagWords(), joinNames(Shells()))
}

func bashScript() string {
	return fmt.Sprintf(`_goppi() {
  local cur prev
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  case "$prev" in
    -m|--model) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    --effort) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    --output-format) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    -C|--cwd) COMPREPLY=( $(compgen -d -- "$cur") ); return ;;
    -r|--resume|export) COMPREPLY=( $(compgen -W "$(goppi complete sessions 2>/dev/null)" -- "$cur") ); return ;;
    completions) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    complete) COMPREPLY=( $(compgen -W "commands models efforts sessions formats shells flags slash" -- "$cur") ); return ;;
  esac
  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "%s %s" -- "$cur") )
    return
  fi
  case "${COMP_WORDS[1]}" in
    sessions)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "list delete" -- "$cur") )
      elif [[ ${COMP_WORDS[2]} == delete || ${COMP_WORDS[2]} == rm ]]; then
        COMPREPLY=( $(compgen -W "$(goppi complete sessions 2>/dev/null)" -- "$cur") )
      fi
      ;;
    inspect) COMPREPLY=( $(compgen -W "--json" -- "$cur") ) ;;
    login) COMPREPLY=( $(compgen -W "--stdin" -- "$cur") ) ;;
  esac
}
complete -F _goppi goppi
`, joinNames(Models()), joinNames(Efforts()), joinNames(Formats()), joinNames(Shells()), joinNames(CLICommands()), joinNames(Flags()))
}

func fishScript() string {
	var b strings.Builder
	b.WriteString("complete -c goppi -f\n")
	for _, c := range CLICommands() {
		fmt.Fprintf(&b, "complete -c goppi -n '__fish_use_subcommand' -a %s -d '%s'\n", c.Name, escapeFish(c.Summary))
	}
	fmt.Fprintf(&b, "complete -c goppi -s p -l prompt -d 'Headless prompt'\n")
	fmt.Fprintf(&b, "complete -c goppi -s m -l model -xa '%s' -d 'Model'\n", joinNames(Models()))
	fmt.Fprintf(&b, "complete -c goppi -l effort -xa '%s' -d 'Reasoning effort'\n", joinNames(Efforts()))
	fmt.Fprintf(&b, "complete -c goppi -s C -l cwd -r -d 'Working directory'\n")
	fmt.Fprintf(&b, "complete -c goppi -s c -l continue -d 'Resume last session'\n")
	fmt.Fprintf(&b, "complete -c goppi -s r -l resume -xa '(goppi complete sessions 2>/dev/null)' -d 'Resume session'\n")
	fmt.Fprintf(&b, "complete -c goppi -l max-turns -d 'Max agent turns'\n")
	fmt.Fprintf(&b, "complete -c goppi -l output-format -xa '%s' -d 'Output format'\n", joinNames(Formats()))
	fmt.Fprintf(&b, "complete -c goppi -l always-approve -l yolo -d 'Skip permission prompts'\n")
	fmt.Fprintf(&b, "complete -c goppi -n '__fish_seen_subcommand_from sessions' -a 'list delete'\n")
	fmt.Fprintf(&b, "complete -c goppi -n '__fish_seen_subcommand_from export' -xa '(goppi complete sessions 2>/dev/null)'\n")
	fmt.Fprintf(&b, "complete -c goppi -n '__fish_seen_subcommand_from completions' -a '%s'\n", joinNames(Shells()))
	fmt.Fprintf(&b, "complete -c goppi -n '__fish_seen_subcommand_from inspect' -l json\n")
	fmt.Fprintf(&b, "complete -c goppi -n '__fish_seen_subcommand_from login' -l stdin\n")
	return b.String()
}

func joinNames(items []Item) string {
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	return strings.Join(names, " ")
}

func flagWords() string {
	var out []string
	for _, f := range Flags() {
		out = append(out, f.Name)
	}
	return strings.Join(out, " ")
}

func escapeZsh(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

func escapeFish(s string) string {
	return strings.ReplaceAll(s, "'", `\'`)
}
