function __armup_subcommands
    command armup __complete subcommands 2>/dev/null
end

function __armup_versions_installed
    command armup __complete versions-installed 2>/dev/null
end

function __armup_versions_available
    command armup __complete versions-available 2>/dev/null
end

# Disable file completion for armup (we always know the candidates).
complete -c armup -f

# Subcommand at position 1.
complete -c armup -n '__fish_use_subcommand' -a '(__armup_subcommands)'

# Per-subcommand argument completion.
complete -c armup -n '__fish_seen_subcommand_from use uninstall rm remove' \
    -a '(__armup_versions_installed)'
complete -c armup -n '__fish_seen_subcommand_from install' \
    -a '(__armup_versions_available)'
complete -c armup -n '__fish_seen_subcommand_from completion' \
    -a 'bash zsh fish powershell'
