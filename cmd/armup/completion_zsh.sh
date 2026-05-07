#compdef armup

_armup() {
  local context state state_descr line
  typeset -A opt_args

  _arguments -C \
    '1: :->subcommand' \
    '*:: :->args'

  case $state in
    subcommand)
      local -a subs
      subs=(${(f)"$(command armup __complete subcommands 2>/dev/null)"})
      _describe 'subcommand' subs
      ;;
    args)
      case $words[1] in
        use|uninstall|rm|remove)
          local -a versions
          versions=(${(f)"$(command armup __complete versions-installed 2>/dev/null)"})
          _describe 'installed version' versions
          ;;
        install)
          local -a versions
          versions=(${(f)"$(command armup __complete versions-available 2>/dev/null)"})
          _describe 'available version' versions
          ;;
        completion)
          _values 'shell' bash zsh
          ;;
      esac
      ;;
  esac
}

compdef _armup armup
