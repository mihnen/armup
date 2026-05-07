#compdef armup

_armup() {
  local -a candidates

  if (( CURRENT == 2 )); then
    candidates=(${(f)"$(command armup __complete subcommands 2>/dev/null)"})
    compadd -a candidates
    return
  fi

  case $words[2] in
    use|uninstall|rm|remove)
      candidates=(${(f)"$(command armup __complete versions-installed 2>/dev/null)"})
      compadd -a candidates
      ;;
    install)
      candidates=(${(f)"$(command armup __complete versions-available 2>/dev/null)"})
      compadd -a candidates
      ;;
    completion)
      compadd bash zsh powershell
      ;;
  esac
}

compdef _armup armup
