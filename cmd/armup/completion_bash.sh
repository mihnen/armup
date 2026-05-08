_armup() {
  local cur sub
  cur="${COMP_WORDS[COMP_CWORD]}"
  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "$(command armup __complete subcommands 2>/dev/null)" -- "$cur") )
    return
  fi
  sub="${COMP_WORDS[1]}"
  case "$sub" in
    use)
      COMPREPLY=( $(compgen -W "$(command armup __complete versions-installed 2>/dev/null)" -- "$cur") )
      ;;
    uninstall|rm|remove)
      COMPREPLY=( $(compgen -W "$(command armup __complete versions-installed 2>/dev/null)" -- "$cur") )
      ;;
    install)
      COMPREPLY=( $(compgen -W "$(command armup __complete versions-available 2>/dev/null)" -- "$cur") )
      ;;
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish powershell" -- "$cur") )
      ;;
  esac
}

complete -F _armup armup
