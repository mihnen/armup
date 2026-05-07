Register-ArgumentCompleter -Native -CommandName armup -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $words = @($commandAst.CommandElements | ForEach-Object { $_.ToString() })

    # Figure out which positional argument the cursor is on (0 = the command).
    # If $wordToComplete is non-empty, the last element of $words IS that
    # partial word, so the "real" position is one less. If it's empty, the
    # cursor is at a fresh space past the last element.
    $position = if ($wordToComplete -ne '') { $words.Count - 1 } else { $words.Count }

    if ($position -le 1) {
        $candidates = & armup __complete subcommands 2>$null
    } else {
        switch ($words[1]) {
            { $_ -in 'use', 'uninstall', 'rm', 'remove' } {
                $candidates = & armup __complete versions-installed 2>$null
            }
            'install' {
                $candidates = & armup __complete versions-available 2>$null
            }
            'completion' {
                $candidates = @('bash', 'zsh', 'powershell')
            }
            default {
                return
            }
        }
    }

    $candidates |
        Where-Object { $_ -like "$wordToComplete*" } |
        ForEach-Object {
            [System.Management.Automation.CompletionResult]::new(
                $_, $_, 'ParameterValue', $_)
        }
}
