# armup

A small CLI for installing and switching between versions of ARM's
`arm-none-eabi` GCC toolchain (the one used for STM32 and other Cortex-M /
Cortex-R / Cortex-A bare-metal work).

Replaces a hand-edited install script with rustup-/nvm-style subcommands. No
sudo, no `/usr/bin` symlink pollution. Switching versions is one command.

## Why

Ubuntu's packaged `gcc-arm-none-eabi` ships with bugs. ARM's official binary
release is solid but has to be downloaded by hand and has no version
management story. This tool:

- Downloads ARM's official binary directly (with sha256 verification).
- Keeps multiple versions side-by-side under `~/.local/share/arm-toolchains/`.
- Exposes the active one through a single `current` symlink that's on
  `PATH`. Switch versions = retarget the symlink, takes effect immediately.
- Lists installed versions and ARM's catalog of available versions.
- Cleans up after itself (extraction is atomic; partial downloads don't
  leave junk).

## Install

### Linux / macOS

```sh
go build -o armup ./cmd/armup
mv armup ~/.local/bin/        # or anywhere on $PATH
armup init                    # creates data dir, appends PATH line to .zshrc/.bashrc
```

Open a fresh shell after `init` so the new `PATH` is loaded.

### Windows

Grab `armup-vX.Y.Z-windows-amd64.zip` from the
[Releases page](../../releases/latest), extract `armup.exe` to a folder on
your PATH (e.g. `%USERPROFILE%\bin`), then:

```cmd
armup init
```

That writes the toolchain `bin` directory into `HKCU\Environment\Path`
(no admin required). **Open a new terminal** so the new PATH is loaded —
already-running shells won't see it. After that, every other command works
the same as on Linux/macOS.

A few Windows-specific things to know:

- `armup use` swaps the active toolchain via an NTFS junction at
  `%LOCALAPPDATA%\armup\current`. Junctions don't need admin or Developer
  Mode.
- The `versions/` directory and `current` junction must live on the same
  volume — they do by default, both under `%LOCALAPPDATA%`.
- ARM ships the toolchain as a `.zip` for Windows (vs `.tar.xz` on
  Linux/macOS). Extraction is single-threaded and antivirus scanners can
  slow the first install considerably; allowlisting `%LOCALAPPDATA%\armup`
  helps.
- Some toolchain include paths can exceed the legacy 260-character limit.
  If you hit `path too long` errors during a build, enable
  [LongPathsEnabled](https://learn.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation)
  in the Group Policy Editor or the registry.

### Cross-compiling from source

```sh
GOOS=linux   GOARCH=amd64 go build -o armup       ./cmd/armup
GOOS=darwin  GOARCH=arm64 go build -o armup       ./cmd/armup
GOOS=windows GOARCH=amd64 go build -o armup.exe   ./cmd/armup
```

## Usage

```
armup init                       # one-time setup
armup available [--refresh]      # list versions you can install
armup install 13.3.rel1          # download, verify, extract
armup install 14.2.rel1          # add another
armup list                       # show what's installed; * marks active
armup use 12.3.rel1              # switch active version
armup current                    # print active version
armup which                      # print active toolchain bin dir
armup uninstall 12.3.rel1        # remove a version (refuses if active without -f)
armup completion zsh             # print shell-completion script (also: bash)
```

`use` updates a single symlink, so the switch is visible immediately to any
shell that has `PATH` set up correctly.

## Shell completion

`armup` ships dynamic completion for bash, zsh, and PowerShell — `armup use
<TAB>` lists installed versions, `armup install <TAB>` lists available
versions, etc. The candidate lists are queried from the binary at completion
time, so they always reflect your current state.

### zsh

Either drop the script into a directory on your `fpath`:

```sh
mkdir -p ~/.zsh/completions
armup completion zsh > ~/.zsh/completions/_armup
```

and add this **before** `compinit` runs (in oh-my-zsh setups, before
`source $ZSH/oh-my-zsh.sh`):

```sh
fpath=(~/.zsh/completions $fpath)
```

Or source it on every shell start (slightly slower, always fresh):

```sh
echo 'source <(armup completion zsh)' >> ~/.zshrc
```

### bash

```sh
echo 'source <(armup completion bash)' >> ~/.bashrc
```

### PowerShell

Append to your profile (`$PROFILE`) so it loads in every new session:

```powershell
if (-not (Test-Path $PROFILE)) { New-Item -Type File -Force -Path $PROFILE }
Add-Content -Path $PROFILE -Value "`narmup completion powershell | Out-String | Invoke-Expression"
```

Then open a new PowerShell session (or run the same line interactively to
load it into the current one). `armup use <TAB>` should now list installed
versions.

## Extraction performance

`install` extracts the toolchain using the fastest path available on the
host:

1. **`xz` + `tar` on PATH** — pipes `xz -T 0 -dc` into `tar -x`, so
   decompression is multi-threaded across all cores. Typical install of a
   ~150 MiB archive: a few seconds.
2. **`tar` only** — `tar -xJf -`, single-threaded native code.
3. **Neither** — pure-Go fallback via `github.com/ulikunitz/xz`.
   Single-threaded; about 3× slower than the multi-threaded path.

`ARMTOOLCHAIN_PURE_GO=1` forces the fallback path (useful for testing).

## Differences from the old install script

- **Location**: installs go under `~/.local/share/arm-toolchains/` instead of
  `/usr/share/`, and binaries are reached through one `PATH` entry rather
  than ~35 symlinks in `/usr/bin/`.
- **Privileges**: no sudo anywhere.

If you're migrating from a script that extracted into `/usr/share/` and
linked into `/usr/bin/`, you can clean those up after switching:

```sh
sudo rm -rf /usr/share/arm-gnu-toolchain-*-x86_64-arm-none-eabi
sudo find /usr/bin -lname '/usr/share/arm-gnu-toolchain-*' -delete
```

## Notes

- ARM's downloads page (`developer.arm.com/downloads/...`) sits behind a
  CDN with bot protection; non-browser scraping returns 403. When `available
  --refresh` hits this, it falls back to HEAD-probing each curated version's
  archive URL (which is on a different host and isn't blocked) to confirm
  availability.
- The list of curated versions lives in `internal/arm/versions.go`. Add new
  releases there as ARM ships them.
- ARM's `<archive>.sha256` file confusingly contains an MD5 hash. The real
  SHA-256 is in `<archive>.sha256asc`. We use the latter.
